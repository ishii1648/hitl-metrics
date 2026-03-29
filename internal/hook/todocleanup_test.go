package hook

import (
	"strings"
	"testing"
)

func TestParseTodoAndExtract(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCompleted  []string
		wantRemaining  string
	}{
		{
			name: "all criteria checked → task removed",
			input: strings.Join([]string{
				"# TODO",
				"",
				"## 未着手",
				"",
				"- タスクA",
				"  - [x] 条件1",
				"  - [x] 条件2",
				"",
				"- タスクB",
				"  - [ ] 条件1",
				"",
				"## 完了",
			}, "\n"),
			wantCompleted: []string{"タスクA"},
			wantRemaining: strings.Join([]string{
				"# TODO",
				"",
				"## 未着手",
				"",
				"",
				"- タスクB",
				"  - [ ] 条件1",
				"",
				"## 完了",
			}, "\n"),
		},
		{
			name: "no completed tasks → no changes",
			input: strings.Join([]string{
				"## 未着手",
				"",
				"- タスクA",
				"  - [ ] 条件1",
				"  - [x] 条件2",
			}, "\n"),
			wantCompleted: nil,
			wantRemaining: strings.Join([]string{
				"## 未着手",
				"",
				"- タスクA",
				"  - [ ] 条件1",
				"  - [x] 条件2",
			}, "\n"),
		},
		{
			name: "multiple completed tasks",
			input: strings.Join([]string{
				"## 未着手",
				"",
				"- タスクA",
				"  - [x] 条件1",
				"",
				"- タスクB",
				"  - [x] 条件1",
				"  - [x] 条件2",
				"",
				"- タスクC",
				"  - [ ] 条件1",
			}, "\n"),
			wantCompleted: []string{"タスクA", "タスクB"},
			wantRemaining: strings.Join([]string{
				"## 未着手",
				"",
				"",
				"",
				"- タスクC",
				"  - [ ] 条件1",
			}, "\n"),
		},
		{
			name: "task without criteria → not removed",
			input: strings.Join([]string{
				"## 未着手",
				"",
				"- タスクA（条件なし）",
				"",
				"- タスクB",
				"  - [x] 条件1",
			}, "\n"),
			wantCompleted: []string{"タスクB"},
			wantRemaining: strings.Join([]string{
				"## 未着手",
				"",
				"- タスクA（条件なし）",
				"",
			}, "\n"),
		},
		{
			name: "other sections untouched",
			input: strings.Join([]string{
				"## 進行中",
				"",
				"- 進行タスク",
				"  - [x] 条件1",
				"",
				"## 未着手",
				"",
				"- タスクA",
				"  - [x] 条件1",
				"",
				"## メモ",
				"",
				"テキスト",
			}, "\n"),
			wantCompleted: []string{"タスクA"},
			wantRemaining: strings.Join([]string{
				"## 進行中",
				"",
				"- 進行タスク",
				"  - [x] 条件1",
				"",
				"## 未着手",
				"",
				"",
				"## メモ",
				"",
				"テキスト",
			}, "\n"),
		},
		{
			name:          "no 未着手 section → no changes",
			input:         "## 進行中\n\n- タスク\n  - [x] 条件\n",
			wantCompleted: nil,
			wantRemaining: "## 進行中\n\n- タスク\n  - [x] 条件\n",
		},
		{
			name: "task at EOF without trailing newline",
			input: strings.Join([]string{
				"## 未着手",
				"- タスクA",
				"  - [x] 条件1",
			}, "\n"),
			wantCompleted: []string{"タスクA"},
			wantRemaining: "## 未着手",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining, completed := ParseTodoAndExtract(tt.input)

			// Check completed names
			if len(completed) != len(tt.wantCompleted) {
				t.Errorf("completed count = %d, want %d\ncompleted: %v",
					len(completed), len(tt.wantCompleted), completed)
			} else {
				for i, name := range completed {
					if name != tt.wantCompleted[i] {
						t.Errorf("completed[%d] = %q, want %q", i, name, tt.wantCompleted[i])
					}
				}
			}

			// Check remaining content
			if remaining != tt.wantRemaining {
				t.Errorf("remaining mismatch:\ngot:\n%s\nwant:\n%s",
					remaining, tt.wantRemaining)
			}
		})
	}
}
