#!/bin/bash
# Deterministic grader. exit 0 = pass, non-zero = fail.
# Optional: print a partial score 0..1 as the LAST stdout line (e.g. "0.75").
# Env: WORKSPACE (cwd), USECASE_DIR, RUN_ID, CONFIG, RESULT_JSON (full claude JSON), RESULT_TEXT (final message)
set -u
cd "$WORKSPACE" || exit 1

# examples:
#   npm test --silent            || { echo "FAIL: tests"; exit 1; }
#   test -f docs/ADR-001.md      || { echo "FAIL: missing file"; exit 1; }
#   grep -q "def migrate" app.py || { echo "FAIL: no migrate()"; exit 1; }

echo "PASS"
exit 0
