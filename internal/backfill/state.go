package backfill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// State holds the cursor for incremental backfill processing.
type State struct {
	LastBackfillOffset int       `json:"last_backfill_offset"`
	LastMetaCheck      time.Time `json:"last_meta_check"`
}

// StatePath returns the default path to hitl-metrics-state.json.
func StatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "hitl-metrics-state.json")
}

// LoadState reads the state file. Returns zero state if the file doesn't exist.
func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, nil
	}
	return s, nil
}

// SaveState writes the state file atomically.
func SaveState(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "hitl-metrics-state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
