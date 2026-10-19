#!/bin/bash
# selftest.sh [variant...]: checks the use case itself, then runs check.sh on known workspace states and compares
# its exit code, score and output with what it should say. Each workspace is built as bench.py builds one
# (workspace/ committed, then setup.sh). Needs Go, git and python3. CHECK_BASH=/bin/bash runs check.sh under
# macOS's bash 3.2.
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
CHECK_BASH=${CHECK_BASH:-bash}
REF=$HERE/reference-adr.md
ADR=docs/adr/0002-store-links-in-postgresql.md

# name|expected exit|expected score|text the output must contain
VARIANTS='perfect|0|1.00|PASS
options-heading|0|1.00|PASS
binary-ignored|0|1.00|PASS
missing-section|1|0.00|sections missing or empty: Consequences
empty-section|1|0.00|sections missing or empty: Context
too-short|1|0.00|an ADR needs at least
no-adr|1|0.00|0 files named docs/adr/0002
wrong-number|1|0.00|0 files named docs/adr/0002
uppercase-title|1|0.00|0 files named docs/adr/0002
wrong-dir|1|0.00|0 files named docs/adr/0002
two-adrs|1|0.00|2 files named docs/adr/0002
extra-file|1|0.00|files added besides the ADR
code-edited|1|0.00|files of the workspace changed
adr0001-edited|1|0.00|files of the workspace changed
symlinked-doc|1|0.00|files of the workspace changed
mode-changed|1|0.00|files of the workspace changed
symlinked-adr|1|0.00|symbolic links added
committed|1|0.00|HEAD moved'

# Facts the rubric gives the judge, with the file that must state them: file|text.
FACTS='docs/brief.md|My gut says SQLite
docs/brief.md|mark it Proposed
docs/adr/0001-keep-links-in-memory.md|single instance
docs/adr/0001-keep-links-in-memory.md|more than one instance
docs/ops/deployment.md|at least two replicas
docs/ops/deployment.md|cannot opt out
docs/ops/deployment.md|no session affinity
docs/ops/deployment.md|up to four instances
docs/ops/deployment.md|no persistent volumes
docs/ops/deployment.md|three failed checks
docs/ops/deployment.md|CGO_ENABLED=0
docs/ops/platform-services.md|Teams do not run
docs/ops/platform-services.md|about 30 seconds
docs/ops/platform-services.md|at most 20 client connections
docs/ops/platform-services.md|from 7 to 35 days
docs/ops/platform-services.md|35 days unless the team changes it
docs/ops/platform-services.md|three working days
docs/ops/platform-services.md|no persistence
docs/ops/platform-services.md|No conditional writes
docs/ops/platform-services.md|20 to 80 ms
docs/metrics.md|4 to 6 per second
docs/metrics.md|3,500 a day
docs/metrics.md|under 100 ms
docs/metrics.md|triple
docs/compliance.md|within 30 days, backups included
docs/compliance.md|`eu-west` or `eu-central`
docs/compliance.md|restore it after an operator
docs/support-tickets.md|#4127
docs/support-tickets.md|#4188
docs/support-tickets.md|GET /links/l7
docs/support-tickets.md|#4260
docs/support-tickets.md|Links I deleted came back
docs/team.md|about a week
docs/team.md|service containers
internal/store/store.go|fmt.Sprintf("l%d", s.seq)
internal/store/store.go|func compareIDs
internal/store/store.go|func (s *Store) List(owner, tag string) []Link
internal/api/api.go|store *store.Store'

# make_workspace <dir>: workspace/ committed as bench.py does, then setup.sh.
make_workspace() {
  cp -R "$HERE/workspace" "$1" &&
    (cd "$1" && git init -q && git add -A &&
      git -c user.name=ccbench -c user.email=ccbench@local -c commit.gpgsign=false commit -q --no-verify -m base &&
      WORKSPACE=$1 USECASE_DIR=$HERE bash "$HERE/setup.sh")
}

# edit_adr <file> <variant>: the reference ADR changed the way the variant needs.
edit_adr() {
  python3 - "$1" "$2" <<'EOF'
import re, sys
path, variant = sys.argv[1:3]
text = open(path).read()
if variant == "options-heading":
    text = text.replace("## Alternatives considered", "## Options considered")
elif variant == "missing-section":
    text = re.sub(r"## Consequences\n.*?(?=\n## )", "", text, flags=re.S)
elif variant == "empty-section":
    text = re.sub(r"(## Context\n).*?(?=\n## )", r"\1", text, flags=re.S)
elif variant == "too-short":
    text = "# 2. Use PostgreSQL\n\n## Status\n\nProposed.\n\n## Context\n\nLinks vanish.\n\n## Decision\n\nPostgreSQL.\n\n" \
           "## Consequences\n\nA database.\n\n## Alternatives considered\n\nSQLite.\n"
open(path, "w").write(text)
EOF
}

# The use case's own invariants: the facts the rubric cites are in the materials, and the workspace builds, vets
# and tests cleanly (the agents may run it).
precheck() {
  local dir ok=0
  # Whitespace is collapsed first: the docs wrap lines in the middle of a fact.
  printf '%s\n' "$FACTS" | python3 -c '
import sys
bad = 0
for line in sys.stdin:
    path, text = line.rstrip("\n").split("|", 1)
    if " ".join(text.split()) not in " ".join(open(sys.argv[1] + "/" + path).read().split()):
        print("FAIL  precheck: %s does not say: %s" % (path, text)); bad = 1
sys.exit(bad)' "$HERE/workspace" || ok=1
  dir=$(mktemp -d "${TMPDIR:-/tmp}/adr-selftest.XXXXXX")
  cp -R "$HERE/workspace" "$dir/ws"
  (cd "$dir/ws" && GOTOOLCHAIN=local go build ./... && GOTOOLCHAIN=local go vet ./... &&
    GOTOOLCHAIN=local go test -count=1 -race ./... >/dev/null) || { echo "FAIL  precheck: the workspace does not build, vet and test"; ok=1; }
  rm -rf "$dir"
  [ "$ok" = 0 ] && echo "ok    precheck: the rubric's facts are in the materials; the workspace builds, vets and tests"
  return "$ok"
}

run_variant() {
  local name=$1 want_exit=$2 want_score=$3 want_text=$4 dir ws out rc score ok=1
  dir=$(mktemp -d "${TMPDIR:-/tmp}/adr-selftest.XXXXXX")
  ws=$dir/ws
  make_workspace "$ws" >/dev/null || { echo "FAIL  $name: workspace or setup.sh"; return 1; }
  case $name in
    no-adr) ;;
    wrong-number) cp "$REF" "$ws/docs/adr/0003-store-links-in-postgresql.md" ;;
    uppercase-title) cp "$REF" "$ws/docs/adr/0002-Store-Links.md" ;;
    wrong-dir) cp "$REF" "$ws/docs/0002-store-links-in-postgresql.md" ;;
    symlinked-adr) cp "$REF" "$dir/adr.md" && ln -s "$dir/adr.md" "$ws/$ADR" ;;
    *) cp "$REF" "$ws/$ADR" ;;
  esac
  case $name in
    options-heading | missing-section | empty-section | too-short) edit_adr "$ws/$ADR" "$name" ;;
    binary-ignored) echo "binary" > "$ws/linkbox" && echo "test binary" > "$ws/api.test" ;;
    two-adrs) cp "$REF" "$ws/docs/adr/0002-use-postgresql.md" ;;
    extra-file) echo "scratch" > "$ws/notes.md" ;;
    code-edited) printf '\n// reviewed\n' >> "$ws/internal/store/store.go" ;;
    adr0001-edited) printf '\nSuperseded by ADR 0002.\n' >> "$ws/docs/adr/0001-keep-links-in-memory.md" ;;
    mode-changed) chmod +x "$ws/internal/store/store.go" ;;
    symlinked-doc) cp "$ws/docs/team.md" "$dir/team.md" && rm "$ws/docs/team.md" && ln -s "$dir/team.md" "$ws/docs/team.md" ;;
    committed) (cd "$ws" && git add -A && git -c user.name=agent -c user.email=agent@local -c commit.gpgsign=false commit -q -m adr) ;;
  esac
  out=$(cd "$ws" && WORKSPACE=$ws USECASE_DIR=$HERE "$CHECK_BASH" "$HERE/check.sh" 2>&1)
  rc=$?
  score=$(printf '%s\n' "$out" | tail -n 1)
  [ "$rc" = "$want_exit" ] && [ "$score" = "$want_score" ] || ok=0
  printf '%s\n' "$out" | grep -qF -- "$want_text" || ok=0
  if [ "$ok" = 1 ]; then
    printf 'ok    %-16s exit %s, score %s\n' "$name" "$rc" "$score"
  else
    printf 'FAIL  %-16s exit %s (want %s), score %s (want %s), wanted text: %s\n' "$name" "$rc" "$want_exit" "$score" \
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
