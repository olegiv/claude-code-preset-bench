#!/bin/bash
# high: Opus main model, Fable advisor, Sonnet subagents, effort xhigh
# Installed as oclaude-high by install-launchers.sh. Extra args from the harness are appended.
# CLAUDE_CODE_SUBAGENT_MODEL is a default: it reaches general-purpose and agents with no model:
# in their frontmatter. Pinned agents keep their model; model: inherit agents (built-in Explore,
# Plan) run on the main model.
# Uncomment FORCE to put every subagent on Sonnet (changes real-usage behaviour). It is an on/off
# switch (any non-empty value): frontmatter models are ignored, inherit included; forks and
# skills with model: inherit keep the main model; the Agent tool loses its model parameter.
# Until then it is unset, so a FORCE exported in your shell cannot leak in.
unset CLAUDE_CODE_SUBAGENT_MODEL_FORCE
# export CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1
export CLAUDE_CODE_SUBAGENT_MODEL=sonnet
exec claude --dangerously-skip-permissions --effort xhigh --model opus --advisor fable "$@"
