package banner

import (
	"strings"
	"testing"
)

func TestCodeLayout(t *testing.T) {
	result := code("Sun City RP (placeId: 13745245092)")
	for _, want := range []string{
		`\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`,
		`⚡ G O J O   M C P  //  O N L I N E ⚡`,
		`ONLINE 🟢`,
		`ROBLOX STUDIO 💻`,
		`TOOLS DISCOVERABLE 🔍`,
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("banner missing %q in %q", want, result)
		}
	}
}
