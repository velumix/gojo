package mcp

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/velumix/gojo/internal/protocol"
)

func TestWriteJSONLine(t *testing.T) {
	var output bytes.Buffer
	message := protocol.Message{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	if err := WriteJSONLine(&output, message); err != nil {
		t.Fatal(err)
	}

	var decoded protocol.Message
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Method != "initialize" || decoded.ID != 1 {
		t.Fatalf("decoded message = %#v", decoded)
	}
}
