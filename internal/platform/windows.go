package platform

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// AttachParentConsole attaches to the parent terminal so a windowsgui
// build can still print help and pull output. No-op without a parent
// console (for example when an MCP host pipes stdio).
func AttachParentConsole() {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	r, _, _ := proc.Call(^uintptr(0) /* ATTACH_PARENT_PROCESS */)
	if r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}

func HiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func IsProcessRunning(processName string) bool {
	output, err := HiddenCommand("tasklist", "/FI", "IMAGENAME eq "+processName, "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(output)), strings.ToLower(processName))
}

func StopProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return HiddenCommand("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
}
