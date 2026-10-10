package collector

import "testing"

func TestCreateCollectorFromRegistry(t *testing.T) {
	RegisterCollector("unit-test", func(params map[string]string) Collector {
		return &SimulatorCollector{}
	})

	c := GetCollectorFactory().CreateCollector("unit-test", map[string]string{})
	if c == nil {
		t.Fatal("expected collector instance")
	}
}

func TestCreateCollectorFallbackSimulator(t *testing.T) {
	c := GetCollectorFactory().CreateCollector("unknown-proto", map[string]string{})
	if _, ok := c.(*SimulatorCollector); !ok {
		t.Fatalf("expected fallback simulator collector, got %T", c)
	}
}
