#!/bin/bash
# Deterministic grader. Contract: exit 0 = pass, non-zero = fail.
# Optionally print a partial score between 0 and 1 as the LAST line of stdout.
# Env available: WORKSPACE, USECASE_DIR, RUN_ID, CONFIG, RESULT_JSON, RESULT_TEXT
set -u
cd "$WORKSPACE" || exit 1

# 1. The test file must be untouched.
if ! cmp -s test_textutils.py "$USECASE_DIR/workspace/test_textutils.py"; then
  echo "FAIL: test_textutils.py was modified"
  exit 1
fi

# 2. The tests must pass.
if ! python3 -m unittest -q test_textutils >/tmp/ccbench-check.$$ 2>&1; then
  tail -n 3 /tmp/ccbench-check.$$
  rm -f /tmp/ccbench-check.$$
  echo "FAIL: tests still failing"
  exit 1
fi
rm -f /tmp/ccbench-check.$$
echo "PASS: tests green, test file untouched"
exit 0
