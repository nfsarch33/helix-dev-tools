package observe

import "testing"

func TestMetricCollector_Counter(t *testing.T) {
	c := NewMetricCollector()
	c.Counter("requests", nil)
	c.Counter("requests", nil)

	v, ok := c.Get("requests")
	if !ok || v != 2 {
		t.Errorf("expected 2, got %f", v)
	}
}

func TestMetricCollector_Gauge(t *testing.T) {
	c := NewMetricCollector()
	c.Gauge("memory_mb", 512, nil)

	v, ok := c.Get("memory_mb")
	if !ok || v != 512 {
		t.Errorf("expected 512, got %f", v)
	}
}

func TestMetricCollector_Labels(t *testing.T) {
	c := NewMetricCollector()
	c.Counter("http_requests", map[string]string{"method": "GET"})
	c.Counter("http_requests", map[string]string{"method": "POST"})

	if c.Count() != 2 {
		t.Errorf("expected 2 distinct metrics, got %d", c.Count())
	}
}

func TestMetricCollector_Reset(t *testing.T) {
	c := NewMetricCollector()
	c.Counter("x", nil)
	c.Reset()
	if c.Count() != 0 {
		t.Error("expected empty after reset")
	}
}

func TestMetricCollector_All(t *testing.T) {
	c := NewMetricCollector()
	c.Counter("a", nil)
	c.Gauge("b", 1, nil)

	all := c.All()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}
