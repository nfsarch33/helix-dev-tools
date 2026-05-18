package preflearn

import (
    "sync"

    "gopkg.in/yaml.v3"
)

type SignalType string

const (
    SignalAccept SignalType = "accept"
    SignalReject SignalType = "reject"
    SignalEdit   SignalType = "edit"
    SignalRevert SignalType = "revert"
)

type Pattern struct {
    Key        string  `yaml:"key"`
    Value      string  `yaml:"value"`
    Count      int     `yaml:"count"`
    Confidence float64 `yaml:"confidence"`
}

type Signal struct {
    Type    SignalType
    AgentID string
    Key     string
    Value   string
}

type PatternDiff struct {
    Key      string
    OldValue string
    NewValue string
    Changed  bool
}

type Learner struct {
    signals map[string][]Signal
    mu      sync.Mutex
}

func NewLearner() *Learner {
    return &Learner{
        signals: make(map[string][]Signal),
    }
}

func (l *Learner) AddSignal(s Signal) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.signals[s.AgentID] = append(l.signals[s.AgentID], s)
}

func (l *Learner) Patterns(agentID string) []Pattern {
    l.mu.Lock()
    defer l.mu.Unlock()

    patterns := make(map[string]Pattern)
    for _, signal := range l.signals[agentID] {
        key := signal.Key + ":" + signal.Value
        pattern := patterns[key]
        pattern.Key = signal.Key
        pattern.Value = signal.Value
        pattern.Count++
        pattern.Confidence = min(1.0, float64(pattern.Count)/10.0)
        patterns[key] = pattern
    }

    result := make([]Pattern, 0, len(patterns))
    for _, pattern := range patterns {
        result = append(result, pattern)
    }
    return result
}

func (l *Learner) Export(agentID string) ([]byte, error) {
    return yaml.Marshal(l.Patterns(agentID))
}

func (l *Learner) Diff(agentID string, baseline []Pattern) []PatternDiff {
    currentPatterns := l.Patterns(agentID)

    baselineMap := make(map[string]string)
    for _, p := range baseline {
        baselineMap[p.Key] = p.Value
    }

    var diffs []PatternDiff
    for _, current := range currentPatterns {
        oldValue, exists := baselineMap[current.Key]
        if exists && oldValue != current.Value {
            diffs = append(diffs, PatternDiff{
                Key:      current.Key,
                OldValue: oldValue,
                NewValue: current.Value,
                Changed:  true,
            })
        } else if !exists {
            diffs = append(diffs, PatternDiff{
                Key:      current.Key,
                NewValue: current.Value,
                Changed:  true,
            })
        }
    }

    return diffs
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}