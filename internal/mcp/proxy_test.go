package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/velumix/gojo/internal/protocol"
)

type testToolHandler struct{}

func (testToolHandler) Handles(name string) bool { return name == "local_tool" }

func (testToolHandler) Definitions() []protocol.ToolDefinition {
	return []protocol.ToolDefinition{{
		Name:        "local_tool",
		Description: "test tool",
		InputSchema: map[string]any{"type": "object"},
	}}
}

func (testToolHandler) Call(string, map[string]any) (protocol.ToolResult, error) {
	return protocol.TextResult("ok", false), nil
}

func TestMergeToolList(t *testing.T) {
	proxy := NewProxy(nil, testToolHandler{})
	proxy.rememberListRequest(7)

	line := `{"jsonrpc":"2.0","id":7,"result":{"tools":[{"name":"native_tool","inputSchema":{"type":"object"}}]}}`
	merged := proxy.mergeToolList([]byte(line + "\n"))

	var response struct {
		Result struct {
			Tools []protocol.ToolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(merged, &response); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range response.Result.Tools {
		names = append(names, tool.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "local_tool") {
		t.Fatalf("merged tools = %v", names)
	}
}
