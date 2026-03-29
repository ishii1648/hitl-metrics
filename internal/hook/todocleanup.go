package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunTodoCleanup handles the SessionStart hook for TODO cleanup.
// On main branch, moves completed TODO tasks (all criteria checked) to CHANGELOG.md.
func RunTodoCleanup(input *HookInput) error {
	cwd := input.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Only run on main branch
	branch := getCurrentBranch(cwd)
	if branch != "main" {
		return nil
	}

	todoPath := filepath.Join(cwd, "TODO.md")
	changelogPath := filepath.Join(cwd, "CHANGELOG.md")

	// Quick check: file exists and has checked items
	content, err := os.ReadFile(todoPath)
	if err != nil || !strings.Contains(string(content), "[x]") {
		return nil
	}

	// Parse and extract completed tasks
	remaining, completedNames := ParseTodoAndExtract(string(content))
	if len(completedNames) == 0 {
		return nil
	}

	// Update CHANGELOG.md (only if file exists, matching original behavior)
	if _, err := os.Stat(changelogPath); err == nil {
		today := time.Now().Format("2006-01-02")
		if err := updateChangelog(changelogPath, today, completedNames); err != nil {
			return err
		}
	}

	// Write updated TODO.md
	if err := os.WriteFile(todoPath, []byte(remaining), 0644); err != nil {
		return err
	}

	fmt.Printf("完了済みタスク %d 件を CHANGELOG.md に移動しました。\n", len(completedNames))
	return nil
}

func getCurrentBranch(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// todoTask represents a task block being parsed in TODO.md.
type todoTask struct {
	lines     []string
	name      string
	criteria  int
	unchecked int
}

// ParseTodoAndExtract parses TODO.md content, removes tasks with all criteria
// checked from the "## 未着手" section, and returns remaining content and
// completed task names.
func ParseTodoAndExtract(content string) (remaining string, completedNames []string) {
	lines := strings.Split(content, "\n")

	var out []string
	var current *todoTask
	inSection := false

	flush := func() {
		if current == nil {
			return
		}
		if current.criteria > 0 && current.unchecked == 0 {
			completedNames = append(completedNames, current.name)
		} else {
			out = append(out, current.lines...)
		}
		current = nil
	}

	for _, line := range lines {
		// Detect target section start
		if line == "## 未着手" {
			inSection = true
			out = append(out, line)
			continue
		}

		// Detect section boundary
		if strings.HasPrefix(line, "## ") {
			if inSection {
				flush()
				inSection = false
			}
			out = append(out, line)
			continue
		}

		// Outside target section: pass through
		if !inSection {
			out = append(out, line)
			continue
		}

		// Inside target section: task header
		if strings.HasPrefix(line, "- ") {
			flush()
			current = &todoTask{
				lines: []string{line},
				name:  strings.TrimPrefix(line, "- "),
			}
			continue
		}

		// Sub-item (indented line starting with -)
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "- ") && current != nil {
			current.lines = append(current.lines, line)
			if strings.Contains(line, "[x]") {
				current.criteria++
			}
			if strings.Contains(line, "[ ]") {
				current.criteria++
				current.unchecked++
			}
			continue
		}

		// Empty line: flush current task
		if strings.TrimSpace(line) == "" {
			if current != nil {
				flush()
			}
			out = append(out, line)
			continue
		}

		// Other content
		out = append(out, line)
	}

	// Flush any remaining task at EOF
	if inSection {
		flush()
	}

	return strings.Join(out, "\n"), completedNames
}

func updateChangelog(path, today string, names []string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	todayHeader := "## " + today

	var entries []string
	for _, name := range names {
		entries = append(entries, "- "+name)
	}

	var out []string
	done := false

	for i := 0; i < len(lines); i++ {
		// Today's section already exists: insert entries after header + blank line
		if !done && lines[i] == todayHeader {
			out = append(out, lines[i])
			if i+1 < len(lines) {
				i++
				if strings.TrimSpace(lines[i]) == "" {
					out = append(out, "")
					out = append(out, entries...)
				} else {
					out = append(out, "")
					out = append(out, entries...)
					out = append(out, lines[i])
				}
			}
			done = true
			continue
		}

		// Insert before first existing date section
		if !done && strings.HasPrefix(lines[i], "## ") && len(lines[i]) > 3 &&
			lines[i][3] >= '0' && lines[i][3] <= '9' {
			out = append(out, todayHeader)
			out = append(out, "")
			out = append(out, entries...)
			out = append(out, "")
			done = true
		}

		out = append(out, lines[i])
	}

	if !done {
		out = append(out, todayHeader)
		out = append(out, "")
		out = append(out, entries...)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}
