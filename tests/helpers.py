"""Shared helpers for the tests: import bench.py and build throwaway fixtures."""

import os
import shutil
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CONFIGS_DIR = os.path.join(ROOT, "configs")
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

import bench  # noqa: E402  pylint: disable=wrong-import-position,unused-import

# Stands in for the Claude Code CLI: prints its arguments and the subagent variables, exits 7.
FAKE_CLAUDE = """#!/bin/bash
i=0
for a in "$@"; do i=$((i+1)); printf 'arg%d=[%s]\\n' "$i" "$a"; done
printf 'CLAUDE_CODE_SUBAGENT_MODEL=%s\\n' "${CLAUDE_CODE_SUBAGENT_MODEL-<unset>}"
printf 'CLAUDE_CODE_SUBAGENT_MODEL_FORCE=%s\\n' "${CLAUDE_CODE_SUBAGENT_MODEL_FORCE-<unset>}"
exit 7
"""


def temp_dir(test):
    """A temporary directory, removed after the test, or after the class when given a TestCase class."""
    path = tempfile.mkdtemp()
    register = test.addClassCleanup if isinstance(test, type) else test.addCleanup
    register(shutil.rmtree, path, ignore_errors=True)
    return path


def write_file(path, text, mode=0o644):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)
    os.chmod(path, mode)
    return path


def make_fake_claude(directory):
    return write_file(os.path.join(directory, "claude"), FAKE_CLAUDE, 0o755)


def config_names():
    return sorted(fn[:-3] for fn in os.listdir(CONFIGS_DIR) if fn.endswith(".sh") and not fn.startswith("_"))


def env_without(*names):
    return {k: v for k, v in os.environ.items() if k not in names}
