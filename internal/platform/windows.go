package platform

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

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
