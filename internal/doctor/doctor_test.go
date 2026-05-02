package doctor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSettings(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func envWith(t *testing.T, settingsPath, dataDir string, lookOK bool) Env {
	t.Helper()
	look := func(string) (string, error) { return "/usr/local/bin/hitl-metrics", nil }
	if !lookOK {
		look = func(string) (string, error) { return "", errors.New("not found") }
	}
	return Env{
		SettingsPath: settingsPath,
		DataDir:      dataDir,
		LookPath:     look,
		BinaryName:   "hitl-metrics",
	}
}

func TestRun_AllChecksPass(t *testing.T) {
	dir := t.TempDir()
	settingsPath := writeSettings(t, dir, `{
		"hooks": {
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-start"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook todo-cleanup"}]}
			],
			"SessionEnd": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-end", "timeout": 10}]}
			],
			"Stop": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook stop"}]}
			]
		}
	}`)

	env := envWith(t, settingsPath, dir, true)

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
		"data dir at " + dir,
		"hook registration:",
		"SessionStart: session-start ✓",
		"SessionStart: todo-cleanup ✓",
		"SessionEnd: session-end ✓",
		"Stop: stop ✓",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestRun_MissingHookEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	settingsPath := writeSettings(t, dir, `{
		"hooks": {
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-start"}]}
			]
		}
	}`)

	env := envWith(t, settingsPath, dir, true)

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasFailure() {
		t.Fatalf("expected failure, got: %s", buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"⚠ hook registration:",
		"SessionStart: todo-cleanup ✗ (not registered in",
		"SessionEnd: session-end ✗",
		"Stop: stop ✗",
		"register manually or via dotfiles",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
	// 登録済みのものは ✓ で表示されている
	if !strings.Contains(out, "SessionStart: session-start ✓") {
		t.Fatalf("registered hook should be marked ✓:\n%s", out)
	}
}

func TestRun_MissingSettingsFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "does-not-exist.json")

	env := envWith(t, settingsPath, dir, true)

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasFailure() {
		t.Fatalf("expected failure when settings.json missing")
	}
	for _, c := range r.Hooks {
		if c.OK {
			t.Fatalf("expected all hooks to be reported missing, got %+v", c)
		}
	}
}

func TestRun_BinaryNotInPath(t *testing.T) {
	dir := t.TempDir()
	settingsPath := writeSettings(t, dir, `{}`)

	env := envWith(t, settingsPath, dir, false)

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if r.Binary.OK {
		t.Fatal("expected binary check to fail")
	}
	if !r.HasFailure() {
		t.Fatal("expected HasFailure when binary missing")
	}
	if !strings.Contains(buf.String(), "✗ binary:") {
		t.Fatalf("expected binary failure marker:\n%s", buf.String())
	}
}

func TestRun_MissingDataDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-dir")
	settingsPath := writeSettings(t, dir, `{}`)

	env := envWith(t, settingsPath, missing, true)

	var buf bytes.Buffer
	r, err := RunWith(&buf, env)
	if err != nil {
		t.Fatal(err)
	}
	if r.DataDir.OK {
		t.Fatal("expected data dir check to fail")
	}
	if !strings.Contains(buf.String(), "✗ data dir:") {
		t.Fatalf("expected data dir failure marker:\n%s", buf.String())
	}
}

// matcher 付き / コマンドに余分な引数があっても、サブコマンド名と
// "hitl-metrics" を両方含めば登録済みと判定する。
func TestIsRegistered_MatchesLooseCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		sub  string
		want bool
	}{
		{"exact", "hitl-metrics hook session-start", "session-start", true},
		{"with args", "hitl-metrics hook session-start --debug", "session-start", true},
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
