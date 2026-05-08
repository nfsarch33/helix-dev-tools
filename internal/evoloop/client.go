// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop client filters Mem0 capsules by the canonical evoloop-daemon source label and wsl1 producer-machine name

// Package evoloop queries Mem0 for EvoLoop capsules (rollups and cycles)
// so any Cursor instance can read fleet-wide EvoLoop-DRL state at session
// start, regardless of which machine wrote it.
//
// Capsules are produced by the evoloop-daemon (go/internal/evoloop) with
// app_id=cursor-global-kb and metadata.kind in {evoloop_rollup, evoloop_cycle}.
// This client is read-only: writes are owned by the daemon.
package evoloop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultAppID is the Mem0 namespace EvoLoop capsules are published under.
const DefaultAppID = "cursor-global-kb"

// DefaultBaseURL is the Mem0 managed API base URL.
const DefaultBaseURL = "https://api.mem0.ai"

// CapsuleKind enumerates the kinds of EvoLoop capsules we know how to read.
type CapsuleKind string

const (
	// KindRollup is a one-per-machine-per-day aggregated capsule. Cheap to
	// fan out to every Cursor session at startup.
	KindRollup CapsuleKind = "evoloop_rollup"
	// KindCycle is a single feedback-loop cycle. Higher volume; useful for
	// drill-down but usually gated behind --kind=cycle on the CLI.
	KindCycle CapsuleKind = "evoloop_cycle"
	// KindPromotion is the capsule emitted by the promote --auto pass when
	// a candidate clears the TDD gate. Read back to dedupe against
	// previously-promoted sources and to correlate rollbacks.
	KindPromotion CapsuleKind = "evoloop_promotion"
	// KindRollback is the capsule emitted when the promote --auto rolling-
	// window check detects KPI regression against a previously-promoted
	// source.
	KindRollback CapsuleKind = "evoloop_rollback"
)

// Capsule is the parsed EvoLoop capsule returned by the client. Fields that
// aren't present in the metadata (e.g. Cycles on a cycle capsule) are left as
// zero values so callers can render either kind with the same struct.
type Capsule struct {
	ID        string            `json:"id"`
	Kind      CapsuleKind       `json:"kind"`
	Text      string            `json:"text"`
	Machine   string            `json:"machine"`
	Day       string            `json:"day,omitempty"`
	CycleID   string            `json:"cycle_id,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	Source    string            `json:"source,omitempty"`
	Event     string            `json:"event,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`

	// Rollup-only fields (zero for cycles).
	Cycles     int     `json:"cycles,omitempty"`
	Improved   int     `json:"improved,omitempty"`
	RolledBack int     `json:"rolled_back,omitempty"`
	Incomplete int     `json:"incomplete,omitempty"`
	NoChange   int     `json:"no_change,omitempty"`
	MeanDelta  float64 `json:"mean_delta,omitempty"`
	P50Delta   float64 `json:"p50_delta,omitempty"`
	P95Delta   float64 `json:"p95_delta,omitempty"`
	LastKPI    float64 `json:"last_kpi,omitempty"`

	// Cycle-only fields (zero for rollups). KPIBefore/KPIAfter come from
	// the canonical kind=evoloop_cycle writer; KPIDelta is preserved when
	// only an outcome capsule (kind=agent_outcome) is available because
	// the daemon emits gated cycles that never reach the canonical writer.
	KPIBefore  float64 `json:"kpi_before,omitempty"`
	KPIAfter   float64 `json:"kpi_after,omitempty"`
	KPIDelta   float64 `json:"kpi_delta,omitempty"`
	DurationMS int64   `json:"duration_ms,omitempty"`
}

// Client reads EvoLoop capsules from Mem0.
type Client struct {
	APIKey  string
	UserID  string
	AppID   string
	BaseURL string
	HTTP    *http.Client
	// Debug, when non-nil, receives a one-line summary of every Mem0
	// request emitted by Recent (POST /v2/memories/ + the resolved
	// filter payload + the requested machine/kinds). The CLI wires this
	// to stderr when --debug is passed so operators can sanity-check
	// what is actually queried before debugging "no capsules found".
	Debug io.Writer
}

// NewClient constructs a read-only EvoLoop client. Empty baseURL defaults to
// the Mem0 managed API; empty appID defaults to cursor-global-kb.
func NewClient(apiKey, userID, baseURL, appID string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if appID == "" {
		appID = DefaultAppID
	}
	return &Client{
		APIKey:  apiKey,
		UserID:  userID,
		AppID:   appID,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// RecentOptions controls the Recent query.
type RecentOptions struct {
	// Kinds restricts results to one or more capsule kinds. Empty means
	// "rollups only" because the rollup is the cheap cross-node summary.
	Kinds []CapsuleKind
	// Machine filters to a single producing machine (e.g. "wsl1"). Empty
	// returns every machine.
	Machine string
	// Limit caps the number of capsules returned after sorting. <=0 means
	// a sensible default (10).
	Limit int
}

// Recent returns the most recent EvoLoop capsules matching opts, newest
// first (by CreatedAt, ties broken by Day then ID).
//
// Mem0 v2 /memories/ rejects metadata.* as a filter key and caps results at
// 100 newest-first entries regardless of page_size, so we fetch once scoped
// by app_id+user_id and filter by kind/machine client-side. For the current
// capsule volume (≈1 rollup/machine/day plus cycles) the last 100 memories
// comfortably cover today's rollups plus recent cycles.
//
// Schema reconciliation (v254 d1): when KindCycle is requested, Recent also
// surfaces "cycle-like" rows that were not written under the canonical
// metadata.kind="evoloop_cycle" shape:
//
//   - kind=agent_outcome + actor=ironclaw-daemon + event has prefix
//     "ironclaw-daemon:cycle:" (current daemon's per-cycle hook, includes
//     gated cycles that never reach the canonical writer).
//   - kind=capsule + source=evoloop-daemon (legacy daemon binaries; kept
//     so a partially-upgraded fleet doesn't blind the reader).
//
// These rows are returned with Kind=KindCycle so downstream rendering and
// JSON consumers don't have to know about the producer-side variants.
func (c *Client) Recent(ctx context.Context, opts RecentOptions) ([]Capsule, error) {
	kinds := opts.Kinds
	if len(kinds) == 0 {
		kinds = []CapsuleKind{KindRollup}
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	wantKind := make(map[CapsuleKind]struct{}, len(kinds))
	for _, k := range kinds {
		wantKind[k] = struct{}{}
	}
	_, wantCycle := wantKind[KindCycle]

	c.logDebug(opts, kinds, limit)

	rows, err := c.listAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("evoloop: listing memories: %w", err)
	}

	all := make([]Capsule, 0, len(rows))
	for _, r := range rows {
		meta := flattenMetadata(r.Metadata)
		var capsule Capsule
		if _, direct := wantKind[CapsuleKind(meta["kind"])]; direct {
			capsule = rowToCapsule(r, "")
		} else if wantCycle && cycleLikeRow(meta) {
			capsule = rowToCapsule(r, KindCycle)
			capsule.Kind = KindCycle
		} else {
			continue
		}
		if opts.Machine != "" && capsule.Machine != opts.Machine {
			continue
		}
		all = append(all, capsule)
	}

	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		if all[i].Day != all[j].Day {
			return all[i].Day > all[j].Day
		}
		return all[i].ID > all[j].ID
	})

	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (c *Client) listAll(ctx context.Context) ([]mem0Row, error) {
	payload := map[string]any{
		"filters": map[string]any{
			"AND": []map[string]string{
				{"app_id": c.AppID},
				{"user_id": c.UserID},
			},
		},
	}
	body, err := c.post(ctx, "/v2/memories/", payload)
	if err != nil {
		return nil, err
	}
	return parseListRows(body)
}

func (c *Client) post(ctx context.Context, path string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+c.APIKey)

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read body: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mem0 api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

type mem0Row struct {
	ID        string         `json:"id"`
	Memory    string         `json:"memory"`
	UserID    string         `json:"user_id"`
	AppID     string         `json:"app_id"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Metadata  map[string]any `json:"metadata"`
}

type mem0ListResult struct {
	Results []mem0Row `json:"results"`
}

func parseListRows(body []byte) ([]mem0Row, error) {
	if len(body) == 0 {
		return nil, nil
	}
	var wrapped mem0ListResult
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Results != nil {
		return wrapped.Results, nil
	}
	var flat []mem0Row
	if err := json.Unmarshal(body, &flat); err != nil {
		return nil, errors.New("evoloop: cannot parse Mem0 list response")
	}
	return flat, nil
}

func rowToCapsule(r mem0Row, fallbackKind CapsuleKind) Capsule {
	meta := flattenMetadata(r.Metadata)
	kind := CapsuleKind(meta["kind"])
	if kind == "" {
		kind = fallbackKind
	}
	if fallbackKind != "" {
		kind = fallbackKind
	}
	c := Capsule{
		ID:       r.ID,
		Kind:     kind,
		Text:     r.Memory,
		Machine:  meta["machine"],
		Day:      meta["day"],
		CycleID:  meta["cycle_id"],
		Source:   meta["source"],
		Event:    meta["event"],
		Metadata: meta,
	}
	if ts := strings.TrimSpace(r.CreatedAt); ts != "" {
		if t, err := parseMem0Time(ts); err == nil {
			c.CreatedAt = t
		}
	}
	c.Cycles = atoi(meta["cycles"])
	c.Improved = atoi(meta["improved"])
	c.RolledBack = atoi(meta["rolled_back"])
	c.Incomplete = atoi(meta["incomplete"])
	c.NoChange = atoi(meta["no_change"])
	c.MeanDelta = atof(meta["mean_delta"])
	c.P50Delta = atof(meta["p50_delta"])
	c.P95Delta = atof(meta["p95_delta"])
	c.LastKPI = atof(meta["last_kpi"])
	c.KPIBefore = atof(meta["kpi_before"])
	c.KPIAfter = atof(meta["kpi_after"])
	c.KPIDelta = atof(meta["kpi_delta"])
	c.DurationMS = atoi64(meta["duration_ms"])
	applyLegacyCycleFallbacks(&c)
	return c
}

var (
	legacyKPIPattern      = regexp.MustCompile(`KPI from ([0-9]+(?:\.[0-9]+)?) to ([0-9]+(?:\.[0-9]+)?)`)
	legacyDurationPattern = regexp.MustCompile(`duration of ([0-9]+(?:\.[0-9]+)?) ms`)
)

func applyLegacyCycleFallbacks(c *Capsule) {
	if c == nil || c.Kind != KindCycle || c.Source != "evoloop-daemon" {
		return
	}
	if c.CycleID == "" {
		c.CycleID = c.Metadata["capsule_id"]
	}
	if c.KPIBefore == 0 && c.KPIAfter == 0 {
		if matches := legacyKPIPattern.FindStringSubmatch(c.Text); len(matches) == 3 {
			c.KPIBefore = atof(matches[1])
			c.KPIAfter = atof(matches[2])
		}
	}
	if c.DurationMS == 0 {
		if matches := legacyDurationPattern.FindStringSubmatch(c.Text); len(matches) == 2 {
			c.DurationMS = int64(atof(matches[1]))
		}
	}
}

// cycleLikeRow reports whether a flattened metadata bag describes a row
// that is semantically a feedbackloop cycle even though its raw
// metadata.kind is not "evoloop_cycle".
//
// Two shapes are recognised:
//
//  1. agent_outcome capsule emitted by the IronClaw/EvoLoop daemon's
//     per-cycle hook (event has prefix "ironclaw-daemon:cycle:"). This is
//     the canonical shape after v253 d5 because it captures gated cycles
//     too -- the canonical evoloop_cycle writer is gated and only fires
//     when KPI delta exceeds the threshold.
//  2. Legacy "kind=capsule + source=evoloop-daemon" shape produced by
//     pre-v253 daemon binaries; kept so a partial fleet upgrade does
//     not blind the reader.
//
// Rows whose metadata.kind is already a canonical evoloop kind are
// excluded -- they do not need promotion.
func cycleLikeRow(meta map[string]string) bool {
	switch CapsuleKind(meta["kind"]) {
	case KindCycle, KindRollup:
		return false
	}
	if meta["kind"] == "agent_outcome" &&
		meta["actor"] == "ironclaw-daemon" &&
		strings.HasPrefix(meta["event"], "ironclaw-daemon:cycle:") {
		return true
	}
	if meta["kind"] == "capsule" && meta["source"] == "evoloop-daemon" {
		return true
	}
	return false
}

// logDebug writes a one-line summary of the resolved Mem0 query to
// c.Debug. It is a no-op when c.Debug is nil. Errors writing to the
// debug sink are swallowed: debug must never affect the primary path.
func (c *Client) logDebug(opts RecentOptions, kinds []CapsuleKind, limit int) {
	if c.Debug == nil {
		return
	}
	payload := map[string]any{
		"filters": map[string]any{
			"AND": []map[string]string{
				{"app_id": c.AppID},
				{"user_id": c.UserID},
			},
		},
	}
	body, _ := json.Marshal(payload)
	parts := []string{
		fmt.Sprintf("POST /v2/memories/ %s", c.BaseURL),
		"body=" + string(body),
		fmt.Sprintf("limit=%d", limit),
	}
	if opts.Machine != "" {
		parts = append(parts, "machine="+opts.Machine)
	}
	for _, k := range kinds {
		parts = append(parts, "kind="+string(k))
	}
	_, _ = fmt.Fprintln(c.Debug, strings.Join(parts, " "))
}

func flattenMetadata(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = metaString(v)
	}
	return out
}

func metaString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// The Mem0 API only returns numbers as JSON numbers for a handful
		// of values; format them back to a stable short form.
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return x.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func atoi64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func atof(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// parseMem0Time accepts both RFC3339 and the fractional variant Mem0 actually
// returns (e.g. "2026-04-23T07:59:53.123456").
func parseMem0Time(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("evoloop: unsupported Mem0 timestamp %q", s)
}
