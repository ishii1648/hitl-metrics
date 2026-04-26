package backfill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ishii1648/hitl-metrics/internal/sessionindex"
)

type group struct {
	repo    string
	branch  string
	entries []sessionindex.Session
}

type result struct {
	group            group
	url              string
	markChecked      bool
	isMerged         bool
	comments         int
	changesRequested int
}

// prJSON represents a PR entry from gh pr list --json output.
type prJSON struct {
	URL      string        `json:"url"`
	State    string        `json:"state"`
	Comments []interface{} `json:"comments"`
	Reviews  []reviewJSON  `json:"reviews"`
}

type reviewJSON struct {
	State string `json:"state"`
}

// MetaCheckInterval is the minimum duration between Phase 2 (merge status) checks.
const MetaCheckInterval = 1 * time.Hour

// Run executes the backfill batch. It finds sessions without pr_urls,
// groups them by (repo, branch), and fetches PR URLs via gh pr list in parallel.
// It also updates is_merged and review_comments for sessions with existing pr_urls.
//
// When statePath is non-empty, cursor-based incremental processing is used:
// - Phase 1 only scans entries after last_backfill_offset
// - Phase 2 only runs if MetaCheckInterval has elapsed since last_meta_check
func Run(indexPath string, recheck bool) error {
	return RunWithState(indexPath, StatePath(), recheck)
}

// RunWithState is like Run but accepts an explicit state file path (for testing).
func RunWithState(indexPath, statePath string, recheck bool) error {
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil
	}

	_, sessions, err := sessionindex.ReadAll(indexPath)
	if err != nil {
		return err
	}

	// Load cursor state
	state, err := LoadState(statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	// Phase 1: Fetch PR URLs for sessions without them
	// Use cursor to skip already-processed entries
	offset := state.LastBackfillOffset
	if offset > len(sessions) || recheck {
		offset = 0
	}
	target := sessions[offset:]

	if err := runURLBackfill(indexPath, target, recheck); err != nil {
		return err
	}

	// Update cursor: advance to total session count
	state.LastBackfillOffset = len(sessions)

	// Phase 2: Update merge status and review comments for sessions with pr_urls
	// Only run if enough time has elapsed since last check
	now := time.Now()
	if now.Sub(state.LastMetaCheck) >= MetaCheckInterval || recheck {
		// Re-read sessions since Phase 1 may have updated them
		_, sessions, err = sessionindex.ReadAll(indexPath)
		if err != nil {
			return err
		}
		if err := runMetaBackfill(indexPath, sessions); err != nil {
			return err
		}
		state.LastMetaCheck = now
	} else {
		fmt.Printf("backfill-meta: スキップ（前回チェックから %s 未経過）\n",
			MetaCheckInterval)
	}

	// Save cursor state
	if statePath != "" {
		if err := SaveState(statePath, state); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	return nil
}

func runURLBackfill(indexPath string, sessions []sessionindex.Session, recheck bool) error {
	// Collect entries with empty pr_urls
	var entries []sessionindex.Session
	for _, s := range sessions {
		if len(s.PRURLs) == 0 && (!s.BackfillChecked || recheck) {
			entries = append(entries, s)
		}
	}

	if len(entries) == 0 {
		fmt.Println("backfill: URL対象エントリなし（全件 pr_urls 補完済み or backfill_checked 済み）")
		return nil
	}

	// Group by (repo, branch)
	type key struct{ repo, branch string }
	groupMap := make(map[key][]sessionindex.Session)
	for _, e := range entries {
		if e.Repo == "" || e.Branch == "" {
			continue
		}
		k := key{e.Repo, e.Branch}
		groupMap[k] = append(groupMap[k], e)
	}

	var groups []group
	for k, es := range groupMap {
		groups = append(groups, group{repo: k.repo, branch: k.branch, entries: es})
	}

	fmt.Printf("backfill: %d エントリ / %d グループを処理中...\n", len(entries), len(groups))

	// Parallel fetch with max 8 workers
	results := make(chan result, len(groups))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, g := range groups {
		wg.Add(1)
		go func(g group) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r := fetchPR(g)
			results <- r
		}(g)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	found, skipped, retried := 0, 0, 0
	for r := range results {
		if r.url != "" {
			found++
			for _, e := range r.group.entries {
				if e.SessionID != "" {
					if _, err := sessionindex.Update(indexPath, e.SessionID, []string{r.url}); err != nil {
						fmt.Fprintf(os.Stderr, "backfill: update %s: %v\n", e.SessionID, err)
					}
				}
			}
			// Also set merge info right away
			if _, err := sessionindex.UpdatePRMeta(indexPath, r.url, r.isMerged, r.comments, r.changesRequested); err != nil {
				fmt.Fprintf(os.Stderr, "backfill: update-meta %s: %v\n", r.url, err)
			}
		} else if r.markChecked {
			skipped++
			var ids []string
			for _, e := range r.group.entries {
				if e.SessionID != "" {
					ids = append(ids, e.SessionID)
				}
			}
			if len(ids) > 0 {
				if _, err := sessionindex.MarkChecked(indexPath, ids); err != nil {
					fmt.Fprintf(os.Stderr, "backfill: mark-checked: %v\n", err)
				}
			}
		} else {
			retried++
		}
	}

	fmt.Printf("backfill: 完了 — URL取得成功 %d グループ / cwd消滅スキップ %d グループ / 再試行待ち %d グループ\n",
		found, skipped, retried)
	return nil
}

// runMetaBackfill updates PR metadata for sessions that have pr_urls.
func runMetaBackfill(indexPath string, sessions []sessionindex.Session) error {
	// Collect unique pr_urls that need meta update.
	type prInfo struct {
		url string
		cwd string // any cwd from sessions with this URL, for running gh commands
	}
	seen := make(map[string]bool)
	var targets []prInfo
	for _, s := range sessions {
		if len(s.PRURLs) == 0 {
			continue
		}
		url := s.PRURLs[len(s.PRURLs)-1]
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		targets = append(targets, prInfo{url: url, cwd: s.CWD})
	}

	if len(targets) == 0 {
		return nil
	}

	fmt.Printf("backfill-meta: %d PR のメタデータを更新中...\n", len(targets))

	type metaResult struct {
		url              string
		isMerged         bool
		comments         int
		changesRequested int
		ok               bool
	}

	results := make(chan metaResult, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, t := range targets {
		wg.Add(1)
		go func(t prInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cwd := t.cwd
			if cwd == "" || !isDir(cwd) {
				results <- metaResult{url: t.url}
				return
			}

			pr, err := fetchPRByURL(t.url, cwd)
			if err != nil {
				results <- metaResult{url: t.url}
				return
			}
			results <- metaResult{
				url:              t.url,
				isMerged:         pr.State == "MERGED",
				comments:         len(pr.Comments),
				changesRequested: countChangesRequested(pr.Reviews),
				ok:               true,
			}
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	updated := 0
	for r := range results {
		if r.ok {
			if _, err := sessionindex.UpdatePRMeta(indexPath, r.url, r.isMerged, r.comments, r.changesRequested); err != nil {
				fmt.Fprintf(os.Stderr, "backfill-meta: update %s: %v\n", r.url, err)
			}
			updated++
		}
	}

	fmt.Printf("backfill-meta: 完了 — %d PR 更新\n", updated)
	return nil
}

// fetchPR fetches PR URL, state, and comments for a branch group.
func fetchPR(g group) result {
	// Use the last entry's cwd (matches Python behavior)
	cwd := g.entries[len(g.entries)-1].CWD
	if cwd == "" || !isDir(cwd) {
		return result{group: g, markChecked: true}
	}

	cmd := exec.Command("gh", "pr", "list",
		"--head", g.branch,
		"--author", "@me",
		"--state", "all",
		"--json", "url,state,comments,reviews",
		"--limit", "1",
	)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return result{group: g}
		}
		prs := parsePRList(stdout.Bytes())
		if len(prs) == 0 || !strings.Contains(prs[0].URL, "github.com") {
			return result{group: g}
		}
		pr := prs[0]
		return result{
			group:            g,
			url:              pr.URL,
			isMerged:         pr.State == "MERGED",
			comments:         len(pr.Comments),
			changesRequested: countChangesRequested(pr.Reviews),
		}
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		return result{group: g}
	}
}

// fetchPRByURL fetches PR metadata for an existing PR URL using gh pr view.
func fetchPRByURL(prURL, cwd string) (*prJSON, error) {
	cmd := exec.Command("gh", "pr", "view", prURL,
		"--json", "url,state,comments,reviews",
	)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("gh pr view: %w: %s", err, stderr.String())
		}
		var pr prJSON
		if err := json.Unmarshal(stdout.Bytes(), &pr); err != nil {
			return nil, err
		}
		return &pr, nil
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("timeout")
	}
}

func parsePRList(data []byte) []prJSON {
	var prs []prJSON
	if err := json.Unmarshal(data, &prs); err != nil {
		return nil
	}
	return prs
}

func countChangesRequested(reviews []reviewJSON) int {
	count := 0
	for _, r := range reviews {
		if r.State == "CHANGES_REQUESTED" {
			count++
		}
	}
	return count
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
