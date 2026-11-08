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
      "command": "C:\\Users\\TheRe\\AppData\\Local\\Programs\\Gojo\\gojo.exe"
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

The same pull runs from the command line without an MCP client:

```powershell
gojo --pull "C:\path\to\project"
gojo --pull ./roblox-place --services ReplicatedStorage,ServerScriptService --max-depth 25
gojo --pull ./roblox-place --studio "Sun City RP"
gojo --help
```

Instance names containing dots (for example `haedrix_react@17.3.10`) stay a
single folder, and same-name siblings of different classes (a `Script` and a
`LocalScript` both named `Boot`) are both written. If two Studio paths would
map to one file (a child named `init`), the pull keeps the parent file and reports the collision count instead of silently overwriting.

The Go code is split into child packages under `internal/`: MCP transport and
proxying, Studio banner, Windows process helpers, supervision, logging, and
script pulling.

## Windows installer

Download and run `Gojo-Setup-<version>-win64.exe`. The installer puts Gojo in
`%LOCALAPPDATA%\Programs\Gojo`, registers it in Windows Installed Apps, and
offers a per-user startup shortcut for the watcher. It displays the exact
`gojo.exe` path to copy into an MCP client, installs a Getting Started guide
and Start menu shortcuts, and can open the guide after setup. Administrator
access is not required. Uninstall it from Windows Installed Apps or the Start
menu. The uninstall process stops the installed Gojo process before removing
its files. The installer does not configure an MCP client automatically.

To build the installer, install Go 1.22 or newer and Inno Setup 6, then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build-installer.ps1 -Version 0.1.0
```

The installer is written to `dist\Gojo-Setup-0.1.0-win64.exe`.
The executable and installer use `assets\gojo.ico`. To regenerate it from a
replacement PNG and update the executable resource, run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\tools\convert-icon.ps1
go run github.com/akavel/rsrc@v0.10.2 -ico assets\gojo.ico -o rsrc_windows_amd64.syso
```

## Install automatic startup from source

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
