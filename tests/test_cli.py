"""The command line, run on a temporary copy of the repository with a fake claude."""

import argparse
import csv
import json
import os
import shutil
import subprocess
import sys
import unittest
from unittest import mock

from helpers import ROOT, bench, config_names, env_without, make_fake_claude, temp_dir, write_file

ROWS = [
    {"run_id": "r1", "timestamp": "2026-10-15T10:00:00", "usecase": "uc", "config": "light", "subtype": "success",
     "wall_s": "10", "total_cost_usd": "0.2", "advisor_cost_usd": "0", "num_turns": "4", "check_pass": "1",
     "judge_score": "9", "models_used": "claude-sonnet-5-5", "cache_read_tokens": "100"},
    {"run_id": "r2", "timestamp": "2026-10-15T10:05:00", "usecase": "uc", "config": "light",
     "subtype": "error_max_turns", "wall_s": "30", "total_cost_usd": "0.4", "check_pass": "0", "notes": "capped"},
    {"run_id": "r3", "timestamp": "2026-10-15T11:00:00", "usecase": "uc", "config": "normal", "subtype": "success",
     "wall_s": "20", "total_cost_usd": "0.5", "num_turns": "6", "check_pass": "1", "judge_score": "8",
     "models_used": "claude-opus-5-5"},
]

# Stands in for Claude Code in a linkbox-storage-adr run: writes the ADR given in REF_ADR and prints a result.
FAKE_ADR_CLAUDE = """#!/bin/bash
if [ "${1:-}" = "--version" ]; then echo "0.0.0 (fake)"; exit 0; fi
cp "$REF_ADR" docs/adr/0002-store-links-in-postgresql.md
printf '{"type": "result", "subtype": "success", "is_error": false, "num_turns": 1, "result": "PostgreSQL.", "modelUsage": {}}\\n'
"""

# Stands in for Claude Code when the API refuses the run: an error result with the status in FAKE_STATUS.
FAKE_REFUSED_CLAUDE = """#!/bin/bash
if [ "${1:-}" = "--version" ]; then echo "0.0.0 (fake)"; exit 0; fi
printf '{"type": "result", "subtype": "success", "is_error": true, "api_error_status": %s, "num_turns": 1, "total_cost_usd": 0, "result": "refused", "modelUsage": {}}\\n' "$FAKE_STATUS"
"""

# The first agent call fails with a server error (marked by creating FAKE_SEEN), every later one succeeds.
FAKE_FLAKY_CLAUDE = """#!/bin/bash
if [ "${1:-}" = "--version" ]; then echo "0.0.0 (fake)"; exit 0; fi
if [ ! -e "$FAKE_SEEN" ]; then
  touch "$FAKE_SEEN"
  printf '{"type": "result", "subtype": "success", "is_error": true, "api_error_status": 500, "num_turns": 1, "result": "overloaded", "modelUsage": {}}\\n'
else
  printf '{"type": "result", "subtype": "success", "is_error": false, "num_turns": 1, "total_cost_usd": 0.01, "result": "done", "modelUsage": {}}\\n'
fi
"""


class RepoTest(unittest.TestCase):
    """A temporary copy of the repository with a fake claude on PATH."""

    def setUp(self):
        self.tmp = temp_dir(self)
        self.root = os.path.join(self.tmp, "repo")
        os.makedirs(self.root)
        shutil.copy2(os.path.join(ROOT, "bench.py"), self.root)
        for d in ("configs", "usecases"):
            shutil.copytree(os.path.join(ROOT, d), os.path.join(self.root, d))
        fake_bin = os.path.join(self.tmp, "bin")
        make_fake_claude(fake_bin)
        self.env = env_without(*bench.SESSION_VARS)
        self.env["PATH"] = fake_bin + os.pathsep + self.env.get("PATH", "")
        self.env["CLAUDE_CONFIG_DIR"] = os.path.join(self.tmp, "claude-config")

    def bench(self, *args):
        return subprocess.run([sys.executable, os.path.join(self.root, "bench.py")] + list(args), cwd=self.root,
                              capture_output=True, text=True, env=self.env, timeout=120, check=False)

    def results(self, name):
        return os.path.join(self.root, "results", name)

    def write_runs(self, rows):
        os.makedirs(os.path.dirname(self.results("runs.csv")))
        with open(self.results("runs.csv"), "w", newline="", encoding="utf-8") as f:
            w = csv.DictWriter(f, fieldnames=bench.CSV_FIELDS)
            w.writeheader()
            for row in rows:
                w.writerow({k: row.get(k, "") for k in bench.CSV_FIELDS})


class CliTest(RepoTest):
    """list, run --dry-run, summarize and argument errors."""

    def test_list_shows_the_configs_and_use_cases(self):
        p = self.bench("list")
        self.assertEqual(p.returncode, 0, p.stderr)
        for name in config_names() + ["example-fix-tests", "meowdoku", "lazygit-conflict-markers", "linkbox-review", "linkbox-storage-adr"]:
            self.assertIn(name, p.stdout)

    def test_dry_run_prints_one_command_per_config_and_writes_no_results(self):
        p = self.bench("run", "--dry-run", "--runs", "1", "--no-judge", "-u", "example-fix-tests")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertEqual(p.stderr.count("DRY RUN"), len(config_names()))
        self.assertFalse(os.path.exists(self.results("runs.csv")))

    def test_a_linkbox_storage_adr_run_passes_its_check_after_bench_stages_the_workspace(self):
        # bench.py stages the workspace (collect_diff) before it runs check.sh, so the check must grade a staged ADR.
        write_file(os.path.join(self.root, "bench-mcp.json"), json.dumps({"mcpServers": {"noop": {"command": "true", "args": []}}}))
        write_file(os.path.join(self.tmp, "bin", "claude"), FAKE_ADR_CLAUDE, 0o755)
        self.env["REF_ADR"] = os.path.join(self.root, "usecases", "linkbox-storage-adr", "reference-adr.md")
        p = self.bench("run", "--runs", "1", "-c", "light", "-u", "linkbox-storage-adr", "--no-judge")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("check: PASS (score 1.00)", p.stderr)

    def test_a_dry_run_enables_the_declared_plugin_even_when_it_is_not_installed(self):
        p = self.bench("run", "--dry-run", "--runs", "1", "--no-judge", "-u", "linkbox-review", "-c", "light")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("""'{"enabledPlugins": {"pr-review-toolkit@claude-plugins-official": true}, "syncClaudeAiPlugins": false}'""", p.stderr)

    def test_an_unknown_config_is_rejected(self):
        p = self.bench("run", "--dry-run", "-c", "nope")
        self.assertNotEqual(p.returncode, 0)
        self.assertIn("unknown config(s): nope", p.stderr)

    def test_no_command_prints_help_and_fails(self):
        p = self.bench()
        self.assertEqual(p.returncode, 1)
        self.assertIn("usage:", p.stdout)

    def test_summarize_without_results_fails(self):
        p = self.bench("summarize")
        self.assertNotEqual(p.returncode, 0)
        self.assertIn("no results yet", p.stderr)

    def test_summarize_writes_the_tables(self):
        self.write_runs(ROWS)
        p = self.bench("summarize")
        self.assertEqual(p.returncode, 0, p.stderr)
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertIn("| uc | light | 2 | 1 | 50% | 9.0 | 20.0 | 10.0-30.0 | $0.300 | $0.000 | $0.600 | 4.0 | 100 |", summary)
        self.assertIn("| uc | normal | 1 | 0 | 100% | 8.0 | 20.0 | 20.0-20.0 | $0.500 | - | $0.500 | 6.0 | - |", summary)
        self.assertIn("- **light**: claude-sonnet-5-5", summary)
        self.assertIn("- `r2`: capped", summary)

    def test_summarize_counts_errored_runs_as_failures_and_missing_costs_as_unknown(self):
        self.write_runs([
            {"run_id": "t1", "timestamp": "2026-10-15T10:00:00", "usecase": "uc", "config": "max", "subtype": "harness_timeout",
             "wall_s": "3600", "check_pass": "1", "notes": "timed out"},
            {"run_id": "t2", "timestamp": "2026-10-15T10:10:00", "usecase": "uc", "config": "max", "subtype": "success",
             "wall_s": "20", "total_cost_usd": "0.3", "check_pass": "1"},
            {"run_id": "t3", "timestamp": "2026-10-15T10:20:00", "usecase": "uc", "config": "max", "subtype": "harness_error",
             "wall_s": "1", "notes": "harness error"},
            {"run_id": "t4", "timestamp": "2026-10-15T10:30:00", "usecase": "nocheck", "config": "max", "subtype": "success",
             "wall_s": "5", "total_cost_usd": "0.1"},
        ])
        p = self.bench("summarize")
        self.assertEqual(p.returncode, 0, p.stderr)
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertIn("| uc | max | 3 | 2 | 33% | - | 20.0 | 1.0-3600.0 | $0.300 | - | - | - | - |", summary)
        self.assertIn("| nocheck | max | 1 | 0 | - | - | 5.0 | 5.0-5.0 | $0.100 | - | $0.100 | - | - |", summary)
        self.assertIn("| ALL | max | 4 | 2 | 33% | - | 12.5 | 1.0-3600.0 | $0.200 | - | - | - | - |", summary)

    def test_summarize_counts_a_harness_error_of_a_checked_use_case_without_check_results(self):
        self.write_runs([
            {"run_id": "h1", "timestamp": "2026-10-15T10:00:00", "usecase": "example-fix-tests", "config": "max",
             "subtype": "success", "wall_s": "20", "total_cost_usd": "0.3", "check_pass": "1"},
            {"run_id": "h2", "timestamp": "2026-10-15T11:00:00", "usecase": "example-fix-tests", "config": "max",
             "subtype": "harness_error", "wall_s": "1", "notes": "harness error"},
        ])
        p = self.bench("summarize", "--since", "2026-10-15T10:30")
        self.assertEqual(p.returncode, 0, p.stderr)
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertIn("| example-fix-tests | max | 1 | 1 | 0% |", summary)

    def test_summarize_since_keeps_only_later_runs(self):
        self.write_runs(ROWS)
        p = self.bench("summarize", "--since", "2026-10-15T10:30")
        self.assertEqual(p.returncode, 0, p.stderr)
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertIn("| uc | normal |", summary)
        self.assertNotIn("| uc | light |", summary)


class ApiErrorTest(RepoTest):
    """Runs the API refused or cut off: what run records and stops, and what summarize leaves out."""

    def refused_run(self, fake, *args):
        write_file(os.path.join(self.root, "bench-mcp.json"), json.dumps({"mcpServers": {"noop": {"command": "true", "args": []}}}))
        write_file(os.path.join(self.tmp, "bin", "claude"), fake, 0o755)
        p = self.bench("run", "--runs", "1", "-c", "light", "normal", "-u", "example-fix-tests", *args)
        with open(self.results("runs.csv"), newline="", encoding="utf-8") as f:
            return p, list(csv.DictReader(f))

    def assert_not_graded(self, p, rows):
        # A run the API refused gets no check and no judge call (which would be refused too).
        self.assertNotIn("  check:", p.stderr)
        self.assertNotIn("  judge:", p.stderr)
        self.assertEqual([(r["check_pass"], r["judge_score"]) for r in rows], [("", "")] * len(rows))

    def test_a_usage_limit_is_recorded_as_such_and_stops_the_run(self):
        self.env["FAKE_STATUS"] = "429"
        p, rows = self.refused_run(FAKE_REFUSED_CLAUDE)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn("stopped at a usage limit after 1 of 2 runs", p.stderr)
        self.assertEqual([r["subtype"] for r in rows], ["usage_limit"])
        self.assertIn("API error 429: refused", rows[0]["notes"])
        self.assert_not_graded(p, rows)

    def test_a_server_error_is_recorded_and_left_out_and_the_run_goes_on(self):
        self.env["FAKE_STATUS"] = "500"
        p, rows = self.refused_run(FAKE_REFUSED_CLAUDE)
        self.assertEqual([r["subtype"] for r in rows], ["api_error", "api_error"])
        self.assertNotIn("stopped at a usage limit", p.stderr)
        self.assertIn("no rows to summarize (2 left out: the API refused or cut them off)", p.stderr)
        self.assert_not_graded(p, rows)

    def test_another_rejection_counts_as_the_presets_failure(self):
        # A 4xx, such as a request too large, may be the preset's doing: it is graded and stays in the results.
        self.env["FAKE_STATUS"] = "400"
        p, rows = self.refused_run(FAKE_REFUSED_CLAUDE, "--no-judge")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertEqual([r["subtype"] for r in rows], ["api_rejected", "api_rejected"])
        self.assertIn("  check:", p.stderr)
        self.assertEqual([r["check_pass"] for r in rows], ["0", "0"])
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertNotIn("Left out", summary)
        self.assertIn("| example-fix-tests | light | 1 | 1 | 0% |", summary)

    def test_a_run_with_a_server_error_exits_nonzero_after_summarizing_the_rest(self):
        self.env["FAKE_SEEN"] = os.path.join(self.tmp, "seen")
        p, rows = self.refused_run(FAKE_FLAKY_CLAUDE, "--no-judge")
        self.assertNotEqual(p.returncode, 0)
        self.assertIn("1 run(s) failed with a server error and are left out of the results; rerun them", p.stderr)
        self.assertEqual([r["subtype"] for r in rows], ["api_error", "success"])
        with open(self.results("summary.md"), encoding="utf-8") as f:
            self.assertIn("Left out: 1 run(s)", f.read())

    def test_summarize_leaves_out_runs_the_api_refused_or_cut_off(self):
        refused = [
            {"run_id": "u1", "timestamp": "2026-10-15T12:00:00", "usecase": "uc", "config": "light", "subtype": "usage_limit",
             "is_error": "True", "wall_s": "3", "total_cost_usd": "0", "check_pass": "0"},
            {"run_id": "u2", "timestamp": "2026-10-15T12:05:00", "usecase": "uc", "config": "light", "subtype": "success",
             "is_error": "True", "wall_s": "2400", "total_cost_usd": "6.3", "check_pass": "0"},
            {"run_id": "u3", "timestamp": "2026-10-15T12:10:00", "usecase": "uc", "config": "normal", "subtype": "api_error",
             "is_error": "True", "wall_s": "2", "check_pass": "0"},
        ]
        self.write_runs(ROWS + refused)
        p = self.bench("summarize")
        self.assertEqual(p.returncode, 0, p.stderr)
        with open(self.results("summary.md"), encoding="utf-8") as f:
            summary = f.read()
        self.assertIn("| uc | light | 2 | 1 | 50% | 9.0 | 20.0 | 10.0-30.0 | $0.300 | $0.000 | $0.600 | 4.0 | 100 |", summary)
        self.assertIn("| uc | normal | 1 | 0 | 100% | 8.0 | 20.0 | 20.0-20.0 | $0.500 | - | $0.500 | 6.0 | - |", summary)
        self.assertIn("Left out: 3 run(s) the API refused or cut off (a usage limit or another API error): u1, u2, u3.", summary)
        self.assertIn("left out: 3 run(s)", p.stdout)

class PreflightTest(unittest.TestCase):
    """What run and smoke check before any model call, run in-process with the checks' inputs patched."""

    def setUp(self):
        self.tmp = temp_dir(self)
        self.args = argparse.Namespace(config=None, usecase=["example-fix-tests"], dry_run=False, no_judge=True, judge_model=None)
        patcher = mock.patch.object(bench, "log")
        patcher.start()
        self.addCleanup(patcher.stop)

    def mcp_config(self, script):
        return write_file(os.path.join(self.tmp, "mcp.json"), json.dumps({"mcpServers": {"srv": {"command": script, "args": ["-x"]}}}))

    def test_run_and_smoke_refuse_a_machine_with_a_managed_mcp_config(self):
        managed = write_file(os.path.join(self.tmp, "managed-mcp.json"), "{}\n")
        with mock.patch.object(bench, "MANAGED_MCP_PATHS", [managed]), mock.patch.object(bench, "claude_version", side_effect=AssertionError("too late")):
            for cmd in (bench.cmd_run, bench.cmd_smoke):
                with self.assertRaises(SystemExit) as cm:
                    cmd(self.args)
                self.assertIn(managed, str(cm.exception))

    def test_run_starts_each_mcp_server_before_scheduling_and_stops_if_one_fails(self):
        seen = os.path.join(self.tmp, "seen")
        good = write_file(os.path.join(self.tmp, "good.sh"), '#!/bin/bash\necho "$@" > "%s"\n' % seen, 0o755)
        bad = write_file(os.path.join(self.tmp, "bad.sh"), "#!/bin/bash\nexit 3\n", 0o755)
        with mock.patch.object(bench, "MANAGED_MCP_PATHS", []), mock.patch.object(bench, "claude_version", side_effect=RuntimeError("scheduling")):
            with mock.patch.object(bench, "MCP_CONFIG", self.mcp_config(bad)), self.assertRaises(SystemExit) as cm:
                bench.cmd_run(self.args)
            self.assertIn("srv", str(cm.exception))
            with mock.patch.object(bench, "MCP_CONFIG", self.mcp_config(good)), self.assertRaises(RuntimeError):
                bench.cmd_run(self.args)
        with open(seen, encoding="utf-8") as f:
            self.assertEqual(f.read(), "-x --help\n")

    def test_run_refuses_a_use_case_whose_plugin_is_not_installed(self):
        config, usecases = os.path.join(self.tmp, "config"), os.path.join(self.tmp, "usecases")
        write_file(os.path.join(usecases, "needs", "prompt.md"), "---\nplugins: kit@m\n---\nDo it.\n")
        args = argparse.Namespace(config=None, usecase=["needs"], dry_run=False, no_judge=True, judge_model=None)
        good = write_file(os.path.join(self.tmp, "good.sh"), "#!/bin/bash\n", 0o755)
        with mock.patch.dict(os.environ, {"CLAUDE_CONFIG_DIR": config}), mock.patch.object(bench, "USECASES_DIR", usecases), \
                mock.patch.object(bench, "MANAGED_MCP_PATHS", []), mock.patch.object(bench, "MCP_CONFIG", self.mcp_config(good)), \
                mock.patch.object(bench, "claude_version", side_effect=RuntimeError("scheduling")):
            with self.assertRaises(SystemExit) as cm:
                bench.cmd_run(args)
            self.assertIn("kit@m", str(cm.exception))
            write_file(os.path.join(config, "plugins", "installed_plugins.json"), json.dumps({"plugins": {"kit@m": [{}]}}))
            with self.assertRaises(RuntimeError):
                bench.cmd_run(args)


if __name__ == "__main__":
    unittest.main()
