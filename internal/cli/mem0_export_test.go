package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMem0ExportCmd_RequiresAllFlag(t *testing.T) {
	rootCmd.SetArgs([]string{"mem0-export"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all")
}

func TestMem0ExportCmd_RequiresBaseURL(t *testing.T) {
	t.Setenv("MEM0_BASE_URL", "")
	rootCmd.SetArgs([]string{"mem0-export", "--all"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MEM0_BASE_URL")
}

func TestMem0ExportCmd_ExportsToFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{
					{"id": "m1", "memory": "hello"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
	}))
	defer srv.Close()

	t.Setenv("MEM0_BASE_URL", srv.URL)

	out := filepath.Join(t.TempDir(), "export.ndjson")
	rootCmd.SetArgs([]string{"mem0-export", "--all", "--output", out})
	err := rootCmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], `"m1"`)
}
