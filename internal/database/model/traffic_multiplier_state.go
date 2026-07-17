package model

// TrafficMultiplierState records the raw and billed counters for one traffic
// source. SourceNodeId is zero for the local Xray process and the node ID for a
// remote snapshot. InboundId keeps clients attached to multiple inbounds
// independent when those inbounds have different multiplier policies.
type TrafficMultiplierState struct {
	Id             int     `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceNodeId   int     `json:"sourceNodeId" gorm:"uniqueIndex:idx_multiplier_source_inbound_email,priority:1;not null;default:0"`
	InboundId      int     `json:"inboundId" gorm:"uniqueIndex:idx_multiplier_source_inbound_email,priority:2;index:idx_multiplier_inbound;not null;default:0"`
	ClientEmail    string  `json:"clientEmail" gorm:"uniqueIndex:idx_multiplier_source_inbound_email,priority:3;index:idx_multiplier_email;not null"`
	LastRawUp      int64   `json:"lastRawUp" gorm:"not null;default:0"`
	LastRawDown    int64   `json:"lastRawDown" gorm:"not null;default:0"`
	LastBilledUp   int64   `json:"lastBilledUp" gorm:"not null;default:0"`
	LastBilledDown int64   `json:"lastBilledDown" gorm:"not null;default:0"`
	ExtraUp        int64   `json:"extraUp" gorm:"not null;default:0"`
	ExtraDown      int64   `json:"extraDown" gorm:"not null;default:0"`
	Factor         float64 `json:"factor" gorm:"not null;default:1"`
	Enabled        bool    `json:"enabled" gorm:"not null;default:false"`
	CreatedAt      int64   `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt      int64   `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}
