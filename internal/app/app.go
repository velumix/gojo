package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/velumix/gojo/internal/banner"
	"github.com/velumix/gojo/internal/logging"
	"github.com/velumix/gojo/internal/mcp"
	"github.com/velumix/gojo/internal/platform"
	"github.com/velumix/gojo/internal/pull"
	"github.com/velumix/gojo/internal/supervisor"
)

const studioProcessName = "RobloxStudioBeta.exe"

const version = "0.1.0"

const usage = `Gojo - Roblox Studio MCP proxy

Usage:
  gojo                              Run as an MCP stdio server (default)
  gojo --watch [--console]          Watch for Studio and keep MCP connected
  gojo --pull [output-dir]          Pull Studio scripts into a local project
    --services A,B,C                Services to pull (default: common script services)
    --max-depth N                   Max hierarchy depth per service (default 25)
    --studio ID                     Studio id or name when several are open
  gojo --help                       Show this help
  gojo --version                    Show version
`

func Run(args []string) {
	if hasArg(args, "--help") || hasArg(args, "-h") || hasArg(args, "help") ||
		hasArg(args, "--version") || hasArg(args, "-v") || hasArg(args, "version") ||
		hasArg(args, "--pull") {
		platform.AttachParentConsole()
	}
	if hasArg(args, "--help") || hasArg(args, "-h") || hasArg(args, "help") {
		fmt.Print(usage)
		return
	}
	if hasArg(args, "--version") || hasArg(args, "-v") || hasArg(args, "version") {
		fmt.Printf("gojo %s\n", version)
		return
	}
	pullMode := hasArg(args, "--pull")
	watchMode := hasArg(args, "--watch")
	closeLog := logging.Configure(!watchMode || hasArg(args, "--console") || pullMode)
	defer closeLog()

	if pullMode {
		outputDir := "roblox-place"
		for i, arg := range args {
			if arg == "--pull" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				outputDir = args[i+1]
				break
			}
		}

		maxDepth := 25
		for i, arg := range args {
			if (arg == "--max-depth" || arg == "--depth") && i+1 < len(args) {
				if d, err := strconv.Atoi(args[i+1]); err == nil && d > 0 {
					maxDepth = d
				}
				break
			}
		}

		studioID := ""
		for i, arg := range args {
			if (arg == "--studio" || arg == "--studio-id" || arg == "-s") && i+1 < len(args) {
				studioID = args[i+1]
				break
			}
		}

		var services []string
		for i, arg := range args {
			if (arg == "--services" || arg == "--service") && i+1 < len(args) {
				for _, name := range strings.Split(args[i+1], ",") {
					if name = strings.TrimSpace(name); name != "" {
						services = append(services, name)
					}
				}
				break
			}
		}

		fmt.Printf("Gojo: connecting to Roblox Studio MCP to pull workspace into %s...\n", outputDir)
		process, err := mcp.Start()
		if err != nil {
			log.Fatalf("Could not start Roblox MCP: %v", err)
		}
		defer func() {
			_ = process.Close()
		}()

		if err := process.Initialize(); err != nil {
			log.Fatalf("Roblox MCP initialize failed: %v", err)
		}

		result, err := pull.Pull(process, pull.Options{
			StudioID:  studioID,
			OutputDir: outputDir,
			Services:  services,
			MaxDepth:  maxDepth,
		})
		if err != nil {
			log.Fatalf("Gojo pull failed: %v", err)
		}

		msg := pull.FormatResult(result)
		log.Println(msg)
		fmt.Println(msg)
		return
	}

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
