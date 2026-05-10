#!/usr/bin/env python3
"""scripts/intent-lookup の振る舞いを fixture git repo + assertions で検証する。

外部依存なし（標準ライブラリ unittest のみ）。fixture は TemporaryDirectory に
fresh な git repo を作って組み立てる。
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SCRIPT = REPO_ROOT / "scripts" / "intent-lookup"


def run_script(*args, cwd: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        cwd=str(cwd), capture_output=True, text=True,
    )


def run_git(*args, cwd: Path, env: dict | None = None) -> str:
    full_env = os.environ.copy()
    if env:
        full_env.update(env)
    res = subprocess.run(
        ["git", *args], cwd=str(cwd), capture_output=True, text=True,
        env=full_env, check=True,
    )
    return res.stdout


def write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(content).lstrip("\n"), encoding="utf-8")


def make_fixture_repo(work: Path) -> dict:
    """Create a deterministic fixture git repo with issues + commits."""
    env = {
        "GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
        "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
        "GIT_AUTHOR_DATE": "2026-01-01T00:00:00Z",
        "GIT_COMMITTER_DATE": "2026-01-01T00:00:00Z",
    }
    run_git("init", "-q", "-b", "main", cwd=work, env=env)
    (work / "issues" / "closed").mkdir(parents=True)
    (work / "issues" / "pending").mkdir(parents=True)
    (work / "internal" / "hook").mkdir(parents=True)
    (work / "internal" / "syncdb").mkdir(parents=True)
    (work / "cmd" / "agent-telemetry").mkdir(parents=True)
    (work / "issues" / "SEQUENCE").write_text("0005\n")

    # 0001 closed — internal/hook/stop.go + internal/syncdb/
    write(work / "issues" / "closed" / "0001-bug-fixture-stop.md", """
        ---
        decision_type: design
        affected_paths:
          - internal/hook/stop.go
          - internal/syncdb/
        tags: [hooks, fixture]
        closed_at: 2026-01-02
        ---

        # Fixture: Stop hook の何かを直す

        Created: 2026-01-01

        ## 概要

        fixture の stop hook 概要文。短い段落。

        ## 対応方針

        最小限の変更で signal handling を堅牢化する。

        ## 解決方法

        ### 段階 1: シグナル捕捉
        SIGTERM をメインループで処理するように変更。

        ### 段階 2: idempotent化
        多重呼び出しに耐える。
        """)

    # 0002 open — internal/hook/ ディレクトリ全体
    write(work / "issues" / "0002-feat-fixture-hook-area.md", """
        ---
        decision_type: implementation
        affected_paths:
          - internal/hook/
        tags: [hooks, fixture, broad]
        ---

        # Fixture: hook ディレクトリ全体の話

        Created: 2026-01-01

        ## 概要

        広い範囲を扱う fixture。
        """)

    # 0003 pending — 関係ない path
    write(work / "issues" / "pending" / "0003-design-fixture-unrelated.md", """
        ---
        decision_type: design
        affected_paths:
          - cmd/agent-telemetry/main.go
        tags: [unrelated, fixture]
        ---

        # Fixture: 関係ない issue

        Created: 2026-01-01

        ## 概要

        無関係。
        """)

    # 0004 — invalid decision_type / missing affected_path / broad path (lint targets)
    # `examples/` は他テストの query path (internal/* / cmd/*) と overlap しない broad dir
    (work / "examples").mkdir()
    write(work / "issues" / "0004-bug-fixture-lint-targets.md", """
        ---
        decision_type: bogus
        affected_paths:
          - examples/
          - examples/does-not-exist.go
        tags: [lint, fixture]
        ---

        # Fixture: lint targets

        Created: 2026-01-01

        ## 概要

        故意に lint で warning を発生させる fixture。
        """)

    # frontmatter なし issue (skip されるべき / lint で warning)
    write(work / "issues" / "no-frontmatter.md", """
        # 古い形式の issue

        frontmatter を持たない。intent-lookup は skip するはず。
        """)

    # 内部ファイル + commits
    (work / "internal" / "hook" / "stop.go").write_text("package hook\n")
    (work / "cmd" / "agent-telemetry" / "main.go").write_text("package main\n")

    run_git("add", "-A", cwd=work, env=env)
    run_git("commit", "-q", "-m", textwrap.dedent("""
        feat(hook): add stop.go

        intent: capture stop signals
        decision: handle SIGTERM in main loop
    """).strip(), cwd=work, env=env)

    env2 = dict(env)
    env2["GIT_AUTHOR_DATE"] = "2026-01-02T00:00:00Z"
    env2["GIT_COMMITTER_DATE"] = "2026-01-02T00:00:00Z"
    (work / "internal" / "hook" / "stop.go").write_text("package hook // updated\n")
    run_git("-c", "commit.gpgsign=false", "commit", "-q", "-am",
            textwrap.dedent("""
                fix(hook): tweak stop.go

                constraint: must remain idempotent
                learned: subagent termination races with parent
            """).strip(), cwd=work, env=env2)

    env3 = dict(env)
    env3["GIT_AUTHOR_DATE"] = "2026-01-03T00:00:00Z"
    env3["GIT_COMMITTER_DATE"] = "2026-01-03T00:00:00Z"
    with (work / "internal" / "hook" / "stop.go").open("a") as f:
        f.write("// noop\n")
    run_git("-c", "commit.gpgsign=false", "commit", "-q", "-am",
            "chore(hook): noop comment", cwd=work, env=env3)

    return env


def make_rename_fixture(work: Path) -> dict:
    """Fixture for rename-aware lookup: file is renamed but issue still references old path."""
    env = {
        "GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
        "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
        "GIT_AUTHOR_DATE": "2026-02-01T00:00:00Z",
        "GIT_COMMITTER_DATE": "2026-02-01T00:00:00Z",
    }
    run_git("init", "-q", "-b", "main", cwd=work, env=env)
    (work / "issues" / "closed").mkdir(parents=True)
    (work / "issues" / "pending").mkdir(parents=True)
    (work / "old").mkdir()
    (work / "issues" / "SEQUENCE").write_text("0002\n")

    # commit 1: create old/file.go
    (work / "old" / "file.go").write_text("package old\n// substantial content\n" * 5)
    write(work / "issues" / "closed" / "0001-design-old-path-decision.md", """
        ---
        decision_type: design
        affected_paths:
          - old/file.go
        tags: [rename, fixture]
        closed_at: 2026-02-01
        ---

        # Fixture: old path で書かれた issue

        Created: 2026-02-01

        ## 概要

        後の rename を見越した issue。affected_paths は古い path のまま。
        """)
    run_git("add", "-A", cwd=work, env=env)
    run_git("commit", "-q", "-m", "feat: add old/file.go", cwd=work, env=env)

    # commit 2: rename to new/file.go (substantial content preserved → git follows)
    env2 = dict(env)
    env2["GIT_AUTHOR_DATE"] = "2026-02-02T00:00:00Z"
    env2["GIT_COMMITTER_DATE"] = "2026-02-02T00:00:00Z"
    (work / "new").mkdir()
    run_git("mv", "old/file.go", "new/file.go", cwd=work, env=env2)
    run_git("-c", "commit.gpgsign=false", "commit", "-q", "-m",
            "refactor: rename old/ -> new/", cwd=work, env=env2)

    return env


class IntentLookupTests(unittest.TestCase):

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.work = Path(self._tmp.name)
        self.env = make_fixture_repo(self.work)

    def tearDown(self):
        self._tmp.cleanup()

    # ----- lookup: JSON structure -----
    def test_lookup_json_path_normalized(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        data = json.loads(r.stdout)
        self.assertEqual(data["path"], "internal/hook/stop.go")

    def test_lookup_json_matches_specific_and_dir_issues(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        ids = sorted(i["id"] for i in data["issues"])
        self.assertEqual(ids, ["0001", "0002"])

    def test_lookup_excludes_unrelated(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        ids = [i["id"] for i in data["issues"]]
        self.assertNotIn("0003", ids)

    def test_lookup_frontmatter_preserved(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        i1 = next(i for i in data["issues"] if i["id"] == "0001")
        self.assertEqual(i1["frontmatter"]["decision_type"], "design")
        self.assertEqual(i1["frontmatter"]["closed_at"], "2026-01-02")
        i2 = next(i for i in data["issues"] if i["id"] == "0002")
        self.assertEqual(i2["status"], "open")

    # ----- lookup: section excerpts -----
    def test_lookup_includes_section_excerpts(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        i1 = next(i for i in data["issues"] if i["id"] == "0001")
        self.assertIn("fixture の stop hook 概要文", i1["sections"]["summary"])
        self.assertIn("最小限の変更", i1["sections"]["approach"])
        # resolution starts with `### 段階 1` heading; excerpt should pull body line too
        self.assertIn("### 段階 1", i1["sections"]["resolution"])
        self.assertIn("SIGTERM をメインループで処理", i1["sections"]["resolution"])
        # but NOT 段階 2 (excerpt cap)
        self.assertNotIn("段階 2", i1["sections"]["resolution"])

    def test_lookup_full_includes_all_subheadings(self):
        r = run_script("--full", "--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        i1 = next(i for i in data["issues"] if i["id"] == "0001")
        self.assertIn("段階 1", i1["sections"]["resolution"])
        self.assertIn("段階 2", i1["sections"]["resolution"])
        self.assertIn("idempotent", i1["sections"]["resolution"])

    def test_markdown_renders_excerpts(self):
        r = run_script("internal/hook/stop.go", cwd=self.work)
        self.assertIn("**概要 (excerpt):**", r.stdout)
        self.assertIn("fixture の stop hook 概要文", r.stdout)
        self.assertIn("**対応方針 (excerpt):**", r.stdout)

    # ----- lookup: commits -----
    def test_lookup_commits_have_actions(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        # 2 commits with action lines (noop excluded)
        self.assertEqual(len(data["commits"]), 2)
        all_actions = [a for c in data["commits"] for a in c["actions"]]
        self.assertTrue(any("intent: capture stop signals" in a for a in all_actions))
        self.assertTrue(any("learned:" in a for a in all_actions))

    # ----- lookup: bidirectional prefix overlap -----
    def test_broad_query_dir_matches_specific(self):
        r = run_script("--format=json", "internal/hook/", cwd=self.work)
        data = json.loads(r.stdout)
        ids = sorted(i["id"] for i in data["issues"])
        self.assertEqual(ids, ["0001", "0002"])

    def test_narrow_query_matches_broader_affected_path(self):
        r = run_script("--format=json", "internal/syncdb/foo.go", cwd=self.work)
        data = json.loads(r.stdout)
        ids = sorted(i["id"] for i in data["issues"])
        self.assertEqual(ids, ["0001"])

    # ----- lookup: no-match -----
    def test_no_match_returns_empty(self):
        r = run_script("--format=json", "totally/unrelated/path", cwd=self.work)
        self.assertEqual(r.returncode, 0)
        data = json.loads(r.stdout)
        self.assertEqual(len(data["issues"]), 0)
        self.assertEqual(len(data["commits"]), 0)

    # ----- markdown structure -----
    def test_markdown_top_structure(self):
        r = run_script("internal/hook/stop.go", cwd=self.work)
        self.assertIn("# Intent for", r.stdout)
        self.assertIn("## Issues", r.stdout)
        self.assertIn("## Commits", r.stdout)
        self.assertIn("#0001", r.stdout)
        self.assertIn("#0002", r.stdout)
        self.assertIn("**lookup index**", r.stdout)

    # ----- CLI -----
    def test_help_exits_zero(self):
        r = run_script("--help", cwd=self.work)
        self.assertEqual(r.returncode, 0)

    def test_no_arg_fails(self):
        r = run_script(cwd=self.work)
        self.assertNotEqual(r.returncode, 0)

    def test_lint_with_path_fails(self):
        r = run_script("--lint", "internal/hook/stop.go", cwd=self.work)
        self.assertNotEqual(r.returncode, 0)

    # ----- frontmatter-less skip -----
    def test_frontmatter_less_skipped_in_lookup(self):
        r = run_script("--format=json", "internal/hook/stop.go", cwd=self.work)
        data = json.loads(r.stdout)
        for i in data["issues"]:
            self.assertNotIn("no-frontmatter", i["path"])

    # ----- lint mode -----
    def test_lint_detects_all_findings(self):
        r = run_script("--lint", "--format=json", cwd=self.work)
        # 0004 has invalid decision_type → error → exit 2 (errors always gate)
        self.assertEqual(r.returncode, 2)
        data = json.loads(r.stdout)
        codes_by_path = {}
        for f in data["findings"]:
            codes_by_path.setdefault(f["path"], []).append(f["code"])

        # frontmatter missing
        nf = [p for p in codes_by_path if "no-frontmatter" in p]
        self.assertEqual(len(nf), 1)
        self.assertIn("frontmatter_missing", codes_by_path[nf[0]])

        # 0004: bogus decision_type (error) + broad + missing
        f0004 = [p for p in codes_by_path if "0004" in p][0]
        self.assertIn("decision_type_invalid", codes_by_path[f0004])
        self.assertIn("affected_path_broad", codes_by_path[f0004])
        self.assertIn("affected_path_missing", codes_by_path[f0004])

        # exit code reflects errors when present
        self.assertEqual(data["errors"], 1)

    def test_lint_errors_force_exit_2(self):
        r = run_script("--lint", "--format=json", cwd=self.work)
        # 0004 has invalid decision_type → error → exit 2
        self.assertEqual(r.returncode, 2)

    def test_lint_warnings_only_exit_zero_by_default(self):
        # Replace 0004 (which has the only error) with a warnings-only fixture.
        (self.work / "issues" / "0004-bug-fixture-lint-targets.md").unlink()
        write(self.work / "issues" / "0004-bug-fixture-warnings-only.md", """
            ---
            decision_type: design
            affected_paths:
              - examples/
              - examples/does-not-exist.go
            tags: [lint, fixture]
            ---

            # Fixture: warnings only
        """)
        r = run_script("--lint", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        self.assertEqual(data["errors"], 0)
        self.assertGreater(data["warnings"], 0)
        # default: warnings do not gate
        self.assertEqual(r.returncode, 0)

    def test_lint_strict_warnings_force_exit_1(self):
        (self.work / "issues" / "0004-bug-fixture-lint-targets.md").unlink()
        write(self.work / "issues" / "0004-bug-fixture-warnings-only.md", """
            ---
            decision_type: design
            affected_paths:
              - examples/
            tags: [lint, fixture]
            ---

            # Fixture: warnings only
        """)
        r = run_script("--lint", "--strict", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        self.assertEqual(data["errors"], 0)
        self.assertGreater(data["warnings"], 0)
        self.assertEqual(r.returncode, 1)

    def test_lint_ignore_broad_suppresses_warning(self):
        # Add an issue that legitimately needs a broad path, with lint_ignore_broad.
        write(self.work / "issues" / "0098-feat-meta-broad.md", """
            ---
            decision_type: process
            affected_paths:
              - examples/
            lint_ignore_broad: [examples/]
            tags: [fixture]
            ---

            # Fixture: legitimate broad
        """)
        r = run_script("--lint", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        for f in data["findings"]:
            if "0098" in f["path"]:
                self.assertNotEqual(f["code"], "affected_path_broad",
                                    msg=f"ignore did not suppress: {f}")

    def test_lint_ignore_missing_suppresses_warning(self):
        write(self.work / "issues" / "0097-feat-future-path.md", """
            ---
            decision_type: design
            affected_paths:
              - future/path.go
            lint_ignore_missing: [future/path.go]
            tags: [fixture]
            ---

            # Fixture: future path
        """)
        r = run_script("--lint", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        for f in data["findings"]:
            if "0097" in f["path"]:
                self.assertNotEqual(f["code"], "affected_path_missing",
                                    msg=f"ignore did not suppress: {f}")

    def test_lint_ignore_only_matches_exact_path(self):
        # ignore "foo/" should NOT suppress warnings for "bar/"
        write(self.work / "issues" / "0096-feat-mismatch.md", """
            ---
            decision_type: design
            affected_paths:
              - bar/
            lint_ignore_broad: [foo/]
            tags: [fixture]
            ---

            # Fixture: ignore mismatch
        """)
        (self.work / "bar").mkdir()
        r = run_script("--lint", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        codes = [f["code"] for f in data["findings"] if "0096" in f["path"]]
        self.assertIn("affected_path_broad", codes)

    def test_lint_top_level_files_not_broad(self):
        # Add a top-level file reference that should NOT be broad
        write(self.work / "issues" / "0099-feat-toplevel-file.md", """
            ---
            decision_type: design
            affected_paths:
              - Makefile
            tags: [fixture]
            ---

            # Fixture: top-level file reference
        """)
        (self.work / "Makefile").write_text("# fixture\n")
        r = run_script("--lint", "--format=json", cwd=self.work)
        data = json.loads(r.stdout)
        for f in data["findings"]:
            if f["path"].endswith("0099-feat-toplevel-file.md"):
                self.assertNotEqual(f["code"], "affected_path_broad",
                                    msg=f"top-level file flagged as broad: {f}")


class RenameAwareTests(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.work = Path(self._tmp.name)
        self.env = make_rename_fixture(self.work)

    def tearDown(self):
        self._tmp.cleanup()

    def test_query_new_path_finds_issue_with_old_path(self):
        r = run_script("--format=json", "new/file.go", cwd=self.work)
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        data = json.loads(r.stdout)
        ids = [i["id"] for i in data["issues"]]
        self.assertIn("0001", ids)
        # resolved_paths should contain the historical old path
        self.assertIn("new/file.go", data["resolved_paths"])
        self.assertTrue(any("old/file.go" in p for p in data["resolved_paths"]))

    def test_query_old_path_still_finds_issue(self):
        # querying the old path itself: file no longer exists, but git log knows it
        r = run_script("--format=json", "old/file.go", cwd=self.work)
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        data = json.loads(r.stdout)
        ids = [i["id"] for i in data["issues"]]
        self.assertIn("0001", ids)


if __name__ == "__main__":
    unittest.main()
