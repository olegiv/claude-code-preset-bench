#!/usr/bin/env python3
"""
bench.py - run the same prompt through several Claude Code presets ("configs")
and compare wall time, cost, turns and graded quality across repeated runs.

    python3 bench.py list                         # show configs and use cases
    python3 bench.py smoke                        # 1-turn sanity run per config (checks billing visibility, judge)
    python3 bench.py run --runs 3                 # every use case x every config, 3 repeats, interleaved
    python3 bench.py run -u my-case -c high light --runs 5
    python3 bench.py summarize                    # tables from results/runs.csv -> results/summary.md

Layout:
    configs/<name>.sh            exec's `claude` with the preset's flags/env; harness appends headless flags
    usecases/<name>/prompt.md    frontmatter (repo, max_turns, max_budget_usd, timeout_seconds, plugins) + prompt body
    usecases/<name>/workspace/   optional: files copied into a fresh workspace for each run
    usecases/<name>/setup.sh     optional: runs in the workspace before the agent (installs, scaffolding)
    usecases/<name>/check.sh     optional: deterministic grader, exit 0 = pass, last stdout line may be a 0..1 score
    usecases/<name>/rubric.md    optional: LLM-judge rubric (fixed judge model, scored 0..10)
    results/runs.csv             one row per run; results/raw/<run_id>.* holds the full JSON, result text, diff

Python 3.8+ standard library only. Designed for macOS/Linux.
"""

import argparse
import csv
import datetime as dt
import glob
import json
import os
import random
import re
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import time

ROOT = os.path.dirname(os.path.abspath(__file__))
CONFIGS_DIR = os.path.join(ROOT, "configs")
USECASES_DIR = os.path.join(ROOT, "usecases")
RESULTS_DIR = os.path.join(ROOT, "results")
RAW_DIR = os.path.join(RESULTS_DIR, "raw")
CSV_PATH = os.path.join(RESULTS_DIR, "runs.csv")
SUMMARY_PATH = os.path.join(RESULTS_DIR, "summary.md")

DEFAULT_MAX_TURNS = 40
DEFAULT_MAX_BUDGET_USD = 10.0
DEFAULT_TIMEOUT_S = 1800

# Runs and the judge see no MCP server the owner has installed or connected (plugins, claude.ai connectors) and no
# Claude in Chrome; runs get only the servers in bench-mcp.json: Chrome DevTools, headless, in a throwaway profile.
MCP_CONFIG = os.path.join(ROOT, "bench-mcp.json")
ISOLATION_FLAGS = ["--no-chrome", "--strict-mcp-config"]
# Claude Code exits at startup when given --strict-mcp-config while an administrator's managed-mcp.json is deployed
# (https://code.claude.com/docs/en/managed-mcp), so no run or judge call could start on such a machine.
MANAGED_MCP_PATHS = ["/Library/Application Support/ClaudeCode/managed-mcp.json", "/etc/claude-code/managed-mcp.json",
                     "C:\\Program Files\\ClaudeCode\\managed-mcp.json"]

# A run gets only the plugins its use case declares (prompt.md `plugins:`), the judge and smoke none: every other plugin
# the owner installed, enabled or synced from claude.ai is turned off with --settings, so that its hooks cannot reach a
# run (security-guidance's Stop hook, for one, reviews the git diff and hands its findings to the agent). Claude Code's
# built-in plugins stay. See plugin_settings().

# Flags the harness appends to every agent run (the config script holds the preset itself).
HEADLESS_FLAGS = ["--output-format", "json", "--no-session-persistence"] + ISOLATION_FLAGS + ["--mcp-config", MCP_CONFIG]

# Judge settings: a fixed model that is NOT one of the arms, no tools, structured output.
JUDGE_DEFAULT_MODEL = "opus"
JUDGE_DEFAULT_EFFORT = "high"
JUDGE_FLAGS = ["--output-format", "json", "--no-session-persistence", "--tools", ""] + ISOLATION_FLAGS + ["--max-turns", "3"]
JUDGE_SCHEMA = json.dumps({
    "type": "object",
    "properties": {
        "score": {"type": "number", "minimum": 0, "maximum": 10},
        "reasoning": {"type": "string"},
    },
    "required": ["score", "reasoning"],
})
DIFF_LIMIT = 120000     # chars of diff shown to the judge
RESULT_LIMIT = 12000    # chars of the agent's final message shown to the judge

# Environment variables that would silently override what the config scripts set.
STRIP_VARS = ["CLAUDE_CODE_EFFORT_LEVEL", "CLAUDE_CODE_SUBAGENT_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE"]
WARN_VARS = ["ANTHROPIC_MODEL", "CLAUDE_CODE_MAX_TURNS", "CLAUDE_CODE_DISABLE_ADVISOR_TOOL"]
# Set by a Claude Code session for the commands it runs. Inherited, they make every run a child of
# that session (child-session flags, its messaging socket and token), so they are dropped. Only the
# names are logged: CLAUDE_CODE_MESSAGING_TOKEN is a secret.
SESSION_VARS = ["CLAUDECODE", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_SESSION_ATTENDED",
                "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXECPATH", "CLAUDE_CODE_SESSION_ID",
                "CLAUDE_CODE_MESSAGING_SOCKET", "CLAUDE_CODE_MESSAGING_TOKEN", "CLAUDE_EFFORT",
                "CLAUDE_PID", "AI_AGENT", "TRACEPARENT"]

CSV_FIELDS = [
    "run_id", "timestamp", "usecase", "config", "rep", "claude_version", "workspace_kind", "base_commit",
    "subtype", "is_error", "exit_code", "timed_out", "main_model", "models_used",
    "wall_s", "duration_ms", "duration_api_ms", "num_turns", "total_cost_usd", "advisor_cost_usd",
    "input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "thinking_tokens",
    "per_model_cost_usd", "subagent_stats", "permission_denials",
    "files_changed", "check_pass", "check_score", "judge_score", "judge_model", "judge_cost_usd", "notes",
]


# ----------------------------------------------------------------------------- helpers

def log(msg):
    sys.stderr.write("[bench %s] %s\n" % (dt.datetime.now().strftime("%H:%M:%S"), msg))
    sys.stderr.flush()


def read(path):
    with open(path, "r", encoding="utf-8") as f:
        return f.read()


def write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def sh(cmd, cwd=None, env=None, timeout=600, check=False):
    p = subprocess.run(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                       text=True, timeout=timeout, check=False)
    if check and p.returncode != 0:
        raise RuntimeError("command failed (%d): %s\n%s" % (p.returncode, " ".join(cmd), p.stderr[-2000:]))
    return p


def parse_frontmatter(text):
    """Very small 'key: value' frontmatter parser (no YAML dependency)."""
    meta = {}
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return meta, text.strip()
    i = 1
    while i < len(lines) and lines[i].strip() != "---":
        line = lines[i].strip()
        if line and not line.startswith("#") and ":" in line:
            k, v = line.split(":", 1)
            meta[k.strip()] = v.strip().strip('"').strip("'")
        i += 1
    body = "\n".join(lines[i + 1:]).strip()
    return meta, body


def fnum(v, default=None):
    try:
        if v is None or v == "":
            return default
        return float(v)
    except (TypeError, ValueError):
        return default


# ----------------------------------------------------------------------------- discovery

def list_configs():
    out = {}
    if not os.path.isdir(CONFIGS_DIR):
        return out
    for fn in sorted(os.listdir(CONFIGS_DIR)):
        if fn.endswith(".sh") and not fn.startswith("_"):
            path = os.path.join(CONFIGS_DIR, fn)
            text = read(path)
            desc = ""
            for line in text.splitlines()[1:]:
                if line.startswith("#"):
                    desc = line.lstrip("# ").strip()
                    break
            code = "\n".join(l for l in text.splitlines() if not l.lstrip().startswith("#"))
            m_model = re.search(r"--model[\s=]+(\S+)", code)
            m_adv = re.search(r"--advisor[\s=]+(\S+)", code)
            m_sub = re.search(r"CLAUDE_CODE_SUBAGENT_MODEL=(\S+)", code)
            out[fn[:-3]] = {
                "name": fn[:-3], "path": path, "desc": desc,
                "model": m_model.group(1) if m_model else None,
                "advisor": m_adv.group(1) if m_adv else None,
                "subagent_model": m_sub.group(1) if m_sub else None,
            }
    return out


def load_usecase(name):
    d = os.path.join(USECASES_DIR, name)
    pm = os.path.join(d, "prompt.md")
    if not os.path.isfile(pm):
        raise RuntimeError("use case %s has no prompt.md" % name)
    meta, prompt = parse_frontmatter(read(pm))
    if not prompt:
        raise RuntimeError("use case %s: prompt.md body is empty" % name)
    repo = meta.get("repo")
    if repo:
        repo = os.path.abspath(os.path.expanduser(repo))
        if not os.path.isdir(os.path.join(repo, ".git")):
            raise RuntimeError("use case %s: repo is not a git repository: %s" % (name, repo))

    def opt(fn):
        p = os.path.join(d, fn)
        return p if os.path.isfile(p) else None

    ws = os.path.join(d, "workspace")
    return {
        "name": name, "dir": d, "prompt": prompt, "repo": repo,
        "plugins": [p for p in re.split(r"[,\s]+", meta.get("plugins", "")) if p],
        "workspace_template": ws if os.path.isdir(ws) else None,
        "max_turns": int(fnum(meta.get("max_turns"), DEFAULT_MAX_TURNS)),
        "max_budget_usd": fnum(meta.get("max_budget_usd"), DEFAULT_MAX_BUDGET_USD),
        "timeout_seconds": int(fnum(meta.get("timeout_seconds"), DEFAULT_TIMEOUT_S)),
        "setup": opt("setup.sh"), "check": opt("check.sh"), "rubric": opt("rubric.md"),
    }


def list_usecases():
    names = []
    if os.path.isdir(USECASES_DIR):
        for fn in sorted(os.listdir(USECASES_DIR)):
            if not fn.startswith("_") and not fn.startswith(".") and os.path.isfile(os.path.join(USECASES_DIR, fn, "prompt.md")):
                names.append(fn)
    return names


def claude_version():
    try:
        p = sh(["claude", "--version"], timeout=60)
        return (p.stdout or p.stderr).strip().splitlines()[0]
    except Exception as e:  # pylint: disable=broad-exception-caught
        return "unknown (%s)" % e


def clean_env():
    env = dict(os.environ)
    for v in STRIP_VARS:
        if v in env:
            log("WARNING: %s=%s is set in your shell; removing it for the benchmark so configs stay in control" % (v, env[v]))
            del env[v]
    session = [v for v in SESSION_VARS if v in env]
    if session:
        log("NOTE: started from inside a Claude Code session; not passing its variables to the runs: %s" % ", ".join(session))
        for v in session:
            del env[v]
    for v in WARN_VARS:
        if v in env:
            log("WARNING: %s=%s is set in your shell and applies to every arm" % (v, env[v]))
    return env


def refuse_managed_mcp():
    found = [p for p in MANAGED_MCP_PATHS if os.path.isfile(p)]
    if found:
        sys.exit("%s is deployed on this machine: Claude Code then exits at startup with --strict-mcp-config, which every run "
                 "and the judge use. Run the benchmark on a machine without it." % found[0])


def claude_config_dir():
    return os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")


def read_json_object(path):
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, ValueError):
        return {}
    return data if isinstance(data, dict) else {}


def installed_plugins():
    data = read_json_object(os.path.join(claude_config_dir(), "plugins", "installed_plugins.json"))
    plugins = data.get("plugins", data)
    return set(plugins) if isinstance(plugins, dict) else set()


def plugin_settings(keep=()):
    # The --settings value that enables the plugins in keep and turns off every other one Claude Code could load: those
    # installed, those named in the user's enabledPlugins, and those synced from claude.ai (plugins/synced/<id>/<name>).
    # syncClaudeAiPlugins false also hides synced plugins and stops downloads for this invocation, so a plugin enabled on
    # claude.ai but not downloaded yet, which no file here names, cannot be synced and loaded at startup.
    config = claude_config_dir()
    names = installed_plugins() | set(keep)
    enabled = read_json_object(os.path.join(config, "settings.json")).get("enabledPlugins")
    if isinstance(enabled, dict):
        names |= set(enabled)
    names |= set("%s@synced" % os.path.basename(os.path.dirname(p)) for p in glob.glob(os.path.join(config, "plugins", "synced", "*", "*", "")))
    return json.dumps({"enabledPlugins": {n: n in keep for n in sorted(names)}, "syncClaudeAiPlugins": False})


# ----------------------------------------------------------------------------- workspaces

GIT_ID = ["-c", "user.name=ccbench", "-c", "user.email=ccbench@local", "-c", "commit.gpgsign=false"]


def make_workspace(uc, env):
    parent = tempfile.mkdtemp(prefix="ccbench-")
    path = os.path.join(parent, "ws")
    if uc["repo"]:
        sh(["git", "-C", uc["repo"], "worktree", "add", "--detach", "-q", path, "HEAD"], env=env, check=True)
        base = sh(["git", "-C", uc["repo"], "rev-parse", "--short", "HEAD"], env=env).stdout.strip()
        kind = "worktree"
    else:
        if uc["workspace_template"]:
            shutil.copytree(uc["workspace_template"], path)
        else:
            os.makedirs(path)
        sh(["git", "init", "-q"], cwd=path, env=env, check=True)
        sh(["git"] + GIT_ID + ["add", "-A"], cwd=path, env=env, check=True)
        sh(["git"] + GIT_ID + ["commit", "-q", "--allow-empty", "--no-verify", "-m", "base"], cwd=path, env=env, check=True)
        base = ""
        kind = "copy"
    if uc["setup"]:
        log("  setup.sh")
        p = sh(["bash", uc["setup"]], cwd=path, env=dict(env, WORKSPACE=path, USECASE_DIR=uc["dir"]), timeout=900)
        if p.returncode != 0:
            raise RuntimeError("setup.sh failed: %s" % p.stderr[-2000:])
    return path, kind, base


def destroy_workspace(uc, path, env, keep=False):
    if keep:
        log("  keeping workspace %s" % path)
        return
    try:
        if uc["repo"]:
            sh(["git", "-C", uc["repo"], "worktree", "remove", "--force", path], env=env)
            sh(["git", "-C", uc["repo"], "worktree", "prune"], env=env)
        shutil.rmtree(os.path.dirname(path), ignore_errors=True)
    except Exception as e:  # pylint: disable=broad-exception-caught
        log("  WARNING: could not clean workspace %s: %s" % (path, e))


def collect_diff(path, env):
    """Stage everything (so new files show) and return (patch_text, files_changed)."""
    try:
        sh(["git"] + GIT_ID + ["add", "-A"], cwd=path, env=env)
        names = sh(["git", "diff", "--cached", "HEAD", "--name-only"], cwd=path, env=env).stdout.strip()
        patch = sh(["git", "diff", "--cached", "HEAD", "--no-color"], cwd=path, env=env).stdout
        n = len([x for x in names.splitlines() if x.strip()])
        return patch, n
    except Exception as e:  # pylint: disable=broad-exception-caught
        return "(diff unavailable: %s)" % e, ""


# ----------------------------------------------------------------------------- agent run

def run_claude(cmd, cwd, env, timeout):
    """Run a claude command line, return (exit_code, stdout, stderr, wall_s, timed_out)."""
    t0 = time.monotonic()
    timed_out = False
    # Not a with-block: its exit would wait() and could hang if the kill below fails.
    p = subprocess.Popen(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,  # pylint: disable=consider-using-with
                         text=True, start_new_session=True)
    try:
        out, err = p.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        timed_out = True
        for sig in (signal.SIGTERM, signal.SIGKILL):
            try:
                os.killpg(os.getpgid(p.pid), sig)
            except Exception:  # pylint: disable=broad-exception-caught
                pass
            try:
                out, err = p.communicate(timeout=20)
                break
            except subprocess.TimeoutExpired:
                continue
        else:
            out, err = "", ""
    return p.returncode, out or "", err or "", time.monotonic() - t0, timed_out


def parse_result_json(out):
    out = out.strip()
    if not out:
        return None
    try:
        return json.loads(out)
    except ValueError:
        pass
    for line in reversed(out.splitlines()):
        line = line.strip()
        if line.startswith("{"):
            try:
                return json.loads(line)
            except ValueError:
                continue
    return None


def extract_metrics(data, advisor=None):
    """advisor: alias from the config (e.g. 'fable'); matched as a substring of modelUsage keys."""
    m = {}
    if not data:
        return m
    usage = data.get("usage") or {}
    mu = data.get("modelUsage") or data.get("model_usage") or {}
    m["subtype"] = data.get("subtype", "")
    m["is_error"] = data.get("is_error", "")
    m["duration_ms"] = data.get("duration_ms", "")
    m["duration_api_ms"] = data.get("duration_api_ms", "")
    m["num_turns"] = data.get("num_turns", "")
    m["total_cost_usd"] = data.get("total_cost_usd", "")
    m["input_tokens"] = usage.get("input_tokens", "")
    m["output_tokens"] = usage.get("output_tokens", "")
    m["cache_read_tokens"] = usage.get("cache_read_input_tokens", "")
    m["cache_creation_tokens"] = usage.get("cache_creation_input_tokens", "")
    m["thinking_tokens"] = (usage.get("output_tokens_details") or {}).get("thinking_tokens",
                                                                          sum(v.get("thinkingTokens", 0) or 0 for v in mu.values()))
    m["models_used"] = ";".join(sorted(mu.keys()))
    adv = {k: v for k, v in mu.items() if advisor and advisor in k}
    cand = {k: v for k, v in mu.items() if k not in adv} or mu
    m["main_model"] = max(cand, key=lambda k: (cand[k].get("costUSD") or 0)) if cand else ""
    m["advisor_cost_usd"] = round(sum(v.get("costUSD") or 0 for v in adv.values()), 6) if advisor else ""
    m["per_model_cost_usd"] = json.dumps({k: round(v.get("costUSD", 0) or 0, 6) for k, v in sorted(mu.items())})
    ss = data.get("subagent_stats")
    m["subagent_stats"] = json.dumps(ss) if ss else ""
    pd = data.get("permission_denials")
    m["permission_denials"] = len(pd) if isinstance(pd, list) else ""
    return m


def run_check(uc, ws, env, run_id, cfg_name, result_json_path, result_text_path):
    cenv = dict(env, WORKSPACE=ws, USECASE_DIR=uc["dir"], RUN_ID=run_id, CONFIG=cfg_name,
                RESULT_JSON=result_json_path, RESULT_TEXT=result_text_path)
    try:
        p = sh(["bash", uc["check"]], cwd=ws, env=cenv, timeout=900)
    except subprocess.TimeoutExpired:
        return 0, 0.0, "check.sh timed out"
    passed = 1 if p.returncode == 0 else 0
    score = float(passed)
    lines = [l.strip() for l in p.stdout.strip().splitlines() if l.strip()]
    if lines:
        v = fnum(lines[-1])
        if v is not None and 0.0 <= v <= 1.0:
            score = v
    tail = (p.stdout.strip().splitlines() or [""])[-1][:200]
    return passed, score, tail


def run_judge(uc, prompt, result_text, patch, check_summary, judge_model, judge_effort, env, cwd):
    judge_prompt = (
        "You are grading the output of a coding agent against a rubric. Be strict and consistent.\n\n"
        "## Rubric\n%s\n\n"
        "## Task given to the agent\n%s\n\n"
        "## Agent's final message\n%s\n\n"
        "## Changes the agent made in the workspace (git diff, may be truncated)\n```diff\n%s\n```\n\n"
        "## Deterministic check\n%s\n\n"
        "Return a score from 0 to 10 and a short reasoning."
    ) % (
        read(uc["rubric"]).strip(),
        prompt,
        (result_text or "(empty)")[:RESULT_LIMIT],
        (patch or "(no changes)")[:DIFF_LIMIT],
        check_summary or "(no deterministic check for this use case)",
    )
    cmd = ["claude", "-p", judge_prompt, "--model", judge_model, "--effort", judge_effort,
           "--json-schema", JUDGE_SCHEMA] + JUDGE_FLAGS + ["--settings", plugin_settings()]
    code, out, err, _, _ = run_claude(cmd, cwd, env, 600)
    data = parse_result_json(out)
    score, cost, raw = "", "", data
    if data:
        cost = data.get("total_cost_usd", "")
        so = data.get("structured_output")
        if so is None and isinstance(data.get("result"), str):
            try:
                so = json.loads(data["result"])
            except ValueError:
                so = None
        if isinstance(so, dict):
            score = fnum(so.get("score"), "")
    if score == "":
        log("  WARNING: judge returned no score (exit %s): %s" % (code, (err or out)[-300:].strip()))
    return score, cost, {"cmd_model": judge_model, "exit": code, "stderr": err[-2000:], "result": raw}


def append_csv(row):
    new = not os.path.isfile(CSV_PATH)
    os.makedirs(RESULTS_DIR, exist_ok=True)
    with open(CSV_PATH, "a", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=CSV_FIELDS, extrasaction="ignore")
        if new:
            w.writeheader()
        w.writerow({k: row.get(k, "") for k in CSV_FIELDS})


# ----------------------------------------------------------------------------- commands

def build_agent_cmd(cfg, uc):
    return (["bash", cfg["path"], "-p", uc["prompt"]] + HEADLESS_FLAGS + ["--settings", plugin_settings(uc.get("plugins", ()))] +
            ["--max-turns", str(uc["max_turns"]), "--max-budget-usd", str(uc["max_budget_usd"])])


def one_run(cfg, uc, rep, args, env, version):
    run_id = "%s-%s-%s-r%d" % (dt.datetime.now().strftime("%Y%m%d-%H%M%S"), uc["name"], cfg["name"], rep)
    row = {"run_id": run_id, "timestamp": dt.datetime.now().isoformat(timespec="seconds"),
           "usecase": uc["name"], "config": cfg["name"], "rep": rep, "claude_version": version, "notes": ""}
    notes = []
    log("RUN %s" % run_id)
    ws = None
    try:
        ws, kind, base = make_workspace(uc, env)
        row["workspace_kind"], row["base_commit"] = kind, base
        cmd = build_agent_cmd(cfg, uc)
        if args.dry_run:
            log("  DRY RUN: (cd %s && %s)" % (ws, " ".join(repr(c) if " " in c else c for c in cmd)))
            return None
        code, out, err, wall, timed_out = run_claude(cmd, ws, env, uc["timeout_seconds"])
        row["exit_code"], row["wall_s"], row["timed_out"] = code, round(wall, 1), int(timed_out)
        data = parse_result_json(out)
        raw_json = os.path.join(RAW_DIR, run_id + ".json")
        raw_txt = os.path.join(RAW_DIR, run_id + ".result.md")
        write(raw_json, json.dumps(data, indent=1) if data else out)
        write(os.path.join(RAW_DIR, run_id + ".stderr.txt"), err)
        result_text = (data or {}).get("result") or ""
        write(raw_txt, result_text if isinstance(result_text, str) else json.dumps(result_text))
        row.update(extract_metrics(data, cfg["advisor"]))
        if data and data.get("is_error") is True and row.get("subtype") == "success":
            # The API answered with an error instead of the agent's result, or cut the agent off. A usage limit (429) or a
            # server error (5xx) is not the preset's doing, and summarize leaves those runs out; any other rejection, such
            # as a request too large (4xx), may be, so it stays in the results as the preset's failure.
            status = data.get("api_error_status")
            row["subtype"] = "usage_limit" if status == 429 else "api_error" if isinstance(status, int) and status >= 500 else "api_rejected"
            notes.append("API error %s: %s" % (data.get("api_error_status"), str(data.get("result"))[:200].replace("\n", " ")))
        graded = row.get("subtype") not in ("usage_limit", "api_error")  # a refused run gets no check and no judge call
        if timed_out:
            row["subtype"] = row.get("subtype") or "harness_timeout"
            notes.append("timed out after %ss" % uc["timeout_seconds"])
        if data is None:
            row["subtype"] = row.get("subtype") or "no_json"
            notes.append("no JSON result; stderr: " + err[-200:].replace("\n", " "))
        if cfg["advisor"] and row.get("models_used") and cfg["advisor"] not in row["models_used"]:
            notes.append("advisor '%s' not in modelUsage" % cfg["advisor"])
        patch, nfiles = collect_diff(ws, env)
        write(os.path.join(RAW_DIR, run_id + ".diff.patch"), patch)
        row["files_changed"] = nfiles
        check_summary = ""
        if uc["check"] and graded:
            passed, score, tail = run_check(uc, ws, env, run_id, cfg["name"], raw_json, raw_txt)
            row["check_pass"], row["check_score"] = passed, score
            check_summary = "%s (score %.2f): %s" % ("PASS" if passed else "FAIL", score, tail)
            log("  check: %s" % check_summary)
        if uc["rubric"] and not args.no_judge and graded:
            score, jcost, jraw = run_judge(uc, uc["prompt"], result_text, patch, check_summary,
                                           args.judge_model, args.judge_effort, env, ws)
            row["judge_score"], row["judge_cost_usd"], row["judge_model"] = score, jcost, args.judge_model
            write(os.path.join(RAW_DIR, run_id + ".judge.json"), json.dumps(jraw, indent=1))
            log("  judge: %s" % score)
        log("  %s | %s | %.0fs | $%s | %s turns | main %s | models %s" % (
            row.get("subtype"), cfg["name"], wall, row.get("total_cost_usd") or "-", row.get("num_turns") or "-",
            row.get("main_model") or "-", row.get("models_used") or "-"))
    except Exception as e:  # pylint: disable=broad-exception-caught
        row["subtype"] = row.get("subtype") or "harness_error"
        notes.append("harness error: %s" % str(e)[:300])
        log("  ERROR: %s" % e)
    finally:
        if ws:
            destroy_workspace(uc, ws, env, keep=args.keep_workspaces)
    row["notes"] = "; ".join(notes)
    append_csv(row)
    return row


def cmd_run(args):
    configs = list_configs()
    names = args.config or list(configs.keys())
    missing = [n for n in names if n not in configs]
    if missing:
        sys.exit("unknown config(s): %s (have: %s)" % (", ".join(missing), ", ".join(configs)))
    uc_names = args.usecase or list_usecases()
    if not uc_names:
        sys.exit("no use cases found under %s" % USECASES_DIR)
    usecases = [load_usecase(n) for n in uc_names]
    env = clean_env()
    if not args.dry_run:
        refuse_managed_mcp()
        installed = installed_plugins()
        for uc in usecases:
            absent = [p for p in uc["plugins"] if p not in installed]
            if absent:
                sys.exit("use case %s needs the plugin(s) %s, which are not installed (claude plugin install <name>)" % (uc["name"], ", ".join(absent)))
        # Start each MCP server once with --help, so that npm fetches its package now and not inside a timed run.
        for name, srv in sorted(json.loads(read(MCP_CONFIG))["mcpServers"].items()):
            if subprocess.run([srv["command"]] + srv.get("args", []) + ["--help"], env=env, capture_output=True, timeout=600, check=False).returncode:
                sys.exit("MCP server %s in %s does not start: it needs node and npx, and npm's registry or cache" % (name, MCP_CONFIG))
    version = claude_version()
    log("claude: %s" % version)
    for uc in usecases:
        if uc["rubric"] and not args.no_judge:
            for n in names:
                if configs[n]["model"] and args.judge_model and configs[n]["model"] == args.judge_model:
                    log("NOTE: judge model '%s' is also the main model of config '%s' - acceptable, but a judge outside the arms is cleaner" % (args.judge_model, n))
    # schedule
    plan = []
    if args.order == "blocked":
        for n in names:
            for uc in usecases:
                for rep in range(1, args.runs + 1):
                    plan.append((configs[n], uc, rep))
    else:
        for rep in range(1, args.runs + 1):
            block = [(configs[n], uc, rep) for uc in usecases for n in names]
            if args.shuffle:
                random.shuffle(block)
            plan.extend(block)
    est = len(plan)
    log("%d runs planned: %d config(s) x %d use case(s) x %d repeat(s), order=%s" % (
        est, len(names), len(usecases), args.runs, args.order))
    stopped, server_errors = None, 0
    for i, (cfg, uc, rep) in enumerate(plan, 1):
        log("--- %d/%d" % (i, est))
        row = one_run(cfg, uc, rep, args, env, version) or {}  # a dry run records nothing
        server_errors += row.get("subtype") == "api_error"
        if row.get("subtype") == "usage_limit":
            # Every later run would fail the same way within seconds.
            stopped = "stopped at a usage limit after %d of %d runs (%s); rerun the rest once it resets" % (i, est, row.get("notes"))
            log("STOPPED: " + stopped)
            break
        if args.pause and i < est:
            time.sleep(args.pause)
    if not args.dry_run:
        log("done. results in %s" % CSV_PATH)
        cmd_summarize(args)
    if stopped:
        sys.exit(stopped)
    if server_errors:
        sys.exit("%d run(s) failed with a server error and are left out of the results; rerun them" % server_errors)


def cmd_smoke(args):
    refuse_managed_mcp()
    configs = list_configs()
    env = clean_env()
    log("claude: %s" % claude_version())
    log("python: %s" % sys.version.split()[0])
    tmp = tempfile.mkdtemp(prefix="ccbench-smoke-")
    prompt = "Reply with exactly the word OK and nothing else."
    try:
        for name, cfg in configs.items():
            if args.config and name not in args.config:
                continue
            cmd = ["bash", cfg["path"], "-p", prompt] + HEADLESS_FLAGS + ["--settings", plugin_settings(), "--max-turns", "1", "--max-budget-usd", "2"]
            log("smoke %s: %s" % (name, cfg["desc"]))
            code, out, err, wall, _ = run_claude(cmd, tmp, env, 300)
            data = parse_result_json(out)
            m = extract_metrics(data, cfg["advisor"])
            if not data:
                log("  FAILED (exit %s): %s" % (code, (err or out)[-400:].strip()))
                continue
            log("  subtype=%s wall=%.1fs cost=$%s models=%s" % (m["subtype"], wall, m["total_cost_usd"], m["models_used"]))
            log("  per-model cost: %s" % m["per_model_cost_usd"])
            if m.get("permission_denials"):
                log("  WARNING: %s permission denial(s) - config lacks --dangerously-skip-permissions?" % m["permission_denials"])
            if not cfg["model"]:
                log("  WARNING: no --model pinned in configs/%s.sh; the main model came from your settings (%s). Pin it so this arm cannot drift." % (name, m["main_model"]))
            if cfg["advisor"]:
                if cfg["advisor"] in m["models_used"]:
                    log("  advisor '%s' appears in modelUsage - its tokens are counted in cost" % cfg["advisor"])
                else:
                    log("  NOTE: advisor '%s' did not appear in modelUsage on this 1-turn run. That is expected if the model never called it; "
                        "check a real run's models_used column. If it never shows up, advisor cost is invisible to the harness." % cfg["advisor"])
            if err.strip():
                log("  stderr: %s" % err.strip()[-300:])
        if not args.no_judge:
            log("smoke judge (%s, %s)" % (args.judge_model, args.judge_effort))
            uc = {"rubric": None}
            rubric_path = os.path.join(tmp, "rubric.md")
            write(rubric_path, "Score 10 if the agent's final message is exactly 'OK', otherwise 0.")
            uc["rubric"] = rubric_path
            score, cost, _ = run_judge(uc, prompt, "OK", "", "", args.judge_model, args.judge_effort, env, tmp)
            log("  judge score=%s cost=$%s %s" % (score, cost, "(OK)" if score == 10 else "(unexpected - inspect)"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def median(vals):
    vals = [v for v in vals if v is not None]
    return statistics.median(vals) if vals else None


def fmt(v, kind="f"):
    if v is None or v == "":
        return "-"
    if kind == "pct":
        return "%d%%" % round(v * 100)
    if kind == "usd":
        return "$%.3f" % v
    if kind == "i":
        return "%d" % round(v)
    return "%.1f" % v


def cmd_summarize(args):
    if not os.path.isfile(CSV_PATH):
        sys.exit("no results yet: %s" % CSV_PATH)
    with open(CSV_PATH, newline="", encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    if getattr(args, "since", None):
        rows = [r for r in rows if r["timestamp"] >= args.since]
    # Runs the API refused or cut off (a usage limit, a server error) say nothing about the preset. Rows written before
    # these subtypes existed show it as an error result with subtype success: in results/runs.csv, the six runs of
    # 2026-10-19 that the weekly usage limit cut off (HTTP 429 in their raw JSON).
    refused = [r for r in rows if r.get("subtype") in ("usage_limit", "api_error") or (r.get("subtype") == "success" and r.get("is_error") == "True")]
    rows = [r for r in rows if r not in refused]
    if not rows:
        sys.exit("no rows to summarize" + (" (%d left out: the API refused or cut them off)" % len(refused) if refused else ""))

    def group(keyf):
        g = {}
        for r in rows:
            g.setdefault(keyf(r), []).append(r)
        return g

    # Every run of a use case that has a check counts. An errored run (capped, timed out, a harness fault)
    # is a failure, also when the check passed what it left in the workspace.
    checked = set(r["usecase"] for r in rows if r["check_pass"] != "" or os.path.isfile(os.path.join(USECASES_DIR, r["usecase"], "check.sh")))

    def stats(rs):
        ok = [r for r in rs if r.get("subtype") == "success"]
        graded = [r for r in rs if r["usecase"] in checked]
        return {
            "n": len(rs),
            "errors": len(rs) - len(ok),
            "pass_rate": sum(fnum(r["check_pass"], 0) for r in graded if r.get("subtype") == "success") / len(graded) if graded else None,
            "judge": median([fnum(r["judge_score"]) for r in rs]),
            "wall": median([fnum(r["wall_s"]) for r in rs]),
            "wall_min": min([fnum(r["wall_s"], 1e9) for r in rs] or [None]),
            "wall_max": max([fnum(r["wall_s"], -1) for r in rs] or [None]),
            "cost": median([fnum(r["total_cost_usd"]) for r in rs]),
            "cost_total": None if any(fnum(r["total_cost_usd"]) is None for r in rs) else sum(fnum(r["total_cost_usd"]) for r in rs),
            "advisor": median([fnum(r.get("advisor_cost_usd")) for r in rs]),
            "turns": median([fnum(r["num_turns"]) for r in rs]),
            "cache": median([fnum(r["cache_read_tokens"]) for r in rs]),
            "models": sorted(set(m for r in rs for m in r["models_used"].split(";") if m)),
        }

    headers = ["use case", "config", "n", "err", "pass", "judge", "wall s (med)", "min-max", "cost (med)", "advisor (med)", "cost (sum)", "turns", "cache read"]

    def line(uc, cfg, s):
        return [uc, cfg, fmt(s["n"], "i"), fmt(s["errors"], "i"), fmt(s["pass_rate"], "pct"), fmt(s["judge"]),
                fmt(s["wall"]), "%s-%s" % (fmt(s["wall_min"]), fmt(s["wall_max"])), fmt(s["cost"], "usd"),
                fmt(s["advisor"], "usd"), fmt(s["cost_total"], "usd"), fmt(s["turns"]), fmt(s["cache"], "i")]

    table = []
    for (uc, cfg), rs in sorted(group(lambda r: (r["usecase"], r["config"])).items()):
        table.append(line(uc, cfg, stats(rs)))
    overall = []
    for cfg, rs in sorted(group(lambda r: r["config"]).items()):
        overall.append(line("ALL", cfg, stats(rs)))

    def render(title, hdr, body):
        widths = [max(len(str(x)) for x in col) for col in zip(hdr, *body)]
        out = [title, ""]
        out.append(" | ".join(str(h).ljust(w) for h, w in zip(hdr, widths)))
        out.append("-|-".join("-" * w for w in widths))
        for b in body:
            out.append(" | ".join(str(x).ljust(w) for x, w in zip(b, widths)))
        return "\n".join(out)

    md = []
    md.append("# Benchmark summary\n")
    md.append("Generated %s from %d runs in `results/runs.csv`.\n" % (dt.datetime.now().strftime("%Y-%m-%d %H:%M"), len(rows)))
    if refused:
        md.append("Left out: %d run(s) the API refused or cut off (a usage limit or another API error): %s.\n" % (
            len(refused), ", ".join(r["run_id"] for r in refused)))
    md.append("Medians unless stated. `pass` = deterministic check pass rate (errored runs count as failures), `judge` = median LLM-judge score (0-10), "
              "`cost` = Claude Code's client-side estimate (`total_cost_usd`, all models incl. subagents and advisor, excludes the judge), "
              "`advisor` = the part of that cost billed to the advisor model, `cache read` = median cache_read_input_tokens.\n")
    md.append("## Per use case\n")
    md.append("| " + " | ".join(headers) + " |")
    md.append("|" + "|".join("---" for _ in headers) + "|")
    for b in table:
        md.append("| " + " | ".join(b) + " |")
    md.append("\n## Per config, all use cases pooled\n")
    md.append("| " + " | ".join(headers) + " |")
    md.append("|" + "|".join("---" for _ in headers) + "|")
    for b in overall:
        md.append("| " + " | ".join(b) + " |")
    md.append("\n## Models billed per config\n")
    for cfg, rs in sorted(group(lambda r: r["config"]).items()):
        md.append("- **%s**: %s" % (cfg, ", ".join(stats(rs)["models"]) or "-"))
    flagged = [r for r in rows if r.get("notes")]
    if flagged:
        md.append("\n## Runs with notes\n")
        for r in flagged:
            md.append("- `%s`: %s" % (r["run_id"], r["notes"]))
    write(SUMMARY_PATH, "\n".join(md) + "\n")
    print(render("Per use case", headers, table))
    print()
    print(render("Per config (pooled)", headers, overall))
    if refused:
        print("\nleft out: %d run(s) the API refused or cut off" % len(refused))
    print("\nsummary written to %s" % SUMMARY_PATH)


def cmd_list(_args):
    print("configs (%s):" % CONFIGS_DIR)
    for n, c in list_configs().items():
        print("  %-12s model=%-8s advisor=%-8s subagents=%-8s  %s" % (n, c["model"] or "(settings)", c["advisor"] or "-", c["subagent_model"] or "(default)", c["desc"]))
    print("use cases (%s):" % USECASES_DIR)
    for n in list_usecases():
        uc = load_usecase(n)
        parts = ["turns<=%d" % uc["max_turns"], "budget<=$%g" % uc["max_budget_usd"], "timeout %ds" % uc["timeout_seconds"]]
        parts.append("repo=%s" % uc["repo"] if uc["repo"] else ("workspace/" if uc["workspace_template"] else "empty dir"))
        parts.append("check.sh" if uc["check"] else "no check")
        parts.append("rubric.md" if uc["rubric"] else "no rubric")
        if uc["plugins"]:
            parts.append("plugins: %s" % " ".join(uc["plugins"]))
        print("  %-24s %s" % (n, ", ".join(parts)))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd")

    def judge_args(p):
        p.add_argument("--judge-model", default=JUDGE_DEFAULT_MODEL, help="model for rubric grading (default %s)" % JUDGE_DEFAULT_MODEL)
        p.add_argument("--judge-effort", default=JUDGE_DEFAULT_EFFORT)
        p.add_argument("--no-judge", action="store_true", help="skip rubric grading")

    p = sub.add_parser("run", help="run the benchmark")
    p.add_argument("-u", "--usecase", nargs="*", help="use case names (default: all)")
    p.add_argument("-c", "--config", nargs="*", help="config names (default: all)")
    p.add_argument("--runs", type=int, default=3, help="repeats per (use case, config) (default 3)")
    p.add_argument("--order", choices=["interleaved", "blocked"], default="interleaved",
                   help="interleaved = A,B,C,A,B,C (default, fairer); blocked = all A, then all B")
    p.add_argument("--shuffle", action="store_true", help="shuffle arms within each repeat")
    p.add_argument("--pause", type=float, default=0, help="seconds to sleep between runs")
    p.add_argument("--keep-workspaces", action="store_true")
    p.add_argument("--dry-run", action="store_true", help="print the commands, run nothing")
    judge_args(p)
    p.set_defaults(func=cmd_run)

    p = sub.add_parser("smoke", help="one cheap 1-turn run per config + one judge call")
    p.add_argument("-c", "--config", nargs="*")
    judge_args(p)
    p.set_defaults(func=cmd_smoke)

    p = sub.add_parser("summarize", help="print and write summary tables")
    p.add_argument("--since", help="only rows with timestamp >= this ISO value, e.g. 2026-10-09")
    p.set_defaults(func=cmd_summarize)

    p = sub.add_parser("list", help="show configs and use cases")
    p.set_defaults(func=cmd_list)

    args = ap.parse_args()
    if not args.cmd:
        ap.print_help()
        sys.exit(1)
    args.func(args)


if __name__ == "__main__":
    main()
