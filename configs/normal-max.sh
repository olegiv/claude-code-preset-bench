#!/bin/bash
# normal-max: normal at effort max - Opus main model, default subagents, no advisor
# Identical to normal.sh except --effort max, so normal vs normal-max shows whether max buys anything
# on Opus. It is what the old oclaude-normal (ccc) ran. Extra args from the harness are appended.
# Unset so a CLAUDE_CODE_SUBAGENT_MODEL_FORCE exported in your shell cannot leak in.
unset CLAUDE_CODE_SUBAGENT_MODEL_FORCE
exec claude --dangerously-skip-permissions --effort max --model opus "$@"
