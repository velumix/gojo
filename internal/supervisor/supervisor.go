package supervisor

import (
	"context"
	"log"
	"time"

	"github.com/velumix/gojo/internal/mcp"
	"github.com/velumix/gojo/internal/platform"
)

type Config struct {
	StudioProcessName string
	PollInterval      time.Duration
	RetryInterval     time.Duration
	Connect           func() (*mcp.Process, error)
}

func Run(ctx context.Context, config Config) {
	if config.PollInterval <= 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.RetryInterval <= 0 {
		config.RetryInterval = 5 * time.Second
	}

	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	var process *mcp.Process
	var nextAttempt time.Time
	var lastStartError string
	studioWasOpen := false
	defer func() {
		if process != nil {
			_ = process.Close()
		}
	}()

	for {
		studioOpen := platform.IsProcessRunning(config.StudioProcessName)
		if studioOpen != studioWasOpen {
			if studioOpen {
				log.Println("Roblox Studio detected")
			} else {
				log.Println("Roblox Studio closed")
			}
			studioWasOpen = studioOpen
		}

		if studioOpen && process == nil && !time.Now().Before(nextAttempt) {
			var err error
			process, err = config.Connect()
			if err != nil {
				if err.Error() != lastStartError {
					log.Printf("Roblox MCP unavailable: %v (retrying in %s)", err, config.RetryInterval)
					lastStartError = err.Error()
				}
				nextAttempt = time.Now().Add(config.RetryInterval)
				process = nil
			} else {
				lastStartError = ""
				nextAttempt = time.Time{}
			}
		}

		if process != nil && process.Exited() {
			log.Printf("Roblox MCP stopped; retrying in %s", config.RetryInterval)
			process = nil
			nextAttempt = time.Now().Add(config.RetryInterval)
		}

		if !studioOpen && process != nil {
			log.Println("Stopping Roblox MCP")
			_ = process.Close()
			process = nil
			nextAttempt = time.Time{}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
