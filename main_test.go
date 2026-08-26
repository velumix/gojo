package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSONLine(t *testing.T) {
	var output bytes.Buffer

	message := jsonRPCMessage{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	if err := writeJSONLine(&output, message); err != nil {
		t.Fatal(err)
	}

	var decoded jsonRPCMessage
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Method != "initialize" || decoded.ID != 1 {
		t.Fatalf("decoded message = %#v", decoded)
	}
}
