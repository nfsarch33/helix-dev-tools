package daemonctl

import "sync"

type DaemonStatus int

const (
	StatusUnknown DaemonStatus = iota
	StatusRunning
	StatusStopped
	StatusFailed
)

type Config struct {
	PlistDir string
}

type Daemon struct {
	Name           string
	BinaryPath     string
	AutoStart      bool
	HealthEndpoint string
}

type HealthSummary struct {
	Running int
	Stopped int
	Failed  int
	Total   int
}

type Controller struct {
	config   Config
	mu       sync.Mutex
	daemons  []Daemon
	statuses map[string]DaemonStatus
}

func New(cfg Config) *Controller {
	return &Controller{
		config:   cfg,
		statuses: make(map[string]DaemonStatus),
	}
}

func (c *Controller) Register(d Daemon) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.daemons = append(c.daemons, d)
}

func (c *Controller) Daemons() []Daemon {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Daemon, len(c.daemons))
	copy(out, c.daemons)
	return out
}

func (c *Controller) SetStatus(name string, s DaemonStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statuses[name] = s
}

func (c *Controller) Status(name string) DaemonStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statuses[name]
}

func (c *Controller) AutoStartDaemons() []Daemon {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []Daemon
	for _, d := range c.daemons {
		if d.AutoStart {
			result = append(result, d)
		}
	}
	return result
}

func (c *Controller) HealthSummary() HealthSummary {
	c.mu.Lock()
	defer c.mu.Unlock()
	var h HealthSummary
	h.Total = len(c.daemons)
	for _, d := range c.daemons {
		switch c.statuses[d.Name] {
		case StatusRunning:
			h.Running++
		case StatusStopped:
			h.Stopped++
		case StatusFailed:
			h.Failed++
		default:
			h.Stopped++
		}
	}
	return h
}
