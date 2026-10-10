#!/usr/bin/env python3
"""Validate a non-publishing ASC release rehearsal."""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

from assemble_release_candidate import assemble


SEMVER = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+$")


class RehearsalError(RuntimeError):
    pass


class RehearsalResult:
    def __init__(
        self,
        *,
        tested_sha: str,
        previous_tag: str | None,
        notes_path: Path,
        checksums_path: Path,
    ) -> None:
        self.tested_sha = tested_sha
        self.previous_tag = previous_tag
        self.notes_path = notes_path
        self.checksums_path = checksums_path


class SourceState:
    def __init__(self, *, tested_sha: str, previous_tag: str | None, subjects: list[str]) -> None:
        self.tested_sha = tested_sha
        self.previous_tag = previous_tag
        self.subjects = subjects


def expected_artifact_names(version: str) -> tuple[str, ...]:
    return (
        f"asc_{version}_macOS_amd64",
        f"asc_{version}_macOS_arm64",
        f"asc_{version}_linux_amd64",
        f"asc_{version}_linux_arm64",
        f"asc_{version}_windows_amd64.exe",
    )


def run_git(root: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=root,
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
        raise RehearsalError(f"git {' '.join(args)} failed: {detail}")
    return result.stdout.strip()


def parse_version(value: str) -> tuple[int, int, int]:
    if not SEMVER.fullmatch(value):
        raise RehearsalError(f"release version must match x.y.z; got {value!r}")
    return tuple(int(part) for part in value.split("."))


def latest_release_tag(root: Path, *, merged_only: bool) -> str | None:
    tag_args = ["tag"]
    if merged_only:
        tag_args.extend(("--merged", "HEAD"))
    tag_args.append("--list")

    candidates: list[tuple[tuple[int, int, int], str]] = []
    for tag in run_git(root, *tag_args).splitlines():
        if SEMVER.fullmatch(tag):
            candidates.append((parse_version(tag), tag))
    if not candidates:
        return None
    return max(candidates)[1]


def commit_subjects(root: Path, previous_tag: str | None) -> list[str]:
    revision = f"{previous_tag}..HEAD" if previous_tag else "HEAD"
    output = run_git(root, "log", "--format=%s", revision)
    return [line for line in output.splitlines() if line.strip()]


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as artifact:
        for chunk in iter(lambda: artifact.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def validate_source(*, root: Path, version: str, expected_sha: str) -> SourceState:
    candidate_version = parse_version(version)
    root = root.resolve()

    tested_sha = run_git(root, "rev-parse", "HEAD")
    resolved_expected_sha = run_git(root, "rev-parse", "--verify", f"{expected_sha}^{{commit}}")
    if tested_sha != resolved_expected_sha:
        raise RehearsalError(
            f"checked-out commit {tested_sha} does not match expected commit {resolved_expected_sha}"
        )

    existing_tag = subprocess.run(
        ["git", "show-ref", "--verify", "--quiet", f"refs/tags/{version}"],
        cwd=root,
        check=False,
    )
    if existing_tag.returncode == 0:
        raise RehearsalError(f"candidate release tag {version} already exists")
    if existing_tag.returncode not in (0, 1):
        raise RehearsalError(f"could not determine whether candidate tag {version} exists")

    latest_tag = latest_release_tag(root, merged_only=False)
    if latest_tag and candidate_version <= parse_version(latest_tag):
        raise RehearsalError(f"candidate version {version} must be newer than {latest_tag}")

    previous_tag = latest_release_tag(root, merged_only=True)
    subjects = commit_subjects(root, previous_tag)
    if not subjects:
        boundary = previous_tag or "the start of repository history"
        raise RehearsalError(f"no commits found after {boundary}")

    return SourceState(tested_sha=tested_sha, previous_tag=previous_tag, subjects=subjects)


def resolve_release_dir(*, root: Path, release_dir: Path) -> Path:
    if not release_dir.is_absolute():
        release_dir = root / release_dir
    return release_dir.resolve()


def ensure_clean_source(
    *, root: Path, release_dir: Path, require_empty_release_dir: bool = False
) -> None:
    root = root.resolve()
    release_dir = resolve_release_dir(root=root, release_dir=release_dir)
    pathspecs = ["."]

    if any(character in str(release_dir) for character in ('$', '`', '"', "\\", "\n", "\r")):
        raise RehearsalError(
            "release output directory cannot be represented safely in the Make build"
        )

    try:
        root.relative_to(release_dir)
    except ValueError:
        pass
    else:
        raise RehearsalError(
            "release output directory must not be the repository root or one of its ancestors"
        )

    try:
        relative_release_dir = release_dir.relative_to(root)
    except ValueError:
        relative_release_dir = None

    if relative_release_dir is not None:
        if ".git" in relative_release_dir.parts:
            raise RehearsalError("release output directory must not contain Git metadata")
        tracked_files = run_git(root, "ls-files", "--", relative_release_dir.as_posix())
        if tracked_files:
            raise RehearsalError(
                "release output directory contains tracked files and cannot be cleaned safely:\n"
                f"{tracked_files}"
            )
        release_pathspec = relative_release_dir.as_posix()
        pathspecs.append(f":(exclude,top,literal){release_pathspec}")

    git_dir = Path(run_git(root, "rev-parse", "--absolute-git-dir")).resolve()
    try:
        git_dir.relative_to(release_dir)
    except ValueError:
        try:
            release_dir.relative_to(git_dir)
        except ValueError:
            pass
        else:
            raise RehearsalError("release output directory must not be inside Git metadata")
    else:
        raise RehearsalError("release output directory must not contain Git metadata")

    default_release_dir = (root / "release").resolve()
    if require_empty_release_dir and release_dir != default_release_dir and release_dir.exists():
        try:
            populated = not release_dir.is_dir() or any(release_dir.iterdir())
        except OSError as error:
            raise RehearsalError(
                f"could not inspect custom release output directory {release_dir}: {error}"
            ) from error
        if populated:
            raise RehearsalError(
                "custom release output directory must be absent or empty before rehearsal"
            )

    status = run_git(
        root,
        "status",
        "--porcelain=v1",
        "--untracked-files=all",
        "--",
        *pathspecs,
    )
    if status:
        raise RehearsalError(f"source tree is dirty outside the release output directory:\n{status}")


def write_outputs(*, source: SourceState, version: str, release_dir: Path) -> RehearsalResult:
    release_dir = release_dir.resolve()
    artifacts: list[Path] = []
    for name in expected_artifact_names(version):
        artifact = release_dir / name
        if not artifact.is_file() or artifact.stat().st_size == 0:
            raise RehearsalError(f"missing release artifact: {artifact}")
        artifacts.append(artifact)

    notes_path = write_release_notes(source=source, version=version, release_dir=release_dir)

    checksums_path = release_dir / f"asc_{version}_checksums.txt"
    checksum_lines = [f"{sha256(artifact)}  {artifact.name}" for artifact in artifacts]
    checksums_path.write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")

    return RehearsalResult(
        tested_sha=source.tested_sha,
        previous_tag=source.previous_tag,
        notes_path=notes_path,
        checksums_path=checksums_path,
    )


def write_release_notes(*, source: SourceState, version: str, release_dir: Path) -> Path:
    notes_path = release_dir / f"asc_{version}_release-notes.md"
    notes = [
        f"# Release {version}",
        "",
        f"Tested commit: `{source.tested_sha}`",
        "",
        f"## Changes since {source.previous_tag or 'repository start'}",
        "",
        *(f"- {subject}" for subject in source.subjects),
        "",
    ]
    notes_path.write_text("\n".join(notes), encoding="utf-8")
    return notes_path


def rehearse(*, root: Path, version: str, expected_sha: str, release_dir: Path) -> RehearsalResult:
    source = validate_source(root=root, version=version, expected_sha=expected_sha)
    return write_outputs(source=source, version=version, release_dir=release_dir)


def run_command(root: Path, *args: str) -> None:
    environment = os.environ.copy()
    environment["GOWORK"] = "off"
    result = subprocess.run(args, cwd=root, check=False, env=environment)
    if result.returncode != 0:
        raise RehearsalError(f"{' '.join(args)} failed with exit {result.returncode}")


def assemble_release_lanes(*, root: Path, version: str, tested_sha: str, release_dir: Path) -> None:
    """Pack the built binaries into the release workflow's per-lane archives and join them."""
    with tempfile.TemporaryDirectory() as workdir:
        workdir = Path(workdir)
        lanes = {"macos": [], "portable": []}
        for name in expected_artifact_names(version):
            lanes["macos" if "_macOS_" in name else "portable"].append(name)
        for lane, names in lanes.items():
            lane_dir = workdir / lane
            (lane_dir / "release").mkdir(parents=True)
            for name in names:
                artifact = release_dir / name
                if not artifact.is_file() or artifact.stat().st_size == 0:
                    raise RehearsalError(f"missing release artifact: {artifact}")
                shutil.move(artifact, lane_dir / "release" / name)
            (lane_dir / "commit").write_text(f"{tested_sha}\n", encoding="utf-8")
            environment = os.environ.copy()
            environment["COPYFILE_DISABLE"] = "1"
            result = subprocess.run(
                ["tar", "-cf", str(workdir / f"{lane}.tar"), "release", "commit"],
                cwd=lane_dir,
                check=False,
                env=environment,
            )
            if result.returncode != 0:
                raise RehearsalError(f"packing the {lane} lane failed with exit {result.returncode}")

        try:
            assemble(
                workdir / "macos.tar",
                workdir / "portable.tar",
                workdir / "candidate",
                version,
                tested_sha,
            )
        except ValueError as error:
            raise RehearsalError(f"release candidate assembly failed: {error}") from error
        for asset in (workdir / "candidate" / "release").iterdir():
            shutil.move(asset, release_dir / asset.name)

    verify = subprocess.run(
        [
            sys.executable,
            str(Path(__file__).with_name("verify_release_assets.py")),
            "--release-dir",
            str(release_dir),
            "--version",
            version,
        ],
        cwd=root,
        check=False,
    )
    if verify.returncode != 0:
        raise RehearsalError(f"release asset verification failed with exit {verify.returncode}")


def run_release_rehearsal(
    *, root: Path, version: str, expected_sha: str, release_dir: Path
) -> RehearsalResult:
    root = root.resolve()
    release_dir = resolve_release_dir(root=root, release_dir=release_dir)
    validate_source(root=root, version=version, expected_sha=expected_sha)
    ensure_clean_source(root=root, release_dir=release_dir, require_empty_release_dir=True)
    run_command(root, "make", "release-guardrails")
    run_command(
        root,
        "make",
        "build-all",
        f"VERSION={version}",
        f"RELEASE_DIR={release_dir}",
    )
    ensure_clean_source(root=root, release_dir=release_dir)
    source = validate_source(root=root, version=version, expected_sha=expected_sha)
    assemble_release_lanes(
        root=root, version=version, tested_sha=source.tested_sha, release_dir=release_dir
    )
    notes_path = write_release_notes(source=source, version=version, release_dir=release_dir)
    return RehearsalResult(
        tested_sha=source.tested_sha,
        previous_tag=source.previous_tag,
        notes_path=notes_path,
        checksums_path=release_dir / f"asc_{version}_checksums.txt",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="Candidate release version in x.y.z form")
    parser.add_argument("--expected-sha", required=True, help="Commit the rehearsal must test")
    parser.add_argument("--release-dir", type=Path, default=Path("release"))
    args = parser.parse_args()

    try:
        result = run_release_rehearsal(
            root=Path.cwd(),
            version=args.version,
            expected_sha=args.expected_sha,
            release_dir=args.release_dir,
        )
    except RehearsalError as error:
        print(f"release rehearsal failed: {error}", file=sys.stderr)
        return 1

    print(f"Release candidate: {args.version}")
    print(f"Tested commit: {result.tested_sha}")
    print(f"Previous release: {result.previous_tag or 'none'}")
    print(f"Release notes preview: {result.notes_path}")
    print(f"Checksums: {result.checksums_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
