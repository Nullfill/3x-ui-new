package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type trafficMultiplierConfig struct {
	Enabled bool
	Factor  float64
}

// effectiveTrafficMultiplier resolves the three-level policy. Explicit client
// and inbound disables are important: they must override an enabled global
// setting rather than merely inherit it.
func effectiveTrafficMultiplier(tx *gorm.DB, inboundID int, email string) (trafficMultiplierConfig, error) {
	global, err := loadTrafficMultiplierConfig(tx)
	if err != nil {
		return global, err
	}
	var client model.ClientRecord
	if email != "" && tx.Select("traffic_multiplier_mode, traffic_multiplier_factor").Where("email = ?", email).First(&client).Error == nil {
		if mode := client.TrafficMultiplierMode; mode == "disabled" {
			return trafficMultiplierConfig{Factor: 1}, nil
		} else if mode == "enabled" {
			return trafficMultiplierConfig{Enabled: true, Factor: validMultiplierFactor(client.TrafficMultiplierFactor)}, nil
		}
	}
	var ib model.Inbound
	if inboundID > 0 && tx.Select("traffic_multiplier_mode, traffic_multiplier_factor").First(&ib, inboundID).Error == nil {
		if mode := ib.TrafficMultiplierMode; mode == "disabled" {
			return trafficMultiplierConfig{Factor: 1}, nil
		} else if mode == "enabled" {
			return trafficMultiplierConfig{Enabled: true, Factor: validMultiplierFactor(ib.TrafficMultiplierFactor)}, nil
		}
	}
	return global, nil
}

func validMultiplierFactor(f float64) float64 {
	if f < 1 || f > 10 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 1
	}
	return f
}

func multiplierDisplayPolicy(tx *gorm.DB, email string, global trafficMultiplierConfig) (trafficMultiplierConfig, string) {
	var client model.ClientRecord
	if tx.Select("traffic_multiplier_mode, traffic_multiplier_factor").Where("email = ?", email).First(&client).Error == nil {
		if client.TrafficMultiplierMode == "disabled" {
			return trafficMultiplierConfig{Factor: 1}, "client"
		}
		if client.TrafficMultiplierMode == "enabled" {
			return trafficMultiplierConfig{Enabled: true, Factor: validMultiplierFactor(client.TrafficMultiplierFactor)}, "client"
		}
	}
	var policies []struct {
		Mode   string  `gorm:"column:mode"`
		Factor float64 `gorm:"column:factor"`
	}
	err := tx.Table("client_inbounds ci").Select("i.traffic_multiplier_mode AS mode, i.traffic_multiplier_factor AS factor").
		Joins("JOIN clients c ON c.id = ci.client_id JOIN inbounds i ON i.id = ci.inbound_id").
		Where("c.email = ? AND i.traffic_multiplier_mode <> '' AND i.traffic_multiplier_mode <> 'inherit'", email).Scan(&policies).Error
	if err == nil && len(policies) > 0 {
		first := policies[0]
		allSame := true
		for _, p := range policies[1:] {
			if p.Mode != first.Mode || validMultiplierFactor(p.Factor) != validMultiplierFactor(first.Factor) {
				allSame = false
				break
			}
		}
		if allSame {
			if first.Mode == "enabled" {
				return trafficMultiplierConfig{Enabled: true, Factor: validMultiplierFactor(first.Factor)}, "inbound"
			}
			return trafficMultiplierConfig{Factor: 1}, "inbound"
		}
		return trafficMultiplierConfig{Enabled: true, Factor: global.Factor}, "mixed"
	}
	return global, "global"
}

func loadTrafficMultiplierConfig(tx *gorm.DB) (trafficMultiplierConfig, error) {
	config := trafficMultiplierConfig{Factor: 1}
	var rows []model.Setting
	if err := tx.Where("key IN ?", []string{"trafficMultiplierEnabled", "trafficMultiplierFactor"}).Find(&rows).Error; err != nil {
		return config, err
	}
	for _, row := range rows {
		switch row.Key {
		case "trafficMultiplierEnabled":
			config.Enabled = row.Value == "true"
		case "trafficMultiplierFactor":
			factor, err := strconv.ParseFloat(row.Value, 64)
			if err != nil {
				return config, fmt.Errorf("invalid traffic multiplier factor: %w", err)
			}
			config.Factor = factor
		}
	}
	if config.Factor < 1 || config.Factor > 10 || math.IsNaN(config.Factor) || math.IsInf(config.Factor, 0) {
		return config, fmt.Errorf("traffic multiplier factor must be between 1 and 10")
	}
	if !config.Enabled {
		config.Factor = 1
	}
	return config, nil
}

func multipliedTrafficDelta(up, down int64, config trafficMultiplierConfig) (int64, int64) {
	if up < 0 || down < 0 {
		return 0, 0
	}
	factor := config.Factor
	if !config.Enabled || factor <= 1 {
		return up, down
	}
	return int64(math.Round(float64(up) * factor)), int64(math.Round(float64(down) * factor))
}

// applyTrafficMultiplier converts a source's already-derived raw delta into a
// billed delta and advances its independent persistent state. currentRaw is nil
// for the local Xray delta stream and the cumulative node counter for a remote
// source.
func applyTrafficMultiplier(tx *gorm.DB, config trafficMultiplierConfig, sourceNodeID, inboundID int, email string, rawUp, rawDown int64, currentRaw *nodeTrafficCounter) (int64, int64, error) {
	if rawUp < 0 || rawDown < 0 {
		logger.Warningf("Traffic multiplier skipped suspicious negative delta for %q source %d", email, sourceNodeID)
		return 0, 0, nil
	}

	var state model.TrafficMultiplierState
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_node_id = ? AND inbound_id = ? AND client_email = ?", sourceNodeID, inboundID, email).First(&state).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}
	newState := errors.Is(err, gorm.ErrRecordNotFound)
	if newState {
		state.SourceNodeId = sourceNodeID
		state.InboundId = inboundID
		state.ClientEmail = email
		state.Factor = config.Factor
		state.Enabled = config.Enabled
		logger.Infof("Traffic multiplier baseline initialized for %q source %d", email, sourceNodeID)
	}

	if currentRaw != nil {
		if !newState && (currentRaw.Up < state.LastRawUp || currentRaw.Down < state.LastRawDown) {
			logger.Warningf("Traffic multiplier baseline refreshed after counter decrease for %q source %d", email, sourceNodeID)
			rawUp, rawDown = 0, 0
		}
		state.LastRawUp, state.LastRawDown = currentRaw.Up, currentRaw.Down
	} else {
		state.LastRawUp += rawUp
		state.LastRawDown += rawDown
	}

	billedUp, billedDown := multipliedTrafficDelta(rawUp, rawDown, config)
	state.LastBilledUp += billedUp
	state.LastBilledDown += billedDown
	extraUp, extraDown := multiplierExtraDelta(rawUp, rawDown, billedUp, billedDown)
	state.ExtraUp += extraUp
	state.ExtraDown += extraDown
	state.Factor = config.Factor
	state.Enabled = config.Enabled
	if err := tx.Save(&state).Error; err != nil {
		return 0, 0, err
	}
	return billedUp, billedDown, nil
}

func multiplierExtraDelta(rawUp, rawDown, billedUp, billedDown int64) (int64, int64) {
	return max(billedUp-rawUp, 0), max(billedDown-rawDown, 0)
}

func attachTrafficMultiplierUsage(tx *gorm.DB, traffics []xray.ClientTraffic) error {
	if len(traffics) == 0 {
		return nil
	}
	emails := make([]string, 0, len(traffics))
	for i := range traffics {
		if traffics[i].Email != "" {
			emails = append(emails, traffics[i].Email)
		}
	}
	config, err := loadTrafficMultiplierConfig(tx)
	if err != nil {
		return err
	}
	var states []model.TrafficMultiplierState
	for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqlInChunk) {
		var page []model.TrafficMultiplierState
		if err := tx.Where("client_email IN ?", batch).Find(&page).Error; err != nil {
			return err
		}
		states = append(states, page...)
	}
	type extraTraffic struct{ up, down int64 }
	extraByEmail := make(map[string]extraTraffic, len(states))
	for i := range states {
		current := extraByEmail[states[i].ClientEmail]
		current.up += states[i].ExtraUp
		current.down += states[i].ExtraDown
		extraByEmail[states[i].ClientEmail] = current
	}
	for i := range traffics {
		extra := extraByEmail[traffics[i].Email]
		traffics[i].MultiplierExtraUp = extra.up
		traffics[i].MultiplierExtraDown = extra.down
		traffics[i].MultiplierFactor = config.Factor
		traffics[i].MultiplierEnabled = config.Enabled
		traffics[i].RawUp = max(traffics[i].Up-extra.up, 0)
		traffics[i].RawDown = max(traffics[i].Down-extra.down, 0)
		displayConfig, source := multiplierDisplayPolicy(tx, traffics[i].Email, config)
		traffics[i].MultiplierMode = "inherit"
		traffics[i].MultiplierSource = source
		traffics[i].MultiplierEnabled = displayConfig.Enabled
		traffics[i].MultiplierFactor = displayConfig.Factor
		if source == "client" || source == "inbound" {
			traffics[i].MultiplierMode = map[bool]string{true: "enabled", false: "disabled"}[displayConfig.Enabled]
		}
	}
	return nil
}

func refreshTrafficMultiplierConfiguration(db *gorm.DB, enabled bool, factor float64) error {
	if !enabled {
		factor = 1
	}
	return db.Model(&model.TrafficMultiplierState{}).Where("1 = 1").Updates(map[string]any{
		"enabled": enabled,
		"factor":  factor,
	}).Error
}

func deleteTrafficMultiplierStates(tx *gorm.DB, emails ...string) error {
	emails = uniqueNonEmptyStrings(emails)
	if len(emails) == 0 {
		return nil
	}
	return tx.Where("client_email IN ?", emails).Delete(&model.TrafficMultiplierState{}).Error
}

func renameTrafficMultiplierStates(tx *gorm.DB, oldEmail, newEmail string) error {
	if oldEmail == newEmail {
		return nil
	}
	return tx.Model(&model.TrafficMultiplierState{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}
