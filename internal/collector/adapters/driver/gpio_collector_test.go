package collector

import (
	"context"
	"testing"

	"data-collection-system/internal/bootstrap"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

type fakeGPIOBackend struct {
	values map[string]bool
}

func (f *fakeGPIOBackend) Read(path string) (bool, error) {
	return f.values[path], nil
}

func (f *fakeGPIOBackend) Write(path string, value bool) error {
	f.values[path] = value
	return nil
}

func TestGPIOCollectorActiveLow(t *testing.T) {
	backend := &fakeGPIOBackend{values: map[string]bool{"/gpio/di1/value": true}}
	lookup := func(label string) (bootstrap.LocalIOChannelSettings, bool) {
		switch label {
		case "DI1":
			return bootstrap.LocalIOChannelSettings{Label: "DI1", Value: "/gpio/di1/value", ActiveLow: true}, true
		case "DO1":
			return bootstrap.LocalIOChannelSettings{Label: "DO1", Value: "/gpio/do1/value"}, true
		default:
			return bootstrap.LocalIOChannelSettings{}, false
		}
	}
	collector := newGPIOCollector(backend, lookup)
	points := []templatebiz.DevicePoint{
		{PointID: 1, LocalIO: &templatebiz.LocalIOPointSpec{Channel: "DI1"}},
	}

	values, err := collector.Collect(context.Background(), 1, points, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := values[1]; got.Kind != valueobj.KindBool || got.Bool {
		t.Fatalf("active_low DI value = %#v, want false", got)
	}

	if err := collector.Write(context.Background(), 1, templatebiz.DevicePoint{
		PointID: 7,
		LocalIO: &templatebiz.LocalIOPointSpec{Channel: "DO1"},
	}, true); err != nil {
		t.Fatal(err)
	}
	if !backend.values["/gpio/do1/value"] {
		t.Fatal("DO write did not persist true")
	}
}
