package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

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

	server := &Server{store: store, agentID: resolveAgentID()}
	server.serve(os.Stdin, os.Stdout)
}

type Server struct {
	store   *sprintboard.Store
	agentID string
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
