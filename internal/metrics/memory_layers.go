package metrics

import (
	"path/filepath"
	"strings"
)

const (
	MemoryLayerMem0        = "mem0"
	MemoryLayerContextMode = "context_mode"
	MemoryLayerGitKB       = "git_kb"
	MemoryLayerAllPepper   = "allpepper"
)

const (
	MemoryOpSearch = "search"
	MemoryOpRead   = "read"
	MemoryOpWrite  = "write"
	MemoryOpUpdate = "update"
)

const (
	MemoryResultHit   = "hit"
	MemoryResultMiss  = "miss"
	MemoryResultEmpty = "empty"
	MemoryResultWrite = "write"
)

var ValidMemoryLayers = []string{
	MemoryLayerMem0,
	MemoryLayerContextMode,
	MemoryLayerGitKB,
	MemoryLayerAllPepper,
}

var ValidMemoryOps = []string{
	MemoryOpSearch,
	MemoryOpRead,
	MemoryOpWrite,
	MemoryOpUpdate,
}

var ValidMemoryResults = []string{
	MemoryResultHit,
	MemoryResultMiss,
	MemoryResultEmpty,
	MemoryResultWrite,
}

func IsValidMemoryLayer(layer string) bool {
	for _, v := range ValidMemoryLayers {
		if layer == v {
			return true
		}
	}
	return false
}

func IsValidMemoryOp(op string) bool {
	for _, v := range ValidMemoryOps {
		if op == v {
			return true
		}
	}
	return false
}

func IsValidMemoryResult(result string) bool {
	for _, v := range ValidMemoryResults {
		if result == v {
			return true
		}
	}
	return false
}

// InferMemoryContextFromMCPDetail classifies memory-layer MCP usage from a
// detail string such as "mem0:search_memories" or a raw tool name.
func InferMemoryContextFromMCPDetail(detail string) (layer, op string) {
	key := EnrichToolDetail(detail)
	idx := strings.Index(key, ":")
	if idx <= 0 {
		return "", ""
	}
	server := CanonicalMCPServerName(key[:idx])
	tool := key[idx+1:]

	switch server {
	case "mem0":
		return MemoryLayerMem0, classifyMem0Tool(tool)
	case "context-mode":
		return MemoryLayerContextMode, classifyContextModeTool(tool)
	case "allPepper-memory-bank":
		return MemoryLayerAllPepper, classifyAllPepperTool(tool)
	default:
		return "", ""
	}
}

func classifyMem0Tool(tool string) string {
	switch tool {
	case "search_memories", "get_memories", "get_memory", "list_entities":
		return MemoryOpSearch
	case "add_memory":
		return MemoryOpWrite
	case "update_memory", "delete_memory", "delete_all_memories", "delete_entities":
		return MemoryOpUpdate
	default:
		return ""
	}
}

func classifyContextModeTool(tool string) string {
	switch tool {
	case "ctx_search":
		return MemoryOpSearch
	case "ctx_execute", "ctx_batch_execute", "ctx_execute_file", "ctx_stats", "ctx_doctor":
		return MemoryOpRead
	case "ctx_index", "ctx_fetch_and_index", "ctx_upgrade":
		return MemoryOpUpdate
	default:
		return ""
	}
}

func classifyAllPepperTool(tool string) string {
	switch tool {
	case "memory_bank_read", "list_projects", "list_project_files":
		return MemoryOpSearch
	case "memory_bank_write":
		return MemoryOpWrite
	case "memory_bank_update":
		return MemoryOpUpdate
	default:
		return ""
	}
}

// InferMemoryContextFromReadPath identifies Git-backed KB reads that count as
// durable memory-layer access instead of ordinary code reads.
//
// Cursor's beforeReadFile hook only fires for files the IDE is about to read,
// which means the file exists on disk. We therefore record the result as "hit"
// for allowed reads: the data is present and will be consumed.
func InferMemoryContextFromReadPath(path string) (layer, op, result string) {
	if path == "" {
		return "", "", ""
	}

	normalised := strings.ToLower(filepath.ToSlash(path))
	if normalised == "" {
		return "", "", ""
	}

	markers := []string{
		"/memo/global-memories/",
		"/code/global-kb/global-memories/",
		"/code/global-kb/learnings/",
		"/code/global-kb/sop/",
		"/code/global-kb/architecture/",
		"/code/global-kb/engineering/",
		"/code/global-kb/investigations/",
		"/code/global-kb/incidents/",
		"/code/global-kb/docs/adr/",
	}
	for _, marker := range markers {
		if strings.Contains(normalised, marker) {
			return MemoryLayerGitKB, MemoryOpRead, MemoryResultHit
		}
	}

	if strings.HasSuffix(normalised, "/code/global-kb/readme.md") {
		return MemoryLayerGitKB, MemoryOpRead, MemoryResultHit
	}
	return "", "", ""
}
