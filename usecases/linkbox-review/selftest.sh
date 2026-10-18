#!/bin/bash
# selftest.sh [variant...]: checks the use case itself, then grades known REVIEW.md variants and compares
# check.sh's exit code, score and output with what the grader should say. Each workspace is built as bench.py
# builds one (workspace/ committed, then setup.sh). Needs Go, git and python3. CHECK_BASH=/bin/bash runs
# check.sh under macOS's bash 3.2.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
CHECK_BASH=${CHECK_BASH:-bash}

# name|expected exit|expected score (or <N)|text the output must contain
VARIANTS='perfect|0|1.00|issues found 10/10
perfect-toolkit-used|0|1.00|ok: pr-review-toolkit used
perfect-toolkit-killed|1|1.00|no completed pr-review-toolkit agent
perfect-no-toolkit|1|1.00|no completed pr-review-toolkit agent
perfect-toolkit-killed-explore-done|1|1.00|no completed pr-review-toolkit agent
seven|0|0.70|issues found 7/10
half|1|0.50|PASS needs 0.60
perfect-plus-two-decoys|0|0.80|decoys flagged
all-decoys|1|0.00|issues found 0/10
duplicates|1|0.10|repeated one
spam-ranges|1|0.00|issues found 0/10
spam-evenly-spaced|1|<0.40|findings read: 25
26-findings|1|1.00|26 findings
wrong-format|1|0.00|no finding in the required format
no-review|1|0.00|no REVIEW.md
code-edited|1|0.00|files of the pull request changed
committed|1|0.00|HEAD moved
extra-file|1|1.00|files added besides REVIEW.md
binary-ignored|0|1.00|issues found 10/10'

# make_workspace <dir>: workspace/ committed as bench.py does, then setup.sh.
make_workspace() {
  cp -R "$HERE/workspace" "$1" &&
    (cd "$1" && git init -q && git add -A &&
      git -c user.name=ccbench -c user.email=ccbench@local -c commit.gpgsign=false commit -q --no-verify -m base &&
      WORKSPACE=$1 USECASE_DIR=$HERE bash "$HERE/setup.sh")
}

# write_review <variant> <workspace>: REVIEW.md built from the answer key's anchors.
write_review() {
  python3 - "$HERE/answer-key.json" "$1" "$2" <<'EOF'
import json, os, sys
key, variant, ws = json.load(open(sys.argv[1])), sys.argv[2], sys.argv[3]

def line_of(path, anchor):
    lines = open(os.path.join(ws, path)).read().split("\n")
    return next(i + 1 for i, l in enumerate(lines) if anchor in l)

def finding(item, severity="high"):
    path, anchor = next(iter(item["anchors"].items()))
    return "- [%s] %s:%d — %s: see the code." % (severity, path, line_of(path, anchor), item["id"])

issues = [finding(i) for i in key["issues"]]
decoys = [finding(d, "medium") for d in key["decoys"]]
files = ["README.md", "cmd/linkbox/main.go", "internal/api/api.go", "internal/api/api_test.go",
         "internal/export/export.go", "internal/export/export_test.go", "internal/preview/preview.go",
         "internal/preview/preview_test.go", "internal/store/store.go"]
body = {
    "seven": issues[:7],
    "half": issues[:5],
    "perfect-plus-two-decoys": issues + decoys[:2],
    "all-decoys": decoys,
    "duplicates": [issues[0]] * 3,
    "spam-ranges": ["- [high] %s:1-400 — everything here: wrong." % f for f in files],
    "spam-evenly-spaced": ["- [medium] %s:%d — looks wrong: maybe." % (f, n) for n in range(10, 200, 20) for f in files][:25],
    "26-findings": issues + ["- [low] README.md:%d — nit: wording." % n for n in range(1, 17)],
    "wrong-format": ["- %s (%s)" % (i["summary"], next(iter(i["anchors"]))) for i in key["issues"]],
}.get(variant, issues)
with open(os.path.join(ws, "REVIEW.md"), "w") as f:
    f.write("# Review\n\nThe pull request needs changes.\n\n## Findings\n\n" + "\n".join(body) + "\n")
EOF
}

# The use case's own invariants: anchors inside their windows, no overlapping windows, and both trees build, vet
# and test cleanly (the agents will run them).
precheck() {
  local dir ok=0
  dir=$(mktemp -d "${TMPDIR:-/tmp}/linkbox-selftest.XXXXXX")
  make_workspace "$dir/pr" >/dev/null || { echo "FAIL  precheck: workspace or setup.sh"; return 1; }
  python3 - "$HERE/answer-key.json" "$dir/pr" <<'EOF' || ok=1
import json, sys
key, ws = json.load(open(sys.argv[1])), sys.argv[2]
bad, spans = 0, []
for kind in ("issues", "decoys"):
    for it in key[kind]:
        for path, anchor in it["anchors"].items():
            lines = open("%s/%s" % (ws, path)).read().split("\n")
            hits = [i + 1 for i, l in enumerate(lines) if anchor in l]
            if len(hits) != 1 or not any(a <= hits[0] <= b for a, b in it["windows"][path]):
                print("FAIL  precheck: %s anchor in %s is at %s" % (it["id"], path, hits)); bad += 1
        spans += [(p, a, b, it["id"]) for p, ws_ in it["windows"].items() for a, b in ws_]
for i, (p, a, b, n) in enumerate(spans):
    for q, c, d, m in spans[i + 1:]:
        if p == q and n != m and a <= d and c <= b:
            print("FAIL  precheck: windows of %s and %s overlap in %s" % (n, m, p)); bad += 1
sys.exit(1 if bad else 0)
EOF
  cp -R "$HERE/workspace" "$dir/base"
  for t in base pr; do
    (cd "$dir/$t" && GOTOOLCHAIN=local go build ./... && GOTOOLCHAIN=local go vet ./... &&
      GOTOOLCHAIN=local go test -count=1 -race ./... >/dev/null) || { echo "FAIL  precheck: $t tree does not build, vet and test"; ok=1; }
  done
  rm -rf "$dir"
  [ "$ok" = 0 ] && echo "ok    precheck: anchors, windows, base and pull request build, vet and test"
  return "$ok"
}

run_variant() {
  local name=$1 want_exit=$2 want_score=$3 want_text=$4 dir ws out rc score ok=1 result=""
  dir=$(mktemp -d "${TMPDIR:-/tmp}/linkbox-selftest.XXXXXX")
  ws=$dir/ws
  make_workspace "$ws" >/dev/null || { echo "FAIL  $name: workspace or setup.sh"; return 1; }
  [ "$name" = no-review ] || write_review "$name" "$ws"
  case $name in
    code-edited) printf '\n// reviewed\n' >> "$ws/internal/api/api.go" ;;
    committed) (cd "$ws" && git add -A && git -c user.name=agent -c user.email=agent@local -c commit.gpgsign=false commit -q -m review) ;;
    extra-file) echo "scratch" > "$ws/notes.txt" ;;
    binary-ignored) echo "binary" > "$ws/linkbox" && echo "test binary" > "$ws/api.test" ;;
  esac
  case $name in
    perfect-toolkit-used) result='{"subagent_stats":{"completed":3,"by_type":{"pr-review-toolkit:code-reviewer":1,"pr-review-toolkit:pr-test-analyzer":1}}}' ;;
    perfect-toolkit-killed) result='{"subagent_stats":{"completed":0,"by_type":{"pr-review-toolkit:code-reviewer":1}}}' ;;
    perfect-no-toolkit) result='{"subagent_stats":{"completed":1,"by_type":{"Explore":1}}}' ;;
    perfect-toolkit-killed-explore-done) result='{"subagent_stats":{"completed":1,"by_type":{"Explore":1,"pr-review-toolkit:code-reviewer":1}}}' ;;
  esac
  if [ -n "$result" ]; then
    echo "$result" > "$dir/result.json"
    out=$(cd "$ws" && WORKSPACE=$ws USECASE_DIR=$HERE RESULT_JSON=$dir/result.json "$CHECK_BASH" "$HERE/check.sh" 2>&1)
  else
    out=$(cd "$ws" && WORKSPACE=$ws USECASE_DIR=$HERE "$CHECK_BASH" "$HERE/check.sh" 2>&1)
  fi
  rc=$?
  score=$(printf '%s\n' "$out" | tail -n 1)
  [ "$rc" = "$want_exit" ] || ok=0
  case $want_score in
    "<"*) awk -v s="$score" -v m="${want_score#<}" 'BEGIN { exit !(s + 0 < m + 0) }' || ok=0 ;;
    *) [ "$score" = "$want_score" ] || ok=0 ;;
  esac
  printf '%s\n' "$out" | grep -qF -- "$want_text" || ok=0
  if [ "$ok" = 1 ]; then
    printf 'ok    %-26s exit %s, score %s\n' "$name" "$rc" "$score"
  else
    printf 'FAIL  %-26s exit %s (want %s), score %s (want %s), wanted text: %s\n' "$name" "$rc" "$want_exit" "$score" \
      "$want_score" "$want_text"
    printf '%s\n' "$out" | sed 's/^/      /'
  fi
  rm -rf "$dir"
  [ "$ok" = 1 ]
}

failed=0
count=0
if [ "$#" -eq 0 ]; then
  precheck || failed=$((failed + 1))
fi
while IFS='|' read -r name want_exit want_score want_text; do
  [ -n "$name" ] || continue
  if [ "$#" -gt 0 ]; then
    case " $* " in *" $name "*) ;; *) continue ;; esac
  fi
  count=$((count + 1))
  run_variant "$name" "$want_exit" "$want_score" "$want_text" || failed=$((failed + 1))
done <<EOF
$VARIANTS
EOF
if [ "$failed" -eq 0 ]; then
  echo "SELFTEST OK: $count variant(s)"
else
  echo "SELFTEST FAILED: $failed check(s)"
  exit 1
fi
