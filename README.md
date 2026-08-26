# Gojo

Gojo is a small Windows MCP proxy for the Roblox Studio built-in MCP server.

When a CLI starts Gojo as an MCP server, Gojo launches Roblox's local server and forwards its stdio traffic. That makes Roblox's tools discoverable to the CLI without exposing a second custom tool list.

The optional watcher mode runs with Windows and watches for `RobloxStudioBeta.exe`. When Studio opens, it starts:

```text
cmd.exe /c cd /d %LOCALAPPDATA%\Roblox && .\mcp.bat
```

It performs the MCP handshake, stays idle, and stops the MCP process when Studio closes.

## CLI MCP configuration

Point your MCP client at `gojo.exe`:

```json
{
  "mcpServers": {
    "Gojo": {
      "command": "C:\\Users\\TheRe\\Desktop\\Gojo\\gojo.exe"
    }
  }
}
```

Gojo forwards `initialize`, `tools/list`, `tools/call`, and other MCP messages to Roblox Studio. It also adds one local, discoverable tool: `pull_roblox_scripts`.
CLI diagnostics go to stderr so stdout remains valid MCP JSON. Gojo logs tool names and state changes without logging tool arguments or polling activity.
When the background watcher connects, it also writes a single branded Gojo MCP banner into the Roblox Studio Output window.

## Pull Studio scripts

Call `pull_roblox_scripts` from any MCP client connected to Gojo. It runs a
Studio-side bulk export through `execute_luau`, keeping normal sources batched
and splitting only oversized sources into continuations, then writes a local
project with `place.json` and mapped `.luau` files under `src/`.

```json
{
  "output_dir": "./roblox-place",
  "studio_id": "optional-when-one-studio-is-open",
  "services": ["ReplicatedStorage", "ServerScriptService"],
  "max_depth": 10
}
```

`output_dir`, `services`, and `max_depth` are optional. If more than one Studio
is open, provide `studio_id` to choose the target. The pull tool is implemented
inside Gojo and does not depend on Abraxius.

The Go code is split into child packages under `internal/`: MCP transport and
proxying, Studio banner, Windows process helpers, supervision, logging, and
script pulling.

## Install automatic startup

Run PowerShell from this directory:

```powershell
.\install.ps1
```

This builds a hidden `gojo.exe` and adds a per-user Windows Startup shortcut for `gojo.exe --watch`. No administrator permissions are required.

To remove automatic startup:

```powershell
.\uninstall.ps1
```

Logs are written to `%LOCALAPPDATA%\Gojo\gojo.log`.

## Development

```powershell
go run .
go run . --watch
go run . --watch --console
go test ./...
```
