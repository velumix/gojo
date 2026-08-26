package app

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/velumix/gojo/internal/banner"
	"github.com/velumix/gojo/internal/logging"
	"github.com/velumix/gojo/internal/mcp"
	"github.com/velumix/gojo/internal/pull"
	"github.com/velumix/gojo/internal/supervisor"
)

const studioProcessName = "RobloxStudioBeta.exe"

func Run(args []string) {
	watchMode := hasArg(args, "--watch")
	closeLog := logging.Configure(!watchMode || hasArg(args, "--console"))
	defer closeLog()

	if watchMode {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		log.Printf("Gojo watcher started; watching for %s", studioProcessName)
		supervisor.Run(ctx, supervisor.Config{
			StudioProcessName: studioProcessName,
			Connect:           connectStudio,
		})
		log.Println("Gojo watcher stopped")
		return
	}

	if err := runProxy(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("Gojo MCP proxy stopped: %v", err)
	}
}

func runProxy() error {
	process, err := mcp.Start()
	if err != nil {
		return err
	}
	defer func() {
		_ = process.Close()
		log.Println("Gojo MCP proxy stopped")
	}()

	proxy := mcp.NewProxy(process, nil)
	proxy.SetHandler(pull.NewToolWithCaller(proxy))
	log.Println("Gojo MCP proxy started")
	return proxy.Run(os.Stdin, os.Stdout)
}

func connectStudio() (*mcp.Process, error) {
	process, err := mcp.Start()
	if err != nil {
		return nil, err
	}
	if err := process.Initialize(); err != nil {
		_ = process.Close()
		return nil, err
	}
	if err := banner.Print(process); err != nil {
		log.Printf("Studio banner skipped: %v", err)
	} else {
		log.Println("Studio banner printed")
	}
	process.Drain()
	log.Println("Roblox MCP connected")
	return process, nil
}

func hasArg(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}
