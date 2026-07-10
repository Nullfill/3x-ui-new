package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type trafficMultiplierConfig struct {
	Enabled bool
	Factor  float64
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
func applyTrafficMultiplier(tx *gorm.DB, config trafficMultiplierConfig, sourceNodeID int, email string, rawUp, rawDown int64, currentRaw *nodeTrafficCounter) (int64, int64, error) {
	if rawUp < 0 || rawDown < 0 {
		logger.Warningf("Traffic multiplier skipped suspicious negative delta for %q source %d", email, sourceNodeID)
		return 0, 0, nil
	}

	var state model.TrafficMultiplierState
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_node_id = ? AND client_email = ?", sourceNodeID, email).First(&state).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}
	newState := errors.Is(err, gorm.ErrRecordNotFound)
	if newState {
		state.SourceNodeId = sourceNodeID
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
	state.Factor = config.Factor
	state.Enabled = config.Enabled
	if err := tx.Save(&state).Error; err != nil {
		return 0, 0, err
	}
	return billedUp, billedDown, nil
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
