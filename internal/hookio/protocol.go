package hookio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Input represents the JSON payload Cursor sends to hooks via stdin.
type Input struct {
	Command   string `json:"command,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolInput string `json:"tool_input,omitempty"`
	Status    string `json:"status,omitempty"`
}

// Response represents the JSON payload hooks return to Cursor via stdout.
type Response struct {
	Continue     bool   `json:"continue,omitempty"`
	Permission   string `json:"permission,omitempty"`
	UserMessage  string `json:"userMessage,omitempty"`
	AgentMessage string `json:"agentMessage,omitempty"`
}

// Handler processes a hook input and returns a response.
type Handler interface {
	Handle(ctx context.Context, input *Input) (*Response, error)
}

// Allow returns a response that allows the operation.
func Allow() *Response {
	return &Response{Continue: true, Permission: "allow"}
}

// Deny returns a response that blocks the operation.
func Deny(userMsg, agentMsg string) *Response {
	return &Response{
		Continue:     false,
		Permission:   "deny",
		UserMessage:  userMsg,
		AgentMessage: agentMsg,
	}
}

// Ask returns a response that requests user confirmation.
func Ask(userMsg, agentMsg string) *Response {
	return &Response{
		Continue:     true,
		Permission:   "ask",
		UserMessage:  userMsg,
		AgentMessage: agentMsg,
	}
}

// Empty returns an informational-only empty JSON response.
func Empty() *Response {
	return &Response{}
}

// ReadInput reads and parses JSON from the given reader.
func ReadInput(r io.Reader) (*Input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return &Input{}, nil
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return &input, nil
}

// ReadStdin reads and parses JSON from os.Stdin.
func ReadStdin() (*Input, error) {
	return ReadInput(os.Stdin)
}

// WriteResponse marshals and writes the response to the given writer.
func WriteResponse(w io.Writer, resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// WriteStdout marshals and writes the response to os.Stdout.
func WriteStdout(resp *Response) error {
	return WriteResponse(os.Stdout, resp)
}

// Run reads input from stdin, passes it to the handler, and writes the response to stdout.
// On handler error it writes an empty response. On deny it exits with code 2.
func Run(h Handler) {
	input, err := ReadStdin()
	if err != nil {
		_ = WriteStdout(Allow())
		return
	}

	resp, err := h.Handle(context.Background(), input)
	if err != nil {
		_ = WriteStdout(Allow())
		return
	}

	_ = WriteStdout(resp)
	if resp.Permission == "deny" {
		os.Exit(2)
	}
}
