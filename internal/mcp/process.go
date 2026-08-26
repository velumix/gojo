package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/velumix/gojo/internal/platform"
	"github.com/velumix/gojo/internal/protocol"
)

const (
	startupTimeout = 5 * time.Second
	toolTimeout    = 45 * time.Second
)

type Process struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	reader    *bufio.Reader
	done      chan struct{}
	closeOnce sync.Once
	drainOnce sync.Once
	callMu    sync.Mutex
	nextID    int
}

// Start launches Roblox's built-in Studio MCP server without opening a window.
func Start() (*Process, error) {
	cmd := platform.HiddenCommand(
		"cmd.exe",
		"/c",
		"cd /d %LOCALAPPDATA%\\Roblox && .\\mcp.bat",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open MCP stdout: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("launch mcp.bat: %w", err)
	}

	process := &Process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReader(stdout),
		done:   make(chan struct{}),
		nextID: 2,
	}
	go func() {
		_ = cmd.Wait()
		close(process.done)
	}()
	return process, nil
}

func (p *Process) Initialize() error {
	p.callMu.Lock()
	defer p.callMu.Unlock()

	request := protocol.Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"gojo","version":"0.1.0"}}`),
	}
	if err := WriteJSONLine(p.stdin, request); err != nil {
		return fmt.Errorf("send MCP initialize: %w", err)
	}

	response, err := p.readResponseLocked(1, startupTimeout)
	if err != nil {
		return fmt.Errorf("read MCP initialize: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("MCP initialize failed: %s", response.Error.Message)
	}

	if err := WriteJSONLine(p.stdin, protocol.Message{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		return fmt.Errorf("send MCP initialized notification: %w", err)
	}
	return nil
}

func (p *Process) CallTool(name string, arguments map[string]any) (protocol.ToolResult, error) {
	p.callMu.Lock()
	defer p.callMu.Unlock()

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

	id := p.nextID
	p.nextID++
	if err := WriteJSONLine(p.stdin, protocol.Message{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params:  params,
	}); err != nil {
		return protocol.ToolResult{}, fmt.Errorf("send %s: %w", name, err)
	}

	response, err := p.readResponseLocked(id, toolTimeout)
	if err != nil {
		return protocol.ToolResult{}, fmt.Errorf("read %s response: %w", name, err)
	}
	return parseToolResult(name, response)
}

func parseToolResult(name string, response protocol.Message) (protocol.ToolResult, error) {
	if response.Error != nil {
		return protocol.ToolResult{}, fmt.Errorf("%s failed: %s", name, response.Error.Message)
	}

	var result protocol.ToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return protocol.ToolResult{}, fmt.Errorf("parse %s result: %w", name, err)
	}
	if result.IsError {
		message := ""
		for _, content := range result.Content {
			if content.Type == "text" && content.Text != "" {
				message = content.Text
				break
			}
		}
		if message != "" {
			return result, fmt.Errorf("%s returned an error: %s", name, message)
		}
		return result, fmt.Errorf("%s returned an error", name)
	}
	return result, nil
}

func (p *Process) readResponseLocked(expectedID int, timeout time.Duration) (protocol.Message, error) {
	for {
		message, err := readJSONLine(p.reader, timeout)
		if err != nil {
			return protocol.Message{}, err
		}
		if message.ID == expectedID {
			return message, nil
		}
	}
}

func (p *Process) Drain() {
	p.drainOnce.Do(func() {
		go func() {
			_, _ = io.Copy(io.Discard, p.reader)
		}()
	})
}

func (p *Process) Exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *Process) Close() error {
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		_ = platform.StopProcessTree(p.cmd)
	})
	return nil
}

func WriteJSONLine(writer io.Writer, value protocol.Message) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "%s\n", data)
	return err
}

func readJSONLine(reader *bufio.Reader, timeout time.Duration) (protocol.Message, error) {
	result := make(chan struct {
		message protocol.Message
		err     error
	}, 1)

	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			result <- struct {
				message protocol.Message
				err     error
			}{err: err}
			return
		}

		var message protocol.Message
		err = json.Unmarshal(line, &message)
		result <- struct {
			message protocol.Message
			err     error
		}{message: message, err: err}
	}()

	select {
	case response := <-result:
		return response.message, response.err
	case <-time.After(timeout):
		return protocol.Message{}, errors.New("timed out")
	}
}
