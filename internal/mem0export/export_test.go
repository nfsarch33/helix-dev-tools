package mem0export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
