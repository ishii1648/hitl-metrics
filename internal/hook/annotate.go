package hook

import (
	"encoding/json"
	"strings"
)

// AnnotateTool returns a tool name annotated with context information.
//   - Bash → Bash(cmd(internal|external))
//   - Read/Write/Edit/Grep → Tool(internal|external)
//   - Other → unchanged
func AnnotateTool(toolName string, toolInput json.RawMessage, cwd string) string {
	switch toolName {
	case "Bash":
		return annotateBash(toolInput, cwd)
	case "Read", "Write", "Edit", "Grep":
		return annotateFileOp(toolName, toolInput, cwd)
	default:
		return toolName
	}
}

func annotateBash(toolInput json.RawMessage, cwd string) string {
	var input struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(toolInput, &input)

	parts := strings.Fields(input.Command)
	if len(parts) == 0 {
		return "Bash"
	}
	cmd := parts[0]
	lastArg := parts[len(parts)-1]

	loc := classifyLocation(lastArg, cwd)
	return "Bash(" + cmd + "(" + loc + "))"
}

func annotateFileOp(toolName string, toolInput json.RawMessage, cwd string) string {
	var input struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	_ = json.Unmarshal(toolInput, &input)

	fp := input.FilePath
	if fp == "" {
		fp = input.Path
	}

	loc := classifyLocation(fp, cwd)
	return toolName + "(" + loc + ")"
}

// classifyLocation returns "internal" if path is under cwd, "external" otherwise.
func classifyLocation(path, cwd string) string {
	if cwd != "" && path != "" && strings.HasPrefix(path, cwd+"/") {
		return "internal"
	}
	return "external"
}
