package fleetagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPSprintBoardClient implements SprintBoardClient via HTTP to SprintBoard MCP.
type HTTPSprintBoardClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewHTTPSprintBoardClient creates a client pointing at the SprintBoard MCP HTTP endpoint.
func NewHTTPSprintBoardClient(baseURL string) *HTTPSprintBoardClient {
	return &HTTPSprintBoardClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type mcpToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *HTTPSprintBoardClient) callTool(ctx context.Context, toolName string, args map[string]interface{}) (json.RawMessage, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  mcpToolCall{Name: toolName, Arguments: args},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var mcpResp mcpResponse
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}
	return mcpResp.Result, nil
}

func (c *HTTPSprintBoardClient) ListReady(ctx context.Context, capabilities []string) ([]Ticket, error) {
	args := map[string]interface{}{}
	if len(capabilities) > 0 {
		args["capabilities"] = capabilities
	}

	result, err := c.callTool(ctx, "ticket_ready_list", args)
	if err != nil {
		return nil, err
	}

	var tickets []Ticket
	if err := json.Unmarshal(result, &tickets); err != nil {
		return nil, fmt.Errorf("unmarshal tickets: %w", err)
	}
	return tickets, nil
}

func (c *HTTPSprintBoardClient) Claim(ctx context.Context, ticketID, agentID string) (ClaimResult, error) {
	args := map[string]interface{}{
		"ticket_id": ticketID,
		"agent_id":  agentID,
		"reason":    "fleet-agent auto-claim",
	}
	result, err := c.callTool(ctx, "task_claim", args)
	if err != nil {
		return ClaimResult{}, err
	}
	var cr ClaimResult
	if err := json.Unmarshal(result, &cr); err != nil {
		return ClaimResult{}, fmt.Errorf("unmarshal claim: %w", err)
	}
	return cr, nil
}

func (c *HTTPSprintBoardClient) Complete(ctx context.Context, ticketID, agentID, evidence string) error {
	args := map[string]interface{}{
		"ticket_id": ticketID,
		"agent_id":  agentID,
		"evidence":  evidence,
	}
	_, err := c.callTool(ctx, "task_complete", args)
	return err
}

func (c *HTTPSprintBoardClient) Block(ctx context.Context, ticketID, agentID, reason string) error {
	args := map[string]interface{}{
		"ticket_id": ticketID,
		"agent_id":  agentID,
		"note":      reason,
		"status":    "blocked",
	}
	_, err := c.callTool(ctx, "ticket_update", args)
	return err
}
