"""The preset scripts in configs/, run against a fake claude."""

import os
import re
import subprocess
import unittest

from helpers import CONFIGS_DIR, bench, config_names, env_without, make_fake_claude, temp_dir

EFFORTS = ("low", "medium", "high", "xhigh", "max")


def exports_force(name):
    with open(os.path.join(CONFIGS_DIR, name + ".sh"), encoding="utf-8") as f:
        return any(line.strip().startswith("export CLAUDE_CODE_SUBAGENT_MODEL_FORCE=") for line in f)


def flag(args, name):
    return args[args.index(name) + 1] if name in args else None


class ConfigsTest(unittest.TestCase):
    """Every config pins model and effort, passes arguments through and controls FORCE."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = temp_dir(cls)
        make_fake_claude(cls.tmp)

    def run_config(self, name, extra_env=None):
        env = env_without("CLAUDE_CODE_SUBAGENT_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE")
        env["PATH"] = self.tmp + os.pathsep + env.get("PATH", "")
        env.update(extra_env or {})
        p = subprocess.run(["bash", os.path.join(CONFIGS_DIR, name + ".sh"), "a b", "-c"],
                           capture_output=True, text=True, env=env, timeout=30, check=False)
        args = re.findall(r"^arg\d+=\[(.*)\]$", p.stdout, re.M)
        subagent_env = dict(re.findall(r"^(CLAUDE_CODE_SUBAGENT_MODEL(?:_FORCE)?)=(.*)$", p.stdout, re.M))
        return p.returncode, args, subagent_env

    def test_there_are_configs(self):
        self.assertTrue(config_names())

    def test_model_and_effort_are_pinned_and_arguments_pass_through(self):
        for name in config_names():
            with self.subTest(config=name):
                code, args, _ = self.run_config(name)
                self.assertEqual(code, 7)
                self.assertEqual(args[-2:], ["a b", "-c"])
                self.assertTrue(flag(args, "--model"))
                self.assertIn(flag(args, "--effort"), EFFORTS)

    def test_force_is_set_only_by_configs_that_export_it_even_if_the_shell_has_it(self):
        for name in config_names():
            expected = "1" if exports_force(name) else "<unset>"
            for shell_env in (None, {"CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1"}):
                with self.subTest(config=name, force_in_shell=bool(shell_env)):
                    _, _, subagent_env = self.run_config(name, shell_env)
                    self.assertEqual(subagent_env["CLAUDE_CODE_SUBAGENT_MODEL_FORCE"], expected)

    def test_list_configs_reports_what_the_scripts_pass(self):
        configs = bench.list_configs()
        for name in config_names():
            with self.subTest(config=name):
                _, args, subagent_env = self.run_config(name)
                sub = subagent_env["CLAUDE_CODE_SUBAGENT_MODEL"]
                self.assertEqual((configs[name]["model"], configs[name]["advisor"], configs[name]["subagent_model"]),
                                 (flag(args, "--model"), flag(args, "--advisor"), None if sub == "<unset>" else sub))

    def test_paired_arms_differ_in_one_factor(self):
        _, normal, normal_env = self.run_config("normal")
        _, normal_max, normal_max_env = self.run_config("normal-max")
        self.assertEqual(normal_env, normal_max_env)
        self.assertEqual([a for a in normal if a != flag(normal, "--effort")],
                         [a for a in normal_max if a != flag(normal_max, "--effort")])
        self.assertNotEqual(flag(normal, "--effort"), flag(normal_max, "--effort"))

        _, light, light_env = self.run_config("light")
        _, light_force, light_force_env = self.run_config("light-force")
        self.assertEqual(light, light_force)
        self.assertEqual(light_env["CLAUDE_CODE_SUBAGENT_MODEL"], light_force_env["CLAUDE_CODE_SUBAGENT_MODEL"])
        self.assertNotEqual(light_env["CLAUDE_CODE_SUBAGENT_MODEL_FORCE"],
                            light_force_env["CLAUDE_CODE_SUBAGENT_MODEL_FORCE"])


if __name__ == "__main__":
    unittest.main()
