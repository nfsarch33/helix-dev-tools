package zdproxy

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// latencyBuckets is the fixed Prometheus histogram bucket schedule used for
// upstream-call latency. The default exponential schedule is intentionally
// chosen to span "fast" (paint-the-screen) up through "slow" (heavy Opus
// thinking with tools). Buckets are inclusive upper bounds in seconds.
var latencyBuckets = []float64{
	0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
}

// Metrics is a tiny Prometheus-text-exposition emitter dedicated to
// zd-claude-proxy. It does not depend on the official Go client to keep
// internal/zdproxy free of external deps (the package is a security-first
// loopback shim). All series are addressed by stable label tuples; updates
// are race-safe.
//
// Tier-A subagent offload telemetry shares the same exposition target
// (127.0.0.1:9787/metrics) under the prefix
// `zd_claude_proxy_tier_a_offload_*`. The plan keeps a single scrape so
// the gateway boundary remains MacBook-loopback only.
type Metrics struct {
	mu sync.RWMutex

	requestsTotal map[reqKey]uint64    // {route, model, upstream_status} → count
	tokensTotal   map[tokKey]uint64    // {model, direction}              → count
	inflight      map[string]int64     // route                           → in-flight count
	latency       map[latencyKey]*hist // {route, model}                  → histogram

	tierAOffloadTotal   map[tierAReqKey]uint64    // {tier, decision, route} → count
	tierAOffloadLatency map[tierALatencyKey]*hist // {tier, route}           → histogram
}

type reqKey struct {
	route          string
	model          string
	upstreamStatus int
}

type tokKey struct {
	model     string
	direction string
}

type latencyKey struct {
	route string
	model string
}

type tierAReqKey struct {
	tier     string
	decision string
	route    string
}

type tierALatencyKey struct {
	tier  string
	route string
}

type hist struct {
	counts []uint64 // counts[i] is the cumulative count at latencyBuckets[i]
	count  uint64
	sum    float64
}

// NewMetrics returns a Metrics instance with empty series. The instance is
// safe for concurrent use.
func NewMetrics() *Metrics {
	return &Metrics{
		requestsTotal:       make(map[reqKey]uint64),
		tokensTotal:         make(map[tokKey]uint64),
		inflight:            make(map[string]int64),
		latency:             make(map[latencyKey]*hist),
		tierAOffloadTotal:   make(map[tierAReqKey]uint64),
		tierAOffloadLatency: make(map[tierALatencyKey]*hist),
	}
}

// RecordRequest tallies one upstream-bound request. route is one of:
// `openai_chat`, `openai_responses`, `bedrock_invoke`,
// `bedrock_invoke_stream`, `bedrock_passthrough`. model is the upstream
// model id; upstreamStatus is the HTTP status code returned by the
// gateway; latency is the wall-clock duration from inbound request start
// to upstream response close.
func (m *Metrics) RecordRequest(route, model string, upstreamStatus int, latency time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rk := reqKey{route: route, model: model, upstreamStatus: upstreamStatus}
	m.requestsTotal[rk]++

	lk := latencyKey{route: route, model: model}
	h, ok := m.latency[lk]
	if !ok {
		h = &hist{counts: make([]uint64, len(latencyBuckets))}
		m.latency[lk] = h
	}
	h.count++
	secs := latency.Seconds()
	h.sum += secs
	for i, b := range latencyBuckets {
		if secs <= b {
			h.counts[i]++
		}
	}
}

// RecordTokens increments the token counter for the given model and
// direction (`input` or `output`). The proxy only emits this when the
// upstream response carried a credible token count (header on Bedrock,
// body on OpenAI non-streaming). Streaming token totals are an explicit
// follow-up.
func (m *Metrics) RecordTokens(model, direction string, n int) {
	if m == nil || n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokensTotal[tokKey{model: model, direction: direction}] += uint64(n)
}

// BeginInflight increments the gauge for the given route. Callers must
// pair with EndInflight in a defer. Used by the request middleware so a
// scrape mid-flight can show how many requests are pending upstream.
func (m *Metrics) BeginInflight(route string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inflight[route]++
}

// EndInflight decrements the gauge for the given route.
func (m *Metrics) EndInflight(route string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inflight[route] > 0 {
		m.inflight[route]--
	}
}

// RecordTierAOffload tallies one tier-A subagent offload decision. The
// label set is intentionally bounded:
//
//   - tier: stable enum string (e.g. "a", "b", "c"). Empty input is
//     dropped to keep cardinality bounded.
//   - decision: "offloaded" | "kept_local" | "declined". Empty input is
//     dropped.
//   - route: stable identifier for the consumer route (e.g.
//     "claude_code_subagent", "codex_subagent", "router_qwen36_27b").
//     Empty input is dropped.
//
// latency may be zero for "declined" decisions; the histogram still
// records the sample so we can compute decline-rate from the count
// series.
func (m *Metrics) RecordTierAOffload(tier, decision, route string, latency time.Duration) {
	if m == nil {
		return
	}
	if tier == "" || decision == "" || route == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	rk := tierAReqKey{tier: tier, decision: decision, route: route}
	m.tierAOffloadTotal[rk]++

	lk := tierALatencyKey{tier: tier, route: route}
	h, ok := m.tierAOffloadLatency[lk]
	if !ok {
		h = &hist{counts: make([]uint64, len(latencyBuckets))}
		m.tierAOffloadLatency[lk] = h
	}
	h.count++
	secs := latency.Seconds()
	if secs < 0 {
		secs = 0
	}
	h.sum += secs
	for i, b := range latencyBuckets {
		if secs <= b {
			h.counts[i]++
		}
	}
}

// ServeHTTP implements the Prometheus text exposition format. The output
// follows the 0.0.4 spec: HELP/TYPE per metric family, then series sorted
// by label tuple for deterministic diffability.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# HELP zd_claude_proxy_requests_total Total upstream-bound requests grouped by route, model and HTTP status.\n")
	sb.WriteString("# TYPE zd_claude_proxy_requests_total counter\n")
	reqKeys := make([]reqKey, 0, len(m.requestsTotal))
	for k := range m.requestsTotal {
		reqKeys = append(reqKeys, k)
	}
	sort.Slice(reqKeys, func(i, j int) bool {
		if reqKeys[i].route != reqKeys[j].route {
			return reqKeys[i].route < reqKeys[j].route
		}
		if reqKeys[i].model != reqKeys[j].model {
			return reqKeys[i].model < reqKeys[j].model
		}
		return reqKeys[i].upstreamStatus < reqKeys[j].upstreamStatus
	})
	for _, k := range reqKeys {
		sb.WriteString("zd_claude_proxy_requests_total{")
		sb.WriteString(`route="`)
		sb.WriteString(escapeLabel(k.route))
		sb.WriteString(`",model="`)
		sb.WriteString(escapeLabel(k.model))
		sb.WriteString(`",upstream_status="`)
		sb.WriteString(strconv.Itoa(k.upstreamStatus))
		sb.WriteString(`"} `)
		sb.WriteString(strconv.FormatUint(m.requestsTotal[k], 10))
		sb.WriteByte('\n')
	}

	sb.WriteString("# HELP zd_claude_proxy_latency_seconds Upstream call latency in seconds, by route and model.\n")
	sb.WriteString("# TYPE zd_claude_proxy_latency_seconds histogram\n")
	latKeys := make([]latencyKey, 0, len(m.latency))
	for k := range m.latency {
		latKeys = append(latKeys, k)
	}
	sort.Slice(latKeys, func(i, j int) bool {
		if latKeys[i].route != latKeys[j].route {
			return latKeys[i].route < latKeys[j].route
		}
		return latKeys[i].model < latKeys[j].model
	})
	for _, k := range latKeys {
		h := m.latency[k]
		labelPrefix := `route="` + escapeLabel(k.route) + `",model="` + escapeLabel(k.model) + `"`
		for i, b := range latencyBuckets {
			sb.WriteString("zd_claude_proxy_latency_seconds_bucket{")
			sb.WriteString(labelPrefix)
			sb.WriteString(`,le="`)
			sb.WriteString(strconv.FormatFloat(b, 'f', -1, 64))
			sb.WriteString(`"} `)
			sb.WriteString(strconv.FormatUint(h.counts[i], 10))
			sb.WriteByte('\n')
		}
		sb.WriteString("zd_claude_proxy_latency_seconds_bucket{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`,le="+Inf"} `)
		sb.WriteString(strconv.FormatUint(h.count, 10))
		sb.WriteByte('\n')
		sb.WriteString("zd_claude_proxy_latency_seconds_sum{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`} `)
		sb.WriteString(strconv.FormatFloat(h.sum, 'f', -1, 64))
		sb.WriteByte('\n')
		sb.WriteString("zd_claude_proxy_latency_seconds_count{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`} `)
		sb.WriteString(strconv.FormatUint(h.count, 10))
		sb.WriteByte('\n')
	}

	sb.WriteString("# HELP zd_claude_proxy_tokens_total Total upstream tokens by model and direction (input|output).\n")
	sb.WriteString("# TYPE zd_claude_proxy_tokens_total counter\n")
	tokKeys := make([]tokKey, 0, len(m.tokensTotal))
	for k := range m.tokensTotal {
		tokKeys = append(tokKeys, k)
	}
	sort.Slice(tokKeys, func(i, j int) bool {
		if tokKeys[i].model != tokKeys[j].model {
			return tokKeys[i].model < tokKeys[j].model
		}
		return tokKeys[i].direction < tokKeys[j].direction
	})
	for _, k := range tokKeys {
		sb.WriteString("zd_claude_proxy_tokens_total{")
		sb.WriteString(`model="`)
		sb.WriteString(escapeLabel(k.model))
		sb.WriteString(`",direction="`)
		sb.WriteString(escapeLabel(k.direction))
		sb.WriteString(`"} `)
		sb.WriteString(strconv.FormatUint(m.tokensTotal[k], 10))
		sb.WriteByte('\n')
	}

	sb.WriteString("# HELP zd_claude_proxy_inflight Currently in-flight upstream calls by route.\n")
	sb.WriteString("# TYPE zd_claude_proxy_inflight gauge\n")
	infKeys := make([]string, 0, len(m.inflight))
	for k := range m.inflight {
		infKeys = append(infKeys, k)
	}
	sort.Strings(infKeys)
	for _, route := range infKeys {
		sb.WriteString("zd_claude_proxy_inflight{")
		sb.WriteString(`route="`)
		sb.WriteString(escapeLabel(route))
		sb.WriteString(`"} `)
		sb.WriteString(strconv.FormatInt(m.inflight[route], 10))
		sb.WriteByte('\n')
	}

	sb.WriteString("# HELP zd_claude_proxy_tier_a_offload_total Total tier-A subagent offload decisions grouped by tier, decision and route.\n")
	sb.WriteString("# TYPE zd_claude_proxy_tier_a_offload_total counter\n")
	taReqKeys := make([]tierAReqKey, 0, len(m.tierAOffloadTotal))
	for k := range m.tierAOffloadTotal {
		taReqKeys = append(taReqKeys, k)
	}
	sort.Slice(taReqKeys, func(i, j int) bool {
		if taReqKeys[i].tier != taReqKeys[j].tier {
			return taReqKeys[i].tier < taReqKeys[j].tier
		}
		if taReqKeys[i].decision != taReqKeys[j].decision {
			return taReqKeys[i].decision < taReqKeys[j].decision
		}
		return taReqKeys[i].route < taReqKeys[j].route
	})
	for _, k := range taReqKeys {
		sb.WriteString("zd_claude_proxy_tier_a_offload_total{")
		sb.WriteString(`tier="`)
		sb.WriteString(escapeLabel(k.tier))
		sb.WriteString(`",decision="`)
		sb.WriteString(escapeLabel(k.decision))
		sb.WriteString(`",route="`)
		sb.WriteString(escapeLabel(k.route))
		sb.WriteString(`"} `)
		sb.WriteString(strconv.FormatUint(m.tierAOffloadTotal[k], 10))
		sb.WriteByte('\n')
	}

	sb.WriteString("# HELP zd_claude_proxy_tier_a_offload_latency_seconds Tier-A subagent offload wall-clock latency by tier and route.\n")
	sb.WriteString("# TYPE zd_claude_proxy_tier_a_offload_latency_seconds histogram\n")
	taLatKeys := make([]tierALatencyKey, 0, len(m.tierAOffloadLatency))
	for k := range m.tierAOffloadLatency {
		taLatKeys = append(taLatKeys, k)
	}
	sort.Slice(taLatKeys, func(i, j int) bool {
		if taLatKeys[i].tier != taLatKeys[j].tier {
			return taLatKeys[i].tier < taLatKeys[j].tier
		}
		return taLatKeys[i].route < taLatKeys[j].route
	})
	for _, k := range taLatKeys {
		h := m.tierAOffloadLatency[k]
		labelPrefix := `tier="` + escapeLabel(k.tier) + `",route="` + escapeLabel(k.route) + `"`
		for i, b := range latencyBuckets {
			sb.WriteString("zd_claude_proxy_tier_a_offload_latency_seconds_bucket{")
			sb.WriteString(labelPrefix)
			sb.WriteString(`,le="`)
			sb.WriteString(strconv.FormatFloat(b, 'f', -1, 64))
			sb.WriteString(`"} `)
			sb.WriteString(strconv.FormatUint(h.counts[i], 10))
			sb.WriteByte('\n')
		}
		sb.WriteString("zd_claude_proxy_tier_a_offload_latency_seconds_bucket{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`,le="+Inf"} `)
		sb.WriteString(strconv.FormatUint(h.count, 10))
		sb.WriteByte('\n')
		sb.WriteString("zd_claude_proxy_tier_a_offload_latency_seconds_sum{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`} `)
		sb.WriteString(strconv.FormatFloat(h.sum, 'f', -1, 64))
		sb.WriteByte('\n')
		sb.WriteString("zd_claude_proxy_tier_a_offload_latency_seconds_count{")
		sb.WriteString(labelPrefix)
		sb.WriteString(`} `)
		sb.WriteString(strconv.FormatUint(h.count, 10))
		sb.WriteByte('\n')
	}

	_, _ = w.Write([]byte(sb.String()))
}

// escapeLabel applies the Prometheus exposition format escape rules for
// label values: backslash → \\, double-quote → \", newline → \n. The
// proxy never logs label values directly so this only protects against a
// pathological model id breaking the format.
func escapeLabel(v string) string {
	if !strings.ContainsAny(v, "\\\"\n") {
		return v
	}
	var sb strings.Builder
	sb.Grow(len(v) + 8)
	for _, r := range v {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
