package hook

import (
	"encoding/json"
	"testing"
)

func TestAnnotateTool(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		toolInput string
		cwd       string
		want      string
	}{
		{
			name:      "Bash command",
			toolName:  "Bash",
			toolInput: `{"command": "cp /home/user/project/foo.txt /home/user/project/bar.txt"}`,
			cwd:       "/home/user/project",
			want:      "Bash(cp)",
		},
		{
			name:      "Bash empty command",
			toolName:  "Bash",
			toolInput: `{"command": ""}`,
			cwd:       "/home/user/project",
			want:      "Bash",
		},
		{
			name:      "Bash single word command",
			toolName:  "Bash",
			toolInput: `{"command": "ls"}`,
			cwd:       "/home/user/project",
			want:      "Bash(ls)",
		},
		{
			name:      "Read root file",
			toolName:  "Read",
			toolInput: `{"file_path": "/home/user/project/main.go"}`,
			cwd:       "/home/user/project",
			want:      "Read(.)",
		},
		{
			name:      "Read nested file",
			toolName:  "Read",
			toolInput: `{"file_path": "/home/user/project/internal/syncdb/syncdb.go"}`,
			cwd:       "/home/user/project",
			want:      "Read(internal/syncdb)",
		},
		{
			name:      "Read external file",
			toolName:  "Read",
			toolInput: `{"file_path": "/etc/hosts"}`,
			cwd:       "/home/user/project",
			want:      "Read(external)",
		},
		{
			name:      "Write internal file",
			toolName:  "Write",
			toolInput: `{"file_path": "/home/user/project/docs/setup.md"}`,
			cwd:       "/home/user/project",
			want:      "Write(docs/setup.md)",
		},
		{
			name:      "Edit internal file",
			toolName:  "Edit",
			toolInput: `{"file_path": "/home/user/project/grafana/dashboards/hitl-metrics.json"}`,
			cwd:       "/home/user/project",
			want:      "Edit(grafana/dashboards)",
		},
		{
			name:      "Grep with path field",
			toolName:  "Grep",
			toolInput: `{"path": "/home/user/project/src"}`,
			cwd:       "/home/user/project",
			want:      "Grep(.)",
		},
		{
			name:      "Relative internal file",
			toolName:  "Edit",
			toolInput: `{"file_path": "internal/hook/annotate.go"}`,
			cwd:       "/home/user/project",
			want:      "Edit(internal/hook)",
		},
		{
			name:      "Grep external path",
			toolName:  "Grep",
			toolInput: `{"path": "/var/log"}`,
			cwd:       "/home/user/project",
			want:      "Grep(external)",
		},
		{
			name:      "Unknown tool unchanged",
			toolName:  "Agent",
			toolInput: `{}`,
			cwd:       "/home/user/project",
			want:      "Agent",
		},
		{
			name:      "Empty cwd treats as external",
			toolName:  "Read",
			toolInput: `{"file_path": "/home/user/project/main.go"}`,
			cwd:       "",
			want:      "Read(external)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnnotateTool(tt.toolName, json.RawMessage(tt.toolInput), tt.cwd)
			if got != tt.want {
				t.Errorf("AnnotateTool(%q, %s, %q) = %q, want %q",
					tt.toolName, tt.toolInput, tt.cwd, got, tt.want)
			}
		})
	}
}
