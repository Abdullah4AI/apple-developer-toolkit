#!/usr/bin/env python3
"""Check repository and website commands with one freshly built CLI."""

import subprocess
import sys
import tempfile
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]


def main() -> int:
    # Never reuse a previously built executable: Go validates the current source
    # and build environment while reusing its own compilation cache.
    with tempfile.TemporaryDirectory(prefix="asc-docs-") as directory:
        binary = str(Path(directory) / "asc-doc-check")
        subprocess.run(["go", "build", "-o", binary, "."], cwd=REPO_ROOT, check=True)
        subprocess.run(
            [sys.executable, str(REPO_ROOT / "scripts/check-commands-docs.py"),
             "--check-generated", "--binary", binary],
            cwd=REPO_ROOT, check=True,
        )
        subprocess.run(
            [sys.executable, str(REPO_ROOT / "scripts/check_website_commands.py"),
             "--binary", binary],
            cwd=REPO_ROOT, check=True,
        )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode)
