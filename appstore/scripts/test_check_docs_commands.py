#!/usr/bin/env python3
"""Verify the shared docs CLI build and its failure boundary."""

import subprocess
import unittest
from pathlib import Path
from unittest.mock import patch

import check_docs_commands


class SharedDocsCommandTests(unittest.TestCase):
    def test_build_once_and_share_fresh_binary(self):
        with patch.object(check_docs_commands.subprocess, "run") as run:
            self.assertEqual(check_docs_commands.main(), 0)
        calls = [call.args[0] for call in run.call_args_list]
        self.assertEqual(len(calls), 3)
        self.assertEqual(calls[0][:3], ["go", "build", "-o"])
        binary = calls[0][3]
        self.assertTrue(Path(binary).is_absolute())
        self.assertFalse(Path(binary).parent.exists())
        self.assertEqual(calls[1][-3:], ["--check-generated", "--binary", binary])
        self.assertEqual(calls[2][-2:], ["--binary", binary])
        self.assertTrue(all(call.kwargs["check"] for call in run.call_args_list))

    def test_build_failure_stops_before_validators(self):
        failure = subprocess.CalledProcessError(2, ["go", "build"])
        with patch.object(check_docs_commands.subprocess, "run", side_effect=failure) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                check_docs_commands.main()
        self.assertEqual(run.call_count, 1)

    def test_later_invocation_rebuilds_in_new_directory(self):
        with patch.object(check_docs_commands.subprocess, "run") as run:
            check_docs_commands.main()
            check_docs_commands.main()
        builds = [call.args[0] for call in run.call_args_list if call.args[0][0] == "go"]
        self.assertEqual(len(builds), 2)
        self.assertNotEqual(builds[0][3], builds[1][3])

    def test_command_failure_stops_before_website_validation(self):
        failure = subprocess.CalledProcessError(1, ["command-check"])
        with patch.object(check_docs_commands.subprocess, "run", side_effect=[None, failure]) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                check_docs_commands.main()
        self.assertEqual(run.call_count, 2)


if __name__ == "__main__":
    unittest.main()
