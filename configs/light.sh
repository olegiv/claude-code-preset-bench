#!/bin/bash
# light: Sonnet main model, Opus advisor, Haiku subagents, effort medium (Sonnet 5.5's default)
# The X-post recipe as published. Installed as oclaude-light by install-launchers.sh. Extra args
# from the harness are appended.
# CLAUDE_CODE_SUBAGENT_MODEL is a default: it reaches general-purpose and agents with no model:
# in their frontmatter. Pinned agents keep their model; model: inherit agents (built-in Explore,
# Plan) run on the main model.
# light-force.sh is this preset with CLAUDE_CODE_SUBAGENT_MODEL_FORCE on (every subagent on
# Haiku); it is unset here, also against a FORCE exported in your shell, so light vs light-force
# isolates the switch.
unset CLAUDE_CODE_SUBAGENT_MODEL_FORCE
export CLAUDE_CODE_SUBAGENT_MODEL=haiku
exec claude --dangerously-skip-permissions --effort medium --model sonnet --advisor opus "$@"
