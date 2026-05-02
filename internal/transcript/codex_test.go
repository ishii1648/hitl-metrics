package transcript

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// writeRollout writes Codex rollout JSONL lines to a temp file and returns
// its path. Each line should be a complete JSON object.
func writeRollout(t *testing.T, name string, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	f.Close()
	return p
}

const codexUserMessage = `{"timestamp":"2026-05-02T12:51:25.022Z","type":"event_msg","payload":{"type":"user_message","message":"hello"}}`
const codexAgentMessage = `{"timestamp":"2026-05-02T12:51:55.881Z","type":"event_msg","payload":{"type":"agent_message","message":"hi"}}`
const codexTokenCountFirst = `{"timestamp":"2026-05-02T12:51:55.899Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":50,"reasoning_output_tokens":10,"total_tokens":160}}}}`
const codexTokenCountLast = `{"timestamp":"2026-05-02T13:00:50.036Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"cached_input_tokens":120,"output_tokens":150,"reasoning_output_tokens":30,"total_tokens":480}}}}`
const codexTurnContext = `{"timestamp":"2026-05-02T12:51:25.021Z","type":"turn_context","payload":{"turn_id":"t1","cwd":"/x","model":"gpt-5.5"}}`
const codexFunctionCall = `{"timestamp":"2026-05-02T13:14:13.535Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{}"}}`
const codexCustomToolCall = `{"timestamp":"2026-05-02T13:14:14.000Z","type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch"}}`
const codexSessionMeta = `{"timestamp":"2026-05-02T12:51:25.020Z","type":"session_meta","payload":{"id":"abc","cli_version":"0.128.0-alpha.1","originator":"Codex Desktop"}}`

func TestParseCodex_BasicCounters(t *testing.T) {
	p := writeRollout(t, "rollout.jsonl", []string{
		codexSessionMeta,
		codexTurnContext,
		codexUserMessage,
		codexFunctionCall,
		codexFunctionCall,
		codexCustomToolCall,
		codexTokenCountFirst,
		codexAgentMessage,
		codexTokenCountLast,
		// second user_message → +1 mid_session
		`{"timestamp":"2026-05-02T13:00:50.040Z","type":"event_msg","payload":{"type":"user_message","message":"again"}}`,
	})

	s := ParseCodex(p)
	if s.ToolUseTotal != 3 {
		t.Errorf("tool_use_total = %d, want 3", s.ToolUseTotal)
	}
	if s.MidSessionMsgs != 1 {
		t.Errorf("mid_session_msgs = %d, want 1", s.MidSessionMsgs)
	}
	if s.AskUserQuestion != 0 {
		t.Errorf("ask_user_question = %d, want 0", s.AskUserQuestion)
	}
	// last token_count wins
	if s.InputTokens != 300 {
		t.Errorf("input_tokens = %d, want 300", s.InputTokens)
	}
	if s.OutputTokens != 150 {
		t.Errorf("output_tokens = %d, want 150", s.OutputTokens)
	}
	if s.CacheReadTokens != 120 {
		t.Errorf("cache_read_tokens = %d, want 120", s.CacheReadTokens)
	}
	if s.CacheWriteTokens != 0 {
		t.Errorf("cache_write_tokens = %d, want 0", s.CacheWriteTokens)
	}
	if s.ReasoningTokens != 30 {
		t.Errorf("reasoning_tokens = %d, want 30", s.ReasoningTokens)
	}
	if s.Model != "gpt-5.5" {
		t.Errorf("model = %q, want gpt-5.5", s.Model)
	}
	if s.IsGhost {
		t.Error("should not be ghost")
	}
	if s.LastTimestamp.IsZero() {
		t.Error("LastTimestamp should be set")
	}
}

func TestParseCodex_GhostWhenNoUserMessage(t *testing.T) {
	p := writeRollout(t, "rollout.jsonl", []string{
		codexSessionMeta,
		codexTurnContext,
		codexAgentMessage,
	})
	s := ParseCodex(p)
	if !s.IsGhost {
		t.Error("expected ghost when no user_message")
	}
}

func TestParseCodex_TokenCountWithoutInfo(t *testing.T) {
	// Some early Codex versions emit token_count with info=null.
	p := writeRollout(t, "rollout.jsonl", []string{
		codexUserMessage,
		`{"timestamp":"2026-05-02T13:00:00Z","type":"event_msg","payload":{"type":"token_count","info":null}}`,
	})
	s := ParseCodex(p)
	if s.InputTokens != 0 || s.OutputTokens != 0 {
		t.Errorf("info=null should leave tokens at 0, got %+v", s)
	}
}

func TestParseCodex_Zstd(t *testing.T) {
	dir := t.TempDir()
	src := []string{
		codexSessionMeta,
		codexTurnContext,
		codexUserMessage,
		codexFunctionCall,
		codexTokenCountLast,
	}
	var raw bytes.Buffer
	for _, l := range src {
		raw.WriteString(l + "\n")
	}

	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	compressed := enc.EncodeAll(raw.Bytes(), nil)
	enc.Close()

	p := filepath.Join(dir, "rollout.jsonl.zst")
	if err := os.WriteFile(p, compressed, 0644); err != nil {
		t.Fatal(err)
	}

	s := ParseCodex(p)
	if s.ToolUseTotal != 1 {
		t.Errorf("tool_use_total under zstd = %d, want 1", s.ToolUseTotal)
	}
	if s.InputTokens != 300 {
		t.Errorf("input_tokens under zstd = %d, want 300", s.InputTokens)
	}
}

func TestReadCodexMeta(t *testing.T) {
	p := writeRollout(t, "rollout.jsonl", []string{
		codexSessionMeta,
		codexTurnContext,
	})
	cli, model, ts, ok := ReadCodexMeta(p)
	if !ok {
		t.Fatal("ReadCodexMeta should succeed")
	}
	if cli != "0.128.0-alpha.1" {
		t.Errorf("cli_version = %q", cli)
	}
	if model != "gpt-5.5" {
		t.Errorf("model = %q", model)
	}
	if ts.IsZero() {
		t.Error("ts should be set")
	}
}

func TestParse_DispatchByAgent(t *testing.T) {
	p := writeRollout(t, "rollout.jsonl", []string{
		codexSessionMeta,
		codexTurnContext,
		codexUserMessage,
		codexFunctionCall,
		codexTokenCountLast,
	})
	s := Parse(p, "codex")
	if s.ToolUseTotal != 1 {
		t.Errorf("Parse(codex) tool_use_total = %d, want 1", s.ToolUseTotal)
	}

	cp := writeRollout(t, "claude.jsonl", []string{
		`{"type":"user","message":{"content":"hello"}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read"}]}}`,
	})
	cs := Parse(cp, "claude")
	if cs.ToolUseTotal != 1 {
		t.Errorf("Parse(claude) tool_use_total = %d, want 1", cs.ToolUseTotal)
	}
}
