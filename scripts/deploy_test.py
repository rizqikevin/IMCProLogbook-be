import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import unittest


SHA = "a" * 40
PREVIOUS_SHA = "b" * 40
SCRIPT = Path(__file__).with_name("deploy.sh")

MOCK_COMMAND = r'''
import json
import os
from pathlib import Path
import sys

command, *args = sys.argv[1:]
with open(os.environ["COMMAND_LOG"], "a") as log:
    log.write(json.dumps([command, *args]) + "\n")
scenario = os.environ["SCENARIO"]
if command == "git" and args == ["rev-parse", "HEAD"]:
    print(os.environ["TEST_SHA"])
elif command == "flock" and args == ["-n", "9"]:
    pass
elif command == "curl":
    sys.exit(43 if scenario == "health_failure" else 0)
elif command == "docker":
    if args[0] == "ps":
        service = args[-1].split("=")[-1]
        print(service + "-container")
    elif args[0] == "inspect":
        template = args[2]
        if "working_dir" in template:
            print(os.environ["OLD_CHECKOUT"])
        elif "config_files" in template:
            print(os.environ["OLD_CHECKOUT"] + "/docker-compose.yml")
        elif template == "{{.Image}}":
            print("sha256:old-" + args[-1].removesuffix("-container"))
        else:
            sys.exit("Unexpected inspect: " + repr(args))
    elif args[0] == "tag":
        pass
    elif args[0] == "compose":
        index = 1
        while index < len(args) and args[index].startswith("-"):
            if args[index] not in ("--project-name", "--project-directory", "--env-file", "-f"):
                sys.exit("Unexpected compose option: " + repr(args))
            index += 2
        action = args[index]
        if action == "config":
            print("services:\n  app:\n    image: previous-app\n  web:\n    image: previous-web")
        elif action == "build":
            sys.exit(41 if scenario == "build_failure" else 0)
        elif action == "run":
            pass
        elif action == "up":
            rollback = any(Path(arg).name == "previous.yml" for arg in args)
            sys.exit(42 if scenario == "up_failure" and not rollback else 0)
        else:
            sys.exit("Unexpected compose action: " + repr(args))
    else:
        sys.exit("Unexpected docker command: " + repr(args))
else:
    sys.exit("Unexpected command: " + repr([command, *args]))
'''


class DeployTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.root = self.directory / "production"
        self.state = self.root / ".cd"
        self.state.mkdir(parents=True)
        (self.root / ".env").write_text("DB_PASSWORD=test-only\n")
        (self.state / "project").write_text("imcpro\n")
        (self.state / "deployed-sha").write_text(PREVIOUS_SHA + "\n")
        self.checkout = self.directory / "checkout"
        self.checkout.mkdir()
        (self.checkout / "docker-compose.yml").write_text("services: {}\n")
        self.script = self.checkout / "deploy.sh"
        source = SCRIPT.read_text()
        assignment = "root=/home/robot/IMCProLogbook-be"
        self.assertEqual(source.count(assignment), 1)
        self.script.write_text(source.replace(assignment, "root=" + shlex.quote(str(self.root)), 1))
        self.log = self.directory / "commands.jsonl"
        mock = self.directory / "mock.py"
        mock.write_text(MOCK_COMMAND)
        binaries = self.directory / "bin"
        binaries.mkdir()
        for command in ("docker", "git", "curl", "flock"):
            executable = binaries / command
            executable.write_text(
                "#!/bin/sh\nexec " + shlex.quote(sys.executable) + " "
                + shlex.quote(str(mock)) + " " + command + ' "$@"\n'
            )
            executable.chmod(0o700)
        self.environment = {
            "PATH": str(binaries) + os.pathsep + "/usr/bin:/bin",
            "HOME": str(self.directory),
            "COMMAND_LOG": str(self.log),
            "TEST_SHA": SHA,
            "OLD_CHECKOUT": str(self.directory / "previous-checkout"),
        }

    def deploy(self, scenario="success", sha=SHA):
        result = subprocess.run(
            ["/bin/bash", str(self.script), sha],
            cwd=self.checkout,
            env={**self.environment, "SCENARIO": scenario},
            capture_output=True,
            text=True,
            timeout=20,
        )
        self.commands = [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
        return result

    def compose_commands(self, action):
        return [command for command in self.commands if command[:2] == ["docker", "compose"] and action in command]

    def assert_database_untouched(self):
        for command in self.commands:
            if command[0] != "docker":
                continue
            self.assertIn(command[1], ("ps", "inspect", "tag", "compose"), command)
            if command[1] == "compose":
                self.assertFalse(set(command) & {"down", "rm", "stop", "kill", "restart", "--volumes", "-v", "prune"}, command)
                if "up" in command:
                    self.assertEqual(command[command.index("up"):], ["up", "-d", "--no-deps", "--no-build", "--wait", "--wait-timeout", "180", "app", "web"])
                if "run" in command:
                    self.assertEqual(command[command.index("run"):], ["run", "--rm", "--no-deps", "migrate"])
                self.assertNotIn("db", command)

    def assert_previous_release_recorded(self):
        self.assertEqual((self.state / "deployed-sha").read_text(), PREVIOUS_SHA + "\n")

    def test_invalid_sha_rejected_before_commands(self):
        for sha in ("", "main", "a" * 39, "a" * 41, "g" * 40, "A" * 40, SHA + "; touch injected"):
            with self.subTest(sha=sha):
                result = self.deploy(sha=sha)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Expected full tested commit SHA", result.stderr)
                self.assertEqual(self.commands, [])
                self.assert_previous_release_recorded()

    def test_build_failure_does_not_replace_or_migrate(self):
        result = self.deploy("build_failure")
        self.assertEqual(result.returncode, 41, result.stderr)
        self.assertEqual(len(self.compose_commands("build")), 1)
        self.assertEqual(self.compose_commands("up"), [])
        self.assertEqual(self.compose_commands("run"), [])
        self.assertFalse(any(command[0] == "curl" for command in self.commands))
        self.assert_previous_release_recorded()
        self.assert_database_untouched()

    def test_success_builds_migrates_replaces_and_checks_health(self):
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        build, = self.compose_commands("build")
        migrate, = self.compose_commands("run")
        up, = self.compose_commands("up")
        health, = [command for command in self.commands if command[0] == "curl"]
        self.assertEqual(build[-3:], ["build", "app", "web"])
        self.assertLess(self.commands.index(build), self.commands.index(migrate))
        self.assertLess(self.commands.index(migrate), self.commands.index(up))
        self.assertLess(self.commands.index(up), self.commands.index(health))
        self.assertIn("http://127.0.0.1:18472/", health)
        self.assertIn("--fail", health)
        self.assertEqual((self.state / "deployed-sha").read_text(), SHA + "\n")
        self.assert_database_untouched()

    def test_failed_up_or_health_restores_previous_images(self):
        for scenario, status in (("up_failure", 42), ("health_failure", 43)):
            with self.subTest(scenario=scenario):
                self.log.unlink(missing_ok=True)
                result = self.deploy(scenario)
                self.assertEqual(result.returncode, status, result.stderr)
                replacement, rollback = self.compose_commands("up")
                self.assertTrue(any(Path(arg).name == "current.yml" for arg in replacement))
                self.assertTrue(any(Path(arg).name == "previous.yml" for arg in rollback))
                images = next(Path(arg) for arg in rollback if Path(arg).name == "old-images.yml")
                self.assertEqual(images.read_text(), "services:\n  app:\n    image: imcpro-app:rollback-" + SHA + "\n  web:\n    image: imcpro-web:rollback-" + SHA + "\n")
                for service in ("app", "web"):
                    tag = ["docker", "tag", "sha256:old-" + service, "imcpro-" + service + ":rollback-" + SHA]
                    self.assertIn(tag, self.commands)
                    self.assertLess(self.commands.index(tag), self.commands.index(replacement))
                self.assertEqual(len(self.compose_commands("run")), 1)
                self.assertEqual(any(command[0] == "curl" for command in self.commands), scenario == "health_failure")
                self.assertIn("Previous app/web containers restored.", result.stderr)
                self.assert_previous_release_recorded()
                self.assert_database_untouched()
                # Each attempt needs fresh release state, even within the same second.
                for release in (self.state / "releases").iterdir():
                    for file in release.iterdir():
                        file.unlink()
                    release.rmdir()


if __name__ == "__main__":
    unittest.main()
