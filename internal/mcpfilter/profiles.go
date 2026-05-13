package mcpfilter

var BuiltinProfiles = map[string]ProfileDef{
	"research": {
		Name:        "research",
		Description: "Academic research, web search, PDF processing, scholarly sources",
		Include: []string{
			"user-exa",
			"user-tavily-mcp",
			"user-perplexity-ask",
			"user-duckduckgo",
			"user-google-scholar",
			"user-fetch",
			"user-pdf-handler",
			"user-context7",
			"user-mem0",
			"user-context-mode",
			"user-time",
		},
	},
	"code-review": {
		Name:        "code-review",
		Description: "Code review, git operations, quality gates",
		Include: []string{
			"user-git-mcp-server",
			"user-github-official",
			"user-sentrux",
			"user-context-mode",
			"user-mem0",
			"user-time",
		},
	},
	"deployment": {
		Name:        "deployment",
		Description: "Infrastructure, CI/CD, container operations",
		Include: []string{
			"user-git-mcp-server",
			"user-github-official",
			"user-context-mode",
			"user-time",
			"cursor-app-control",
		},
	},
	"debug": {
		Name:        "debug",
		Description: "Debugging, browser testing, dev tools",
		Include: []string{
			"cursor-ide-browser",
			"user-playwright",
			"user-chrome-devtools",
			"user-git-mcp-server",
			"user-sentrux",
			"user-context-mode",
			"user-mem0",
			"user-time",
		},
	},
	"writing": {
		Name:        "writing",
		Description: "Content creation, documentation, word processing",
		Include: []string{
			"user-word-document-server",
			"user-pdf-handler",
			"user-mermaid",
			"user-plantuml",
			"user-mem0",
			"user-context-mode",
			"user-fetch",
			"user-time",
		},
	},
	"job-hunt": {
		Name:        "job-hunt",
		Description: "LinkedIn, Upwork, Seek job search and application workflows",
		Include: []string{
			"user-linkedin-mcp",
			"user-upwork-mcp",
			"user-mem0",
			"user-context-mode",
			"user-fetch",
			"user-pdf-handler",
			"user-word-document-server",
			"user-time",
		},
	},
	"minimal": {
		Name:        "minimal",
		Description: "Absolute minimum for basic operations",
		Include: []string{
			"user-mem0",
			"user-context-mode",
			"user-time",
			"cursor-app-control",
		},
	},
}

func ListProfiles() []ProfileDef {
	profiles := make([]ProfileDef, 0, len(BuiltinProfiles))
	for _, p := range BuiltinProfiles {
		profiles = append(profiles, p)
	}
	return profiles
}

func GetProfile(name string) (ProfileDef, bool) {
	p, ok := BuiltinProfiles[name]
	return p, ok
}
