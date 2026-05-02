package e2e_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ishii1648/hitl-metrics/internal/syncdb"
)

// TestGenTestDB generates e2e/testdata/hitl-metrics.db from test fixtures.
// Run: CGO_ENABLED=0 GOTOOLCHAIN=local go test -run TestGenTestDB -v ./e2e/
func TestGenTestDB(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatal(err)
	}

	indexPath := filepath.Join(projectRoot, "e2e", "testdata", "session-index.jsonl")
	dbPath := filepath.Join(projectRoot, "e2e", "testdata", "hitl-metrics.db")

	tmpIndex := filepath.Join(t.TempDir(), "session-index.jsonl")
	if err := rewriteFixture(indexPath, tmpIndex, projectRoot, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	if err := syncdb.RunWithPaths(tmpIndex, dbPath); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Generated %s (%d bytes)", dbPath, info.Size())
}

// rewriteFixture reads the fixture JSONL, resolves relative transcript paths,
// and shifts every timestamp / ended_at by a single offset so the latest entry
// lands at refTime. This keeps the fixture readable as fixed dates while making
// the generated DB always fall within "Last 30 days" at test execution time.
func rewriteFixture(src, dst, root string, refTime time.Time) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	type entry struct {
		raw map[string]interface{}
	}
	var entries []entry
	var maxTS time.Time
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		entries = append(entries, entry{raw: m})
		for _, key := range []string{"timestamp", "ended_at"} {
			if s, ok := m[key].(string); ok && s != "" {
				if t, err := time.Parse(time.RFC3339, s); err == nil && t.After(maxTS) {
					maxTS = t
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	offset := refTime.Sub(maxTS)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	w := bufio.NewWriter(out)

	for i, e := range entries {
		m := e.raw
		if t, ok := m["transcript"].(string); ok && t != "" && !filepath.IsAbs(t) {
			m["transcript"] = filepath.Join(root, t)
		}
		for _, key := range []string{"timestamp", "ended_at"} {
			if s, ok := m[key].(string); ok && s != "" {
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					m[key] = t.Add(offset).UTC().Format(time.RFC3339)
				}
			}
		}
		rewritten, err := json.Marshal(m)
		if err != nil {
			continue
		}
		if i > 0 {
			w.WriteByte('\n')
		}
		w.Write(rewritten)
	}
	w.WriteByte('\n')
	return w.Flush()
}
