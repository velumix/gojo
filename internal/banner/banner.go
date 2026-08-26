package banner

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/velumix/gojo/internal/protocol"
)

const (
	attempts   = 5
	retryDelay = time.Second
	width      = 56
)

type studio struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func Print(caller protocol.ToolCaller) error {
	var target studio
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := caller.CallTool("list_roblox_studios", map[string]any{})
		if err == nil {
			target, err = parseStudios(result)
		}
		if err == nil {
			break
		}
		lastErr = err
		if attempt < attempts-1 {
			time.Sleep(retryDelay)
		}
	}
	if target.ID == "" {
		if lastErr == nil {
			lastErr = errors.New("no connected Studio instance")
		}
		return lastErr
	}

	_, err := caller.CallTool("execute_luau", map[string]any{
		"code":           code(target.Name),
		"datamodel_type": "Edit",
		"studio_id":      target.ID,
	})
	return err
}

func parseStudios(result protocol.ToolResult) (studio, error) {
	text, err := firstText(result)
	if err != nil {
		return studio{}, err
	}
	var listing struct {
		Studios []studio `json:"studios"`
	}
	if err := json.Unmarshal([]byte(text), &listing); err != nil {
		return studio{}, fmt.Errorf("parse Studio details: %w", err)
	}
	if len(listing.Studios) == 0 {
		return studio{}, errors.New("no connected Studio instance")
	}
	return listing.Studios[0], nil
}

func firstText(result protocol.ToolResult) (string, error) {
	for _, content := range result.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", errors.New("Studio response was empty")
}

func code(studioName string) string {
	studioName = strings.NewReplacer("\r", " ", "\n", " ").Replace(studioName)
	rule := strings.Repeat("━", width)
	lines := []string{
		"",
		rule,
		center("⚡ G O J O   M C P  //  O N L I N E ⚡", width),
		"   STATUS  :  ONLINE 🟢",
		"   LINK    :  ROBLOX STUDIO 💻",
		"   TARGET  :  " + studioName + " 🏷️",
		"   READY   :  TOOLS DISCOVERABLE 🔍",
		rule,
	}
	return "print(" + quote(strings.Join(lines, "\n")) + ")"
}

func center(value string, targetWidth int) string {
	valueWidth := len([]rune(value))
	if valueWidth >= targetWidth {
		return value
	}
	return strings.Repeat(" ", (targetWidth-valueWidth)/2) + value
}

func quote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}
