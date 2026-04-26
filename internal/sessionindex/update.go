package sessionindex

import (
	"encoding/json"
	"os"
	"sort"
)

// Update adds new PR URLs to the session identified by sessionID.
// Returns true if the file was modified.
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
		existing := make(map[string]struct{})
		for _, u := range s.PRURLs {
			existing[u] = struct{}{}
		}
		before := len(existing)
		for _, u := range newURLs {
			existing[u] = struct{}{}
		}
		if len(existing) > before {
			merged := make([]string, 0, len(existing))
			for u := range existing {
				merged = append(merged, u)
			}
			sort.Strings(merged)
			s.PRURLs = merged
			sessions[i] = s
			raw, err := remarshalWithUpdate(raws[i], "pr_urls", merged)
			if err != nil {
				return false, err
			}
			raws[i] = raw
			updated = true
		}
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
		// Check if pr_urls field exists and URL is not already present
		has := false
		for _, u := range s.PRURLs {
			if u == newURL {
				has = true
				break
			}
		}
		if has {
			continue
		}
		// Merge new URL
		urlSet := make(map[string]struct{})
		for _, u := range s.PRURLs {
			urlSet[u] = struct{}{}
		}
		urlSet[newURL] = struct{}{}
		merged := make([]string, 0, len(urlSet))
		for u := range urlSet {
			merged = append(merged, u)
		}
		sort.Strings(merged)

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
func UpdatePRMeta(indexPath string, prURL string, isMerged bool, reviewComments, changesRequested int) (bool, error) {
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
		if s.IsMerged == isMerged && s.ReviewComments == reviewComments && s.ChangesRequested == changesRequested {
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
	// Preserve a stable, predictable order matching the original Python output.
	// Use the order: timestamp, session_id, cwd, repo, branch, pr_urls, transcript,
	// parent_session_id, backfill_checked, then any extras sorted.
	order := map[string]int{
		"timestamp":         0,
		"session_id":        1,
		"cwd":               2,
		"repo":              3,
		"branch":            4,
		"pr_urls":           5,
		"transcript":        6,
		"parent_session_id": 7,
		"backfill_checked":  8,
		"is_merged":         9,
		"review_comments":   10,
		"changes_requested": 11,
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
