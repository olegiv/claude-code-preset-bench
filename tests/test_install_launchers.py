"""install-launchers.sh, run against a temporary target directory."""

import os
import subprocess
import unittest

from helpers import CONFIGS_DIR, ROOT, config_names, temp_dir, write_file

INSTALLER = os.path.join(ROOT, "install-launchers.sh")


class InstallLaunchersTest(unittest.TestCase):
    """Every config becomes an oclaude-* symlink; a launcher is never lost."""

    def setUp(self):
        self.target = os.path.join(temp_dir(self), "bin")

    def install(self):
        return subprocess.run(["bash", INSTALLER, self.target], capture_output=True, text=True, timeout=60, check=False)

    def launcher(self, name):
        return os.path.join(self.target, "oclaude-" + name)

    def test_links_every_config(self):
        p = self.install()
        self.assertEqual(p.returncode, 0, p.stderr)
        linked = sorted(fn[len("oclaude-"):] for fn in os.listdir(self.target))
        self.assertEqual(linked, config_names())
        for name in linked:
            self.assertEqual(os.path.realpath(self.launcher(name)), os.path.realpath(os.path.join(CONFIGS_DIR, name + ".sh")))

    def test_a_regular_launcher_is_kept_as_bak_and_a_rerun_changes_nothing_more(self):
        old = write_file(self.launcher("high"), "old launcher\n", 0o755)
        self.assertEqual(self.install().returncode, 0)
        self.assertTrue(os.path.islink(old))
        with open(old + ".bak", encoding="utf-8") as f:
            self.assertEqual(f.read(), "old launcher\n")
        p = self.install()
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertNotIn("backed up", p.stdout)
        with open(old + ".bak", encoding="utf-8") as f:
            self.assertEqual(f.read(), "old launcher\n")

    def test_refuses_and_changes_nothing_when_a_backup_already_exists(self):
        current = write_file(self.launcher("high"), "current launcher\n", 0o755)
        write_file(current + ".bak", "older backup\n")
        p = self.install()
        self.assertEqual(p.returncode, 1)
        self.assertIn("oclaude-high.bak already exists", p.stderr)
        self.assertFalse(os.path.islink(current))
        with open(current, encoding="utf-8") as f:
            self.assertEqual(f.read(), "current launcher\n")
        self.assertEqual(sorted(os.listdir(self.target)), ["oclaude-high", "oclaude-high.bak"])


if __name__ == "__main__":
    unittest.main()
