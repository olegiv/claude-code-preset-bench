#!/bin/bash
# Gates a linkbox-storage-adr run (bash 3.2 or later); the judge grades the ADR itself. The run passes, with a
# score of 1.00, when it added exactly one ADR, docs/adr/0002-<title>.md, with the sections of ADR 0001, changed
# nothing else and did not commit. Any failed gate scores 0.00.
set -u

USECASE_DIR=${USECASE_DIR:-$(cd "$(dirname "$0")" && pwd)}
WS=${WORKSPACE:-$PWD}

main() {
  command -v python3 >/dev/null 2>&1 || { echo "FAIL: harness: python3 not found"; echo "0.00"; return 2; }
  command -v git >/dev/null 2>&1 || { echo "FAIL: harness: git not found"; echo "0.00"; return 2; }
  python3 - "$USECASE_DIR" "$WS" <<'EOF'
import fnmatch, os, re, subprocess, sys

# Files a run may leave besides the ADR: the workspace's .gitignore patterns (a built binary, test output).
IGNORED = ["linkbox", "bin/*", "*.test", "*.out"]
ADR = re.compile(r"^docs/adr/0002-[a-z0-9]+(-[a-z0-9]+)*\.md$")
# ADR 0001's sections; the last may also be called "Alternatives" or "Options considered".
SECTIONS = [("Status", {"status"}), ("Context", {"context"}), ("Decision", {"decision"}),
            ("Consequences", {"consequences"}),
            ("Alternatives considered", {"alternatives considered", "considered alternatives", "alternatives",
                                         "options considered", "considered options"})]
MIN_WORDS = 200

usecase, ws = sys.argv[1:3]
template = os.path.join(usecase, "workspace")
out = []


def finish(code, score, lines):
    for line in lines:
        print(line)
    print("%.2f" % score)
    sys.exit(code)


def fail(reason):
    finish(1, 0.0, out + ["FAIL: " + reason])


def files(root):
    found = set()
    for d, dirs, names in os.walk(root):
        dirs[:] = [x for x in dirs if x != ".git"]
        for n in names:
            found.add(os.path.relpath(os.path.join(d, n), root))
    return found


# The workspace as bench.py copied it must be unchanged: code, docs and ADR 0001. This compares the files themselves,
# whatever git's index says.
want = files(template)
changed = sorted(p for p in want if os.path.islink(os.path.join(ws, p)) or not os.path.isfile(os.path.join(ws, p))
                 or open(os.path.join(ws, p), "rb").read() != open(os.path.join(template, p), "rb").read())
if changed:
    fail("files of the workspace changed or missing: " + ", ".join(changed[:10]))
out.append("ok: the workspace's files are unchanged")

# No commits: bench.py diffs the working tree against HEAD for the judge, so a commit would hide the ADR from it.
git_dir = subprocess.run(["git", "rev-parse", "--absolute-git-dir"], cwd=ws, capture_output=True, text=True).stdout.strip()
if not git_dir:
    fail("the workspace is no longer a git repository")
try:
    base = open(os.path.join(git_dir, "ccbench-base")).read().strip()
except OSError:
    finish(2, 0.0, out + ["FAIL: harness: no record of the starting commit (setup.sh writes .git/ccbench-base)"])
head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ws, capture_output=True, text=True).stdout.strip()
if head != base:
    fail("HEAD moved from the starting commit: the run committed, which the judge's diff would not show")

# Grade what the judge sees: stage the workspace as bench.py's collect_diff does before it runs this check (so this is
# then a no-op), and read the staged diff against HEAD. It shows a changed mode, and a symlink, to a file or a
# directory, as one entry with mode 120000.
subprocess.run(["git", "add", "-A"], cwd=ws, capture_output=True, check=False)
raw = subprocess.run(["git", "diff", "--cached", "HEAD", "--raw", "-z", "--no-renames"], cwd=ws, capture_output=True,
                     text=True).stdout.split("\0")
staged = [(meta.split()[1], meta.split()[4], path) for meta, path in zip(raw[0::2], raw[1::2]) if meta]
touched = sorted(path for _, status, path in staged if status != "A")
if touched:
    fail("files of the workspace changed or missing: " + ", ".join(touched[:10]))
added = sorted(path for _, status, path in staged if status == "A" and not any(fnmatch.fnmatch(path, g) for g in IGNORED))
# A symlink would let the gates read text from outside the workspace that the judge's diff does not show.
links = sorted(path for mode, status, path in staged if status == "A" and mode == "120000")
if links:
    fail("symbolic links added: " + ", ".join(links[:10]))
adrs = [p for p in added if ADR.match(p)]
extra = [p for p in added if p not in adrs]
if len(adrs) != 1:
    fail("%d files named docs/adr/0002-<lowercase-title>.md, want exactly 1 (added: %s)" % (len(adrs), ", ".join(added[:10]) or "nothing"))
if extra:
    fail("files added besides the ADR: " + ", ".join(extra[:10]))
adr = adrs[0]
out.append("ok: one ADR added, " + adr)

# Sections: the ## headings of ADR 0001, each with some text under it.
# Text the judge's diff can show: UTF-8 without NUL bytes, which make git call a file binary.
try:
    text = open(os.path.join(ws, adr), "rb").read().decode("utf-8")
except UnicodeDecodeError:
    text = "\0"
if "\0" in text:
    fail("the ADR is not UTF-8 text, so the judge's diff would not show it")
body, heading = {}, None
for line in text.splitlines():
    m = re.match(r"^##\s+(.+?)\s*:?\s*$", line)
    if m:
        heading = m.group(1).lower()
        body.setdefault(heading, "")
    elif heading is not None:
        body[heading] += line.strip()
missing = [name for name, names in SECTIONS if not any(body.get(n) for n in names)]
if missing:
    fail("sections missing or empty: " + ", ".join(missing))
out.append("ok: sections " + ", ".join(name for name, _ in SECTIONS))
words = len(text.split())
if words < MIN_WORDS:
    fail("%d words; an ADR needs at least %d" % (words, MIN_WORDS))
finish(0, 1.0, out + ["ok: %d words" % words, "PASS"])
EOF
}

if [ -n "${RESULT_JSON:-}" ]; then
  main "$@" 2>&1 | tee "${RESULT_JSON%.json}.check.log"
  exit "${PIPESTATUS[0]}"
fi
main "$@"
exit $?
