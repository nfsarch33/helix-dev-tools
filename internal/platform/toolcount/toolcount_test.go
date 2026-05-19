package toolcount

import (
	"sync"
	"testing"
)

func TestCounter_Increment(t *testing.T) {
	c := New()
	c.Increment("shell")
	c.Increment("shell")
	c.Increment("read")

	if c.Total() != 3 {
		t.Fatalf("expected total 3, got %d", c.Total())
	}
}

func TestCounter_Report(t *testing.T) {
	c := New()
	c.Increment("shell")
	c.Increment("shell")
	c.Increment("read")

	r := c.Report()
	if r["shell"] != 2 {
		t.Fatalf("expected shell=2, got %d", r["shell"])
	}
	if r["read"] != 1 {
		t.Fatalf("expected read=1, got %d", r["read"])
	}
}

func TestCounter_ConcurrentAccess(t *testing.T) {
	c := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Increment("concurrent")
		}()
	}
	wg.Wait()

	if c.Total() != 100 {
		t.Fatalf("expected 100 after concurrent increments, got %d", c.Total())
	}
}
