package install

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed hooks
var embeddedHooks embed.FS

// DefaultHooksDir returns ~/.local/share/hitl-metrics/hooks.
func DefaultHooksDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "hitl-metrics", "hooks")
}

// ExtractHooks writes embedded hook scripts to destDir with executable permissions.
func ExtractHooks(destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(embeddedHooks, "hooks", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := embeddedHooks.ReadFile(path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.Base(path))
		return os.WriteFile(dest, data, 0755)
	})
}
