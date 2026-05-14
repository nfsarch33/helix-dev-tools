package mem0export

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Memory represents a single Mem0 memory record.
type Memory struct {
	ID        string                 `json:"id"`
	Memory    string                 `json:"memory"`
	UserID    string                 `json:"user_id,omitempty"`
	Hash      string                 `json:"hash,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
	UpdatedAt string                 `json:"updated_at,omitempty"`
}

// MemoriesResponse is the envelope returned by GET /v1/memories/.
type MemoriesResponse struct {
	Results []Memory `json:"results"`
}

// Exporter pages through the Mem0 API and writes NDJSON.
type Exporter struct {
	BaseURL  string
	APIKey   string
	PageSize int
	Client   *http.Client
}

func (e *Exporter) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (e *Exporter) pageSize() int {
	if e.PageSize > 0 {
		return e.PageSize
	}
	return 100
}

// Export fetches all memories and writes one JSON object per line.
// Returns the total number of memories written.
func (e *Exporter) Export(w io.Writer) (int, error) {
	total := 0
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	for page := 1; ; page++ {
		memories, err := e.fetchPage(page)
		if err != nil {
			return total, fmt.Errorf("fetch page %d: %w", page, err)
		}
		if len(memories) == 0 {
			break
		}
		for _, m := range memories {
			if err := enc.Encode(m); err != nil {
				return total, fmt.Errorf("encode memory %s: %w", m.ID, err)
			}
			total++
		}
		if len(memories) < e.pageSize() {
			break
		}
	}
	return total, nil
}

// ExportToFile writes NDJSON to a file path (satisfies mem0backup.FileExporter).
func (e *Exporter) ExportToFile(path string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	return e.Export(f)
}

func (e *Exporter) fetchPage(page int) ([]Memory, error) {
	url := e.BaseURL + "/v1/memories/?page=" + strconv.Itoa(page)
	if e.pageSize() != 100 {
		url += "&page_size=" + strconv.Itoa(e.pageSize())
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := e.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var envelope MemoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return envelope.Results, nil
}
