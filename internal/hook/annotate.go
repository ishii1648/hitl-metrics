package hook

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// AnnotateTool returns a tool name annotated with context information.
//   - Bash → Bash(cmd)
//   - Read/Write/Edit/Grep → Tool(dir/subdir), Tool(.), or Tool(external)
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
	return "Bash(" + cmd + ")"
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

	pattern := directoryPattern(fp, cwd)
	return toolName + "(" + pattern + ")"
}

func directoryPattern(path, cwd string) string {
	if cwd == "" || path == "" {
		return "external"
	}

	var rel string
	if filepath.IsAbs(path) {
		cleanCWD := filepath.Clean(cwd)
		cleanPath := filepath.Clean(path)
		if cleanPath == cleanCWD {
			return "."
		}
		r, err := filepath.Rel(cleanCWD, cleanPath)
		if err != nil || r == ".." || strings.HasPrefix(r, "../") {
			return "external"
		}
		rel = r
	} else {
		rel = filepath.Clean(path)
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			return "external"
		}
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) <= 1 {
		return "."
	}
	if len(parts) == 2 {
		return filepath.ToSlash(filepath.Join(parts[0], parts[1]))
	}
	return filepath.ToSlash(filepath.Join(parts[0], parts[1]))
}
