package setup

import "testing"

// SetSettingsPathForTest overrides settingsPathFn for the duration of t.
func SetSettingsPathForTest(t *testing.T, path string) {
	t.Helper()
	orig := settingsPathFn
	settingsPathFn = func() string { return path }
	t.Cleanup(func() { settingsPathFn = orig })
}
