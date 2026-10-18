#!/bin/bash
# Grades a lazygit-conflict-markers run (bash 3.2 or later). The workspace is copied into a container without
# network, where its integration tests are replaced by upstream's at the base commit plus the hidden ones;
# lazygit is then built, checked and tested there. The score is the share of hidden acceptance tests that
# pass; a hard-gate failure scores 0.00, and a failed soft gate fails the check without lowering the score.
set -u

USECASE_DIR=${USECASE_DIR:-$(cd "$(dirname "$0")" && pwd)}
WS=${WORKSPACE:-$PWD}
. "$USECASE_DIR/pins.sh"
HIDDEN=$USECASE_DIR/hidden
# Each fails at the base commit and passes at the reference (selftest.sh checks both).
ACCEPTANCE="conflicts/conflict_marker_size_resolve conflicts/conflict_marker_size_not_auto_staged
conflicts/conflict_marker_size_short_markers_are_content conflicts/conflict_marker_size_from_pattern
conflicts/conflict_marker_size_resolve_all_hunks conflicts/conflict_marker_size_mixed_sizes
conflicts/conflict_marker_size_other_attribute_sources"
# Container limits. go test stops the unit tests at 60 s (normally 5 s) and the integration run at 4 minutes
# (normally 20 s; 3 minutes in a rerun), inside the container's limit, so that the results so far survive a hung
# test. The worst case, 360 + 240 + 240 s, stays under bench.py's 900 s.
GRADE_TIMEOUT=360
RERUN_TIMEOUT=240
MAX_RETRIES=10
# selftest.sh shortens the first integration run's timeout to exercise the rerun.
INTEGRATION_TIMEOUT=${LAZYGIT_CHECK_INTEGRATION_TIMEOUT:-4m}

# Runs in the container: unpacks the tree from stdin, writes logs and exit codes to /out, sends /out back.
# Progress goes to stderr, so a stopped container still says how far it got.
GRADE='set -u
mkdir -p /work /out && tar -x -C /work && cd /work || exit 90
export GOTOOLCHAIN=local GOFLAGS="-mod=vendor -buildvcs=false"
step() { name=$1; shift; echo "step: $name" >&2; "$@" > /out/$name.log 2>&1; echo $? > /out/$name.rc; }
step generate go generate ./pkg/integration/tests/...
step build go build ./...
if [ "$(cat /out/build.rc)" = 0 ]; then
  echo "step: gofmt" >&2
  find . -name "*.go" -not -path "./vendor/*" -not -path "./test/_results/*" -print0 | xargs -0 gofmt -l > /out/gofmt.log 2>&1
  step vet go vet ./...
  step unit go test -count=1 -short -timeout 60s ./...
  echo "step: integration" >&2
  go test -count=1 -json -timeout "$INTEGRATION_TIMEOUT" pkg/integration/clients/*.go -run TestIntegration > /out/integration.json 2> /out/integration.log
fi
echo "step: done" >&2
cd /out && tar -cf - .'
# Reruns integration tests: all of them when $RERUN is ALL, otherwise those named in $RERUN, one go test each.
RERUN_SCRIPT='set -u
mkdir -p /work /out && tar -x -C /work && cd /work || exit 90
export GOTOOLCHAIN=local GOFLAGS="-mod=vendor -buildvcs=false"
go generate ./pkg/integration/tests/... > /out/generate.log 2>&1
if [ "$RERUN" = ALL ]; then
  go test -count=1 -json -timeout 3m pkg/integration/clients/*.go -run TestIntegration > /out/rerun.json 2> /out/rerun.log
else
  for t in $RERUN; do
    go test -count=1 -json -timeout 1m pkg/integration/clients/*.go -run "TestIntegration/$t\$" >> /out/rerun.json 2>> /out/rerun.log
  done
fi
cd /out && tar -cf - .'

harness() { echo "FAIL: harness: $*"; echo "0.00"; exit 2; }
hard() { echo "FAIL: $*"; echo "FAIL: hard gate"; echo "0.00"; exit 1; }
show() { [ -s "$1" ] && sed -e 's/^/    /' "$1" | tail -n "${2:-40}"; return 0; }

# container <out dir> <seconds> <script> [docker args...]: pipes $TMP/tree into a fresh container, stops it in time.
container() {
  local out=$1 limit=$2 script=$3 name pid t=0
  shift 3
  name=ccbench-lazygit-$$-$RANDOM
  mkdir -p "$out"
  tar -C "$TMP/tree" -cf - . | docker run --rm -i --name "$name" --network none --pull never "$@" "$IMAGE" \
    bash -c "$script" > "$out.tar" 2> "$out.err" &
  pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [ "$t" -ge "$limit" ]; then
      docker kill "$name" >/dev/null 2>&1
      echo "WARN: container stopped after ${limit}s, at $(grep '^step: ' "$out.err" | tail -n 1)"
      break
    fi
    sleep 1
    t=$((t + 1))
  done
  wait "$pid"
  tar -x -C "$out" -f "$out.tar" 2>/dev/null
  # Agent code ran in the container and could have left links to host files among the results: keep only
  # regular files and directories.
  find "$out" -mindepth 1 ! -type f ! -type d -exec rm -f {} +
  return 0
}

# tally <go test -json files...>: one line per expected test, "acceptance|regression <name> <action>". A test's
# first final event counts, so a later file only fills in tests that earlier ones left without a result; a test
# with none (hung, or not started before a timeout) is "missing".
tally() {
  python3 - "$ACCEPTANCE" "$TMP/expected.txt" "$@" <<'EOF'
import json, sys
acceptance = sys.argv[1].split()
expected = [l.strip() for l in open(sys.argv[2], encoding="utf-8") if l.strip()]
final = {}
for path in sys.argv[3:]:
    try:
        lines = open(path, encoding="utf-8", errors="replace").readlines()
    except OSError:
        continue
    for line in lines:
        try:
            e = json.loads(line)
        except ValueError:
            continue
        t = e.get("Test", "")
        if t.startswith("TestIntegration/") and e.get("Action") in ("pass", "fail", "skip"):
            final.setdefault(t[len("TestIntegration/"):], e["Action"])
for name in expected:
    print("%s %s %s" % ("acceptance" if name in acceptance else "regression", name, final.get(name, "missing")))
EOF
}

main() {
  local T0=$SECONDS soft=0 f total passed score regress failed_acc
  command -v docker >/dev/null 2>&1 || harness "docker not found"
  docker info >/dev/null 2>&1 || harness "the Docker daemon is not running"
  docker image inspect "$IMAGE" >/dev/null 2>&1 || harness "image $IMAGE is missing (setup.sh pulls it)"
  command -v python3 >/dev/null 2>&1 || harness "python3 not found"
  git -C "$CACHE" rev-parse -q --verify "$BASE_REF^{commit}" >/dev/null 2>&1 || harness "$CACHE lacks the base commit (setup.sh fetches it)"
  [ -f "$WS/go.mod" ] || hard "no go.mod in the workspace"

  # Hard gate: dependencies exactly as at the base commit, so the offline build tests the agent's code only.
  mkdir -p "$TMP/base" "$TMP/tree"
  git -C "$CACHE" archive "$BASE_REF" go.mod go.sum vendor pkg/integration/tests pkg/integration/components \
    pkg/integration/clients | tar -x -C "$TMP/base" || harness "cannot read the base commit from $CACHE"
  for f in go.mod go.sum; do
    cmp -s "$TMP/base/$f" "$WS/$f" || hard "$f changed or missing"
  done
  diff -rq "$TMP/base/vendor" "$WS/vendor" > "$TMP/vendor.diff" 2>&1 || { show "$TMP/vendor.diff" 10; hard "vendor/ changed"; }
  echo "ok: go.mod, go.sum and vendor/ unchanged"

  # The grading tree: the workspace, with upstream's integration tests at the base commit and the hidden ones in
  # place of whatever the agent left there, so its own tests can neither collide with nor stand in for them.
  # rm -rf and cp below would follow a link out of $TMP, so these must be real directories.
  for f in pkg pkg/integration; do
    [ ! -L "$WS/$f" ] || hard "$f is a symbolic link"
    [ -d "$WS/$f" ] || hard "$f is missing"
  done
  tar -C "$WS" --exclude ./test/_results -cf - . | tar -x -C "$TMP/tree" || harness "cannot copy the workspace"
  [ ! -L "$TMP/tree/pkg" ] && [ ! -L "$TMP/tree/pkg/integration" ] || harness "the copy of pkg/integration is a link"
  rm -rf "$TMP/tree/pkg/integration/tests" "$TMP/tree/pkg/integration/components" "$TMP/tree/pkg/integration/clients"
  cp -R "$TMP/base/pkg/integration/tests" "$TMP/base/pkg/integration/components" "$TMP/base/pkg/integration/clients" \
    "$TMP/tree/pkg/integration/" && cp -R "$HIDDEN/pkg" "$TMP/tree/" || harness "cannot place the hidden tests"
  # Every integration test lives in a subdirectory; a test's name is its path there, without .go.
  (cd "$TMP/tree/pkg/integration/tests" && grep -rl --include='*.go' 'NewIntegrationTest(' . | sed 's#^\./##; s#\.go$##' |
    grep / | sort) > "$TMP/expected.txt"

  container "$TMP/out" "$GRADE_TIMEOUT" "$GRADE" -e INTEGRATION_TIMEOUT="$INTEGRATION_TIMEOUT"
  [ -f "$TMP/out/build.rc" ] || { show "$TMP/out.err"; harness "the grading container produced no results"; }
  [ "$(cat "$TMP/out/generate.rc")" = 0 ] || { show "$TMP/out/generate.log"; harness "go generate failed on the integration tests"; }
  if [ "$(cat "$TMP/out/build.rc")" != 0 ]; then
    show "$TMP/out/build.log"
    hard "go build ./... failed"
  fi
  echo "ok: go build ./..."
  if [ -s "$TMP/out/gofmt.log" ]; then
    echo "FAIL: gofmt -l lists:"; show "$TMP/out/gofmt.log" 20; soft=$((soft + 1))
  else
    echo "ok: gofmt"
  fi
  if [ "$(cat "$TMP/out/vet.rc" 2>/dev/null)" != 0 ]; then
    echo "FAIL: go vet ./..."; show "$TMP/out/vet.log"; soft=$((soft + 1))
  else
    echo "ok: go vet ./..."
  fi
  if [ "$(cat "$TMP/out/unit.rc" 2>/dev/null)" != 0 ]; then
    echo "FAIL: go test -short ./... (the unit tests in the workspace)"
    grep -E '^(--- FAIL|FAIL|panic)' "$TMP/out/unit.log" | head -n 20 | sed 's/^/    /'
    soft=$((soft + 1))
  else
    echo "ok: go test -short ./..."
  fi

  tally "$TMP/out/integration.json" > "$TMP/tally.txt"
  # A hung test stops the run at go test's timeout and leaves tests without a result: run the suite once more.
  if grep -q ' missing$' "$TMP/tally.txt"; then
    if ! grep -q '^step: done' "$TMP/out.err"; then
      show "$TMP/out.err"
      harness "the grading container stopped before the integration tests finished"
    fi
    echo "WARN: $(grep -c ' missing$' "$TMP/tally.txt") integration test(s) without a result (first: $(grep -m 1 ' missing$' "$TMP/tally.txt" | awk '{print $2}')); running the suite again"
    container "$TMP/again" "$RERUN_TIMEOUT" "$RERUN_SCRIPT" -e RERUN=ALL
    tally "$TMP/out/integration.json" "$TMP/again/rerun.json" > "$TMP/tally.txt"
  fi
  if ! grep -q ' pass$' "$TMP/tally.txt"; then
    show "$TMP/out/integration.log"
    hard "the integration tests did not run (they must still compile against the changed code)"
  fi
  echo "integration tests: $(grep -c ' pass$' "$TMP/tally.txt") passed, $(grep -c ' fail$' "$TMP/tally.txt") failed, $(grep -c ' skip$' "$TMP/tally.txt") skipped, $(grep -c ' missing$' "$TMP/tally.txt") without a result"

  # A regression test that failed gets one more run in a new container; only a pass there clears it.
  regress=$(grep '^regression ' "$TMP/tally.txt" | grep -E ' (fail|missing)$' | awk '{print $2}')
  if [ -n "$regress" ] && [ "$(echo "$regress" | wc -l | tr -d ' ')" -le "$MAX_RETRIES" ]; then
    container "$TMP/retry" "$RERUN_TIMEOUT" "$RERUN_SCRIPT" -e RERUN="$(echo "$regress" | tr '\n' ' ')"
    tally "$TMP/retry/rerun.json" > "$TMP/retry.txt"
    regress=$(for f in $regress; do grep -qFx "regression $f pass" "$TMP/retry.txt" || echo "$f"; done)
  fi
  if [ -n "$regress" ]; then
    echo "FAIL: existing behavior broken, $(echo "$regress" | wc -l | tr -d ' ') integration test(s): $(echo "$regress" | head -n 15 | paste -sd, - | sed 's/,/, /g')"
    soft=$((soft + 1))
  else
    echo "ok: no regression in the other integration tests"
  fi

  total=$(grep -c '^acceptance ' "$TMP/tally.txt")
  passed=$(grep -c '^acceptance .* pass$' "$TMP/tally.txt")
  failed_acc=$(grep '^acceptance ' "$TMP/tally.txt" | grep -v ' pass$' | awk '{print $2 " (" $3 ")"}' | paste -sd, - | sed 's/,/, /g')
  score=$(awk -v p="$passed" -v t="$total" 'BEGIN { printf "%.2f", (t > 0 ? p / t : 0) }')
  if [ -n "$failed_acc" ]; then
    echo "acceptance $passed/$total; failed: $failed_acc"
  else
    echo "acceptance $passed/$total"
  fi
  echo "time: $((SECONDS - T0)) s"
  if [ "$soft" -eq 0 ] && [ "$score" = "1.00" ]; then
    echo "PASS"
    echo "$score"
    return 0
  fi
  echo "FAIL: $soft gate(s) failed, acceptance $passed/$total"
  echo "$score"
  return 1
}

TMP=$(mktemp -d "${TMPDIR:-/tmp}/lazygit-check.XXXXXX") || { echo "FAIL: harness: mktemp"; echo "0.00"; exit 2; }
trap 'rm -rf "$TMP"' EXIT
if [ -n "${RESULT_JSON:-}" ]; then
  main "$@" 2>&1 | tee "${RESULT_JSON%.json}.check.log"
  exit "${PIPESTATUS[0]}"
fi
main "$@"
exit $?
