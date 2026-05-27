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

// RESTSprintBoardClient implements SprintBoardClient via the SprintBoard REST API.
type RESTSprintBoardClient struct {
	BaseURL    string
	SprintID   string
	HTTPClient *http.Client
}

// NewRESTSprintBoardClient creates a client pointing at the SprintBoard REST API.
func NewRESTSprintBoardClient(baseURL, sprintID string) *RESTSprintBoardClient {
	return &RESTSprintBoardClient{
		BaseURL:  baseURL,
		SprintID: sprintID,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *RESTSprintBoardClient) ListReady(ctx context.Context, _ []string) ([]Ticket, error) {
	url := fmt.Sprintf("%s/api/v1/tickets/ready?sprint_id=%s", c.BaseURL, c.SprintID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}

	var tickets []Ticket
	if err := json.Unmarshal(body, &tickets); err != nil {
		return nil, nil
	}
	return tickets, nil
}

func (c *RESTSprintBoardClient) Claim(ctx context.Context, ticketID, agentID string) (ClaimResult, error) {
	url := fmt.Sprintf("%s/api/v1/tickets/%s/claim", c.BaseURL, ticketID)
	payload, _ := json.Marshal(map[string]string{
		"agent_id": agentID,
		"reason":   "fleet-agent auto-claim",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return ClaimResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ClaimResult{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ClaimResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		var errResp struct {
			Error   string `json:"error"`
			ClaimBy string `json:"claimed_by"`
		}
		json.Unmarshal(body, &errResp)
		return ClaimResult{Success: false, TicketID: ticketID, ConflictBy: errResp.ClaimBy}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return ClaimResult{}, fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}

	return ClaimResult{Success: true, TicketID: ticketID}, nil
}

func (c *RESTSprintBoardClient) Complete(ctx context.Context, ticketID, agentID, evidence string) error {
	url := fmt.Sprintf("%s/api/v1/tickets/%s/complete", c.BaseURL, ticketID)
	payload, _ := json.Marshal(map[string]string{
		"agent_id": agentID,
		"evidence": evidence,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *RESTSprintBoardClient) Block(ctx context.Context, ticketID, agentID, reason string) error {
	url := fmt.Sprintf("%s/api/v1/tickets/%s/block", c.BaseURL, ticketID)
	payload, _ := json.Marshal(map[string]string{
		"agent_id": agentID,
		"reason":   reason,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
