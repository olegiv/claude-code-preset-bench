#!/bin/bash
# normal: Opus main model, default subagents, no advisor, effort medium (Opus 5.5's default)
# Installed as oclaude-normal by install-launchers.sh. --model and --effort are pinned so the arm
# cannot drift with settings or with a new default in a later Claude Code release.
# ASSUMPTION: your settings default was Opus. If it was Sonnet, change --model below (one word).
# Unset so a CLAUDE_CODE_SUBAGENT_MODEL_FORCE exported in your shell cannot leak in.
unset CLAUDE_CODE_SUBAGENT_MODEL_FORCE
exec claude --dangerously-skip-permissions --effort medium --model opus "$@"
