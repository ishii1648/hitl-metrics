package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookDef maps a hook event name to the script filename.
var hookDefs = []struct {
	Event  string
	Script string
}{
	{"SessionStart", "session-index.sh"},
	{"PermissionRequest", "permission-log.sh"},
	{"Stop", "stop.sh"},
}

// settingsPathFn returns ~/.claude/settings.json. Replaceable in tests.
var settingsPathFn = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// Run registers hitl-metrics hooks into ~/.claude/settings.json.
// hooksDir is the absolute path to the hooks/ directory.
// Idempotent: skips hooks whose command path is already registered.
func Run(hooksDir string) error {
	path := settingsPathFn()

	// Read existing settings or start with empty object
	settings := make(map[string]json.RawMessage)
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Parse hooks section
	hooks := make(map[string]json.RawMessage)
	if raw, ok := settings["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("parse hooks: %w", err)
		}
	}

	added := 0
	skipped := 0

	for _, def := range hookDefs {
		scriptPath := filepath.Join(hooksDir, def.Script)

		// Verify script exists
		if _, err := os.Stat(scriptPath); err != nil {
			return fmt.Errorf("hook script not found: %s", scriptPath)
		}

		// Parse existing entries for this event
		var entries []hookEntry
		if raw, ok := hooks[def.Event]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				return fmt.Errorf("parse hooks.%s: %w", def.Event, err)
			}
		}

		// Check if already registered
		if containsCommand(entries, scriptPath) {
			fmt.Printf("install: %s — スキップ（登録済み）\n", def.Event)
			skipped++
			continue
		}

		// Append new entry
		entries = append(entries, hookEntry{
			Matcher: "",
			Hooks: []hookCommand{{
				Type:    "command",
				Command: scriptPath,
			}},
		})

		raw, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		hooks[def.Event] = json.RawMessage(raw)
		fmt.Printf("install: %s — 登録\n", def.Event)
		added++
	}

	// Write back
	hooksRaw, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = json.RawMessage(hooksRaw)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	fmt.Printf("install: 完了 — 追加 %d / スキップ %d（%s）\n", added, skipped, path)
	return nil
}

type hookEntry struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookCommand `json:"hooks"`
}

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// containsCommand checks if any entry already references the given command path.
func containsCommand(entries []hookEntry, command string) bool {
	for _, e := range entries {
		for _, h := range e.Hooks {
			if h.Command == command {
				return true
			}
		}
	}
	return false
}
