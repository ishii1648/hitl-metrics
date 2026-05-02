package doctor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ishii1648/hitl-metrics/internal/agent"
)

// fakeLoader returns a SettingsLoader that yields the given commands per
// (agent name → event → command) lookup.
func fakeLoader(byAgent map[string]map[string][]string) func(*agent.Agent) map[string][]string {
	return func(a *agent.Agent) map[string][]string {
		return byAgent[a.Name]
	}
}

func envWith(t *testing.T, agents []*agent.Agent, lookOK bool, settings map[string]map[string][]string) Env {
	t.Helper()
	look := func(string) (string, error) { return "/usr/local/bin/hitl-metrics", nil }
	if !lookOK {
		look = func(string) (string, error) { return "", errors.New("not found") }
	}
	return Env{
		LookPath:       look,
		BinaryName:     "hitl-metrics",
		Agents:         agents,
		SettingsLoader: fakeLoader(settings),
	}
}

func TestRun_ClaudeAllChecksPass(t *testing.T) {
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameClaude, DataDir: dir}

	env := envWith(t, []*agent.Agent{a}, true, map[string]map[string][]string{
		agent.NameClaude: {
			"SessionStart": {"hitl-metrics hook session-start", "hitl-metrics hook todo-cleanup"},
			"SessionEnd":   {"hitl-metrics hook session-end"},
			"Stop":         {"hitl-metrics hook stop"},
		},
	})

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if r.HasFailure() {
		t.Fatalf("expected no failures, got: %s", buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"binary at /usr/local/bin/hitl-metrics",
		"[claude] data dir at " + dir,
		"[claude] hook registration:",
		"SessionStart: session-start ✓",
		"Stop: stop ✓",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestRun_CodexOptionalDoesNotFail(t *testing.T) {
	dir := t.TempDir()
	a := &agent.Agent{Name: agent.NameCodex, DataDir: dir}

	env := envWith(t, []*agent.Agent{a}, true, map[string]map[string][]string{
		agent.NameCodex: {
			"SessionStart": {"hitl-metrics hook session-start --agent codex"},
			"Stop":         {"hitl-metrics hook stop --agent codex"},
			// PostToolUse intentionally missing (optional)
		},
	})

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if r.HasFailure() {
		t.Fatalf("missing optional should not fail: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "post-tool-use ⚠ (optional") {
		t.Fatalf("optional warning missing:\n%s", buf.String())
	}
}

func TestRun_BothAgentsListed(t *testing.T) {
	dir := t.TempDir()
	c := &agent.Agent{Name: agent.NameClaude, DataDir: filepath.Join(dir, ".claude")}
	x := &agent.Agent{Name: agent.NameCodex, DataDir: filepath.Join(dir, ".codex")}
	os.MkdirAll(c.DataDir, 0755)
	os.MkdirAll(x.DataDir, 0755)

	env := envWith(t, []*agent.Agent{c, x}, true, map[string]map[string][]string{
		agent.NameClaude: {},
		agent.NameCodex:  {},
	})

	var buf bytes.Buffer
	if _, err := RunWith(&buf, env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[claude] hook registration") ||
		!strings.Contains(buf.String(), "[codex] hook registration") {
		t.Errorf("expected both agents listed:\n%s", buf.String())
	}
}

func TestRun_BinaryMissing(t *testing.T) {
	dir := t.TempDir()
	env := envWith(t, []*agent.Agent{{Name: agent.NameClaude, DataDir: dir}}, false, nil)
	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasFailure() {
		t.Fatal("expected failure when binary missing")
	}
}

func TestIsRegistered_MatchesLooseCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		sub  string
		want bool
	}{
		{"exact", "hitl-metrics hook session-start", "session-start", true},
		{"with --agent", "hitl-metrics hook session-start --agent codex", "session-start", true},
		{"absolute path", "/usr/local/bin/hitl-metrics hook stop", "stop", true},
		{"different sub", "hitl-metrics hook session-end", "session-start", false},
		{"unrelated", "/other/script.sh stop", "stop", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRegistered([]string{tc.cmd}, tc.sub)
			if got != tc.want {
				t.Fatalf("isRegistered(%q, %q) = %v, want %v", tc.cmd, tc.sub, got, tc.want)
			}
		})
	}
}
