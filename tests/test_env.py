"""What the runs inherit: the shell's variables (clean_env) and the owner's plugins (plugin_settings)."""

import io
import json
import os
import unittest
from contextlib import redirect_stderr
from unittest import mock

from helpers import bench, temp_dir, write_file


class CleanEnvTest(unittest.TestCase):
    """Overrides and a parent Claude Code session's variables are dropped; values are never logged."""

    def clean_env(self, environ):
        err = io.StringIO()
        with mock.patch.dict(os.environ, environ, clear=True), redirect_stderr(err):
            env = bench.clean_env()
        return env, err.getvalue()

    def test_drops_overrides_and_session_variables_and_keeps_the_rest(self):
        env, log = self.clean_env({
            "PATH": "/usr/bin", "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION": "true",
            "CLAUDE_CODE_SUBAGENT_MODEL": "haiku", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1",
            "CLAUDECODE": "1", "CLAUDE_CODE_CHILD_SESSION": "1", "CLAUDE_EFFORT": "max",
            "CLAUDE_CODE_MESSAGING_TOKEN": "secret-token-value",
        })
        self.assertEqual(env, {"PATH": "/usr/bin", "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION": "true"})
        self.assertIn("CLAUDE_CODE_MESSAGING_TOKEN", log)
        self.assertNotIn("secret-token-value", log)

    def test_every_session_variable_is_dropped(self):
        env, _ = self.clean_env(dict.fromkeys(bench.SESSION_VARS, "x"))
        self.assertEqual(env, {})

    def test_warn_variables_stay_but_are_reported(self):
        env, log = self.clean_env({"ANTHROPIC_MODEL": "opus"})
        self.assertEqual(env, {"ANTHROPIC_MODEL": "opus"})
        self.assertIn("ANTHROPIC_MODEL", log)

    def test_a_terminal_environment_is_untouched_and_quiet(self):
        env, log = self.clean_env({"PATH": "/usr/bin", "HOME": "/home/u"})
        self.assertEqual(env, {"PATH": "/usr/bin", "HOME": "/home/u"})
        self.assertEqual(log, "")


class PluginSettingsTest(unittest.TestCase):
    """--settings turns off every plugin Claude Code could load, except those a use case declares."""

    def setUp(self):
        self.config = temp_dir(self)
        patcher = mock.patch.dict(os.environ, {"CLAUDE_CONFIG_DIR": self.config})
        patcher.start()
        self.addCleanup(patcher.stop)

    def settings(self, keep=()):
        return json.loads(bench.plugin_settings(keep))["enabledPlugins"]

    def write(self, rel, text):
        write_file(os.path.join(self.config, rel), text)

    def test_installed_enabled_and_synced_plugins_are_off_except_the_declared_ones(self):
        self.write("plugins/installed_plugins.json", json.dumps({"version": 2, "plugins": {"sec@m": [{}], "kit@m": [{}]}}))
        self.write("settings.json", json.dumps({"enabledPlugins": {"other@m": True, "off@m": False}}))
        self.write("plugins/synced/org_user/design/.claude-plugin/plugin.json", "{}")
        self.assertEqual(self.settings(["kit@m"]), {"sec@m": False, "kit@m": True, "other@m": False, "off@m": False, "design@synced": False})
        self.assertEqual(bench.installed_plugins(), {"sec@m", "kit@m"})

    def test_an_installed_plugins_file_without_a_plugins_key_is_read_whole(self):
        self.write("plugins/installed_plugins.json", json.dumps({"sec@m": [{}]}))
        self.assertEqual(bench.installed_plugins(), {"sec@m"})

    def test_missing_or_broken_files_leave_only_the_declared_plugins(self):
        self.assertEqual(self.settings(), {})
        self.write("plugins/installed_plugins.json", "not json")
        self.write("settings.json", "[1]")
        self.assertEqual(self.settings(["kit@m"]), {"kit@m": True})
        self.assertEqual(bench.installed_plugins(), set())


if __name__ == "__main__":
    unittest.main()
