package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

func Configure(toConsole bool) func() {
	log.SetFlags(log.LstdFlags)

	var file *os.File
	if baseDir := os.Getenv("LOCALAPPDATA"); baseDir != "" {
		logDir := filepath.Join(baseDir, "Gojo")
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			file, _ = os.OpenFile(filepath.Join(logDir, "gojo.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		}
	}

	output := io.Writer(io.Discard)
	if file != nil {
		output = file
	}
	if toConsole {
		if file != nil {
			output = io.MultiWriter(file, os.Stderr)
		} else {
			output = os.Stderr
		}
	}
	log.SetOutput(output)

	return func() {
		if file != nil {
			_ = file.Close()
		}
	}
}
