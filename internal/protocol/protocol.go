package protocol

import "encoding/json"

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content,omitempty"`
	IsError bool           `json:"isError,omitempty"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

type ToolCaller interface {
	CallTool(name string, arguments map[string]any) (ToolResult, error)
}

type ToolHandler interface {
	Handles(name string) bool
	Definitions() []ToolDefinition
	Call(name string, arguments map[string]any) (ToolResult, error)
}

func TextResult(text string, isError bool) ToolResult {
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
		IsError: isError,
	}
}
