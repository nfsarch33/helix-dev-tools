package agentrace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CloudIngestHandler struct {
	storeDir string
	mu       sync.Mutex
	count    int64
	logger   *slog.Logger
}

func NewCloudIngestHandler(storeDir string, logger *slog.Logger) (*CloudIngestHandler, error) {
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CloudIngestHandler{
		storeDir: storeDir,
		logger:   logger,
	}, nil
}

func (h *CloudIngestHandler) Count() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func (h *CloudIngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.Contains(ct, "ndjson") && !strings.Contains(ct, "json") {
		http.Error(w, "content-type must be application/x-ndjson or application/json", http.StatusUnsupportedMediaType)
		return
	}

	events, errs := h.parseAndStore(r.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted": events,
		"errors":   errs,
	})
}

func (h *CloudIngestHandler) parseAndStore(body io.Reader) (accepted int, parseErrors int) {
	filename := fmt.Sprintf("ingest-%s.jsonl", time.Now().UTC().Format("20060102T150405"))
	path := filepath.Join(h.storeDir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		h.logger.Error("failed to open ingest file", "path", path, "err", err)
		return 0, 1
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			parseErrors++
			continue
		}

		if _, ok := obj["ts"]; !ok {
			obj["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
		}

		out, err := json.Marshal(obj)
		if err != nil {
			parseErrors++
			continue
		}

		_, _ = w.Write(out)
		_ = w.WriteByte('\n')
		accepted++
	}

	h.mu.Lock()
	h.count += int64(accepted)
	h.mu.Unlock()

	h.logger.Info("ingested events", "accepted", accepted, "errors", parseErrors, "file", filename)
	return accepted, parseErrors
}
