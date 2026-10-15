"""clean_env(): what the runs inherit from the shell."""

import io
import os
import unittest
from contextlib import redirect_stderr
from unittest import mock

from helpers import bench


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


if __name__ == "__main__":
    unittest.main()
