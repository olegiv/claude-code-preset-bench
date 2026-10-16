#!/bin/bash
# Deterministic grader for meowdoku. Contract (bench.py): runs with cwd = WORKSPACE; exit 0 = pass;
# the last stdout line is the score: the fraction of hidden acceptance tests passed, 0.00 when a
# hard gate fails. Env: WORKSPACE, USECASE_DIR, RUN_ID, CONFIG, RESULT_JSON (optional), RESULT_TEXT.
# Runs under macOS /bin/bash 3.2 and bash 5: no mapfile, associative arrays or ${x,,}.
# Needs go (1.27+), perl, and curl for the -addr check.
set -u

# GOENV=off ignores a go env file the agent may have written with `go env -w`.
export GOTOOLCHAIN=local GOPROXY=off GOWORK=off GOFLAGS= GOENV=off CGO_ENABLED=1

# The whole check has BUDGET seconds and every step gets at most what is left. The worst case
# ends a few seconds after the budget (the stopped step's grace period, then the summary), well
# under 780 s and bench.py's 900 s.
BUDGET=750
T0=$SECONDS

# bounded CAP command...: run command in its own process group for at most min(CAP, time left)
# seconds; on timeout, or when this script is terminated, kill the whole group. Exit 124 on
# timeout, otherwise the command's status.
RUNNER='
  my $limit = shift;
  my $pid = fork; die "fork: $!" unless defined $pid;
  if ($pid == 0) { setpgrp(0, 0); exec @ARGV or die "exec $ARGV[0]: $!\n" }
  my $stop = sub { kill "TERM", -$pid; sleep 2; kill "KILL", -$pid; waitpid($pid, 0); exit $_[0] };
  $SIG{ALRM} = sub { $stop->(124) };
  $SIG{TERM} = sub { $stop->(143) };
  alarm $limit; waitpid($pid, 0); alarm 0;
  exit($? & 127 ? 128 + ($? & 127) : $? >> 8);
'
bounded() {
  local cap=$1 left
  shift
  left=$((BUDGET - (SECONDS - T0)))
  [ "$left" -lt "$cap" ] && cap=$left
  [ "$cap" -le 0 ] && return 124
  perl -e "$RUNNER" "$cap" "$@"
}

# Indented tail of a log, so failures are readable in the full log.
show() { tail -n "${2:-40}" "$1" | sed 's/^/    /'; }

# timed_out STATUS: says why a bounded step stopped.
timed_out() { [ "$1" -eq 124 ] && echo " (stopped: time limit)"; }

# The tally tool reads `go test -json` output, copies the test output to stdout and writes one
# line per top-level test, "<package> <test> <pass|fail|skip>", to the file given by -o. A test
# that reports fail at any point stays failed, so PASS lines printed by the code under test
# cannot turn a failure into a pass.
TALLY_GO='package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type event struct{ Action, Package, Test, Output string }

func main() {
	out := flag.String("o", "", "result file")
	flag.Parse()
	status := map[string]string{}
	for _, name := range flag.Args() {
		f, err := os.Open(name)
		if err != nil {
			fmt.Println("tally:", err)
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			var e event
			if json.Unmarshal(sc.Bytes(), &e) != nil || e.Action == "" {
				fmt.Println(sc.Text())
				continue
			}
			switch e.Action {
			case "output", "build-output":
				fmt.Print(e.Output)
			case "pass", "fail", "skip":
				if e.Test != "" && !strings.Contains(e.Test, "/") && status[e.Package+" "+e.Test] != "fail" {
					status[e.Package+" "+e.Test] = e.Action
				}
			}
		}
		f.Close()
	}
	var lines []string
	for k, v := range status {
		lines = append(lines, k+" "+v+"\n")
	}
	sort.Strings(lines)
	if err := os.WriteFile(*out, []byte(strings.Join(lines, "")), 0o644); err != nil {
		fmt.Println("tally:", err)
		os.Exit(1)
	}
}
'

TMP=""
cleanup() {
  if [ -n "$TMP" ] && [ -d "$TMP" ]; then
    chmod -R u+w "$TMP" 2>/dev/null
    rm -rf "$TMP"
  fi
}

main() {
  local orig="$USECASE_DIR/workspace" hidden="$USECASE_DIR/hidden"
  local soft_failed=0 t0 rc

  cd "$WORKSPACE" || { echo "FAIL: harness: cannot cd to WORKSPACE=$WORKSPACE"; echo "0.00"; return 2; }

  # Harness problems: not the agent's fault, exit 2.
  if ! command -v go >/dev/null 2>&1; then
    echo "FAIL: harness: go is not on PATH"
    echo "0.00"
    return 2
  fi
  if ! command -v perl >/dev/null 2>&1; then
    echo "FAIL: harness: perl is not on PATH"
    echo "0.00"
    return 2
  fi
  local gover minor
  gover=$(go env GOVERSION 2>/dev/null)
  minor=$(printf '%s\n' "$gover" | sed -n 's/.*go1\.\([0-9][0-9]*\).*/\1/p' | head -n 1)
  if [ -z "$minor" ] || [ "$minor" -lt 27 ]; then
    echo "FAIL: harness: need go1.27 or later, have '${gover:-unknown}'"
    echo "0.00"
    return 2
  fi
  echo "ok: harness: $gover"

  # Hard gates: any failure scores 0.00. With go.mod unchanged and GOPROXY=off, the build below
  # also refuses third-party modules.
  local f changed=""
  for f in go.mod web/static/htmx.min.js web/static/alpine.min.js; do
    cmp -s "$f" "$orig/$f" || changed="$changed $f"
  done
  if [ -n "$changed" ]; then
    echo "FAIL: protected files changed or missing:$changed"
    echo "FAIL: hard gate"
    echo "0.00"
    return 1
  fi
  echo "ok: go.mod and the vendored scripts unchanged"

  t0=$SECONDS
  bounded 300 go build ./... >"$TMP/build.log" 2>&1
  rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: go build ./... (offline, standard library only)$(timed_out $rc)"
    show "$TMP/build.log"
    echo "FAIL: hard gate"
    echo "0.00"
    return 1
  fi
  echo "ok: go build ($((SECONDS - t0))s)"

  # Soft gates: a failure fails the check but leaves the score alone.
  bounded 120 gofmt -l . >"$TMP/gofmt.log" 2>&1
  rc=$?
  if [ "$rc" -ne 0 ] || [ -s "$TMP/gofmt.log" ]; then
    echo "FAIL: gofmt -l . reports:$(timed_out $rc)" $(cat "$TMP/gofmt.log")
    soft_failed=$((soft_failed + 1))
  else
    echo "ok: gofmt"
  fi

  t0=$SECONDS
  bounded 300 go vet ./... >"$TMP/vet.log" 2>&1
  rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "ok: go vet ($((SECONDS - t0))s)"
  else
    echo "FAIL: go vet ./...$(timed_out $rc)"
    show "$TMP/vet.log"
    soft_failed=$((soft_failed + 1))
  fi

  local d missing=""
  for d in engine web; do
    grep -rqs --include='*_test.go' '^func Test' "$d" || missing="$missing $d/"
  done
  if [ -n "$missing" ]; then
    echo "FAIL: no func Test in:$missing"
    soft_failed=$((soft_failed + 1))
  else
    echo "ok: own tests in engine/ and web/"
  fi

  t0=$SECONDS
  bounded 300 go test -count=1 -timeout 240s ./... >"$TMP/test.log" 2>&1
  rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "ok: go test ./... ($((SECONDS - t0))s)"
  else
    echo "FAIL: go test ./... ($((SECONDS - t0))s)$(timed_out $rc)"
    show "$TMP/test.log"
    soft_failed=$((soft_failed + 1))
  fi

  # The command: it must build, and serve the app on the address given by -addr. The port is one
  # the OS reports free; the check waits up to about 15 s for GET / to answer while the server is
  # still running (so no other process on the port can pass for it), then kills the server's whole
  # process group. The default address (127.0.0.1:8080) is not tried: it may be in use on the
  # grading machine.
  bounded 180 go build -o "$TMP/meowdoku" ./cmd/meowdoku >"$TMP/cmd.log" 2>&1
  rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "FAIL: cmd/meowdoku does not build$(timed_out $rc)"
    show "$TMP/cmd.log" 10
    soft_failed=$((soft_failed + 1))
  elif ! command -v curl >/dev/null 2>&1; then
    echo "WARN: no curl; cmd/meowdoku builds, -addr not tried"
  else
    local served="" port spid i
    port=$(perl -MIO::Socket::INET -e 'my $s = IO::Socket::INET->new(Listen => 1, LocalAddr => "127.0.0.1", LocalPort => 0) or exit 1; print $s->sockport')
    if [ -n "$port" ] && [ $((BUDGET - (SECONDS - T0))) -gt 30 ]; then
      perl -e "$RUNNER" 30 "$TMP/meowdoku" -addr "127.0.0.1:$port" >"$TMP/serve.log" 2>&1 &
      spid=$!
      for i in $(seq 60); do
        kill -0 "$spid" 2>/dev/null || break
        if curl -fsS -o /dev/null --max-time 2 "http://127.0.0.1:$port/" 2>/dev/null && kill -0 "$spid" 2>/dev/null; then
          served=1
          break
        fi
        sleep 0.25
      done
      kill -TERM "$spid" 2>/dev/null
      wait "$spid" 2>/dev/null
    fi
    if [ -n "$served" ]; then
      echo "ok: cmd/meowdoku builds and serves GET / on -addr"
    else
      echo "FAIL: cmd/meowdoku did not answer GET / on the address given by -addr"
      show "$TMP/serve.log" 10
      soft_failed=$((soft_failed + 1))
    fi
  fi

  # Structure the prompt requires. Engine purity: a best-effort denylist of I/O packages, checked
  # against engine's direct imports only (fmt, for one, reaches os indirectly and is fine).
  local imports io_imports
  if imports=$(bounded 120 go list -f '{{join .Imports " "}}' ./engine 2>&1); then
    io_imports=$(printf '%s\n' $imports | grep -xE 'os|os/exec|os/signal|net|net/http|io/ioutil|log|log/slog|syscall|crypto/rand' | paste -sd' ' -)
    if [ -n "$io_imports" ]; then
      echo "FAIL: engine imports I/O packages: $io_imports"
      soft_failed=$((soft_failed + 1))
    else
      echo "ok: engine imports no I/O packages"
    fi
  else
    echo "FAIL: engine imports: go list ./engine failed"
    soft_failed=$((soft_failed + 1))
  fi
  if bounded 120 go list -deps ./web 2>/dev/null | grep -qx 'html/template'; then
    echo "ok: web uses html/template"
  else
    echo "FAIL: web does not use html/template"
    soft_failed=$((soft_failed + 1))
  fi
  local embedded
  embedded=$(bounded 120 go list -deps -f '{{if .Module}}{{if eq .Module.Path "meowdoku"}}{{join .EmbedFiles " "}}{{end}}{{end}}' ./web 2>/dev/null | tr ' ' '\n')
  if printf '%s\n' "$embedded" | grep -q 'htmx\.min\.js$' && printf '%s\n' "$embedded" | grep -q 'alpine\.min\.js$'; then
    echo "ok: web embeds the vendored scripts"
  else
    echo "FAIL: web does not embed htmx.min.js and alpine.min.js with embed"
    soft_failed=$((soft_failed + 1))
  fi

  # Hidden acceptance tests, in a copy with the agent's own tests removed.
  local ws="$TMP/ws"
  mkdir -p "$ws"
  cp -R "$WORKSPACE/." "$ws/" 2>"$TMP/copy.log" || { echo "FAIL: harness: copying the workspace"; show "$TMP/copy.log"; echo "0.00"; return 2; }
  rm -rf "$ws/.git"
  mkdir -p "$ws/engine" "$ws/web"
  find "$ws/engine" "$ws/web" -type f -name '*_test.go' -exec rm -f {} +
  cp "$hidden"/engine/* "$ws/engine/" && cp "$hidden"/web/* "$ws/web/" || { echo "FAIL: harness: copying the hidden tests"; echo "0.00"; return 2; }

  mkdir -p "$TMP/tally"
  printf 'module tally\n' >"$TMP/tally/go.mod"
  printf '%s' "$TALLY_GO" >"$TMP/tally/main.go"
  if ! (cd "$TMP/tally" && bounded 120 go build -o tally .) >"$TMP/tally.log" 2>&1; then
    echo "FAIL: harness: building the result tally"
    show "$TMP/tally.log"
    echo "0.00"
    return 2
  fi

  # The race detector needs cgo and a C toolchain; probe it once, independently of the agent's code.
  local race="-race"
  mkdir -p "$TMP/raceprobe"
  printf 'module raceprobe\n' >"$TMP/raceprobe/go.mod"
  printf 'package main\n\nfunc main() {}\n' >"$TMP/raceprobe/main.go"
  if ! (cd "$TMP/raceprobe" && bounded 120 go build -race -o "$TMP/raceprobe/probe" .) >"$TMP/raceprobe.log" 2>&1; then
    race=""
    echo "WARN: no -race"
  fi

  # -vet=off: vet is a soft gate above, so a vet finding must not also fail every hidden test.
  local rc_engine rc_web
  t0=$SECONDS
  (cd "$ws" && bounded 240 go test -json -vet=off -count=1 -timeout 180s -run '^TestHidden' ./engine) >"$TMP/hidden-engine.json" 2>"$TMP/hidden-engine.err"
  rc_engine=$?
  echo "hidden engine tests ran ($((SECONDS - t0))s, exit $rc_engine)$(timed_out $rc_engine)"
  t0=$SECONDS
  (cd "$ws" && bounded 480 go test -json -vet=off -count=1 -timeout 450s $race -run '^TestHidden' ./web) >"$TMP/hidden-web.json" 2>"$TMP/hidden-web.err"
  rc_web=$?
  echo "hidden web tests ran ($((SECONDS - t0))s, ${race:-no -race}, exit $rc_web)$(timed_out $rc_web)"
  "$TMP/tally/tally" -o "$TMP/results.txt" "$TMP/hidden-engine.json" "$TMP/hidden-web.json" >"$TMP/hidden.log" 2>&1
  cat "$TMP/hidden-engine.err" "$TMP/hidden-web.err" >>"$TMP/hidden.log"
  show "$TMP/hidden.log" 600
  if grep -q '^panic: test timed out' "$TMP/hidden.log"; then
    echo "WARN: a hidden test package hit its go test -timeout (see the panic above the goroutine dump)"
  fi

  { grep -h '^func TestHidden' "$hidden"/engine/*_test.go | sed 's/^func \(TestHidden[A-Za-z0-9_]*\).*/meowdoku\/engine \1/'
    grep -h '^func TestHidden' "$hidden"/web/*_test.go | sed 's/^func \(TestHidden[A-Za-z0-9_]*\).*/meowdoku\/web \1/'
  } | sort -u >"$TMP/all.txt"
  awk '$3 == "pass" { print $1 " " $2 }' "$TMP/results.txt" 2>/dev/null | sort -u >"$TMP/passed-raw.txt"
  comm -12 "$TMP/all.txt" "$TMP/passed-raw.txt" >"$TMP/passed.txt"
  comm -23 "$TMP/all.txt" "$TMP/passed.txt" >"$TMP/failed.txt"
  local total passed score failed
  total=$(wc -l <"$TMP/all.txt" | tr -d ' ')
  passed=$(wc -l <"$TMP/passed.txt" | tr -d ' ')
  score=$(awk -v p="$passed" -v t="$total" 'BEGIN { if (t > 0) printf "%.2f", p / t; else printf "0.00" }')
  failed=$(awk '{ print $2 }' "$TMP/failed.txt" | paste -sd, - | sed 's/,/, /g')
  if [ -n "$failed" ]; then
    echo "hidden $passed/$total; failed: $failed"
  else
    echo "hidden $passed/$total"
  fi
  # A hidden test binary that exits non-zero (a crash, a race report after the last test, a
  # timeout) fails the check even when every test reported a pass.
  if [ "$score" = "1.00" ] && { [ "$rc_engine" -ne 0 ] || [ "$rc_web" -ne 0 ]; }; then
    echo "FAIL: hidden test binaries exited with $rc_engine (engine) and $rc_web (web)"
    soft_failed=$((soft_failed + 1))
  fi

  echo "time: $((SECONDS - T0)) s (budget $BUDGET s)"
  if [ "$soft_failed" -eq 0 ] && [ "$score" = "1.00" ]; then
    echo "PASS"
    echo "$score"
    return 0
  fi
  echo "FAIL: $soft_failed gate(s) failed, hidden $passed/$total"
  echo "$score"
  return 1
}

# The temporary directory belongs to this top-level shell, so its EXIT trap removes it even when
# main runs in the subshell of the logging pipeline (bash 3.2 runs no EXIT trap there).
TMP=$(mktemp -d "${TMPDIR:-/tmp}/meowdoku-check.XXXXXX") || { echo "FAIL: harness: mktemp"; echo "0.00"; exit 2; }
trap cleanup EXIT
if [ -n "${RESULT_JSON:-}" ]; then
  main "$@" 2>&1 | tee "${RESULT_JSON%.json}.check.log"
  exit "${PIPESTATUS[0]}"
fi
main "$@"
exit $?
