package pull

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/velumix/gojo/internal/protocol"
)

type fakeCaller struct{}

func (fakeCaller) CallTool(name string, arguments map[string]any) (protocol.ToolResult, error) {
	switch name {
	case "list_roblox_studios":
		return protocol.TextResult(`{"studios":[{"id":"studio-1","name":"Test Place"}]}`, false), nil
	case "execute_luau":
		return protocol.TextResult(`{"total":2,"offset":0,"scripts":[{"className":"Script","path":["ServerScriptService","Main"],"source":"print('main')\nreturn true","hasChildren":true,"sourceOffset":0,"sourceLength":25,"complete":true},{"className":"ModuleScript","path":["ServerScriptService","Main","Config"],"source":"return {Enabled = true}","hasChildren":false,"sourceOffset":0,"sourceLength":23,"complete":true}]}`, false), nil
	default:
		return protocol.ToolResult{}, nil
	}
}

func TestPullWritesMappedScripts(t *testing.T) {
	outputDir := t.TempDir()
	result, err := Pull(fakeCaller{}, Options{
		OutputDir: outputDir,
		Services:  []string{"ServerScriptService"},
		MaxDepth:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scripts != 2 {
		t.Fatalf("scripts = %d, want 2", result.Scripts)
	}

	mainPath := filepath.Join(outputDir, "src", "ServerScriptService", "Main", "init.server.luau")
	mainSource, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(mainSource) != "print('main')\nreturn true" {
		t.Fatalf("main source = %q", mainSource)
	}

	configPath := filepath.Join(outputDir, "src", "ServerScriptService", "Main", "Config.luau")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatal(err)
	}

	project, err := os.ReadFile(filepath.Join(outputDir, "place.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(project, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["format"] != "gojo-v1" {
		t.Fatalf("project format = %#v", decoded["format"])
	}
}

type dottedCaller struct{}

func (dottedCaller) CallTool(name string, arguments map[string]any) (protocol.ToolResult, error) {
	switch name {
	case "list_roblox_studios":
		return protocol.TextResult(`{"studios":[{"id":"studio-1","name":"Test Place"}]}`, false), nil
	case "execute_luau":
		return protocol.TextResult(`{"total":3,"offset":0,"scripts":[{"className":"ModuleScript","path":["ReplicatedStorage","pkg@17.3.10","React.new"],"source":"return 1","hasChildren":false,"sourceOffset":0,"sourceLength":8,"complete":true},{"className":"Script","path":["ReplicatedStorage","Boot"],"source":"print('server')","hasChildren":false,"sourceOffset":0,"sourceLength":15,"complete":true},{"className":"LocalScript","path":["ReplicatedStorage","Boot"],"source":"print('client')","hasChildren":false,"sourceOffset":0,"sourceLength":15,"complete":true}]}`, false), nil
	default:
		return protocol.ToolResult{}, nil
	}
}

func TestPullKeepsDottedNamesAndSameNameSiblings(t *testing.T) {
	outputDir := t.TempDir()
	result, err := Pull(dottedCaller{}, Options{
		OutputDir: outputDir,
		Services:  []string{"ReplicatedStorage"},
		MaxDepth:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scripts != 3 {
		t.Fatalf("scripts = %d, want 3", result.Scripts)
	}
	for _, rel := range []string{
		filepath.Join("src", "ReplicatedStorage", "pkg@17.3.10", "React.new.luau"),
		filepath.Join("src", "ReplicatedStorage", "Boot.server.luau"),
		filepath.Join("src", "ReplicatedStorage", "Boot.client.luau"),
	} {
		if _, err := os.Stat(filepath.Join(outputDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "src", "ReplicatedStorage", "pkg@17")); !os.IsNotExist(err) {
		t.Fatalf("dotted folder was split into nested directories")
	}
}
