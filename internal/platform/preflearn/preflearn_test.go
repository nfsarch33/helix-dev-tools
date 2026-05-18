package preflearn

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestAddSignal_AccumulatesCount(t *testing.T) {
    learner := NewLearner()
    signal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    learner.AddSignal(signal)
    learner.AddSignal(signal)
    learner.AddSignal(signal)

    patterns := learner.Patterns("agent1")
    assert.Len(t, patterns, 1)
    assert.Equal(t, 3, patterns[0].Count)
}

func TestPattern_Confidence_Low(t *testing.T) {
    learner := NewLearner()
    signal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    learner.AddSignal(signal)
    learner.AddSignal(signal)

    patterns := learner.Patterns("agent1")
    assert.Len(t, patterns, 1)
    assert.Less(t, patterns[0].Confidence, 0.5)
}

func TestPattern_Confidence_High(t *testing.T) {
    learner := NewLearner()
    signal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    for i := 0; i < 5; i++ {
        learner.AddSignal(signal)
    }

    patterns := learner.Patterns("agent1")
    assert.Len(t, patterns, 1)
    assert.Equal(t, 0.5, patterns[0].Confidence)
}

func TestPattern_HighConfidence_Count10(t *testing.T) {
    learner := NewLearner()
    signal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    for i := 0; i < 10; i++ {
        learner.AddSignal(signal)
    }

    patterns := learner.Patterns("agent1")
    assert.Len(t, patterns, 1)
    assert.Equal(t, 1.0, patterns[0].Confidence)
}

func TestExport_YAML(t *testing.T) {
    learner := NewLearner()
    signal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    learner.AddSignal(signal)
    learner.AddSignal(signal)

    yamlBytes, err := learner.Export("agent1")
    assert.NoError(t, err)
    assert.NotEmpty(t, yamlBytes)
}

func TestDiff_DetectsChange(t *testing.T) {
    learner := NewLearner()
    baseline := []Pattern{
        {
            Key:   "indent_style",
            Value: "tabs",
        },
    }

    newSignal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "spaces",
    }

    learner.AddSignal(newSignal)

    diffs := learner.Diff("agent1", baseline)
    assert.Len(t, diffs, 1)
    assert.True(t, diffs[0].Changed)
    assert.Equal(t, "tabs", diffs[0].OldValue)
    assert.Equal(t, "spaces", diffs[0].NewValue)
}

func TestDiff_NoChange(t *testing.T) {
    learner := NewLearner()
    baseline := []Pattern{
        {
            Key:   "indent_style",
            Value: "tabs",
        },
    }

    newSignal := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }

    learner.AddSignal(newSignal)

    diffs := learner.Diff("agent1", baseline)
    assert.Len(t, diffs, 0)
}

func TestLearner_MultipleAgents(t *testing.T) {
    learner := NewLearner()
    signal1 := Signal{
        Type:    SignalAccept,
        AgentID: "agent1",
        Key:     "indent_style",
        Value:   "tabs",
    }
    signal2 := Signal{
        Type:    SignalAccept,
        AgentID: "agent2",
        Key:     "indent_style",
        Value:   "spaces",
    }

    learner.AddSignal(signal1)
    learner.AddSignal(signal1)
    learner.AddSignal(signal2)

    patterns1 := learner.Patterns("agent1")
    patterns2 := learner.Patterns("agent2")

    assert.Len(t, patterns1, 1)
    assert.Equal(t, "tabs", patterns1[0].Value)
    assert.Len(t, patterns2, 1)
    assert.Equal(t, "spaces", patterns2[0].Value)
}