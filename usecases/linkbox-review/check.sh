#!/bin/bash
# Grades a linkbox-review run (bash 3.2 or later): matches the findings in REVIEW.md against answer-key.json.
# Score = (planted issues found - decoys flagged) / issues, at least 0. Hard gates (0.00): REVIEW.md missing, a
# file of the pull request changed, or a commit (HEAD moved from setup.sh's record). Soft gates fail the check
# without lowering the score. PASS needs a score of at least PASS_SCORE and no failed gate.
set -u

USECASE_DIR=${USECASE_DIR:-$(cd "$(dirname "$0")" && pwd)}
WS=${WORKSPACE:-$PWD}

main() {
  command -v python3 >/dev/null 2>&1 || { echo "FAIL: harness: python3 not found"; echo "0.00"; return 2; }
  command -v git >/dev/null 2>&1 || { echo "FAIL: harness: git not found"; echo "0.00"; return 2; }
  python3 - "$USECASE_DIR" "$WS" "${RESULT_JSON:-}" <<'EOF'
import fnmatch, json, os, re, shutil, subprocess, sys, tempfile

PASS_SCORE = 0.60
MAX_FINDINGS = 25
# Files a run may leave besides REVIEW.md: the workspace's .gitignore patterns (a built binary, test output).
IGNORED = ["linkbox", "bin/*", "*.test", "*.out"]
FINDING = re.compile(r"^\s*[-*]\s+\[(critical|high|medium|low)\]\s+`?([\w./-]+\.\w+):(\d+)(?:-\d+)?`?\s+(?:—|–|-{1,2})\s+\S")

usecase, ws, result_json = sys.argv[1:4]


def finish(code, score, lines):
    for line in lines:
        print(line)
    print("%.2f" % score)
    sys.exit(code)


def hard(reason):
    finish(1, 0.0, ["FAIL: " + reason, "FAIL: hard gate"])


# The pull request as setup.sh laid it out: workspace/ with feature.patch applied.
expected = tempfile.mkdtemp(prefix="linkbox-check-")
try:
    tree = os.path.join(expected, "tree")
    shutil.copytree(os.path.join(usecase, "workspace"), tree)
    p = subprocess.run(["git", "apply", os.path.join(usecase, "feature.patch")], cwd=tree, capture_output=True, text=True)
    if p.returncode != 0:
        finish(2, 0.0, ["FAIL: harness: feature.patch does not apply: " + p.stderr.strip()[:300]])
    want = {}
    for d, _, files in os.walk(tree):
        for f in files:
            path = os.path.join(d, f)
            want[os.path.relpath(path, tree)] = open(path, "rb").read()
finally:
    shutil.rmtree(expected, ignore_errors=True)

out, soft = [], []

# Hard gate: the review must leave the pull request as it is.
changed = [p for p, data in sorted(want.items())
           if not os.path.isfile(os.path.join(ws, p)) or open(os.path.join(ws, p), "rb").read() != data]
if changed:
    hard("files of the pull request changed or missing: " + ", ".join(changed[:10]))
out.append("ok: the pull request's files are unchanged")

# Hard gate: no commits. bench.py diffs the working tree against HEAD for the judge, so a commit would hide the
# review and the pull request from it.
git_dir = subprocess.run(["git", "rev-parse", "--absolute-git-dir"], cwd=ws, capture_output=True, text=True).stdout.strip()
if not git_dir:
    hard("the workspace is no longer a git repository")
try:
    base = open(os.path.join(git_dir, "ccbench-base")).read().strip()
except OSError:
    finish(2, 0.0, out + ["FAIL: harness: no record of the starting commit (setup.sh writes .git/ccbench-base)"])
head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ws, capture_output=True, text=True).stdout.strip()
if head != base:
    hard("HEAD moved from the starting commit: the run committed, which the judge's diff would not show")

extra = []
for d, dirs, files in os.walk(ws):
    dirs[:] = [x for x in dirs if x != ".git"]
    for f in files:
        rel = os.path.relpath(os.path.join(d, f), ws)
        if rel == "REVIEW.md" or rel in want or any(fnmatch.fnmatch(rel, g) for g in IGNORED):
            continue
        extra.append(rel)
if extra:
    out.append("FAIL: files added besides REVIEW.md: " + ", ".join(sorted(extra)[:10]))
    soft.append("extra files")

review = os.path.join(ws, "REVIEW.md")
if not os.path.isfile(review):
    hard("no REVIEW.md")

findings = []
for line in open(review, encoding="utf-8", errors="replace"):
    m = FINDING.match(line)
    if m:
        path = m.group(2)
        path = path[2:] if path.startswith("./") else path
        findings.append((m.group(1), path, int(m.group(3))))
if not findings:
    out.append("FAIL: no finding in the required format (- [severity] path:line — title: explanation)")
    soft.append("format")
if len(findings) > MAX_FINDINGS:
    out.append("FAIL: %d findings; only the first %d are read" % (len(findings), MAX_FINDINGS))
    soft.append("too many findings")
findings = findings[:MAX_FINDINGS]

key = json.load(open(os.path.join(usecase, "answer-key.json")))


def inside(item, path, line):
    return any(a <= line <= b for a, b in item["windows"].get(path, []))


found, decoys, duplicates = {}, set(), 0
for severity, path, line in findings:
    issue = next((i for i in key["issues"] if inside(i, path, line)), None)
    if issue is not None:
        if issue["id"] in found:
            duplicates += 1
        else:
            found[issue["id"]] = severity
        continue
    decoy = next((d for d in key["decoys"] if inside(d, path, line)), None)
    if decoy is not None:
        decoys.add(decoy["id"])

total = len(key["issues"])
missed = [i["id"] for i in key["issues"] if i["id"] not in found]
score = max(0, len(found) - len(decoys)) / total
out.append("findings read: %d (%d matched a planted issue, %d repeated one, %d flagged a decoy)" % (
    len(findings), len(found), duplicates, len(decoys)))
out.append("issues found %d/%d: %s" % (len(found), total, ", ".join(found) or "none"))
if missed:
    out.append("issues missed: " + ", ".join(missed))
if decoys:
    out.append("decoys flagged (each costs one issue): " + ", ".join(sorted(decoys)))

# Soft gate: the review must have used pr-review-toolkit. bench.py passes the run's result JSON; an agent counts
# only if it completed (one killed when the session ended did not review anything).
if result_json:
    try:
        stats = json.load(open(result_json)).get("subagent_stats") or {}
    except (OSError, ValueError):
        stats = {}
    by_type = stats.get("by_type") or {}
    toolkit = sorted(t for t in by_type if t.startswith("pr-review-toolkit:"))
    others = sum(n for t, n in by_type.items() if not t.startswith("pr-review-toolkit:"))
    # Completions are counted over all agents: a toolkit agent completed only if more agents completed than
    # agents of other types were started.
    if toolkit and (stats.get("completed") or 0) > others:
        out.append("ok: pr-review-toolkit used (%s)" % ", ".join(toolkit))
    else:
        out.append("FAIL: no completed pr-review-toolkit agent in the run")
        soft.append("toolkit")
else:
    out.append("toolkit use: not checked (no RESULT_JSON)")

if score >= PASS_SCORE and not soft:
    finish(0, score, out + ["PASS"])
finish(1, score, out + ["FAIL: %d gate(s) failed, score %.2f (PASS needs %.2f)" % (len(soft), score, PASS_SCORE)])
EOF
}

if [ -n "${RESULT_JSON:-}" ]; then
  main "$@" 2>&1 | tee "${RESULT_JSON%.json}.check.log"
  exit "${PIPESTATUS[0]}"
fi
main "$@"
exit $?
