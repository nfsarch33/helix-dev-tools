package mem0export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeMemories(n int) []Memory {
	mems := make([]Memory, n)
	for i := range n {
		mems[i] = Memory{
			ID:     fmt.Sprintf("mem-%03d", i),
			Memory: fmt.Sprintf("test memory %d", i),
			UserID: "u1",
		}
	}
	return mems
}

func TestExportAll_WritesNDJSON(t *testing.T) {
	mems := makeMemories(5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/memories/", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			json.NewEncoder(w).Encode(MemoriesResponse{Results: mems})
			return
		}
		json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	exp := &Exporter{
		BaseURL: srv.URL,
		APIKey:  "test-key",
	}

	n, err := exp.Export(&buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 5)

	for i, line := range lines {
		var m Memory
		require.NoError(t, json.Unmarshal([]byte(line), &m), "line %d", i)
		assert.Equal(t, mems[i].ID, m.ID)
		assert.Equal(t, mems[i].Memory, m.Memory)
	}
}

func TestExportAll_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	exp := &Exporter{BaseURL: srv.URL, APIKey: "k"}

	n, err := exp.Export(&buf)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestExportAll_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	exp := &Exporter{BaseURL: srv.URL, APIKey: "k"}

	_, err := exp.Export(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestExportAll_Pagination(t *testing.T) {
	page1 := makeMemories(10)
	page2 := func() []Memory {
		m := make([]Memory, 10)
		for i := range 10 {
			m[i] = Memory{
				ID:     fmt.Sprintf("mem-1%02d", i),
				Memory: fmt.Sprintf("page2 memory %d", i),
				UserID: "u1",
			}
		}
		return m
	}()
	page3 := func() []Memory {
		m := make([]Memory, 10)
		for i := range 10 {
			m[i] = Memory{
				ID:     fmt.Sprintf("mem-2%02d", i),
				Memory: fmt.Sprintf("page3 memory %d", i),
				UserID: "u1",
			}
		}
		return m
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page1})
		case "2":
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page2})
		case "3":
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page3})
		default:
			json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	exp := &Exporter{BaseURL: srv.URL, APIKey: "k", PageSize: 10}

	n, err := exp.Export(&buf)
	require.NoError(t, err)
	assert.Equal(t, 30, n)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 30)

	var first, last Memory
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[29]), &last))
	assert.Equal(t, "mem-000", first.ID)
	assert.Equal(t, "mem-209", last.ID)
}

func TestExporter_RoundTrip_ExportImportDiff(t *testing.T) {
	original := []Memory{
		{
			ID:        "rt-001",
			Memory:    "round trip test memory one",
			UserID:    "user-alpha",
			Hash:      "abc123",
			Metadata:  map[string]interface{}{"source": "test", "priority": "high"},
			CreatedAt: "2026-05-10T00:00:00Z",
			UpdatedAt: "2026-05-14T12:00:00Z",
		},
		{
			ID:        "rt-002",
			Memory:    "round trip with special chars: 日本語 & <html>",
			UserID:    "user-beta",
			Hash:      "def456",
			Metadata:  map[string]interface{}{"tag": "unicode"},
			CreatedAt: "2026-05-11T00:00:00Z",
			UpdatedAt: "2026-05-14T13:00:00Z",
		},
		{
			ID:     "rt-003",
			Memory: "minimal memory",
			UserID: "user-alpha",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			json.NewEncoder(w).Encode(MemoriesResponse{Results: original})
			return
		}
		json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.ndjson")
	exp := &Exporter{BaseURL: srv.URL, APIKey: "rt-key"}

	n, err := exp.ExportToFile(path)
	require.NoError(t, err)
	assert.Equal(t, len(original), n)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, len(original))

	for i, line := range lines {
		var got Memory
		require.NoError(t, json.Unmarshal([]byte(line), &got), "line %d", i)
		assert.Equal(t, original[i].ID, got.ID, "ID drift at %d", i)
		assert.Equal(t, original[i].Memory, got.Memory, "Memory drift at %d", i)
		assert.Equal(t, original[i].UserID, got.UserID, "UserID drift at %d", i)
		assert.Equal(t, original[i].Hash, got.Hash, "Hash drift at %d", i)
		assert.Equal(t, original[i].CreatedAt, got.CreatedAt, "CreatedAt drift at %d", i)
		assert.Equal(t, original[i].UpdatedAt, got.UpdatedAt, "UpdatedAt drift at %d", i)
		if original[i].Metadata != nil {
			require.NotNil(t, got.Metadata, "Metadata nil at %d", i)
			for k, v := range original[i].Metadata {
				assert.Equal(t, v, got.Metadata[k], "Metadata[%s] drift at %d", k, i)
			}
		}
	}
}

func TestExporter_LargeExport_1000Memories(t *testing.T) {
	const total = 1000
	const perPage = 10
	const pages = total / perPage

	allMems := makeMemories(total)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		p := 1
		if pageStr != "" {
			var err error
			p, err = strconv.Atoi(pageStr)
			if err != nil || p < 1 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if p > pages {
			json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
			return
		}
		start := (p - 1) * perPage
		end := start + perPage
		json.NewEncoder(w).Encode(MemoriesResponse{Results: allMems[start:end]})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	exp := &Exporter{BaseURL: srv.URL, APIKey: "k", PageSize: perPage}

	n, err := exp.Export(&buf)
	require.NoError(t, err)
	assert.Equal(t, total, n)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, total)

	seen := make(map[string]bool, total)
	for i, line := range lines {
		var m Memory
		require.NoError(t, json.Unmarshal([]byte(line), &m), "invalid NDJSON at line %d", i)
		assert.False(t, seen[m.ID], "duplicate ID %s at line %d", m.ID, i)
		seen[m.ID] = true
	}
	assert.Len(t, seen, total)
}

func TestExporter_ResumeAfterError(t *testing.T) {
	const perPage = 5

	page1 := makeMemories(5)
	page2 := func() []Memory {
		m := make([]Memory, 5)
		for i := range 5 {
			m[i] = Memory{ID: fmt.Sprintf("mem-1%02d", i), Memory: fmt.Sprintf("p2 mem %d", i), UserID: "u1"}
		}
		return m
	}()
	page3 := func() []Memory {
		m := make([]Memory, 3)
		for i := range 3 {
			m[i] = Memory{ID: fmt.Sprintf("mem-2%02d", i), Memory: fmt.Sprintf("p3 mem %d", i), UserID: "u1"}
		}
		return m
	}()

	var attempt atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page1})
		case "2":
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page2})
		case "3":
			if attempt.Load() == 0 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"transient failure"}`))
				return
			}
			json.NewEncoder(w).Encode(MemoriesResponse{Results: page3})
		default:
			json.NewEncoder(w).Encode(MemoriesResponse{Results: nil})
		}
	}))
	defer srv.Close()

	exp := &Exporter{BaseURL: srv.URL, APIKey: "k", PageSize: perPage}

	var buf1 bytes.Buffer
	_, err := exp.Export(&buf1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")

	attempt.Store(1)

	var buf2 bytes.Buffer
	n, err := exp.Export(&buf2)
	require.NoError(t, err)
	assert.Equal(t, 13, n)

	lines := strings.Split(strings.TrimSpace(buf2.String()), "\n")
	assert.Len(t, lines, 13)

	seen := make(map[string]bool)
	for _, line := range lines {
		var m Memory
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		assert.False(t, seen[m.ID], "duplicate: %s", m.ID)
		seen[m.ID] = true
	}
}
