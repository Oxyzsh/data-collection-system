package collector

import (
	"context"
	"math/rand"
	"testing"

	templatebiz "data-collection-system/internal/template/domain"
)

func TestSimulatorCollectorProducesAlarmAndRecoveryWindows(t *testing.T) {
	collector := &SimulatorCollector{
		rng: rand.New(rand.NewSource(1)),
	}

	points := []templatebiz.DevicePoint{
		{PointID: 1, Name: "温度"},
		{PointID: 2, Name: "湿度"},
	}

	var (
		tempAboveStart bool
		tempBelowEnd   bool
		humAboveStart  bool
		humBelowEnd    bool
	)

	temps := make([]float64, 0, simulatorFullCycleSamples)
	hums := make([]float64, 0, simulatorFullCycleSamples)

	for i := 0; i < simulatorFullCycleSamples; i++ {
		values, err := collector.Collect(context.Background(), 1, points, nil)
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}

		temp := values[1].Number
		hum := values[2].Number
		temps = append(temps, temp)
		hums = append(hums, hum)

		if temp > 60 {
			tempAboveStart = true
		}
		if temp <= 35 {
			tempBelowEnd = true
		}
		if hum > 80 {
			humAboveStart = true
		}
		if hum <= 60 {
			humBelowEnd = true
		}
	}

	if !tempAboveStart || !tempBelowEnd {
		t.Fatalf("temperature series does not cross both alarm thresholds: %v", temps)
	}
	if !humAboveStart || !humBelowEnd {
		t.Fatalf("humidity series does not cross both alarm thresholds: %v", hums)
	}
}

func TestSimulatorCollectorChangesDirectionEveryTenSamples(t *testing.T) {
	collector := &SimulatorCollector{
		rng: rand.New(rand.NewSource(2)),
	}

	points := []templatebiz.DevicePoint{{PointID: 1, Name: "温度"}}
	temps := make([]float64, 0, simulatorFullCycleSamples)
	for i := 0; i < simulatorFullCycleSamples; i++ {
		values, err := collector.Collect(context.Background(), 1, points, nil)
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}
		temps = append(temps, values[1].Number)
	}

	if !(temps[0] < temps[5] && temps[5] < temps[9]) {
		t.Fatalf("temperature should rise in first half-cycle, got %v", temps[:10])
	}
	if !(temps[10] > temps[14] && temps[14] > temps[19]) {
		t.Fatalf("temperature should fall in second half-cycle, got %v", temps[10:])
	}
}
