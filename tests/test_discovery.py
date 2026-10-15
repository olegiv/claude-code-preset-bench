"""Finding configs and use cases."""

import os
import unittest
from unittest import mock

from helpers import bench, temp_dir, write_file


class TempDirTestCase(unittest.TestCase):
    """Points one of bench's directory constants at a temporary directory."""

    patched = None

    def setUp(self):
        self.tmp = temp_dir(self)
        patcher = mock.patch.object(bench, self.patched, self.tmp)
        patcher.start()
        self.addCleanup(patcher.stop)


class ListConfigsTest(TempDirTestCase):
    """list_configs() reads model, advisor and subagent model from the scripts."""

    patched = "CONFIGS_DIR"

    def config(self, text, name="arm.sh"):
        write_file(os.path.join(self.tmp, name), text, 0o755)

    def test_reads_the_settings_and_the_description(self):
        self.config('#!/bin/bash\n# arm: the description\n# --model in-a-comment\n'
                    'export CLAUDE_CODE_SUBAGENT_MODEL=haiku\n'
                    'exec claude --effort max --model sonnet --advisor opus "$@"\n')
        c = bench.list_configs()["arm"]
        self.assertEqual((c["model"], c["advisor"], c["subagent_model"], c["desc"]),
                         ("sonnet", "opus", "haiku", "arm: the description"))

    def test_the_force_line_is_not_the_subagent_model_in_either_order(self):
        force, model = "export CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1", "export CLAUDE_CODE_SUBAGENT_MODEL=haiku"
        for first, second in ((force, model), (model, force)):
            with self.subTest(first=first):
                self.config('#!/bin/bash\n# arm\n%s\n%s\nexec claude --model sonnet "$@"\n' % (first, second))
                self.assertEqual(bench.list_configs()["arm"]["subagent_model"], "haiku")

    def test_commented_out_settings_are_ignored(self):
        self.config('#!/bin/bash\n# arm\n# export CLAUDE_CODE_SUBAGENT_MODEL=sonnet\n# exec claude --model opus\n'
                    'exec claude "$@"\n')
        c = bench.list_configs()["arm"]
        self.assertEqual((c["model"], c["advisor"], c["subagent_model"]), (None, None, None))

    def test_only_sh_files_not_starting_with_an_underscore(self):
        self.config("#!/bin/bash\n# a\n", "a.sh")
        self.config("#!/bin/bash\n# draft\n", "_draft.sh")
        self.config("notes\n", "notes.txt")
        self.assertEqual(list(bench.list_configs()), ["a"])


class UseCaseTest(TempDirTestCase):
    """load_usecase() and list_usecases() read usecases/<name>/."""

    patched = "USECASES_DIR"

    def usecase(self, name, prompt_md, files=()):
        d = os.path.join(self.tmp, name)
        write_file(os.path.join(d, "prompt.md"), prompt_md)
        for fn in files:
            write_file(os.path.join(d, fn), "#!/bin/bash\n", 0o755)
        return d

    def test_defaults(self):
        self.usecase("plain", "Fix it.\n")
        uc = bench.load_usecase("plain")
        self.assertEqual((uc["max_turns"], uc["max_budget_usd"], uc["timeout_seconds"]),
                         (bench.DEFAULT_MAX_TURNS, bench.DEFAULT_MAX_BUDGET_USD, bench.DEFAULT_TIMEOUT_S))
        self.assertEqual(uc["prompt"], "Fix it.")
        self.assertEqual((uc["repo"], uc["workspace_template"], uc["check"], uc["rubric"], uc["setup"]),
                         (None, None, None, None, None))

    def test_frontmatter_and_optional_files(self):
        d = self.usecase("full", "---\nmax_turns: 12\nmax_budget_usd: 2.5\ntimeout_seconds: 60\n---\nDo it.\n",
                         files=("check.sh", "rubric.md", "setup.sh", "workspace/a.txt"))
        uc = bench.load_usecase("full")
        self.assertEqual((uc["max_turns"], uc["max_budget_usd"], uc["timeout_seconds"]), (12, 2.5, 60))
        self.assertEqual((uc["check"], uc["rubric"], uc["setup"]),
                         tuple(os.path.join(d, fn) for fn in ("check.sh", "rubric.md", "setup.sh")))
        self.assertEqual(uc["workspace_template"], os.path.join(d, "workspace"))

    def test_repo_must_be_a_git_repository(self):
        repo = os.path.join(self.tmp, "repo")
        os.makedirs(os.path.join(repo, ".git"))
        self.usecase("withrepo", "---\nrepo: %s\n---\nDo it.\n" % repo)
        self.assertEqual(bench.load_usecase("withrepo")["repo"], os.path.abspath(repo))
        self.usecase("badrepo", "---\nrepo: %s\n---\nDo it.\n" % self.tmp)
        with self.assertRaisesRegex(RuntimeError, "not a git repository"):
            bench.load_usecase("badrepo")

    def test_missing_or_empty_prompt(self):
        os.makedirs(os.path.join(self.tmp, "noprompt"))
        self.usecase("empty", "---\nmax_turns: 3\n---\n")
        for name, message in (("noprompt", "no prompt.md"), ("empty", "body is empty")):
            with self.subTest(name=name):
                with self.assertRaisesRegex(RuntimeError, message):
                    bench.load_usecase(name)

    def test_list_skips_underscore_dot_and_folders_without_a_prompt(self):
        for name in ("b", "a", "_template", ".hidden"):
            self.usecase(name, "x\n")
        os.makedirs(os.path.join(self.tmp, "noprompt"))
        self.assertEqual(bench.list_usecases(), ["a", "b"])


if __name__ == "__main__":
    unittest.main()
