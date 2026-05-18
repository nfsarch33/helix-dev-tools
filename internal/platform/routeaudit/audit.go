package routeaudit

// Route describes an LLM inference route
type Route struct {
	Name    string
	Backend string // e.g. "local-vllm", "minimax", "openai", "anthropic"
	Active  bool
}

// AuditResult summarises route health
type AuditResult struct {
	TotalRoutes   int
	LocalRoutes   int
	ExternalRoutes int
	RetiredRoutes []string
	LocalOnlyOK   bool // true if all active routes are local
}

// Audit inspects routes and returns a report
func Audit(routes []Route) AuditResult {
	res := AuditResult{}
	for _, r := range routes {
		res.TotalRoutes++
		if !r.Active {
			res.RetiredRoutes = append(res.RetiredRoutes, r.Name)
			continue
		}
		if r.Backend == "local-vllm" || r.Backend == "local" {
			res.LocalRoutes++
		} else {
			res.ExternalRoutes++
		}
	}
	res.LocalOnlyOK = res.ExternalRoutes == 0 && res.LocalRoutes > 0
	return res
}

// RetiredNames returns the names of inactive routes
func RetiredNames(routes []Route) []string {
	var names []string
	for _, r := range routes {
		if !r.Active {
			names = append(names, r.Name)
		}
	}
	return names
}
