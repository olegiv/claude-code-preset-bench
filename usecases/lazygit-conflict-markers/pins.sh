# Sourced by setup.sh, check.sh and selftest.sh: the upstream commits, the local cache and the grader's image.
REPO_URL=https://github.com/jesseduffield/lazygit.git
# The parent of PR #5902 ("Honor the conflict-marker-size gitattribute"): the agent's starting point.
BASE=5dec89abfe8f3a79d7b4f3ff04e9df616e1dbe15
# The PR's merge commit: the reference solution, used only by selftest.sh.
REFERENCE=f162dc5aecfcbcc47ecfd6cd892013106800f9b1
CACHE=${CCBENCH_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/ccbench}/lazygit.git
BASE_REF=refs/ccbench/base
REFERENCE_REF=refs/ccbench/reference
# golang:1.27.2, pinned to its multi-platform index (linux/amd64 and linux/arm64).
IMAGE=golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8
