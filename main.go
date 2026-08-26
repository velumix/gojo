package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	studioProcessName = "RobloxStudioBeta.exe"
	pollInterval      = 2 * time.Second
	mcpStartupTimeout = 5 * time.Second
)

type jsonRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	done   <-chan error
	closed bool
}

func main() {
	closeLog := configureLogging()
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Gojo started; watching for %s", studioProcessName)
	runSupervisor(ctx)
	log.Println("Gojo stopped")
}

func runSupervisor(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var mcp *mcpProcess
	defer func() {
		if mcp != nil {
			mcp.stop()
		}
	}()

	for {
		studioOpen := isStudioOpen()

		if studioOpen && mcp == nil {
			var err error
			mcp, err = startMCP()
			if err != nil {
				log.Printf("could not start Roblox MCP: %v", err)
				mcp = nil
			}
		}

		if mcp != nil && mcp.exited() {
			log.Println("Roblox MCP exited; will retry")
			mcp = nil
		}

		if !studioOpen && mcp != nil {
			log.Println("Roblox Studio closed; stopping Roblox MCP")
			mcp.stop()
			mcp = nil
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func isStudioOpen() bool {
	output, err := hiddenCommand("tasklist", "/FI", "IMAGENAME eq "+studioProcessName, "/NH").Output()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(output)), strings.ToLower(studioProcessName))
}

func startMCP() (*mcpProcess, error) {
	cmd := exec.Command(
		"cmd.exe",
		"/c",
		"cd /d %LOCALAPPDATA%\\Roblox && .\\mcp.bat",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open MCP stdout: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch mcp.bat: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	if err := initializeMCP(stdin, reader); err != nil {
		_ = stopProcessTree(cmd)
		return nil, err
	}

	// Keep consuming stdout so a server notification cannot fill the pipe.
	go func() {
		_, _ = io.Copy(io.Discard, reader)
	}()

	log.Println("Roblox MCP connected")
	return &mcpProcess{cmd: cmd, stdin: stdin, done: done}, nil
}

func initializeMCP(stdin io.Writer, reader *bufio.Reader) error {
	request := jsonRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"gojo","version":"0.1.0"}}`),
	}

	if err := writeJSONLine(stdin, request); err != nil {
		return fmt.Errorf("send MCP initialize: %w", err)
	}

	response, err := readJSONLine(reader, mcpStartupTimeout)
	if err != nil {
		return fmt.Errorf("read MCP initialize: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("MCP initialize failed: %s", response.Error.Message)
	}

	notification := jsonRPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}
	if err := writeJSONLine(stdin, notification); err != nil {
		return fmt.Errorf("send MCP initialized notification: %w", err)
	}

	return nil
}

func writeJSONLine(writer io.Writer, value jsonRPCMessage) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "%s\n", data)
	return err
}

func readJSONLine(reader *bufio.Reader, timeout time.Duration) (jsonRPCMessage, error) {
	result := make(chan struct {
		message jsonRPCMessage
		err     error
	}, 1)

	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			result <- struct {
				message jsonRPCMessage
				err     error
			}{err: err}
			return
		}

		var message jsonRPCMessage
		err = json.Unmarshal(line, &message)
		result <- struct {
			message jsonRPCMessage
			err     error
		}{message: message, err: err}
	}()

	select {
	case response := <-result:
		return response.message, response.err
	case <-time.After(timeout):
		return jsonRPCMessage{}, errors.New("timed out")
	}
}

func (p *mcpProcess) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *mcpProcess) stop() {
	if p == nil || p.closed {
		return
	}
	p.closed = true
	_ = p.stdin.Close()
	_ = stopProcessTree(p.cmd)
}

func stopProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return hiddenCommand("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
}

func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func configureLogging() func() {
	log.SetFlags(log.LstdFlags)

	baseDir := os.Getenv("LOCALAPPDATA")
	if baseDir == "" {
		return func() {}
	}

	logDir := filepath.Join(baseDir, "Gojo")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return func() {}
	}

	file, err := os.OpenFile(filepath.Join(logDir, "gojo.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return func() {}
	}

	log.SetOutput(file)
	return func() { _ = file.Close() }
}
