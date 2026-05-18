package sprintboard_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/sprintboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTFIDF_EmptyCorpus(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	results := idx.Search("hello", 5)
	assert.Empty(t, results)
}

func TestTFIDF_SingleDocument(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "implement docker identity isolation for git operations")
	results := idx.Search("docker identity", 5)
	require.Len(t, results, 1)
	assert.Equal(t, "t1", results[0].ID)
	assert.Greater(t, results[0].Score, 0.0)
}

func TestTFIDF_MultipleDocuments(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "docker identity isolation for git push operations")
	idx.AddDocument("t2", "mem0 performance benchmark harness with diagnosis")
	idx.AddDocument("t3", "sprintboard semantic search using tfidf cosine")

	results := idx.Search("mem0 performance", 5)
	require.NotEmpty(t, results)
	assert.Equal(t, "t2", results[0].ID)
}

func TestTFIDF_RelevanceRanking(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "eval harness runner grader metrics report")
	idx.AddDocument("t2", "eval harness activation yaml definitions batch run")
	idx.AddDocument("t3", "docker container isolation network scrub tokens")

	results := idx.Search("eval harness", 3)
	require.Len(t, results, 2)
	assert.Equal(t, "t1", results[0].ID)
	assert.Equal(t, "t2", results[1].ID)
}

func TestTFIDF_LimitResults(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	for i := 0; i < 20; i++ {
		idx.AddDocument(
			"t"+string(rune('A'+i)),
			"common term document number plus specific content",
		)
	}
	results := idx.Search("common term", 5)
	assert.LessOrEqual(t, len(results), 5)
}

func TestTFIDF_RemoveDocument(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "docker identity isolation")
	idx.AddDocument("t2", "mem0 performance benchmark")
	idx.RemoveDocument("t1")

	results := idx.Search("docker identity", 5)
	assert.Empty(t, results)
}

func TestTFIDF_UpdateDocument(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "old content about docker")
	idx.AddDocument("t1", "new content about kubernetes helm charts")

	results := idx.Search("kubernetes helm", 5)
	require.Len(t, results, 1)
	assert.Equal(t, "t1", results[0].ID)

	results = idx.Search("docker", 5)
	assert.Empty(t, results)
}

func TestTFIDF_CaseInsensitive(t *testing.T) {
	idx := sprintboard.NewTFIDFIndex()
	idx.AddDocument("t1", "Docker Identity ISOLATION")
	results := idx.Search("docker identity isolation", 5)
	require.Len(t, results, 1)
	assert.Equal(t, "t1", results[0].ID)
}

func TestSearchResult_Fields(t *testing.T) {
	r := sprintboard.SearchResult{ID: "t1", Score: 0.85}
	assert.Equal(t, "t1", r.ID)
	assert.InDelta(t, 0.85, r.Score, 0.001)
}

func TestAgentraceBridge_EventFormat(t *testing.T) {
	event := sprintboard.NewAgentraceEvent("ticket_search", map[string]string{
		"query": "docker isolation",
		"limit": "5",
	})
	assert.Equal(t, "mcp_tool_call", event.EventType)
	assert.Equal(t, "ticket_search", event.ToolName)
	assert.NotEmpty(t, event.Timestamp)
	assert.Equal(t, "docker isolation", event.Args["query"])
}

func TestAgentraceBridge_ToNDJSON(t *testing.T) {
	event := sprintboard.NewAgentraceEvent("sprint_create", map[string]string{
		"id": "v6300",
	})
	line := event.ToNDJSON()
	assert.Contains(t, line, `"event_type":"mcp_tool_call"`)
	assert.Contains(t, line, `"tool_name":"sprint_create"`)
	assert.Contains(t, line, `"v6300"`)
	assert.True(t, line[len(line)-1] == '\n')
}
