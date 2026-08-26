# Gojo

Gojo is a small Windows background supervisor for the Roblox Studio built-in MCP server.

When Gojo is running, it watches for `RobloxStudioBeta.exe`. When Studio opens, Gojo starts:

```text
cmd.exe /c cd /d %LOCALAPPDATA%\Roblox && .\mcp.bat
```

It performs the MCP handshake, stays idle, and stops the MCP process when Studio closes.

## Install automatic startup

Run PowerShell from this directory:

```powershell
.\install.ps1
```

This builds a hidden `gojo.exe` and adds a per-user Windows Startup shortcut. No administrator permissions are required.

To remove automatic startup:

```powershell
.\uninstall.ps1
```

Logs are written to `%LOCALAPPDATA%\Gojo\gojo.log`.

## Development

```powershell
go run .
go test ./...
```
