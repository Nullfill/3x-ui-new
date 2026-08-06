package service

import "testing"

func TestTrafficMultiplierFutureOnlyAccounting(t *testing.T) {
	historicalUp, historicalDown := int64(50_000), int64(75_000)
	config := trafficMultiplierConfig{Enabled: true, Factor: 2}
	up, down := multipliedTrafficDelta(100, 500, config)
	if historicalUp+up != 50_200 || historicalDown+down != 76_000 {
		t.Fatalf("historical traffic changed or future delta incorrect: up=%d down=%d", historicalUp+up, historicalDown+down)
	}
	up, down = multipliedTrafficDelta(0, 0, config)
	if up != 0 || down != 0 {
		t.Fatalf("idle cycle added traffic: up=%d down=%d", up, down)
	}
}

func TestTrafficMultiplierFactorsAndDisabledMode(t *testing.T) {
	tests := []struct {
		name     string
		config   trafficMultiplierConfig
		wantUp   int64
		wantDown int64
	}{
		{"factor one", trafficMultiplierConfig{Enabled: true, Factor: 1}, 100, 200},
		{"decimal", trafficMultiplierConfig{Enabled: true, Factor: 1.5}, 150, 300},
		{"disabled", trafficMultiplierConfig{Factor: 3}, 100, 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			up, down := multipliedTrafficDelta(100, 200, tc.config)
			if up != tc.wantUp || down != tc.wantDown {
				t.Fatalf("up=%d down=%d, want up=%d down=%d", up, down, tc.wantUp, tc.wantDown)
			}
		})
	}
}

func TestTrafficMultiplierFactorChangeAndMultiCycleDoNotCompound(t *testing.T) {
	total := int64(0)
	for range 2 {
		_, down := multipliedTrafficDelta(0, 100, trafficMultiplierConfig{Enabled: true, Factor: 2})
		total += down
	}
	_, down := multipliedTrafficDelta(0, 100, trafficMultiplierConfig{Enabled: true, Factor: 3})
	total += down
	if total != 700 {
		t.Fatalf("multi-cycle traffic compounded: got %d, want 700", total)
	}
}

func TestTrafficMultiplierRejectsNegativeDelta(t *testing.T) {
	up, down := multipliedTrafficDelta(-100, 100, trafficMultiplierConfig{Enabled: true, Factor: 2})
	if up != 0 || down != 0 {
		t.Fatalf("negative/reset delta was billed: up=%d down=%d", up, down)
	}
}
