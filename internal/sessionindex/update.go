package sessionindex

import (
	"encoding/json"
	"os"
	"sort"
)

// appendUniqueURLs returns existing URLs followed by URLs from `add` that are
// not already present, preserving both the existing order and the order in
// which new URLs are appended. The returned slice is non-nil when at least one
// URL is added (added=true); otherwise the original slice is returned.
func appendUniqueURLs(existing, add []string) (merged []string, added bool) {
	seen := make(map[string]struct{}, len(existing))
	for _, u := range existing {
		seen[u] = struct{}{}
	}
	merged = append([]string(nil), existing...)
	for _, u := range add {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		merged = append(merged, u)
		added = true
	}
	if !added {
		return existing, false
	}
	return merged, true
}

// Update adds new PR URLs to the session identified by sessionID.
// Returns true if the file was modified.
//
// Sessions with pr_pinned=true are intentionally skipped: their pr_urls
// has been authoritatively bound by the Stop hook (PinPR) and must not
// be polluted by stray PR URLs scraped from PostToolUse / backfill.
func Update(indexPath string, sessionID string, newURLs []string) (bool, error) {
	if sessionID == "" || len(newURLs) == 0 {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	updated := false
	for i, s := range sessions {
		if s.SessionID != sessionID {
			continue
		}
		if s.PRPinned {
			continue
		}
		merged, added := appendUniqueURLs(s.PRURLs, newURLs)
		if !added {
			continue
		}
		s.PRURLs = merged
		sessions[i] = s
		raw, err := remarshalWithUpdate(raws[i], "pr_urls", merged)
		if err != nil {
			return false, err
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// PinPR authoritatively binds the given session to a single PR URL and
// sets pr_pinned=true so subsequent Update / UpdateByBranch / backfill
// passes leave the entry alone. Called from the Stop hook once `gh pr view`
// has resolved the branch's PR. Returns true if the file was modified.
//
// pr_urls is replaced (not appended) with [prURL] — the pinned URL is the
// source of truth, and any pre-existing entries are assumed to be polluted
// regex matches from PostToolUse.
func PinPR(indexPath string, sessionID string, prURL string) (bool, error) {
	if sessionID == "" || prURL == "" {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	updated := false
	for i, s := range sessions {
		if s.SessionID != sessionID {
			continue
		}
		alreadyPinned := s.PRPinned &&
			len(s.PRURLs) == 1 &&
			s.PRURLs[0] == prURL
		if alreadyPinned {
			continue
		}
		raw := raws[i]
		raw, err = remarshalWithUpdate(raw, "pr_urls", []string{prURL})
		if err != nil {
			return false, err
		}
		raw, err = remarshalWithUpdate(raw, "pr_pinned", true)
		if err != nil {
			return false, err
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// MarkChecked sets backfill_checked=true for the given session IDs.
// Returns true if the file was modified.
func MarkChecked(indexPath string, sessionIDs []string) (bool, error) {
	if len(sessionIDs) == 0 {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	targetSet := make(map[string]struct{})
	for _, id := range sessionIDs {
		targetSet[id] = struct{}{}
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	updated := false
	for i, s := range sessions {
		if _, ok := targetSet[s.SessionID]; !ok {
			continue
		}
		if s.BackfillChecked {
			continue
		}
		raw, err := remarshalWithUpdate(raws[i], "backfill_checked", true)
		if err != nil {
			return false, err
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// UpdateByBranch adds a PR URL to all sessions matching repo+branch that don't already have it.
// Returns true if the file was modified.
func UpdateByBranch(indexPath string, targetRepo, targetBranch, newURL string) (bool, error) {
	if targetRepo == "" || targetBranch == "" || newURL == "" {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	normalizedTarget := NormalizeRepo(targetRepo)
	updated := false

	for i, s := range sessions {
		if NormalizeRepo(s.Repo) != normalizedTarget {
			continue
		}
		if s.Branch != targetBranch {
			continue
		}
		if s.PRPinned {
			continue
		}
		merged, added := appendUniqueURLs(s.PRURLs, []string{newURL})
		if !added {
			continue
		}

		raw, err := remarshalWithUpdate(raws[i], "pr_urls", merged)
		if err != nil {
			return false, err
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// UpdateEnd records the end timestamp and reason for the given session.
// Returns true if the file was modified.
func UpdateEnd(indexPath string, sessionID string, endedAt string, reason string) (bool, error) {
	if sessionID == "" || endedAt == "" {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	updated := false
	for i, s := range sessions {
		if s.SessionID != sessionID {
			continue
		}
		if s.EndedAt == endedAt && s.EndReason == reason {
			continue
		}

		raw := raws[i]
		raw, err = remarshalWithUpdate(raw, "ended_at", endedAt)
		if err != nil {
			return false, err
		}
		raw, err = remarshalWithUpdate(raw, "end_reason", reason)
		if err != nil {
			return false, err
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// UpdatePRMeta updates PR metadata for all sessions that have the given pr_url.
// Returns true if the file was modified.
func UpdatePRMeta(indexPath string, prURL string, isMerged bool, reviewComments, changesRequested int, prTitle string) (bool, error) {
	if prURL == "" {
		return false, nil
	}
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false, nil
	}

	raws, sessions, err := ReadAll(indexPath)
	if err != nil {
		return false, err
	}

	updated := false
	for i, s := range sessions {
		// Match if any of the session's pr_urls matches
		match := false
		for _, u := range s.PRURLs {
			if u == prURL {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		if s.IsMerged == isMerged && s.ReviewComments == reviewComments && s.ChangesRequested == changesRequested && s.PRTitle == prTitle {
			continue
		}

		raw := raws[i]
		raw, err = remarshalWithUpdate(raw, "is_merged", isMerged)
		if err != nil {
			return false, err
		}
		raw, err = remarshalWithUpdate(raw, "review_comments", reviewComments)
		if err != nil {
			return false, err
		}
		raw, err = remarshalWithUpdate(raw, "changes_requested", changesRequested)
		if err != nil {
			return false, err
		}
		// gh が title を返さなかったケース（取得失敗）で既存 pr_title を空で上書きしないよう、
		// 空文字列の prTitle は書かない。明示的にクリアしたい場合はそもそも呼び出さない。
		if prTitle != "" {
			raw, err = remarshalWithUpdate(raw, "pr_title", prTitle)
			if err != nil {
				return false, err
			}
		}
		raws[i] = raw
		updated = true
	}

	if updated {
		return true, WriteAll(indexPath, raws)
	}
	return false, nil
}

// remarshalWithUpdate decodes raw JSON as a map, sets key=value, and re-encodes.
// This preserves all original fields while updating the target field.
func remarshalWithUpdate(raw json.RawMessage, key string, value any) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	vBytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	m[key] = json.RawMessage(vBytes)

	// Re-encode preserving field order by using a sorted key approach.
	// Python's json.dumps outputs keys in insertion order; we'll use sorted for consistency.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Stable field order. coding_agent / agent_version come first so that
	// readers can identify the source agent without parsing the whole record.
	order := map[string]int{
		"coding_agent":      -2,
		"agent_version":     -1,
		"timestamp":         0,
		"session_id":        1,
		"cwd":               2,
		"repo":              3,
		"branch":            4,
		"pr_urls":           5,
		"pr_pinned":         6,
		"pr_title":          7,
		"transcript":        8,
		"parent_session_id": 9,
		"ended_at":          10,
		"end_reason":        11,
		"backfill_checked":  12,
		"is_merged":         13,
		"review_comments":   14,
		"changes_requested": 15,
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oki := order[keys[i]]
		oj, okj := order[keys[j]]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return keys[i] < keys[j]
	})

	buf := []byte{'{'}
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, _ := json.Marshal(k)
		buf = append(buf, ' ')
		buf = append(buf, kb...)
		buf = append(buf, ':')
		buf = append(buf, ' ')
		buf = append(buf, m[k]...)
	}
	buf = append(buf, '}')
	return json.RawMessage(buf), nil
}
