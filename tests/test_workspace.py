"""Workspaces, the deterministic check and running a command with a timeout."""

import os
import subprocess
import tempfile
import time
import unittest
from unittest import mock

from helpers import bench, temp_dir, write_file


class WorkspaceTest(unittest.TestCase):
    """make_workspace(), collect_diff() and destroy_workspace() for copied workspaces."""

    def setUp(self):
        self.tmp = temp_dir(self)
        self.env = dict(os.environ)
        real_mkdtemp = tempfile.mkdtemp
        for patcher in (mock.patch.object(bench.tempfile, "mkdtemp",
                                          side_effect=lambda prefix: real_mkdtemp(prefix=prefix, dir=self.tmp)),
                        mock.patch.object(bench, "log")):
            patcher.start()
            self.addCleanup(patcher.stop)

    def usecase(self, name, template=None, setup=None):
        d = os.path.join(self.tmp, name)
        os.makedirs(d)
        uc = {"dir": d, "repo": None, "workspace_template": None, "setup": None}
        if template is not None:
            uc["workspace_template"] = os.path.join(d, "workspace")
            os.makedirs(uc["workspace_template"])
            for fn, text in template.items():
                write_file(os.path.join(uc["workspace_template"], fn), text)
        if setup is not None:
            uc["setup"] = write_file(os.path.join(d, "setup.sh"), setup, 0o755)
        return uc

    def make(self, uc):
        path, kind, base = bench.make_workspace(uc, self.env)
        self.addCleanup(bench.destroy_workspace, uc, path, self.env)
        return path, kind, base

    def test_a_copied_workspace_starts_from_a_base_commit(self):
        path, kind, base = self.make(self.usecase("uc", {"a.txt": "one\n"}))
        self.assertEqual((kind, base), ("copy", ""))
        self.assertEqual(bench.read(os.path.join(path, "a.txt")), "one\n")
        log = subprocess.run(["git", "log", "--format=%s"], cwd=path, capture_output=True, text=True, check=True)
        self.assertEqual(log.stdout.strip(), "base")

    def test_without_a_template_the_workspace_is_an_empty_repository(self):
        path, _, _ = self.make(self.usecase("uc"))
        self.assertEqual(os.listdir(path), [".git"])

    def test_collect_diff_sees_changed_and_new_files(self):
        path, _, _ = self.make(self.usecase("uc", {"a.txt": "one\n"}))
        bench.write(os.path.join(path, "a.txt"), "two\n")
        bench.write(os.path.join(path, "new.txt"), "new\n")
        patch, changed = bench.collect_diff(path, self.env)
        self.assertEqual(changed, 2)
        self.assertIn("+two", patch)
        self.assertIn("+new", patch)

    def test_setup_runs_inside_the_workspace(self):
        uc = self.usecase("uc", {}, setup='#!/bin/bash\necho "$USECASE_DIR" > from-setup.txt\n')
        path, _, _ = self.make(uc)
        self.assertEqual(bench.read(os.path.join(path, "from-setup.txt")).strip(), uc["dir"])

    def test_a_failing_setup_stops_the_run(self):
        uc = self.usecase("uc", {}, setup="#!/bin/bash\necho no-network >&2\nexit 1\n")
        with self.assertRaisesRegex(RuntimeError, "setup.sh failed: no-network"):
            bench.make_workspace(uc, self.env)

    def test_destroy_removes_the_workspace_unless_kept(self):
        uc = self.usecase("uc", {"a.txt": "x\n"})
        path, _, _ = bench.make_workspace(uc, self.env)
        bench.destroy_workspace(uc, path, self.env, keep=True)
        self.assertTrue(os.path.isdir(path))
        bench.destroy_workspace(uc, path, self.env)
        self.assertFalse(os.path.exists(os.path.dirname(path)))


class RunCheckTest(unittest.TestCase):
    """run_check(): exit code decides pass, a last line between 0 and 1 is a partial score."""

    def check(self, script):
        tmp = temp_dir(self)
        ws = os.path.join(tmp, "ws")
        os.makedirs(ws)
        uc = {"dir": tmp, "check": write_file(os.path.join(tmp, "check.sh"), script, 0o755)}
        return bench.run_check(uc, ws, dict(os.environ), "run-1", "light", "/r.json", "/r.md")

    def test_the_exit_code_decides_pass(self):
        self.assertEqual(self.check("#!/bin/bash\necho fine\n"), (1, 1.0, "fine"))
        self.assertEqual(self.check("#!/bin/bash\necho broken\nexit 3\n"), (0, 0.0, "broken"))

    def test_a_last_line_between_0_and_1_is_a_partial_score(self):
        self.assertEqual(self.check("#!/bin/bash\necho 2 of 4\necho 0.5\nexit 1\n"), (0, 0.5, "0.5"))
        self.assertEqual(self.check("#!/bin/bash\necho 7\n"), (1, 1.0, "7"))

    def test_the_run_details_reach_the_script(self):
        _, _, tail = self.check('#!/bin/bash\necho "$RUN_ID $CONFIG $RESULT_JSON $RESULT_TEXT $(basename "$PWD")"\n')
        self.assertEqual(tail, "run-1 light /r.json /r.md ws")


class RunClaudeTest(unittest.TestCase):
    """run_claude() returns exit code and output, and kills the whole process group on timeout."""

    def test_exit_code_and_output(self):
        code, out, err, wall, timed_out = bench.run_claude(
            ["bash", "-c", "echo out; echo err >&2; exit 4"], None, dict(os.environ), 30)
        self.assertEqual((code, out, err, timed_out), (4, "out\n", "err\n", False))
        self.assertGreaterEqual(wall, 0)

    def test_a_timeout_kills_the_process_group(self):
        start = time.monotonic()
        code, _, _, _, timed_out = bench.run_claude(
            ["bash", "-c", "sleep 30 & sleep 30; wait"], None, dict(os.environ), 1)
        self.assertTrue(timed_out)
        self.assertNotEqual(code, 0)
        self.assertLess(time.monotonic() - start, 15)


if __name__ == "__main__":
    unittest.main()
