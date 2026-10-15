#!/bin/bash
# light-force: light with every subagent forced onto Haiku - Sonnet main, Opus advisor, effort medium
# Identical to light.sh except CLAUDE_CODE_SUBAGENT_MODEL_FORCE, so light vs light-force isolates
# the switch. Extra args from the harness are appended.
# FORCE is an on/off switch (any non-empty value): frontmatter models are ignored, inherit included,
# so Explore, Plan and pinned agents (pr-review-toolkit, project agents) run on Haiku too; forks and
# skills with model: inherit keep the main model; the Agent tool loses its model parameter.
export CLAUDE_CODE_SUBAGENT_MODEL=haiku
export CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1
exec claude --dangerously-skip-permissions --effort medium --model sonnet --advisor opus "$@"
