"""The command line, run on a temporary copy of the repository with a fake claude."""

import csv
import os
import shutil
import subprocess
import sys
import unittest

from helpers import ROOT, bench, config_names, env_without, make_fake_claude, temp_dir

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


class CliTest(unittest.TestCase):
    """list, run --dry-run, summarize and argument errors."""

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

    def test_list_shows_the_configs_and_use_cases(self):
        p = self.bench("list")
        self.assertEqual(p.returncode, 0, p.stderr)
        for name in config_names() + ["example-fix-tests", "meowdoku"]:
            self.assertIn(name, p.stdout)

    def test_dry_run_prints_one_command_per_config_and_writes_no_results(self):
        p = self.bench("run", "--dry-run", "--runs", "1", "--no-judge", "-u", "example-fix-tests")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertEqual(p.stderr.count("DRY RUN"), len(config_names()))
        self.assertFalse(os.path.exists(self.results("runs.csv")))

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


if __name__ == "__main__":
    unittest.main()
