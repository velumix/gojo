package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/velumix/gojo/internal/protocol"
)

type Proxy struct {
	process   *Process
	handler   protocol.ToolHandler
	outputMu  sync.Mutex
	writeMu   sync.Mutex
	listMu    sync.Mutex
	listIDs   map[int]struct{}
	pendingMu sync.Mutex
	pending   map[int]chan protocol.Message
	nextID    int
}

func NewProxy(process *Process, handler protocol.ToolHandler) *Proxy {
	return &Proxy{
		process: process,
		handler: handler,
		listIDs: make(map[int]struct{}),
		pending: make(map[int]chan protocol.Message),
		nextID:  -1,
	}
}

func (p *Proxy) SetHandler(handler protocol.ToolHandler) {
	p.handler = handler
}

func (p *Proxy) CallTool(name string, arguments map[string]any) (protocol.ToolResult, error) {
	if p.process == nil {
		return protocol.ToolResult{}, errors.New("MCP process is unavailable")
	}

	encodedArguments, err := json.Marshal(arguments)
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("encode %s arguments: %w", name, err)
	}
	params, err := json.Marshal(struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}{Name: name, Arguments: encodedArguments})
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("encode %s request: %w", name, err)
	}

	p.pendingMu.Lock()
	id := p.nextID
	p.nextID--
	responseCh := make(chan protocol.Message, 1)
	p.pending[id] = responseCh
	p.pendingMu.Unlock()
	defer p.removePending(id)

	if err := p.writeNativeMessage(protocol.Message{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params:  params,
	}); err != nil {
		return protocol.ToolResult{}, fmt.Errorf("send %s: %w", name, err)
	}

	timer := time.NewTimer(toolTimeout)
	defer timer.Stop()
	select {
	case response := <-responseCh:
		return parseToolResult(name, response)
	case <-p.process.done:
		return protocol.ToolResult{}, fmt.Errorf("%s stopped before returning a response", name)
	case <-timer.C:
		return protocol.ToolResult{}, fmt.Errorf("%s timed out", name)
	}
}

func (p *Proxy) Run(input io.Reader, output io.Writer) error {
	results := make(chan error, 2)
	go func() {
		results <- p.relayOutput(output)
	}()
	go func() {
		results <- p.relayInput(input, output)
	}()
	return <-results
}

func (p *Proxy) relayInput(input io.Reader, output io.Writer) error {
	defer p.process.stdin.Close()

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var message protocol.Message
		_ = json.Unmarshal(line, &message)

		if message.Method == "tools/list" {
			p.rememberListRequest(message.ID)
		}

		if message.Method == "tools/call" {
			name, arguments, ok := parseToolCall(message.Params)
			if ok && p.handler != nil && p.handler.Handles(name) {
				log.Printf("MCP tool call: %s", name)
				p.handleCustomCall(message.ID, name, arguments, output)
				continue
			}
		}

		logMCPRequest(line)
		forwarded := append(append([]byte(nil), line...), '\n')
		if err := p.writeNative(forwarded); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (p *Proxy) relayOutput(output io.Writer) error {
	for {
		line, err := p.process.reader.ReadBytes('\n')
		if len(line) > 0 {
			var message protocol.Message
			if json.Unmarshal(line, &message) == nil && p.deliverPending(message) {
				if err != nil {
					return err
				}
				continue
			}
			p.writeOutput(output, p.mergeToolList(line))
		}
		if err != nil {
			return err
		}
	}
}

func (p *Proxy) writeNative(line []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err := p.process.stdin.Write(line)
	return err
}

func (p *Proxy) writeNativeMessage(message protocol.Message) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return p.writeNative(append(data, '\n'))
}

func (p *Proxy) deliverPending(message protocol.Message) bool {
	p.pendingMu.Lock()
	responseCh, ok := p.pending[message.ID]
	if ok {
		delete(p.pending, message.ID)
	}
	p.pendingMu.Unlock()
	if !ok {
		return false
	}
	responseCh <- message
	return true
}

func (p *Proxy) removePending(id int) {
	p.pendingMu.Lock()
	delete(p.pending, id)
	p.pendingMu.Unlock()
}

func (p *Proxy) handleCustomCall(id int, name string, arguments map[string]any, output io.Writer) {
	result, err := p.handler.Call(name, arguments)
	if err != nil {
		result = protocol.TextResult(err.Error(), true)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		encoded, _ = json.Marshal(protocol.TextResult(err.Error(), true))
	}
	p.writeOutput(output, mustJSONLine(protocol.Message{
		JSONRPC: "2.0",
		ID:      id,
		Result:  encoded,
	}))
}

func (p *Proxy) mergeToolList(line []byte) []byte {
	if p.handler == nil {
		return line
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(line, &envelope); err != nil {
		return line
	}
	var id int
	if err := json.Unmarshal(envelope["id"], &id); err != nil || !p.takeListRequest(id) {
		return line
	}

	var result struct {
		Tools []protocol.ToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(envelope["result"], &result); err != nil {
		return line
	}
	result.Tools = append(result.Tools, p.handler.Definitions()...)
	encoded, err := json.Marshal(result)
	if err != nil {
		return line
	}
	envelope["result"] = encoded
	encoded, err = json.Marshal(envelope)
	if err != nil {
		return line
	}
	return append(encoded, '\n')
}

func (p *Proxy) writeOutput(output io.Writer, line []byte) {
	p.outputMu.Lock()
	defer p.outputMu.Unlock()
	_, _ = output.Write(line)
}

func (p *Proxy) rememberListRequest(id int) {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	p.listIDs[id] = struct{}{}
}

func (p *Proxy) takeListRequest(id int) bool {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	if _, ok := p.listIDs[id]; !ok {
		return false
	}
	delete(p.listIDs, id)
	return true
}

func parseToolCall(params json.RawMessage) (string, map[string]any, bool) {
	var call struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil || call.Name == "" {
		return "", nil, false
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return call.Name, call.Arguments, true
}

func logMCPRequest(line []byte) {
	var message struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(line, &message); err != nil || message.Method == "" {
		return
	}

	if message.Method == "tools/call" {
		var params struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(message.Params, &params) == nil && params.Name != "" {
			log.Printf("MCP tool call: %s", params.Name)
			return
		}
	}
	log.Printf("MCP request: %s", message.Method)
}

func mustJSONLine(message protocol.Message) []byte {
	data, err := json.Marshal(message)
	if err != nil {
		return []byte("{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{\"isError\":true}}\n")
	}
	return append(data, '\n')
}
