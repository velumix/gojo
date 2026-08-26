package pull

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/velumix/gojo/internal/protocol"
)

const (
	ToolName        = "pull_roblox_scripts"
	toolAttempts    = 5
	toolRetryDelay  = time.Second
	bulkChunkSize   = 50
	minChunkSize    = 1
	maxPayloadSize  = 90000
	sourceChunkSize = 40000
)

var DefaultServices = []string{
	"Workspace",
	"Lighting",
	"ReplicatedFirst",
	"ReplicatedStorage",
	"ServerScriptService",
	"ServerStorage",
	"StarterGui",
	"StarterPack",
	"StarterPlayer",
	"SoundService",
	"Teams",
	"Chat",
	"TextChatService",
}

type SessionFactory func() (protocol.ToolCaller, io.Closer, error)

type Tool struct {
	factory SessionFactory
	caller  protocol.ToolCaller
}

type Options struct {
	StudioID  string
	OutputDir string
	Services  []string
	MaxDepth  int
}

type Result struct {
	StudioName string
	OutputDir  string
	Scripts    int
	Skipped    int
	Failed     int
	Chunks     int
}

type scriptFile struct {
	ClassName    string `json:"className"`
	Path         string `json:"path"`
	Source       string `json:"source"`
	HasChildren  bool   `json:"hasChildren"`
	SourceOffset int    `json:"sourceOffset"`
	SourceLength int    `json:"sourceLength"`
	Complete     bool   `json:"complete"`
	Encoding     string `json:"encoding"`
}

type bulkResponse struct {
	Total   int          `json:"total"`
	Offset  int          `json:"offset"`
	Scripts []scriptFile `json:"scripts"`
}

func NewTool(factory SessionFactory) *Tool {
	return &Tool{factory: factory}
}

func NewToolWithCaller(caller protocol.ToolCaller) *Tool {
	return &Tool{caller: caller}
}

func (t *Tool) Handles(name string) bool {
	return name == ToolName
}

func (t *Tool) Definitions() []protocol.ToolDefinition {
	return []protocol.ToolDefinition{{
		Name: ToolName,
		Description: "Pull Roblox Script, LocalScript, and ModuleScript sources from Studio in bulk into a local Gojo project. " +
			"Uses one Studio-side execute_luau export per chunk and writes place.json plus mapped .luau files under src/. " +
			"The first connected Studio is selected when studio_id is omitted.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"studio_id": map[string]any{
					"type":        "string",
					"description": "Roblox Studio id. Omit when exactly one Studio is connected.",
				},
				"output_dir": map[string]any{
					"type":        "string",
					"description": "Local destination directory. Defaults to ./roblox-place.",
				},
				"services": map[string]any{
					"type":        "array",
					"description": "Optional service names to pull. Defaults to common script services.",
					"items":       map[string]any{"type": "string"},
				},
				"max_depth": map[string]any{
					"type":        "integer",
					"description": "Maximum hierarchy depth per service. Studio limits this to 10. Defaults to 10.",
					"minimum":     1,
					"maximum":     10,
				},
			},
		},
		Annotations: map[string]any{
			"title":           "Pull Roblox Scripts",
			"readOnlyHint":    false,
			"destructiveHint": false,
			"idempotentHint":  true,
			"openWorldHint":   false,
		},
	}}
}

func (t *Tool) Call(name string, arguments map[string]any) (protocol.ToolResult, error) {
	if !t.Handles(name) {
		return protocol.ToolResult{}, fmt.Errorf("unknown Gojo tool: %s", name)
	}
	options, err := parseOptions(arguments)
	if err != nil {
		return protocol.ToolResult{}, err
	}
	caller := t.caller
	var closer io.Closer
	if caller == nil {
		if t.factory == nil {
			return protocol.ToolResult{}, fmt.Errorf("%s is not configured", ToolName)
		}
		caller, closer, err = t.factory()
		if err != nil {
			return protocol.ToolResult{}, err
		}
	}
	if closer != nil {
		defer closer.Close()
	}

	result, err := Pull(caller, options)
	if err != nil {
		return protocol.ToolResult{}, err
	}
	return protocol.TextResult(formatResult(result), false), nil
}

func Pull(caller protocol.ToolCaller, options Options) (Result, error) {
	if caller == nil {
		return Result{}, fmt.Errorf("Studio MCP caller is unavailable")
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = 10
	}
	if options.MaxDepth < 1 || options.MaxDepth > 10 {
		return Result{}, fmt.Errorf("max_depth must be between 1 and 10")
	}
	if len(options.Services) == 0 {
		options.Services = append([]string(nil), DefaultServices...)
	}

	studioID, studioName, err := resolveStudio(caller, options.StudioID)
	if err != nil {
		return Result{}, err
	}

	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = "roblox-place"
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}

	result := Result{StudioName: studioName, OutputDir: outputDir}
	filesByPath := make(map[string]scriptFile)
	offset := 0
	for {
		chunk, err := readBulkChunk(caller, studioID, options.Services, options.MaxDepth, offset)
		if err != nil {
			return Result{}, err
		}
		result.Chunks++
		for _, file := range chunk.Scripts {
			if err := decodeSource(&file); err != nil {
				return Result{}, fmt.Errorf("decode source %s: %w", file.Path, err)
			}
			file.Path = normalizePath(file.Path)
			if file.Path != "" && scriptExtension(file.ClassName) != "" {
				filesByPath[file.Path] = file
			} else {
				result.Skipped++
			}
		}
		if len(chunk.Scripts) == 0 || offset+len(chunk.Scripts) >= chunk.Total {
			break
		}
		offset += len(chunk.Scripts)
	}

	if err := completeSources(caller, studioID, filesByPath); err != nil {
		return Result{}, err
	}

	files := make([]scriptFile, 0, len(filesByPath))
	for _, file := range filesByPath {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	tree := map[string]any{"$className": "DataModel"}
	for _, file := range files {
		service, localPath, err := localScriptPath(outputDir, file)
		if err != nil {
			result.Skipped++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return Result{}, fmt.Errorf("create %s directory: %w", file.Path, err)
		}
		if err := os.WriteFile(localPath, []byte(file.Source), 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", file.Path, err)
		}
		tree[service] = map[string]any{
			"$path": filepath.ToSlash(filepath.Join("src", service)),
		}
		result.Scripts++
	}

	if err := writeProject(outputDir, tree); err != nil {
		return Result{}, err
	}
	return result, nil
}

func readBulkChunk(caller protocol.ToolCaller, studioID string, services []string, maxDepth, offset int) (bulkResponse, error) {
	limit := bulkChunkSize
	var lastErr error
	for limit >= minChunkSize {
		response, err := callWithRetry(caller, "execute_luau", map[string]any{
			"code":           bulkCode(offset, limit, services, maxDepth),
			"datamodel_type": "Edit",
			"studio_id":      studioID,
		})
		if err == nil {
			chunk, parseErr := parseBulkResponse(response)
			if parseErr == nil {
				return chunk, nil
			}
			lastErr = parseErr
		} else {
			lastErr = err
		}
		limit /= 2
	}
	return bulkResponse{}, fmt.Errorf("bulk export chunk at offset %d: %w", offset, lastErr)
}

func completeSources(caller protocol.ToolCaller, studioID string, files map[string]scriptFile) error {
	for path, file := range files {
		if file.Complete {
			continue
		}
		if file.SourceLength < len(file.Source) {
			return fmt.Errorf("source length mismatch for %s", path)
		}
		for len(file.Source) < file.SourceLength {
			chunk, err := readSourceChunk(caller, studioID, path, len(file.Source))
			if err != nil {
				return fmt.Errorf("read remaining source for %s: %w", path, err)
			}
			if len(chunk.Scripts) != 1 {
				return fmt.Errorf("source chunk for %s was empty", path)
			}
			next := chunk.Scripts[0]
			if err := decodeSource(&next); err != nil {
				return fmt.Errorf("decode source chunk for %s: %w", path, err)
			}
			if next.SourceOffset != len(file.Source) || len(next.Source) == 0 {
				return fmt.Errorf("source chunk for %s was out of order", path)
			}
			file.Source += next.Source
			file.Complete = next.Complete || len(file.Source) >= file.SourceLength
		}
		files[path] = file
	}
	return nil
}

func readSourceChunk(caller protocol.ToolCaller, studioID, path string, offset int) (bulkResponse, error) {
	response, err := callWithRetry(caller, "execute_luau", map[string]any{
		"code":           sourceChunkCode(path, offset, sourceChunkSize),
		"datamodel_type": "Edit",
		"studio_id":      studioID,
	})
	if err != nil {
		return bulkResponse{}, err
	}
	return parseBulkResponse(response)
}

func bulkCode(offset, limit int, services []string, maxDepth int) string {
	servicesJSON, _ := json.Marshal(services)
	return fmt.Sprintf(`local HttpService = game:GetService("HttpService")
local requested = HttpService:JSONDecode(%q)
local offset = %d
local limit = %d
local maxDepth = %d
local maxPayloadSize = %d
local maxSourceSize = %d
local scripts = {}
local total = 0
local payloadSize = 0
local collecting = true

local function rootService(instance)
	local cursor = instance
	local depth = 0
	while cursor.Parent and cursor.Parent ~= game do
		cursor = cursor.Parent
		depth += 1
	end
	return cursor.Name, depth
end

local function allowed(instance)
	if not requested then
		return true
	end
	local root, depth = rootService(instance)
	if depth > maxDepth then
		return false
	end
	for _, serviceName in ipairs(requested) do
		if root == serviceName then
			return true
		end
	end
	return false
end

local function pathOf(instance)
	local parts = {}
	local cursor = instance
	while cursor and cursor ~= game do
		table.insert(parts, 1, cursor.Name)
		cursor = cursor.Parent
	end
	return table.concat(parts, ".")
end

local hexDigits = "0123456789abcdef"
local function hexEncode(value)
	local parts = {}
	for index = 1, #value do
		local byte = string.byte(value, index)
		local high = math.floor(byte / 16) + 1
		local low = (byte %% 16) + 1
		parts[index] = string.sub(hexDigits, high, high) .. string.sub(hexDigits, low, low)
	end
	return table.concat(parts)
end

for _, instance in ipairs(game:GetDescendants()) do
	if instance:IsA("LuaSourceContainer") and allowed(instance) then
		total += 1
		if total > offset and #scripts < limit and collecting then
			local path = pathOf(instance)
			local source = instance.Source
			local sourceChunk = source
			local complete = true
			if #source > maxSourceSize then
				sourceChunk = string.sub(source, 1, maxSourceSize)
				complete = false
			end
			local estimatedSize = (#sourceChunk * 3) + (#path * 2) + 256
			if #scripts > 0 and payloadSize + estimatedSize > maxPayloadSize then
				collecting = false
			else
				table.insert(scripts, {
					className = instance.ClassName,
					path = path,
					source = hexEncode(sourceChunk),
					hasChildren = #instance:GetChildren() > 0,
					sourceOffset = 0,
					sourceLength = #source,
					complete = complete,
					encoding = "hex",
				})
				payloadSize += estimatedSize
			end
		end
	end
end

return HttpService:JSONEncode({
	total = total,
	offset = offset,
	scripts = scripts,
})`, string(servicesJSON), offset, limit, maxDepth, maxPayloadSize, sourceChunkSize)
}

func sourceChunkCode(path string, offset, limit int) string {
	pathJSON, _ := json.Marshal(path)
	return fmt.Sprintf(`local HttpService = game:GetService("HttpService")
local requestedPath = HttpService:JSONDecode(%q)
local offset = %d
local limit = %d

local function pathOf(instance)
	local parts = {}
	local cursor = instance
	while cursor and cursor ~= game do
		table.insert(parts, 1, cursor.Name)
		cursor = cursor.Parent
	end
	return table.concat(parts, ".")
end

local hexDigits = "0123456789abcdef"
local function hexEncode(value)
	local parts = {}
	for index = 1, #value do
		local byte = string.byte(value, index)
		local high = math.floor(byte / 16) + 1
		local low = (byte %% 16) + 1
		parts[index] = string.sub(hexDigits, high, high) .. string.sub(hexDigits, low, low)
	end
	return table.concat(parts)
end

for _, instance in ipairs(game:GetDescendants()) do
	if instance:IsA("LuaSourceContainer") and pathOf(instance) == requestedPath then
		local source = instance.Source
		local finish = math.min(#source, offset + limit)
		local chunk = string.sub(source, offset + 1, finish)
		return HttpService:JSONEncode({
			total = 1,
			offset = 0,
			scripts = {{
				className = instance.ClassName,
				path = requestedPath,
				source = hexEncode(chunk),
				hasChildren = #instance:GetChildren() > 0,
				sourceOffset = offset,
				sourceLength = #source,
				complete = finish >= #source,
				encoding = "hex",
			}},
		})
	end
end

return HttpService:JSONEncode({
	total = 0,
	offset = 0,
	scripts = {},
})`, string(pathJSON), offset, limit)
}

func parseBulkResponse(result protocol.ToolResult) (bulkResponse, error) {
	text, err := firstText(result)
	if err != nil {
		return bulkResponse{}, err
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return bulkResponse{}, fmt.Errorf("bulk response did not contain JSON")
	}
	var response bulkResponse
	if err := json.Unmarshal([]byte(text[start:end+1]), &response); err != nil {
		return bulkResponse{}, fmt.Errorf("parse bulk response: %w (payload %d bytes)", err, len(text))
	}
	return response, nil
}

func decodeSource(file *scriptFile) error {
	switch file.Encoding {
	case "", "utf8":
		return nil
	case "hex":
		decoded, err := hex.DecodeString(file.Source)
		if err != nil {
			return err
		}
		file.Source = string(decoded)
		file.Encoding = ""
		return nil
	default:
		return fmt.Errorf("unsupported source encoding %q", file.Encoding)
	}
}

func parseOptions(arguments map[string]any) (Options, error) {
	options := Options{OutputDir: "roblox-place", MaxDepth: 10}
	if value, ok := arguments["studio_id"].(string); ok {
		options.StudioID = strings.TrimSpace(value)
	}
	if value, ok := arguments["output_dir"].(string); ok && strings.TrimSpace(value) != "" {
		options.OutputDir = strings.TrimSpace(value)
	}
	if value, ok := arguments["max_depth"]; ok {
		depth, err := numberValue(value)
		if err != nil {
			return Options{}, fmt.Errorf("max_depth: %w", err)
		}
		options.MaxDepth = depth
	}
	if value, ok := arguments["services"]; ok {
		switch items := value.(type) {
		case []any:
			for _, item := range items {
				service, ok := item.(string)
				if !ok {
					return Options{}, fmt.Errorf("services must be an array of strings")
				}
				options.Services = append(options.Services, service)
			}
		case []string:
			options.Services = append(options.Services, items...)
		default:
			return Options{}, fmt.Errorf("services must be an array of strings")
		}
	}
	return options, nil
}

func numberValue(value any) (int, error) {
	switch value := value.(type) {
	case int:
		return value, nil
	case float64:
		return int(value), nil
	case json.Number:
		parsed, err := strconv.Atoi(string(value))
		return parsed, err
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func resolveStudio(caller protocol.ToolCaller, requestedID string) (string, string, error) {
	if requestedID != "" {
		return requestedID, requestedID, nil
	}
	result, err := callWithRetry(caller, "list_roblox_studios", map[string]any{})
	if err != nil {
		return "", "", fmt.Errorf("list Studios: %w", err)
	}
	text, err := firstText(result)
	if err != nil {
		return "", "", err
	}
	var listing struct {
		Studios []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"studios"`
	}
	if err := json.Unmarshal([]byte(text), &listing); err != nil {
		return "", "", fmt.Errorf("parse Studios: %w", err)
	}
	if len(listing.Studios) == 0 {
		return "", "", fmt.Errorf("no connected Roblox Studio instances")
	}
	if len(listing.Studios) > 1 {
		return "", "", fmt.Errorf("studio_id is required when multiple Studios are connected")
	}
	return listing.Studios[0].ID, listing.Studios[0].Name, nil
}

func callWithRetry(caller protocol.ToolCaller, name string, arguments map[string]any) (protocol.ToolResult, error) {
	var lastErr error
	for attempt := 0; attempt < toolAttempts; attempt++ {
		result, err := caller.CallTool(name, arguments)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt < toolAttempts-1 {
			time.Sleep(toolRetryDelay)
		}
	}
	return protocol.ToolResult{}, lastErr
}

func localScriptPath(outputDir string, file scriptFile) (string, string, error) {
	parts := strings.Split(normalizePath(file.Path), ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid script path %q", file.Path)
	}
	service := sanitizeName(parts[0])
	name := sanitizeName(parts[len(parts)-1])
	directories := make([]string, 0, len(parts)-1)
	for _, part := range parts[1 : len(parts)-1] {
		directories = append(directories, sanitizeName(part))
	}

	base := filepath.Join(outputDir, "src", service)
	if len(directories) > 0 {
		base = filepath.Join(append([]string{base}, directories...)...)
	}
	extension := scriptExtension(file.ClassName)
	if file.HasChildren {
		base = filepath.Join(base, name)
		return service, filepath.Join(base, "init"+extension), nil
	}
	return service, filepath.Join(base, name+extension), nil
}

func scriptExtension(className string) string {
	switch className {
	case "Script":
		return ".server.luau"
	case "LocalScript":
		return ".client.luau"
	case "ModuleScript":
		return ".luau"
	default:
		return ""
	}
}

func sanitizeName(value string) string {
	value = strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(value)
	if value == "" || value == "." || value == ".." {
		return "_"
	}
	return value
}

func normalizePath(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "game.")
}

func firstText(result protocol.ToolResult) (string, error) {
	for _, content := range result.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", fmt.Errorf("Studio response was empty")
}

func writeProject(outputDir string, tree map[string]any) error {
	name := filepath.Base(filepath.Clean(outputDir))
	project := map[string]any{
		"name":   name,
		"format": "gojo-v1",
		"tree":   tree,
	}
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode place.json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outputDir, "place.json"), data, 0o644); err != nil {
		return fmt.Errorf("write place.json: %w", err)
	}
	return nil
}

func formatResult(result Result) string {
	message := fmt.Sprintf("Pulled %d scripts from %s into %s in %d bulk chunks", result.Scripts, result.StudioName, result.OutputDir, result.Chunks)
	if result.Skipped > 0 || result.Failed > 0 {
		message += fmt.Sprintf(" (%d skipped, %d failed)", result.Skipped, result.Failed)
	}
	return message
}
