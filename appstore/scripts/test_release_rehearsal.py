#!/usr/bin/env python3

import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("release_rehearsal.py")
SPEC = importlib.util.spec_from_file_location("release_rehearsal", MODULE_PATH)
release_rehearsal = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release_rehearsal)


class ReleaseRehearsalTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tempdir = tempfile.TemporaryDirectory()
        self.root = Path(self.tempdir.name)
        self.git("init")
        self.git("config", "user.name", "Test User")
        self.git("config", "user.email", "test@example.com")
        self.commit("initial release")
        self.git("tag", "1.2.3")

    def tearDown(self) -> None:
        self.tempdir.cleanup()

    def git(self, *args: str) -> str:
        result = subprocess.run(
            ["git", *args],
            cwd=self.root,
            check=True,
            capture_output=True,
            text=True,
        )
        return result.stdout.strip()

    def commit(self, message: str) -> None:
        marker = self.root / "history.txt"
        previous = marker.read_text() if marker.exists() else ""
        marker.write_text(f"{previous}{message}\n")
        self.git("add", "history.txt")
        self.git("commit", "-m", message)

    def create_artifacts(self, version: str, release_dir: Path | None = None) -> Path:
        release_dir = release_dir or self.root / "release"
        release_dir.mkdir()
        for name in release_rehearsal.expected_artifact_names(version):
            (release_dir / name).write_bytes(f"binary:{name}".encode())
            (release_dir / name).chmod(0o755)
        return release_dir

    def test_generates_notes_and_checksums_for_exact_commit(self) -> None:
        self.commit("add upload reconciliation")
        release_dir = self.create_artifacts("1.2.4")
        head = self.git("rev-parse", "HEAD")

        result = release_rehearsal.rehearse(
            root=self.root,
            version="1.2.4",
            expected_sha=head,
            release_dir=release_dir,
        )

        self.assertEqual(result.tested_sha, head)
        self.assertEqual(result.previous_tag, "1.2.3")
        notes = result.notes_path.read_text()
        self.assertIn("# Release 1.2.4", notes)
        self.assertIn(f"Tested commit: `{head}`", notes)
        self.assertIn("- add upload reconciliation", notes)
        checksums = result.checksums_path.read_text().splitlines()
        self.assertEqual(len(checksums), 5)
        self.assertTrue(all("  asc_1.2.4_" in line for line in checksums))

    def test_rejects_invalid_version(self) -> None:
        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "x.y.z"):
            release_rehearsal.rehearse(
                root=self.root,
                version="v1.2.4",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=self.root / "release",
            )

    def test_rejects_mismatched_commit(self) -> None:
        previous = self.git("rev-parse", "HEAD")
        self.commit("candidate change")
        self.create_artifacts("1.2.4")

        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "does not match"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.2.4",
                expected_sha=previous,
                release_dir=self.root / "release",
            )

    def test_rejects_empty_changelog(self) -> None:
        self.create_artifacts("1.2.4")

        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "no commits"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.2.4",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=self.root / "release",
            )

    def test_rejects_existing_candidate_tag(self) -> None:
        self.git("tag", "1.2.4")

        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "already exists"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.2.4",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=self.root / "release",
            )

    def test_rejects_candidate_older_than_previous_tag(self) -> None:
        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "must be newer"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.2.2",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=self.root / "release",
            )

    def test_rejects_candidate_older_than_tag_on_other_ref(self) -> None:
        candidate_branch = self.git("branch", "--show-current")
        self.git("checkout", "-b", "newer-release")
        self.commit("newer release")
        self.git("tag", "2.0.0")
        self.git("checkout", candidate_branch)
        self.commit("candidate change")

        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "newer than 2.0.0"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.9.0",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=self.root / "release",
            )

    def test_rejects_missing_artifact(self) -> None:
        self.commit("candidate change")
        release_dir = self.create_artifacts("1.2.4")
        (release_dir / "asc_1.2.4_linux_arm64").unlink()

        with self.assertRaisesRegex(release_rehearsal.RehearsalError, "missing release artifact"):
            release_rehearsal.rehearse(
                root=self.root,
                version="1.2.4",
                expected_sha=self.git("rev-parse", "HEAD"),
                release_dir=release_dir,
            )

    def test_run_rejects_invalid_version_before_make(self) -> None:
        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "x.y.z"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="$(warning expanded)",
                    expected_sha=self.git("rev-parse", "HEAD"),
                    release_dir=self.root / "release",
                )

        run_command.assert_not_called()

    def test_run_command_disables_go_workspace(self) -> None:
        completed = subprocess.CompletedProcess(args=[], returncode=0)
        with mock.patch.object(
            release_rehearsal.subprocess, "run", return_value=completed
        ) as subprocess_run:
            release_rehearsal.run_command(self.root, "make", "release-guardrails")

        self.assertEqual(subprocess_run.call_args.kwargs["env"]["GOWORK"], "off")

    def test_run_rejects_make_unsafe_release_dir_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")
        release_dir = self.root / "release$(shell unsafe)"

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(
                release_rehearsal.RehearsalError, "cannot be represented safely"
            ):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=release_dir,
                )

        run_command.assert_not_called()

    def test_run_rejects_repository_root_release_dir_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(
                release_rehearsal.RehearsalError, "must not be the repository root"
            ):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=self.root,
                )

        run_command.assert_not_called()

    def test_run_rejects_tracked_release_dir_before_make(self) -> None:
        release_dir = self.root / "tracked-output"
        release_dir.mkdir()
        tracked_file = release_dir / "keep.txt"
        tracked_file.write_text("keep me\n")
        self.git("add", "tracked-output/keep.txt")
        self.git("commit", "-m", "add tracked output")
        head = self.git("rev-parse", "HEAD")

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "contains tracked files"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=release_dir,
                )

        run_command.assert_not_called()
        self.assertTrue(tracked_file.is_file())

    def test_run_rejects_git_metadata_release_dir_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")
        git_dir = self.root / ".git"

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "Git metadata"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=git_dir,
                )

        run_command.assert_not_called()
        self.assertTrue(git_dir.exists())
        self.assertEqual(self.git("rev-parse", "HEAD"), head)

    def test_run_rejects_populated_external_release_dir_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")

        with tempfile.TemporaryDirectory() as external_tempdir:
            release_dir = Path(external_tempdir) / "existing-output"
            release_dir.mkdir()
            existing_file = release_dir / "keep.txt"
            existing_file.write_text("keep me\n")

            with mock.patch.object(release_rehearsal, "run_command") as run_command:
                with self.assertRaisesRegex(
                    release_rehearsal.RehearsalError, "must be absent or empty"
                ):
                    release_rehearsal.run_release_rehearsal(
                        root=self.root,
                        version="1.2.4",
                        expected_sha=head,
                        release_dir=release_dir,
                    )

            run_command.assert_not_called()
            self.assertEqual(existing_file.read_text(), "keep me\n")

    def test_run_rejects_modified_tracked_source_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")
        (self.root / "history.txt").write_text("modified after commit\n")

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "source tree is dirty"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=self.root / "release",
                )

        run_command.assert_not_called()

    def test_run_rejects_untracked_source_before_make(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")
        (self.root / "candidate.go").write_text("package candidate\n")

        with mock.patch.object(release_rehearsal, "run_command") as run_command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "source tree is dirty"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=self.root / "release",
                )

        run_command.assert_not_called()

    def test_run_rejects_source_changed_by_build(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")

        def run_command(root: Path, *args: str) -> None:
            if args[:2] == ("make", "build-all"):
                self.create_artifacts("1.2.4")
                (self.root / "history.txt").write_text("modified by build\n")

        with mock.patch.object(release_rehearsal, "run_command", side_effect=run_command) as command:
            with self.assertRaisesRegex(release_rehearsal.RehearsalError, "source tree is dirty"):
                release_rehearsal.run_release_rehearsal(
                    root=self.root,
                    version="1.2.4",
                    expected_sha=head,
                    release_dir=self.root / "release",
                )

        self.assertEqual(command.call_count, 2)

    def test_workflow_entrypoint_invokes_guardrails_before_build(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")

        def run_command(root: Path, *args: str) -> None:
            if args[:2] == ("make", "build-all"):
                self.create_artifacts("1.2.4")

        with mock.patch.object(release_rehearsal, "run_command", side_effect=run_command) as command:
            result = release_rehearsal.run_release_rehearsal(
                root=self.root,
                version="1.2.4",
                expected_sha=head,
                release_dir=self.root / "release",
            )

        self.assertEqual(result.tested_sha, head)
        release_dir = self.root / "release"
        self.assertEqual(
            result.checksums_path.read_text().splitlines(),
            [
                f"{release_rehearsal.sha256(release_dir / name)}  {name}"
                for name in sorted(release_rehearsal.expected_artifact_names("1.2.4"))
            ],
        )
        self.assertEqual(
            command.call_args_list,
            [
                mock.call(self.root.resolve(), "make", "release-guardrails"),
                mock.call(
                    self.root.resolve(),
                    "make",
                    "build-all",
                    "VERSION=1.2.4",
                    f"RELEASE_DIR={(self.root / 'release').resolve()}",
                ),
            ],
        )

    def test_run_passes_custom_release_dir_to_build(self) -> None:
        self.commit("candidate change")
        head = self.git("rev-parse", "HEAD")
        release_dir = self.root / "custom-output"

        def run_command(root: Path, *args: str) -> None:
            if args[:2] == ("make", "build-all"):
                self.create_artifacts("1.2.4", release_dir)

        with mock.patch.object(release_rehearsal, "run_command", side_effect=run_command) as command:
            result = release_rehearsal.run_release_rehearsal(
                root=self.root,
                version="1.2.4",
                expected_sha=head,
                release_dir=release_dir,
            )

        self.assertEqual(result.notes_path.parent, release_dir.resolve())
        self.assertEqual(
            command.call_args_list,
            [
                mock.call(self.root.resolve(), "make", "release-guardrails"),
                mock.call(
                    self.root.resolve(),
                    "make",
                    "build-all",
                    "VERSION=1.2.4",
                    f"RELEASE_DIR={release_dir.resolve()}",
                ),
            ],
        )


if __name__ == "__main__":
    unittest.main()
