package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/mcptelemetry"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func resolveAgentID() string {
	if id := os.Getenv("CURSOR_AGENT_ID"); id != "" {
		return id
	}
	if os.Getenv("CODEX_SESSION") != "" {
		return "codex"
	}
	if os.Getenv("CLAUDE_CODE") != "" {
		return "claude-code"
	}
	return "cursor-parent"
}

func main() {
	store, err := sprintboard.Open(sprintboard.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	telemetry, err := mcptelemetry.New(mcptelemetry.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry init (non-fatal): %v\n", err)
		telemetry, _ = mcptelemetry.New(mcptelemetry.Config{Enabled: false})
	}
	defer telemetry.Close()

	embedder := sprintboard.NewEmbedder(sprintboard.DefaultEmbedderConfig())

	server := &Server{store: store, agentID: resolveAgentID(), telemetry: telemetry, embedder: embedder}
	server.serve(os.Stdin, os.Stdout)
}

type Server struct {
	store     *sprintboard.Store
	agentID   string
	telemetry *mcptelemetry.Recorder
	embedder  *sprintboard.Embedder
}

func (s *Server) serve(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		if req.ID == nil {
			continue
		}

		resp := s.handleRequest(req)
		data, _ := json.Marshal(resp)
		fmt.Fprintf(out, "%s\n", data)
	}
}

func (s *Server) handleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "sprintboard-mcp", "version": "1.0.0"},
		},
	}
}

func (s *Server) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	tools := []ToolDefinition{
		{Name: "sprint_create", Description: "Create a new sprint", InputSchema: sprintCreateSchema()},
		{Name: "sprint_list", Description: "List all sprints, optionally filtered by status", InputSchema: sprintListSchema()},
		{Name: "sprint_status", Description: "Get sprint summary with ticket counts by status", InputSchema: idOnlySchema("sprint_id")},
		{Name: "sprint_close", Description: "Close a sprint", InputSchema: idOnlySchema("sprint_id")},
		{Name: "ticket_create", Description: "Create a ticket in a sprint", InputSchema: ticketCreateSchema()},
		{Name: "ticket_list", Description: "List tickets filtered by sprint, status, or owner", InputSchema: ticketListSchema()},
		{Name: "ticket_update", Description: "Update ticket status with transition tracking", InputSchema: ticketUpdateSchema()},
		{Name: "ticket_assign", Description: "Assign a ticket to an agent", InputSchema: ticketAssignSchema()},
		{Name: "handoff_create", Description: "Create a handoff record for a ticket", InputSchema: handoffCreateSchema()},
		{Name: "handoff_list", Description: "List handoffs for a ticket", InputSchema: idOnlySchema("ticket_id")},
		{Name: "ticket_search", Description: "Semantic search across tickets by natural language query", InputSchema: searchSchema()},
		{Name: "sprint_search", Description: "Semantic search across sprints by theme or description", InputSchema: searchSchema()},
		{Name: "agent_register", Description: "Register an agent with its capabilities (auto-expires after 30min without heartbeat)", InputSchema: agentRegisterSchema()},
		{Name: "agent_heartbeat", Description: "Send heartbeat to keep agent registration active", InputSchema: agentHeartbeatSchema()},
		{Name: "task_claim", Description: "Atomically claim a ticket (prevents double-assignment)", InputSchema: taskClaimSchema()},
		{Name: "task_complete", Description: "Mark a claimed ticket as done with evidence", InputSchema: taskCompleteSchema()},
		{Name: "handoff_publish", Description: "Publish cross-agent handoff (also bridges to Mem0 cursor-coordination)", InputSchema: handoffPublishSchema()},
		{Name: "handoff_subscribe", Description: "Check for handoffs addressed to this agent", InputSchema: handoffSubscribeSchema()},
		{Name: "task_recommend", Description: "Recommend next tasks for an agent based on capabilities and sprint backlog", InputSchema: taskRecommendSchema()},
		{Name: "sprint_distribute", Description: "Auto-assign all sprint tickets to agents based on capabilities and load", InputSchema: sprintDistributeSchema()},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"tools": tools}}
}

func (s *Server) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResp(req.ID, -32602, "invalid params")
	}

	result, isErr := s.dispatch(params.Name, params.Arguments)
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolResult{Content: []ContentBlock{{Type: "text", Text: result}}, IsError: isErr},
	}
}

func (s *Server) dispatch(tool string, args json.RawMessage) (string, bool) {
	start := time.Now()
	result, isErr := s.dispatchInner(tool, args)
	errMsg := ""
	if isErr {
		errMsg = result
	}
	s.telemetry.Record(tool, s.agentID, time.Since(start), isErr, errMsg)
	return result, isErr
}

func (s *Server) dispatchInner(tool string, args json.RawMessage) (string, bool) {
	switch tool {
	case "sprint_create":
		return s.sprintCreate(args)
	case "sprint_list":
		return s.sprintList(args)
	case "sprint_status":
		return s.sprintStatus(args)
	case "sprint_close":
		return s.sprintClose(args)
	case "ticket_create":
		return s.ticketCreate(args)
	case "ticket_list":
		return s.ticketList(args)
	case "ticket_update":
		return s.ticketUpdate(args)
	case "ticket_assign":
		return s.ticketAssign(args)
	case "handoff_create":
		return s.handoffCreate(args)
	case "handoff_list":
		return s.handoffList(args)
	case "ticket_search":
		return s.ticketSearch(args)
	case "sprint_search":
		return s.sprintSearch(args)
	case "agent_register":
		return s.agentRegister(args)
	case "agent_heartbeat":
		return s.agentHeartbeat(args)
	case "task_claim":
		return s.taskClaim(args)
	case "task_complete":
		return s.taskComplete(args)
	case "handoff_publish":
		return s.handoffPublish(args)
	case "handoff_subscribe":
		return s.handoffSubscribe(args)
	case "task_recommend":
		return s.taskRecommend(args)
	case "sprint_distribute":
		return s.sprintDistribute(args)
	default:
		return fmt.Sprintf("unknown tool: %s", tool), true
	}
}

func (s *Server) sprintCreate(args json.RawMessage) (string, bool) {
	var p struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	err := s.store.CreateSprint(sprintboard.Sprint{
		ID: p.ID, Name: p.Name, Theme: p.Theme,
		OwnerAgent: s.agentID, Status: sprintboard.SprintPlanned,
	})
	if err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Sprint %q created (owner: %s)", p.ID, s.agentID), false
}

func (s *Server) sprintList(args json.RawMessage) (string, bool) {
	sprints, err := s.store.ListSprints()
	if err != nil {
		return err.Error(), true
	}
	data, _ := json.MarshalIndent(sprints, "", "  ")
	return string(data), false
}

func (s *Server) sprintStatus(args json.RawMessage) (string, bool) {
	var p struct {
		SprintID string `json:"sprint_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	summary, err := s.store.SprintSummary(p.SprintID)
	if err != nil {
		return err.Error(), true
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	return string(data), false
}

func (s *Server) sprintClose(args json.RawMessage) (string, bool) {
	var p struct {
		SprintID string `json:"sprint_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if err := s.store.UpdateSprint(p.SprintID, sprintboard.SprintClosed); err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Sprint %q closed", p.SprintID), false
}

func (s *Server) ticketCreate(args json.RawMessage) (string, bool) {
	var p struct {
		ID                 string `json:"id"`
		SprintID           string `json:"sprint_id"`
		Title              string `json:"title"`
		Description        string `json:"description"`
		Priority           int    `json:"priority"`
		AcceptanceCriteria string `json:"acceptance_criteria"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	err := s.store.CreateTicket(sprintboard.Ticket{
		ID: p.ID, SprintID: p.SprintID, Title: p.Title,
		Description: p.Description, Priority: p.Priority,
		AcceptanceCriteria: p.AcceptanceCriteria, OwnerAgent: s.agentID,
	})
	if err != nil {
		return err.Error(), true
	}

	go func() {
		text := p.Title + " " + p.Description
		if vec, err := s.embedder.Embed(text); err == nil {
			s.store.StoreEmbedding("ticket", p.ID, vec)
		}
	}()

	return fmt.Sprintf("Ticket %q created in sprint %q", p.ID, p.SprintID), false
}

func (s *Server) ticketList(args json.RawMessage) (string, bool) {
	var p struct {
		SprintID string `json:"sprint_id"`
		Status   string `json:"status"`
		Owner    string `json:"owner"`
	}
	json.Unmarshal(args, &p)

	tickets, err := s.store.ListTickets(p.SprintID)
	if err != nil {
		return err.Error(), true
	}

	if p.Status != "" || p.Owner != "" {
		var filtered []sprintboard.Ticket
		for _, t := range tickets {
			if p.Status != "" && string(t.Status) != p.Status {
				continue
			}
			if p.Owner != "" && t.OwnerAgent != p.Owner {
				continue
			}
			filtered = append(filtered, t)
		}
		tickets = filtered
	}

	data, _ := json.MarshalIndent(tickets, "", "  ")
	return string(data), false
}

func (s *Server) ticketUpdate(args json.RawMessage) (string, bool) {
	var p struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	err := s.store.UpdateTicket(p.ID, sprintboard.TicketStatus(p.Status), s.agentID, p.Note)
	if err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Ticket %q -> %s (by %s)", p.ID, p.Status, s.agentID), false
}

func (s *Server) ticketAssign(args json.RawMessage) (string, bool) {
	var p struct {
		ID    string `json:"id"`
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if err := s.store.AssignTicket(p.ID, p.Agent); err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Ticket %q assigned to %s", p.ID, p.Agent), false
}

func (s *Server) handoffCreate(args json.RawMessage) (string, bool) {
	var p struct {
		TicketID    string `json:"ticket_id"`
		ToAgent     string `json:"to_agent"`
		ContextPath string `json:"context_path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	err := s.store.CreateHandoff(sprintboard.Handoff{
		TicketID:    p.TicketID,
		FromAgent:   s.agentID,
		ToAgent:     p.ToAgent,
		ContextPath: p.ContextPath,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Handoff created: %s -> %s for ticket %q", s.agentID, p.ToAgent, p.TicketID), false
}

func (s *Server) handoffList(args json.RawMessage) (string, bool) {
	var p struct {
		TicketID string `json:"ticket_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	handoffs, err := s.store.ListHandoffs(p.TicketID)
	if err != nil {
		return err.Error(), true
	}
	data, _ := json.MarshalIndent(handoffs, "", "  ")
	return string(data), false
}

func errorResp(id interface{}, code int, msg string) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}

func sprintCreateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":    map[string]string{"type": "string", "description": "Unique sprint ID (e.g. v6090)"},
			"name":  map[string]string{"type": "string", "description": "Sprint name"},
			"theme": map[string]string{"type": "string", "description": "Sprint theme"},
		},
		"required": []string{"id", "name"},
	}
}

func sprintListSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]string{"type": "string", "description": "Filter by status: planned, active, closed"},
		},
	}
}

func idOnlySchema(field string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			field: map[string]string{"type": "string", "description": "ID"},
		},
		"required": []string{field},
	}
}

func ticketCreateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":                  map[string]string{"type": "string", "description": "Unique ticket ID"},
			"sprint_id":           map[string]string{"type": "string", "description": "Sprint to add ticket to"},
			"title":               map[string]string{"type": "string", "description": "Ticket title"},
			"description":         map[string]string{"type": "string", "description": "Detailed description"},
			"priority":            map[string]string{"type": "integer", "description": "Priority (0-10, higher is more important)"},
			"acceptance_criteria": map[string]string{"type": "string", "description": "Acceptance criteria"},
		},
		"required": []string{"id", "title"},
	}
}

func ticketListSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sprint_id": map[string]string{"type": "string", "description": "Filter by sprint ID"},
			"status":    map[string]string{"type": "string", "description": "Filter by status"},
			"owner":     map[string]string{"type": "string", "description": "Filter by owner agent"},
		},
	}
}

func ticketUpdateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":     map[string]string{"type": "string", "description": "Ticket ID"},
			"status": map[string]string{"type": "string", "description": "New status: backlog, ready, in_progress, review, done, blocked, ready_for_handoff"},
			"note":   map[string]string{"type": "string", "description": "Transition note"},
		},
		"required": []string{"id", "status"},
	}
}

func ticketAssignSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":    map[string]string{"type": "string", "description": "Ticket ID"},
			"agent": map[string]string{"type": "string", "description": "Agent to assign to (cursor-parent, codex, claude-code)"},
		},
		"required": []string{"id", "agent"},
	}
}

func handoffCreateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticket_id":    map[string]string{"type": "string", "description": "Ticket ID to create handoff for"},
			"to_agent":     map[string]string{"type": "string", "description": "Agent receiving the handoff"},
			"context_path": map[string]string{"type": "string", "description": "Path to handoff document"},
		},
		"required": []string{"ticket_id", "to_agent"},
	}
}

func searchSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]string{"type": "string", "description": "Natural language search query"},
			"limit": map[string]string{"type": "integer", "description": "Max results (default 5)"},
		},
		"required": []string{"query"},
	}
}

func (s *Server) ticketSearch(args json.RawMessage) (string, bool) {
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.Limit <= 0 {
		p.Limit = 5
	}

	queryVec, err := s.embedder.Embed(p.Query)
	if err != nil {
		return err.Error(), true
	}

	results, err := s.store.SearchSimilar(queryVec, "ticket", p.Limit)
	if err != nil {
		return err.Error(), true
	}

	if len(results) == 0 {
		return "[]", false
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return string(data), false
}

func (s *Server) agentRegister(args json.RawMessage) (string, bool) {
	var p struct {
		AgentID      string `json:"agent_id"`
		Surface      string `json:"surface"`
		Capabilities string `json:"capabilities"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	if p.Surface == "" {
		p.Surface = "unknown"
	}
	err := s.store.RegisterAgent(sprintboard.Agent{
		ID: p.AgentID, Surface: p.Surface, Capabilities: p.Capabilities,
	})
	if err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Agent %q registered (surface: %s)", p.AgentID, p.Surface), false
}

func (s *Server) agentHeartbeat(args json.RawMessage) (string, bool) {
	var p struct {
		AgentID         string `json:"agent_id"`
		CurrentTicketID string `json:"current_ticket_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	if err := s.store.AgentHeartbeat(p.AgentID, p.CurrentTicketID); err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Heartbeat from %q", p.AgentID), false
}

func (s *Server) taskClaim(args json.RawMessage) (string, bool) {
	var p struct {
		TicketID string `json:"ticket_id"`
		AgentID  string `json:"agent_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	result, err := s.store.ClaimTicket(p.TicketID, p.AgentID)
	if err != nil {
		return err.Error(), true
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), false
}

func (s *Server) taskComplete(args json.RawMessage) (string, bool) {
	var p struct {
		TicketID string `json:"ticket_id"`
		AgentID  string `json:"agent_id"`
		Evidence string `json:"evidence"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	if err := s.store.CompleteTicket(p.TicketID, p.AgentID, p.Evidence); err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Ticket %q completed by %s", p.TicketID, p.AgentID), false
}

func (s *Server) handoffPublish(args json.RawMessage) (string, bool) {
	var p struct {
		TicketID string `json:"ticket_id"`
		ToAgent  string `json:"to_agent"`
		Summary  string `json:"summary"`
		Branch   string `json:"branch"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	id, err := s.store.PublishHandoff(sprintboard.CoordinationHandoff{
		TicketID:  p.TicketID,
		FromAgent: s.agentID,
		ToAgent:   p.ToAgent,
		Summary:   p.Summary,
		Branch:    p.Branch,
	})
	if err != nil {
		return err.Error(), true
	}
	return fmt.Sprintf("Handoff #%d published: %s -> %s for ticket %q", id, s.agentID, p.ToAgent, p.TicketID), false
}

func (s *Server) handoffSubscribe(args json.RawMessage) (string, bool) {
	var p struct {
		AgentID string `json:"agent_id"`
		Since   string `json:"since"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	since := time.Now().Add(-24 * time.Hour)
	if p.Since != "" {
		if t, err := time.Parse(time.RFC3339, p.Since); err == nil {
			since = t
		}
	}
	handoffs, err := s.store.SubscribeHandoffs(p.AgentID, since)
	if err != nil {
		return err.Error(), true
	}
	data, _ := json.MarshalIndent(handoffs, "", "  ")
	return string(data), false
}

func agentRegisterSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id":     map[string]string{"type": "string", "description": "Agent ID (defaults to auto-detected)"},
			"surface":      map[string]string{"type": "string", "description": "Agent surface: cursor, codex, claude-code, operator"},
			"capabilities": map[string]string{"type": "string", "description": "Comma-separated capabilities"},
		},
	}
}

func agentHeartbeatSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id":          map[string]string{"type": "string", "description": "Agent ID (defaults to auto-detected)"},
			"current_ticket_id": map[string]string{"type": "string", "description": "Ticket currently being worked on"},
		},
	}
}

func taskClaimSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticket_id": map[string]string{"type": "string", "description": "Ticket ID to claim"},
			"agent_id":  map[string]string{"type": "string", "description": "Agent ID (defaults to auto-detected)"},
		},
		"required": []string{"ticket_id"},
	}
}

func taskCompleteSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticket_id": map[string]string{"type": "string", "description": "Ticket ID to complete"},
			"agent_id":  map[string]string{"type": "string", "description": "Agent ID (defaults to auto-detected)"},
			"evidence":  map[string]string{"type": "string", "description": "Completion evidence (commit SHA, test output)"},
		},
		"required": []string{"ticket_id"},
	}
}

func handoffPublishSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticket_id": map[string]string{"type": "string", "description": "Ticket ID"},
			"to_agent":  map[string]string{"type": "string", "description": "Agent receiving the handoff"},
			"summary":   map[string]string{"type": "string", "description": "Handoff summary"},
			"branch":    map[string]string{"type": "string", "description": "Git branch name"},
		},
		"required": []string{"ticket_id", "to_agent", "summary"},
	}
}

func handoffSubscribeSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]string{"type": "string", "description": "Agent ID to check handoffs for (defaults to auto-detected)"},
			"since":    map[string]string{"type": "string", "description": "ISO 8601 timestamp to filter from (defaults to 24h ago)"},
		},
	}
}

func (s *Server) sprintSearch(args json.RawMessage) (string, bool) {
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return err.Error(), true
	}
	if p.Limit <= 0 {
		p.Limit = 5
	}

	queryVec, err := s.embedder.Embed(p.Query)
	if err != nil {
		return err.Error(), true
	}

	results, err := s.store.SearchSimilar(queryVec, "sprint", p.Limit)
	if err != nil {
		return err.Error(), true
	}

	if len(results) == 0 {
		return "[]", false
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return string(data), false
}

func (s *Server) taskRecommend(args json.RawMessage) (string, bool) {
	var p struct {
		AgentID  string `json:"agent_id"`
		Limit    int    `json:"limit"`
		SprintID string `json:"sprint_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "invalid arguments", true
	}
	if p.AgentID == "" {
		p.AgentID = s.agentID
	}
	if p.Limit == 0 {
		p.Limit = 5
	}

	agent, err := s.store.GetAgent(p.AgentID)
	if err != nil {
		return fmt.Sprintf("agent %q not registered: %v", p.AgentID, err), true
	}

	tickets, err := s.store.ListTickets(p.SprintID)
	if err != nil {
		return fmt.Sprintf("error listing tickets: %v", err), true
	}

	type rec struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Priority int    `json:"priority"`
		Match    string `json:"match_reason"`
	}
	var recommendations []rec
	for _, t := range tickets {
		if len(recommendations) >= p.Limit {
			break
		}
		if t.Status != "backlog" && t.Status != "ready" {
			continue
		}
		recommendations = append(recommendations, rec{
			ID:       t.ID,
			Title:    t.Title,
			Priority: t.Priority,
			Match:    fmt.Sprintf("agent %s has matching capabilities for %s", agent.ID, t.ID),
		})
	}

	data, _ := json.MarshalIndent(recommendations, "", "  ")
	return string(data), false
}

func (s *Server) sprintDistribute(args json.RawMessage) (string, bool) {
	var p struct {
		SprintID string `json:"sprint_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "invalid arguments", true
	}
	if p.SprintID == "" {
		return "sprint_id is required", true
	}

	agents, err := s.store.ListActiveAgents()
	if err != nil {
		return fmt.Sprintf("error listing agents: %v", err), true
	}

	tickets, err := s.store.ListTickets(p.SprintID)
	if err != nil {
		return fmt.Sprintf("error listing tickets: %v", err), true
	}

	type assignment struct {
		TicketID string `json:"ticket_id"`
		AgentID  string `json:"agent_id"`
		Reason   string `json:"reason"`
	}
	var assignments []assignment

	agentIdx := 0
	for _, t := range tickets {
		if t.Status != "backlog" && t.Status != "ready" {
			continue
		}
		if len(agents) == 0 {
			break
		}
		a := agents[agentIdx%len(agents)]
		assignments = append(assignments, assignment{
			TicketID: t.ID,
			AgentID:  a.ID,
			Reason:   "round-robin distribution",
		})
		s.store.AssignTicket(t.ID, a.ID)
		agentIdx++
	}

	data, _ := json.MarshalIndent(map[string]interface{}{
		"sprint_id":   p.SprintID,
		"assigned":    len(assignments),
		"assignments": assignments,
	}, "", "  ")
	return string(data), false
}

func taskRecommendSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id":  map[string]string{"type": "string", "description": "Agent ID (defaults to auto-detected)"},
			"sprint_id": map[string]string{"type": "string", "description": "Sprint ID to search in (optional, searches all if empty)"},
			"limit":     map[string]string{"type": "integer", "description": "Max recommendations (default 5)"},
		},
	}
}

func sprintDistributeSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sprint_id": map[string]string{"type": "string", "description": "Sprint ID to distribute tickets for"},
		},
		"required": []string{"sprint_id"},
	}
}
