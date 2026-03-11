package cli

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
)

var mem0ParityFlags struct {
	apiKey         string
	userID         string
	appID          string
	mcpJSON        string
	pageSize       int
	export         string
	strict         bool
	syncProvenance bool
	dryRun         bool
}

var mem0ParityCmd = &cobra.Command{
	Use:   "mem0-parity",
	Short: "Audit Git-backed memory parity against Mem0",
	Long:  "Builds a manifest from durable Git-backed memory sources and compares it against managed Mem0 records using source provenance metadata when available.",
	RunE:  runMem0Parity,
}

func init() {
	p := config.DefaultPaths()
	mem0ParityCmd.Flags().StringVar(&mem0ParityFlags.apiKey, "api-key", "", "Mem0 API key (defaults to env or ~/.cursor/mcp.json)")
	mem0ParityCmd.Flags().StringVar(&mem0ParityFlags.userID, "user-id", "", "Mem0 user ID (defaults to env or ~/.cursor/mcp.json)")
	mem0ParityCmd.Flags().StringVar(&mem0ParityFlags.appID, "app-id", "cursor-global-kb", "Mem0 app_id used for the global-kb memory namespace")
	mem0ParityCmd.Flags().StringVar(&mem0ParityFlags.mcpJSON, "mcp-json", p.CursorMCPConfig(), "Path to Cursor MCP config for resolving Mem0 credentials")
	mem0ParityCmd.Flags().IntVar(&mem0ParityFlags.pageSize, "page-size", 100, "Page size when listing Mem0 memories")
	mem0ParityCmd.Flags().StringVar(&mem0ParityFlags.export, "export", "", "Optional path to export a Markdown audit report")
	mem0ParityCmd.Flags().BoolVar(&mem0ParityFlags.strict, "strict", false, "Exit non-zero when parity is not proven")
	mem0ParityCmd.Flags().BoolVar(&mem0ParityFlags.syncProvenance, "sync-provenance", false, "Backfill provenance metadata for content-matched memories and add missing manifest entries")
	mem0ParityCmd.Flags().BoolVar(&mem0ParityFlags.dryRun, "dry-run", false, "Show planned provenance sync actions without mutating Mem0")
}

type mem0AuditConfig struct {
	APIKey string
	UserID string
	AppID  string
}

type mem0MCPConfig struct {
	MCPServers map[string]struct {
		Env map[string]string `json:"env"`
	} `json:"mcpServers"`
}

type parityManifestEntry struct {
	Kind        string
	SourcePath  string
	SourceID    string
	Title       string
	Snippet     string
	Fingerprint string
}

type mem0RemoteMemory struct {
	ID       string                 `json:"id"`
	Memory   string                 `json:"memory"`
	Metadata map[string]any         `json:"metadata"`
	UserID   string                 `json:"user_id"`
	AppID    string                 `json:"app_id"`
	Raw      map[string]interface{} `json:"-"`
}

type mem0ListResponse struct {
	Results []mem0RemoteMemory `json:"results"`
	Total   int                `json:"total"`
}

type parityMatch struct {
	Entry parityManifestEntry
	Type  string
	Note  string
}

type parityReport struct {
	GeneratedAt             time.Time
	AppID                   string
	UserID                  string
	ManifestEntries         int
	ManifestByKind          map[string]int
	RemoteMemories          int
	RemoteWithProvenance    int
	RemoteWithoutProvenance int
	ExactMatches            []parityMatch
	ContentMatches          []parityMatch
	Missing                 []parityManifestEntry
}

func runMem0Parity(_ *cobra.Command, _ []string) error {
	paths := config.DefaultPaths()
	cfg, err := resolveMem0AuditConfig(paths, mem0ParityFlags)
	if err != nil {
		return err
	}

	manifest, err := buildParityManifest(paths)
	if err != nil {
		return err
	}

	timeout := 60 * time.Second
	if mem0ParityFlags.syncProvenance {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	remote, err := listMem0Memories(ctx, cfg, mem0ParityFlags.pageSize)
	if err != nil {
		return err
	}

	report := compareParity(manifest, remote, cfg)
	printParityReport(report)

	if mem0ParityFlags.syncProvenance {
		updated, added, err := syncParity(ctx, cfg, report, remote, mem0ParityFlags.dryRun)
		if err != nil {
			return err
		}
		fmt.Printf("\n  Provenance sync actions: updated=%d added=%d dry_run=%t\n", updated, added, mem0ParityFlags.dryRun)
		if !mem0ParityFlags.dryRun {
			remote, err = listMem0Memories(ctx, cfg, mem0ParityFlags.pageSize)
			if err != nil {
				return err
			}
			report = compareParity(manifest, remote, cfg)
			fmt.Println("\n  Post-sync audit:")
			printParityReport(report)
		}
	}

	if mem0ParityFlags.export != "" {
		if err := os.WriteFile(mem0ParityFlags.export, []byte(report.Markdown()), 0o644); err != nil {
			return fmt.Errorf("writing parity export: %w", err)
		}
		clilog.Success("exported to %s", mem0ParityFlags.export)
	}

	if mem0ParityFlags.strict && !report.Proven() {
		return fmt.Errorf("mem0 parity is not yet proven: exact=%d missing=%d remote_without_provenance=%d", len(report.ExactMatches), len(report.Missing), report.RemoteWithoutProvenance)
	}
	return nil
}

func resolveMem0AuditConfig(paths config.Paths, flags struct {
	apiKey         string
	userID         string
	appID          string
	mcpJSON        string
	pageSize       int
	export         string
	strict         bool
	syncProvenance bool
	dryRun         bool
}) (mem0AuditConfig, error) {
	cfg := mem0AuditConfig{
		APIKey: strings.TrimSpace(flags.apiKey),
		UserID: strings.TrimSpace(flags.userID),
		AppID:  strings.TrimSpace(flags.appID),
	}
	if cfg.APIKey == "" {
		cfg.APIKey = strings.TrimSpace(os.Getenv("MEM0_API_KEY"))
	}
	if cfg.UserID == "" {
		cfg.UserID = strings.TrimSpace(os.Getenv("MEM0_DEFAULT_USER_ID"))
	}
	if cfg.APIKey != "" && cfg.UserID != "" {
		return cfg, nil
	}

	data, err := os.ReadFile(flags.mcpJSON)
	if err != nil {
		if cfg.APIKey == "" || cfg.UserID == "" {
			return mem0AuditConfig{}, fmt.Errorf("reading mcp config for Mem0 defaults: %w", err)
		}
		return cfg, nil
	}

	var mcpCfg mem0MCPConfig
	if err := json.Unmarshal(data, &mcpCfg); err != nil {
		return mem0AuditConfig{}, fmt.Errorf("parsing mcp config for Mem0 defaults: %w", err)
	}
	mem0Cfg, ok := mcpCfg.MCPServers["mem0"]
	if !ok {
		return mem0AuditConfig{}, fmt.Errorf("mem0 server missing from %s", flags.mcpJSON)
	}
	if cfg.APIKey == "" {
		cfg.APIKey = resolveSecretValue(mem0Cfg.Env["MEM0_API_KEY"])
	}
	if cfg.UserID == "" {
		cfg.UserID = resolveSecretValue(mem0Cfg.Env["MEM0_DEFAULT_USER_ID"])
	}
	if cfg.APIKey == "" {
		return mem0AuditConfig{}, fmt.Errorf("missing Mem0 API key; set MEM0_API_KEY or configure mem0 in %s", flags.mcpJSON)
	}
	if cfg.UserID == "" {
		return mem0AuditConfig{}, fmt.Errorf("missing Mem0 user ID; set MEM0_DEFAULT_USER_ID or configure mem0 in %s", flags.mcpJSON)
	}
	_ = paths
	return cfg, nil
}

func resolveSecretValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "$") && len(raw) > 1 {
		return strings.TrimSpace(os.Getenv(strings.TrimPrefix(raw, "$")))
	}
	return raw
}

func buildParityManifest(paths config.Paths) ([]parityManifestEntry, error) {
	var entries []parityManifestEntry

	patterns, err := buildPatternManifest(filepath.Join(paths.GlobalLearningsDir(), "PATTERNS.md"))
	if err != nil {
		return nil, err
	}
	entries = append(entries, patterns...)

	for _, item := range []struct {
		path string
		kind string
	}{
		{filepath.Join(paths.GlobalLearningsDir(), "LEARNINGS.md"), "learning"},
		{filepath.Join(paths.GlobalLearningsDir(), "ERRORS.md"), "error"},
		{filepath.Join(paths.GlobalLearningsDir(), "FEATURE_REQUESTS.md"), "feature_request"},
		{filepath.Join(paths.GlobalMemoriesDir(), "error-patterns.md"), "error_pattern"},
	} {
		sections, err := buildSectionManifest(item.path, item.kind)
		if err != nil {
			return nil, err
		}
		entries = append(entries, sections...)
	}

	episodes, err := buildEpisodeManifest(filepath.Join(paths.GlobalLearningsDir(), "episodes"))
	if err != nil {
		return nil, err
	}
	entries = append(entries, episodes...)

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].SourcePath == entries[j].SourcePath {
			return entries[i].SourceID < entries[j].SourceID
		}
		return entries[i].SourcePath < entries[j].SourcePath
	})
	return entries, nil
}

func buildPatternManifest(path string) ([]parityManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading patterns manifest source: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	var entries []parityManifestEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| pat-") {
			continue
		}
		parts := splitTableRow(trimmed)
		if len(parts) < 2 {
			continue
		}
		id := parts[0]
		title := parts[1]
		entries = append(entries, newManifestEntry("pattern", path, id, title, title))
	}
	return entries, nil
}

func buildSectionManifest(path, kind string) ([]parityManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest source %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	var entries []parityManifestEntry
	var currentHeading string
	var section []string
	flush := func() {
		if strings.TrimSpace(currentHeading) == "" {
			return
		}
		title := strings.TrimSpace(strings.TrimPrefix(currentHeading, "## "))
		sourceID := title
		snippet := buildSnippet(section)
		entries = append(entries, newManifestEntry(kind, path, sourceID, title, snippet))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			currentHeading = line
			section = section[:0]
			continue
		}
		if currentHeading != "" {
			section = append(section, line)
		}
	}
	flush()
	return entries, nil
}

func buildEpisodeManifest(dir string) ([]parityManifestEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading episodes directory: %w", err)
	}
	var out []parityManifestEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading episode %s: %w", entry.Name(), err)
		}
		title, snippet := firstMarkdownHeadingAndSnippet(string(data), entry.Name())
		out = append(out, newManifestEntry("episode", path, entry.Name(), title, snippet))
	}
	return out, nil
}

func newManifestEntry(kind, path, sourceID, title, snippet string) parityManifestEntry {
	fingerprintInput := strings.TrimSpace(kind + "\n" + path + "\n" + sourceID + "\n" + title + "\n" + snippet)
	sum := sha1.Sum([]byte(fingerprintInput))
	return parityManifestEntry{
		Kind:        kind,
		SourcePath:  filepath.ToSlash(path),
		SourceID:    strings.TrimSpace(sourceID),
		Title:       strings.TrimSpace(title),
		Snippet:     strings.TrimSpace(snippet),
		Fingerprint: hex.EncodeToString(sum[:]),
	}
}

func splitTableRow(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func buildSnippet(lines []string) string {
	var picked []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		picked = append(picked, line)
		if len(picked) >= 3 {
			break
		}
	}
	return strings.Join(picked, " ")
}

func firstMarkdownHeadingAndSnippet(content, fallback string) (string, string) {
	lines := strings.Split(content, "\n")
	title := fallback
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title = strings.TrimSpace(strings.TrimLeft(line, "#"))
			break
		}
	}
	return title, buildSnippet(lines)
}

func listMem0Memories(ctx context.Context, cfg mem0AuditConfig, pageSize int) ([]mem0RemoteMemory, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var all []mem0RemoteMemory
	for page := 1; ; page++ {
		payload := map[string]interface{}{
			"filters": map[string]interface{}{
				"AND": []map[string]string{
					{"user_id": cfg.UserID},
					{"app_id": cfg.AppID},
				},
			},
			"fields":    []string{"id", "memory", "metadata", "created_at", "updated_at", "user_id", "app_id"},
			"page":      page,
			"page_size": pageSize,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding mem0 parity request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mem0.ai/v2/memories/", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("building mem0 parity request: %w", err)
		}
		req.Header.Set("Authorization", "Token "+cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("listing mem0 memories: %w", err)
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading mem0 list response: %w", readErr)
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("mem0 list failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
		}

		pageItems, total, err := decodeMem0ListResponse(data)
		if err != nil {
			return nil, err
		}
		all = append(all, pageItems...)
		if len(pageItems) == 0 || len(pageItems) < pageSize {
			break
		}
		if total > 0 && len(all) >= total {
			break
		}
	}
	return all, nil
}

func decodeMem0ListResponse(data []byte) ([]mem0RemoteMemory, int, error) {
	var wrapped mem0ListResponse
	if err := json.Unmarshal(data, &wrapped); err == nil && (len(wrapped.Results) > 0 || bytes.Contains(data, []byte(`"results"`))) {
		return wrapped.Results, wrapped.Total, nil
	}

	var items []mem0RemoteMemory
	if err := json.Unmarshal(data, &items); err == nil {
		return items, len(items), nil
	}
	return nil, 0, fmt.Errorf("parsing mem0 list response")
}

func compareParity(manifest []parityManifestEntry, remote []mem0RemoteMemory, cfg mem0AuditConfig) parityReport {
	report := parityReport{
		GeneratedAt:             time.Now().UTC(),
		AppID:                   cfg.AppID,
		UserID:                  cfg.UserID,
		ManifestEntries:         len(manifest),
		ManifestByKind:          map[string]int{},
		RemoteMemories:          len(remote),
		ExactMatches:            []parityMatch{},
		ContentMatches:          []parityMatch{},
		Missing:                 []parityManifestEntry{},
		RemoteWithProvenance:    0,
		RemoteWithoutProvenance: 0,
	}

	exactIndex := make(map[string][]mem0RemoteMemory)
	for _, item := range remote {
		sourcePath := sourcePathFromMetadata(item.Metadata)
		sourceID := sourceIDFromMetadata(item.Metadata)
		if sourcePath != "" && sourceID != "" {
			report.RemoteWithProvenance++
			exactIndex[parityKey(sourcePath, sourceID)] = append(exactIndex[parityKey(sourcePath, sourceID)], item)
		} else {
			report.RemoteWithoutProvenance++
		}
	}

	usedRemote := make(map[string]bool)
	for _, entry := range manifest {
		report.ManifestByKind[entry.Kind]++
		key := parityKey(entry.SourcePath, entry.SourceID)
		if candidates := exactIndex[key]; len(candidates) > 0 {
			report.ExactMatches = append(report.ExactMatches, parityMatch{Entry: entry, Type: "exact", Note: candidates[0].ID})
			usedRemote[candidates[0].ID] = true
			continue
		}

		matched := false
		normTitle := normaliseText(entry.Title)
		normSnippet := normaliseText(entry.Snippet)
		for _, item := range remote {
			if usedRemote[item.ID] {
				continue
			}
			memText := normaliseText(item.Memory)
			if memText == "" {
				continue
			}
			if normTitle != "" && strings.Contains(memText, normTitle) {
				report.ContentMatches = append(report.ContentMatches, parityMatch{Entry: entry, Type: "content", Note: item.ID})
				usedRemote[item.ID] = true
				matched = true
				break
			}
			if normSnippet != "" && len(normSnippet) > 32 && strings.Contains(memText, normSnippet) {
				report.ContentMatches = append(report.ContentMatches, parityMatch{Entry: entry, Type: "content", Note: item.ID})
				usedRemote[item.ID] = true
				matched = true
				break
			}
		}
		if !matched {
			report.Missing = append(report.Missing, entry)
		}
	}

	return report
}

func printParityReport(report parityReport) {
	clilog.Header("cursor-tools mem0-parity")
	fmt.Printf("\n  User/App: %s / %s\n", report.UserID, report.AppID)
	fmt.Printf("  Manifest entries: %d\n", report.ManifestEntries)
	fmt.Printf("  Remote Mem0 memories: %d\n", report.RemoteMemories)
	fmt.Printf("  Remote memories with provenance metadata: %d\n", report.RemoteWithProvenance)
	fmt.Printf("  Remote memories without provenance metadata: %d\n", report.RemoteWithoutProvenance)
	fmt.Printf("  Exact provenance matches: %d\n", len(report.ExactMatches))
	fmt.Printf("  Content-only matches: %d\n", len(report.ContentMatches))
	fmt.Printf("  Missing manifest entries: %d\n", len(report.Missing))

	if len(report.ManifestByKind) > 0 {
		fmt.Println("\n  Manifest breakdown:")
		kinds := make([]string, 0, len(report.ManifestByKind))
		for kind := range report.ManifestByKind {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			fmt.Printf("    %-16s %d\n", kind, report.ManifestByKind[kind])
		}
	}

	if len(report.Missing) > 0 {
		fmt.Println("\n  Missing entries (first 10):")
		for i, entry := range report.Missing {
			if i >= 10 {
				break
			}
			fmt.Printf("    - [%s] %s :: %s\n", entry.Kind, entry.SourcePath, entry.SourceID)
		}
	}
}

func syncParity(ctx context.Context, cfg mem0AuditConfig, report parityReport, remote []mem0RemoteMemory, dryRun bool) (int, int, error) {
	remoteByID := make(map[string]mem0RemoteMemory, len(remote))
	for _, item := range remote {
		remoteByID[item.ID] = item
	}

	updated := 0
	for _, match := range report.ContentMatches {
		item, ok := remoteByID[match.Note]
		if !ok {
			continue
		}
		metadata := mergeMetadata(item.Metadata, provenanceMetadata(match.Entry))
		if dryRun {
			updated++
			continue
		}
		if err := updateMem0Memory(ctx, cfg, item.ID, item.Memory, metadata); err != nil {
			return updated, 0, fmt.Errorf("updating mem0 memory %s: %w", item.ID, err)
		}
		updated++
	}

	added := 0
	for _, entry := range report.Missing {
		if dryRun {
			added++
			continue
		}
		if err := addMem0Memory(ctx, cfg, entry); err != nil {
			return updated, added, fmt.Errorf("adding mem0 memory %s: %w", entry.SourceID, err)
		}
		added++
	}
	return updated, added, nil
}

func parityKey(path, sourceID string) string {
	return normalisePath(path) + "::" + strings.TrimSpace(sourceID)
}

func normalisePath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	for _, marker := range []string{"/Code/global-kb/", "/memo/"} {
		if idx := strings.Index(path, marker); idx >= 0 {
			path = path[idx+1:]
			path = strings.TrimPrefix(path, strings.TrimPrefix(marker, "/"))
			break
		}
	}
	return strings.TrimPrefix(path, "/")
}

func normaliseText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func sourcePathFromMetadata(metadata map[string]any) string {
	for _, key := range []string{"source_path", "sourcePath", "source_file", "path"} {
		if value := stringMetadata(metadata, key); value != "" {
			return value
		}
	}
	return ""
}

func sourceIDFromMetadata(metadata map[string]any) string {
	for _, key := range []string{"source_id", "sourceId", "source_entry_id", "entry_id"} {
		if value := stringMetadata(metadata, key); value != "" {
			return value
		}
	}
	return ""
}

func provenanceMetadata(entry parityManifestEntry) map[string]any {
	host, _ := os.Hostname()
	return map[string]any{
		"source_path":        normalisePath(entry.SourcePath),
		"source_id":          entry.SourceID,
		"kind":               entry.Kind,
		"project":            "cursor-global-kb",
		"machine":            host,
		"confidence":         1.0,
		"provenance_version": "v1",
		"fingerprint":        entry.Fingerprint,
	}
}

func mergeMetadata(existing map[string]any, overlay map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range overlay {
		merged[key] = value
	}
	return merged
}

func updateMem0Memory(ctx context.Context, cfg mem0AuditConfig, memoryID, text string, metadata map[string]any) error {
	payload := map[string]any{
		"text":     text,
		"metadata": metadata,
	}
	return doMem0JSON(ctx, http.MethodPut, "https://api.mem0.ai/v1/memories/"+memoryID+"/", cfg.APIKey, payload)
}

func addMem0Memory(ctx context.Context, cfg mem0AuditConfig, entry parityManifestEntry) error {
	payload := map[string]any{
		"user_id":    cfg.UserID,
		"app_id":     cfg.AppID,
		"version":    "v2",
		"infer":      false,
		"async_mode": false,
		"metadata":   provenanceMetadata(entry),
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": renderManifestMemory(entry),
			},
		},
	}
	return doMem0JSON(ctx, http.MethodPost, "https://api.mem0.ai/v1/memories/", cfg.APIKey, payload)
}

func renderManifestMemory(entry parityManifestEntry) string {
	if entry.Kind == "pattern" {
		return entry.Title
	}
	if entry.Snippet == "" {
		return entry.Title
	}
	return entry.Title + "\n\n" + entry.Snippet
}

func doMem0JSON(ctx context.Context, method, url, apiKey string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding mem0 payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building mem0 request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("sending mem0 request: %w", err)
	}
	data, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("reading mem0 response: %w", readErr)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (r parityReport) Proven() bool {
	return len(r.Missing) == 0 && r.RemoteWithoutProvenance == 0 && len(r.ExactMatches) == r.ManifestEntries
}

func (r parityReport) Markdown() string {
	var b strings.Builder
	b.WriteString("# Mem0 Parity Audit\n\n")
	b.WriteString(fmt.Sprintf("- Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- User/App: `%s` / `%s`\n", r.UserID, r.AppID))
	b.WriteString(fmt.Sprintf("- Manifest entries: %d\n", r.ManifestEntries))
	b.WriteString(fmt.Sprintf("- Remote Mem0 memories: %d\n", r.RemoteMemories))
	b.WriteString(fmt.Sprintf("- Remote memories with provenance metadata: %d\n", r.RemoteWithProvenance))
	b.WriteString(fmt.Sprintf("- Remote memories without provenance metadata: %d\n", r.RemoteWithoutProvenance))
	b.WriteString(fmt.Sprintf("- Exact provenance matches: %d\n", len(r.ExactMatches)))
	b.WriteString(fmt.Sprintf("- Content-only matches: %d\n", len(r.ContentMatches)))
	b.WriteString(fmt.Sprintf("- Missing manifest entries: %d\n", len(r.Missing)))
	b.WriteString(fmt.Sprintf("- Parity proven: `%t`\n", r.Proven()))

	if len(r.ManifestByKind) > 0 {
		b.WriteString("\n## Manifest Breakdown\n\n")
		kinds := make([]string, 0, len(r.ManifestByKind))
		for kind := range r.ManifestByKind {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			b.WriteString(fmt.Sprintf("- `%s`: %d\n", kind, r.ManifestByKind[kind]))
		}
	}

	if len(r.Missing) > 0 {
		b.WriteString("\n## Missing Entries\n\n")
		for _, entry := range r.Missing {
			b.WriteString(fmt.Sprintf("- `%s` | `%s` | `%s`\n", entry.Kind, normalisePath(entry.SourcePath), entry.SourceID))
		}
	}

	if len(r.ContentMatches) > 0 {
		b.WriteString("\n## Content-Only Matches\n\n")
		b.WriteString("These likely exist in Mem0, but are not provenance-safe enough to prove parity.\n\n")
		for _, match := range r.ContentMatches {
			b.WriteString(fmt.Sprintf("- `%s` | `%s` | `%s`\n", match.Entry.Kind, normalisePath(match.Entry.SourcePath), match.Entry.SourceID))
		}
	}

	return b.String()
}
