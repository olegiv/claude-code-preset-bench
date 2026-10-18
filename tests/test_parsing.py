"""Pure helpers: numbers, frontmatter, Claude Code result JSON, cost accounting, formatting."""

import argparse
import json
import os
import re
import unittest
from unittest import mock

from helpers import ROOT, bench, temp_dir, write_file

RESULT = {
    "subtype": "success", "is_error": False, "duration_ms": 1200, "duration_api_ms": 900, "num_turns": 4,
    "total_cost_usd": 0.75,
    "usage": {"input_tokens": 10, "output_tokens": 200, "cache_read_input_tokens": 5000,
              "cache_creation_input_tokens": 300, "output_tokens_details": {"thinking_tokens": 50}},
    "modelUsage": {
        "claude-sonnet-5-5": {"costUSD": 0.25, "thinkingTokens": 7},
        "claude-opus-5-5": {"costUSD": 0.5, "thinkingTokens": 11},
    },
    "subagent_stats": {"spawned": 0},
    "permission_denials": [],
}


class FnumTest(unittest.TestCase):
    """fnum() turns CSV and JSON values into floats."""

    def test_numbers_and_numeric_strings(self):
        self.assertEqual(bench.fnum("1.5"), 1.5)
        self.assertEqual(bench.fnum(3), 3.0)

    def test_empty_and_invalid_values_give_the_default(self):
        self.assertIsNone(bench.fnum(None))
        self.assertEqual(bench.fnum("", 4), 4)
        self.assertEqual(bench.fnum("abc", -1), -1)
        self.assertEqual(bench.fnum([1], -1), -1)


class FrontmatterTest(unittest.TestCase):
    """parse_frontmatter() reads prompt.md without a YAML dependency."""

    def test_keys_comments_quotes_and_body(self):
        meta, body = bench.parse_frontmatter('---\nmax_turns: 12\n# a comment\nrepo: "~/x"\n---\nDo it.\n')
        self.assertEqual(meta, {"max_turns": "12", "repo": "~/x"})
        self.assertEqual(body, "Do it.")

    def test_a_value_may_contain_colons(self):
        meta, _ = bench.parse_frontmatter("---\nurl: http://example.com:8080\n---\nx")
        self.assertEqual(meta["url"], "http://example.com:8080")

    def test_without_frontmatter_the_whole_text_is_the_prompt(self):
        self.assertEqual(bench.parse_frontmatter("  just a prompt \n"), ({}, "just a prompt"))

    def test_unterminated_frontmatter_leaves_an_empty_prompt(self):
        self.assertEqual(bench.parse_frontmatter("---\nmax_turns: 3\nDo it.\n"), ({"max_turns": "3"}, ""))


class ParseResultJsonTest(unittest.TestCase):
    """parse_result_json() finds the result object in the CLI's stdout."""

    def test_whole_output_is_json(self):
        self.assertEqual(bench.parse_result_json(' {"a": 1} \n'), {"a": 1})

    def test_the_last_json_line_wins_over_other_output(self):
        self.assertEqual(bench.parse_result_json('warning: x\n{"a": 1}\nnoise\n{"b": 2}\n'), {"b": 2})

    def test_broken_json_lines_are_skipped(self):
        self.assertEqual(bench.parse_result_json('{"ok": true}\n{broken\n'), {"ok": True})

    def test_nothing_usable(self):
        self.assertIsNone(bench.parse_result_json(""))
        self.assertIsNone(bench.parse_result_json("plain text"))


class ExtractMetricsTest(unittest.TestCase):
    """extract_metrics() turns a result into CSV columns, splitting out the advisor's cost."""

    def test_advisor_cost_is_split_out_and_main_model_is_the_costliest_other_model(self):
        m = bench.extract_metrics(RESULT, advisor="opus")
        self.assertEqual(m["advisor_cost_usd"], 0.5)
        self.assertEqual(m["main_model"], "claude-sonnet-5-5")
        self.assertEqual(m["models_used"], "claude-opus-5-5;claude-sonnet-5-5")
        self.assertEqual(json.loads(m["per_model_cost_usd"]), {"claude-opus-5-5": 0.5, "claude-sonnet-5-5": 0.25})

    def test_without_an_advisor_the_costliest_model_is_main(self):
        m = bench.extract_metrics(RESULT)
        self.assertEqual(m["main_model"], "claude-opus-5-5")
        self.assertEqual(m["advisor_cost_usd"], "")

    def test_an_advisor_that_never_answered_costs_zero(self):
        m = bench.extract_metrics(RESULT, advisor="fable")
        self.assertEqual(m["advisor_cost_usd"], 0)
        self.assertEqual(m["main_model"], "claude-opus-5-5")

    def test_usage_columns(self):
        m = bench.extract_metrics(RESULT)
        self.assertEqual((m["input_tokens"], m["output_tokens"], m["cache_read_tokens"], m["cache_creation_tokens"]),
                         (10, 200, 5000, 300))
        self.assertEqual((m["subtype"], m["num_turns"], m["total_cost_usd"]), ("success", 4, 0.75))
        self.assertEqual(m["thinking_tokens"], 50)
        self.assertEqual(m["permission_denials"], 0)
        self.assertEqual(json.loads(m["subagent_stats"]), {"spawned": 0})

    def test_thinking_tokens_fall_back_to_the_per_model_sum(self):
        self.assertEqual(bench.extract_metrics(dict(RESULT, usage={"output_tokens": 1}))["thinking_tokens"], 18)

    def test_no_result(self):
        self.assertEqual(bench.extract_metrics(None), {})


class CommandAndFormatTest(unittest.TestCase):
    """The agent command line and the summary's number formatting."""

    def installed(self, *names):
        config = temp_dir(self)
        write_file(os.path.join(config, "plugins", "installed_plugins.json"), json.dumps({"plugins": {n: [{}] for n in names}}))
        return mock.patch.dict(os.environ, {"CLAUDE_CONFIG_DIR": config})

    def test_build_agent_cmd_appends_the_headless_flags_plugins_and_caps(self):
        with self.installed("sec@m"):
            cmd = bench.build_agent_cmd({"path": "/c/light.sh"}, {"prompt": "Do it", "max_turns": 5, "max_budget_usd": 1.5, "plugins": ["kit@m"]})
        self.assertEqual(cmd, ["bash", "/c/light.sh", "-p", "Do it", "--output-format", "json",
                               "--no-session-persistence", "--no-chrome", "--strict-mcp-config",
                               "--mcp-config", os.path.join(ROOT, "bench-mcp.json"),
                               "--settings", '{"enabledPlugins": {"kit@m": true, "sec@m": false}}',
                               "--max-turns", "5", "--max-budget-usd", "1.5"])

    def test_smoke_and_its_judge_get_no_plugins(self):
        seen = []
        args = argparse.Namespace(config=["light"], no_judge=False, judge_model="opus", judge_effort="high")
        with self.installed("sec@m"), mock.patch.object(bench, "MANAGED_MCP_PATHS", []), mock.patch.object(bench, "log"), \
                mock.patch.object(bench, "claude_version", return_value="test"), \
                mock.patch.object(bench, "run_claude", lambda cmd, cwd, env, timeout: seen.append(cmd) or (1, "", "", 0, False)):
            bench.cmd_smoke(args)
        self.assertEqual(len(seen), 2)
        for cmd in seen:
            self.assertEqual(cmd[cmd.index("--settings") + 1], '{"enabledPlugins": {"sec@m": false}}')

    def test_bench_mcp_config_holds_only_an_isolated_headless_chrome_devtools(self):
        with open(os.path.join(ROOT, "bench-mcp.json"), encoding="utf-8") as f:
            servers = json.load(f)["mcpServers"]
        self.assertEqual(list(servers), ["chrome-devtools"])
        args = servers["chrome-devtools"]["args"]
        self.assertEqual(servers["chrome-devtools"]["command"], "npx")
        self.assertTrue(any(re.match(r"^chrome-devtools-mcp@\d+\.\d+\.\d+$", a) for a in args), args)
        self.assertIn("--isolated", args)
        self.assertIn("--headless", args)

    def test_the_judge_gets_no_mcp_servers_no_plugins_and_up_to_120000_characters_of_diff(self):
        seen = []
        rubric = write_file(os.path.join(temp_dir(self), "rubric.md"), "Score it.\n")
        with mock.patch.object(bench, "run_claude", lambda cmd, cwd, env, timeout: seen.append(cmd) or (1, "", "", 0, False)), \
                mock.patch.object(bench, "log"), self.installed("sec@m"):
            bench.run_judge({"rubric": rubric}, "Do it", "Done", "x" * 130000, "PASS (score 1.00): 1.00", "opus", "high", {}, ".")
        prompt, flags = seen[0][2], seen[0][3:]
        self.assertTrue("--no-chrome" in flags and "--strict-mcp-config" in flags, flags)
        self.assertNotIn("--mcp-config", flags)
        self.assertEqual(flags[flags.index("--settings") + 1], '{"enabledPlugins": {"sec@m": false}}')
        self.assertTrue("x" * 120000 in prompt and "x" * 120001 not in prompt, "the diff is not cut at 120 000 characters")

    def test_median_ignores_missing_values(self):
        self.assertEqual(bench.median([3, None, 1, 2]), 2)
        self.assertIsNone(bench.median([None]))

    def test_fmt(self):
        self.assertEqual(bench.fmt(None), "-")
        self.assertEqual(bench.fmt(0.5, "pct"), "50%")
        self.assertEqual(bench.fmt(0.1234, "usd"), "$0.123")
        self.assertEqual(bench.fmt(2.6, "i"), "3")
        self.assertEqual(bench.fmt(2.34), "2.3")


if __name__ == "__main__":
    unittest.main()
