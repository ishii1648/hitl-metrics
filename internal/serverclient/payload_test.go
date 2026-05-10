package serverclient

import "testing"

func TestSessionPair_HashStableAcrossRuns(t *testing.T) {
	p := samplePair("claude", "abc-123")
	h1, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := p.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("hash unstable: %s vs %s", h1, h2)
	}
}

func TestSessionPair_HashChangesWithIsMerged(t *testing.T) {
	a := samplePair("claude", "abc-123")
	b := samplePair("claude", "abc-123")
	b.Session.IsMerged = 1

	ha, _ := a.Hash()
	hb, _ := b.Hash()
	if ha == hb {
		t.Errorf("hash unchanged after IsMerged flip: %s", ha)
	}
}

func TestSessionPair_HashChangesWithStats(t *testing.T) {
	a := samplePair("claude", "abc-123")
	b := samplePair("claude", "abc-123")
	b.Stats.InputTokens = 999

	ha, _ := a.Hash()
	hb, _ := b.Hash()
	if ha == hb {
		t.Errorf("hash unchanged after stats change: %s", ha)
	}
}

func TestSessionPair_KeyDistinguishesAgents(t *testing.T) {
	claude := samplePair("claude", "shared-uuid")
	codex := samplePair("codex", "shared-uuid")
	if claude.Key() == codex.Key() {
		t.Errorf("composite key collision: %s", claude.Key())
	}
}

func TestSelectChanged_SkipsUnchanged(t *testing.T) {
	p := samplePair("claude", "abc-123")
	h, _ := p.Hash()
	versions := map[string]string{p.Key(): h}

	changed, hashes, err := SelectChanged([]SessionPair{p}, versions, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Errorf("expected 0 changed, got %d", len(changed))
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty hash map for skipped, got %d", len(hashes))
	}
}

func TestSelectChanged_FullForcesResend(t *testing.T) {
	p := samplePair("claude", "abc-123")
	h, _ := p.Hash()
	versions := map[string]string{p.Key(): h}

	changed, hashes, err := SelectChanged([]SessionPair{p}, versions, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 {
		t.Fatalf("--full should resend, got %d", len(changed))
	}
	if hashes[p.Key()] != h {
		t.Errorf("hash mismatch: got %s want %s", hashes[p.Key()], h)
	}
}

func TestSelectChanged_DetectsBackfillUpdate(t *testing.T) {
	original := samplePair("claude", "abc-123")
	originalHash, _ := original.Hash()

	updated := samplePair("claude", "abc-123")
	updated.Session.IsMerged = 1 // simulate backfill marking PR as merged

	versions := map[string]string{original.Key(): originalHash}
	changed, _, err := SelectChanged([]SessionPair{updated}, versions, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 {
		t.Errorf("expected backfilled session to be resent, got %d", len(changed))
	}
}

func TestSplitBatches_RespectsLimit(t *testing.T) {
	pairs := []SessionPair{
		samplePair("claude", "a"),
		samplePair("claude", "b"),
		samplePair("claude", "c"),
	}
	// Force a tiny limit so each pair lands in its own batch.
	batches, err := SplitBatches(pairs, "1.0.0", "abc", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 {
		t.Errorf("expected 3 batches under tight limit, got %d", len(batches))
	}
	for _, b := range batches {
		if len(b.Sessions) != 1 || len(b.TranscriptStats) != 1 {
			t.Errorf("batch shape unexpected: %d sessions, %d stats", len(b.Sessions), len(b.TranscriptStats))
		}
	}
}

func TestSplitBatches_CombinesUnderLimit(t *testing.T) {
	pairs := []SessionPair{
		samplePair("claude", "a"),
		samplePair("claude", "b"),
	}
	batches, err := SplitBatches(pairs, "1.0.0", "abc", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Errorf("expected single batch under generous limit, got %d", len(batches))
	}
	if len(batches[0].Sessions) != 2 {
		t.Errorf("expected 2 sessions in batch, got %d", len(batches[0].Sessions))
	}
}

func samplePair(agentName, sessionID string) SessionPair {
	return SessionPair{
		Session: SessionRow{
			SessionID:    sessionID,
			CodingAgent:  agentName,
			AgentVersion: "1.0.0",
			UserID:       "alice@example.com",
			Timestamp:    "2026-05-10 10:00:00",
			Repo:         "ishii1648/agent-telemetry",
			Branch:       "feat/foo",
			PRURL:        "https://github.com/ishii1648/agent-telemetry/pull/42",
			PRTitle:      "feat: add foo",
			Transcript:   "/tmp/transcript.jsonl",
			EndedAt:      "2026-05-10 11:00:00",
			EndReason:    "exit",
			TaskType:     "feat",
		},
		Stats: StatsRow{
			SessionID:    sessionID,
			CodingAgent:  agentName,
			ToolUseTotal: 12,
			InputTokens:  4000,
			OutputTokens: 800,
			Model:        "claude-sonnet",
		},
	}
}
