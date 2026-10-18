#!/bin/bash
# selftest.sh [variant...]: grades known variants of the workspace and compares check.sh's exit code, score and
# output with what the grader should say. Each workspace is built as bench.py builds one (an empty repository,
# then setup.sh). Needs Docker, and network the first time (setup.sh fills its cache). CHECK_BASH=/bin/bash runs
# check.sh under macOS's bash 3.2.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
. "$HERE/pins.sh"
CHECK_BASH=${CHECK_BASH:-bash}

# name|expected exit|expected score|text the output must contain (or, after "!", must not contain)
VARIANTS='untouched|1|0.00|acceptance 0/5
reference|0|1.00|ok: no regression
reference-committed|0|1.00|acceptance 5/5
reference-agent-tests-broken|0|1.00|acceptance 5/5
reference-no-test-updates|1|1.00|FAIL: go test -short
reference-gofmt-miss|1|1.00|FAIL: gofmt
hardcoded-32|1|0.80|conflict_marker_size_from_pattern
auto-stage-default|1|<1|conflict_marker_size_not_auto_staged
unset-means-32|1|1.00|conflict_marker_size_other_files_unaffected
spins-on-pattern|1|0.80|conflict_marker_size_from_pattern (fail)
first-run-times-out|0|1.00|running the suite again
vendor-changed|1|0.00|vendor/ changed
go-mod-changed|1|0.00|go.mod changed
build-broken|1|0.00|go build ./... failed
unit-test-hangs|1|1.00|FAIL: go test -short
result-symlink|1|1.00|!CANARY-7f3a91
pkg-integration-symlink|1|0.00|pkg/integration is a symbolic link
docker-down|2|0.00|harness: the Docker daemon is not running'

# mutate <file> <perl substitution>: applies it, and fails when it changes nothing.
mutate() {
  local before
  before=$(shasum -a 256 "$1")
  perl -0pi -e "$2" "$1"
  [ "$(shasum -a 256 "$1")" != "$before" ] || { echo "selftest: mutation did not apply to $1: $2" >&2; return 1; }
}

# The reference: every file the upstream PR changed, as it left them (its tests included).
reference() {
  git fetch -q --depth 1 "$CACHE" "$REFERENCE_REF"
  git checkout -q FETCH_HEAD -- $(git diff --name-only "$BASE" FETCH_HEAD)
}

apply() {
  case $1 in
    untouched|docker-down) ;;
    reference|first-run-times-out) reference ;;
    reference-committed)
      reference
      git add -A && git -c user.name=agent -c user.email=agent@local -c commit.gpgsign=false commit -q -m fix ;;
    reference-agent-tests-broken)
      reference
      printf 'package conflicts\n\nvar Broken = undefinedThing\n' > pkg/integration/tests/conflicts/agent_broken.go ;;
    reference-no-test-updates)
      reference
      git checkout -q "$BASE" -- pkg/commands/git_commands/file_loader_test.go pkg/gui/mergeconflicts/find_conflicts_test.go \
        pkg/gui/mergeconflicts/state_test.go ;;
    reference-gofmt-miss)
      reference
      mutate pkg/gui/mergeconflicts/find_conflicts.go 's/^const defaultConflictMarkerSize = 7$/const   defaultConflictMarkerSize = 7/m' ;;
    hardcoded-32)
      reference
      mutate pkg/commands/git_commands/file_loader.go 's/markerSizes\[fields\[i\]\] = markerSize/markerSizes[fields[i]] = 32/' ;;
    auto-stage-default)
      reference
      mutate pkg/gui/controllers/helpers/refresh_helper.go 's/(filepath\.Join\(env\.git\.RepoPaths\.WorktreePath\(\), file\.Path\),\s*)file\.ConflictMarkerSize\)/${1}0)/' ;;
    unset-means-32)
      reference
      mutate pkg/commands/git_commands/file_loader.go 's/(markerSizes\[fields\[i\]\] = markerSize\n\t\t\})/$1 else {\n\t\t\tmarkerSizes[fields[i]] = 32\n\t\t}/' ;;
    spins-on-pattern)
      # lazygit spins forever on a marker length of 12, which only conflict_marker_size_from_pattern uses; the test
      # driver gives up waiting, so the test fails rather than hangs.
      reference
      mutate pkg/gui/mergeconflicts/find_conflicts.go 's/(func effectiveMarkerSize\(markerSize int\) int \{\n)/${1}\tfor markerSize == 12 {\n\t}\n/' ;;
    vendor-changed)
      reference
      f=$(ls vendor/github.com/samber/lo/*.go | head -n 1)
      printf '\n// changed\n' >> "$f" ;;
    go-mod-changed)
      reference
      printf '\n// changed\n' >> go.mod ;;
    build-broken)
      reference
      printf 'package mergeconflicts\n\nfunc broken() {\n' > pkg/gui/mergeconflicts/broken.go ;;
    unit-test-hangs)
      # A unit test of the agent's that never returns: a failed soft gate, with the integration tests still run.
      reference
      printf 'package utils\n\nimport (\n\t"testing"\n\t"time"\n)\n\nfunc TestZzHangs(t *testing.T) {\n\ttime.Sleep(time.Hour)\n}\n' \
        > pkg/utils/zz_hangs_test.go ;;
    result-symlink)
      # A unit test, run inside the grading container, swaps the vet log for a link to a host file (a canary) and
      # marks vet as failed, so that check.sh would print the log.
      reference
      echo "CANARY-7f3a91" > "$dir/canary.txt"
      printf 'package utils\n\nimport (\n\t"os"\n\t"testing"\n)\n\nfunc TestZzResultSymlink(t *testing.T) {\n\t_ = os.Remove("/out/vet.log")\n\t_ = os.Symlink("%s", "/out/vet.log")\n\t_ = os.WriteFile("/out/vet.rc", []byte("1\\n"), 0o644)\n}\n' \
        "$dir/canary.txt" > pkg/utils/zz_result_symlink_test.go ;;
    pkg-integration-symlink)
      # pkg/integration points outside the workspace: check.sh must not delete or write through it.
      reference
      mv pkg/integration "$dir/outside-integration"
      ln -s "$dir/outside-integration" pkg/integration ;;
    *) echo "selftest: unknown variant $1" >&2; return 1 ;;
  esac
}

run_variant() {
  local name=$1 want_exit=$2 want_score=$3 want_text=$4 dir ws out rc score ok=1 path=$PATH timeout
  dir=$(mktemp -d "${TMPDIR:-/tmp}/lazygit-selftest.XXXXXX")
  ws=$dir/ws
  mkdir "$ws"
  (cd "$ws" && git init -q && git -c user.name=ccbench -c user.email=ccbench@local -c commit.gpgsign=false \
    commit -q --allow-empty --no-verify -m base) || return 1
  (cd "$ws" && WORKSPACE=$ws USECASE_DIR=$HERE bash "$HERE/setup.sh") || { echo "selftest: setup.sh failed" >&2; return 1; }
  (cd "$ws" && apply "$name") || return 1
  # docker-down: a docker that fails, wherever the real one is installed.
  if [ "$name" = docker-down ]; then
    mkdir "$dir/bin" && printf '#!/bin/sh\nexit 1\n' > "$dir/bin/docker" && chmod 755 "$dir/bin/docker"
    path=$dir/bin:$PATH
  fi
  # first-run-times-out: the first integration run stops after 2 s, before any test ends, so check.sh must rerun it.
  timeout=4m
  [ "$name" = first-run-times-out ] && timeout=2s
  out=$(cd "$ws" && PATH=$path WORKSPACE=$ws USECASE_DIR=$HERE LAZYGIT_CHECK_INTEGRATION_TIMEOUT=$timeout \
    "$CHECK_BASH" "$HERE/check.sh" 2>&1)
  rc=$?
  score=$(printf '%s\n' "$out" | tail -n 1)
  [ "$rc" = "$want_exit" ] || ok=0
  case $want_score in
    "<1") awk -v s="$score" 'BEGIN { exit !(s + 0 < 1) }' || ok=0 ;;
    *) [ "$score" = "$want_score" ] || ok=0 ;;
  esac
  case $want_text in
    !*) ! printf '%s\n' "$out" | grep -qF -- "${want_text#!}" || ok=0 ;;
    *) printf '%s\n' "$out" | grep -qF -- "$want_text" || ok=0 ;;
  esac
  if [ "$name" = pkg-integration-symlink ] && [ ! -d "$dir/outside-integration/tests" ]; then
    echo "selftest: check.sh deleted $dir/outside-integration/tests"
    ok=0
  fi
  if [ "$ok" = 1 ]; then
    printf 'ok    %-30s exit %s, score %s\n' "$name" "$rc" "$score"
  else
    printf 'FAIL  %-30s exit %s (want %s), score %s (want %s), wanted text: %s\n' "$name" "$rc" "$want_exit" "$score" \
      "$want_score" "$want_text"
    printf '%s\n' "$out" | sed 's/^/      /'
  fi
  rm -rf "$dir"
  [ "$ok" = 1 ]
}

failed=0
count=0
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
  echo "SELFTEST FAILED: $failed of $count variant(s)"
  exit 1
fi
