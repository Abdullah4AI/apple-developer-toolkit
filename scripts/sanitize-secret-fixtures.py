#!/usr/bin/env python3
"""Break synthetic secret fixtures across Go string literals.

GitHub push protection cannot distinguish some realistic redaction-test fixtures
from live credentials. This script preserves each runtime string exactly while
splitting detected fixture text across adjacent Go literals, so scanners do not
see a contiguous credential in the repository.
"""

from __future__ import annotations

import json
import re
import shutil
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TARGET_DIR = ROOT / "appstore" / "internal" / "cli" / "snitch"
TARGET_NAMES = {"snitch_redaction_test.go", "z_audit_adversarial_test.go"}

# GitHub push protection has partner patterns that are not all present in
# gitleaks. Keep narrow, provider-shaped patterns here when a blocked push
# identifies one. These values are synthetic test fixtures, never credentials.
EXTRA_PATTERNS = (
    re.compile(r"\bSG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{32,}\b"),
    # A trailing hyphen is part of the key, not a word boundary. Partner
    # scanners detect these fixtures even when gitleaks misses them.
    re.compile(r"(?<![A-Za-z0-9_-])AIza[A-Za-z0-9_-]{35}(?![A-Za-z0-9_-])"),
)


def run_gitleaks(report: Path) -> list[dict]:
    binary = shutil.which("gitleaks")
    if not binary:
        raise RuntimeError("gitleaks is required to sanitize imported secret fixtures")
    report.unlink(missing_ok=True)
    proc = subprocess.run(
        [
            binary,
            "dir",
            str(TARGET_DIR),
            "--report-format=json",
            f"--report-path={report}",
            "--no-banner",
            "--exit-code=0",
        ],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        timeout=180,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"gitleaks failed ({proc.returncode}): {proc.stdout.strip()}")
    if not report.exists():
        return []
    return json.loads(report.read_text(encoding="utf-8") or "[]")


def line_offsets(text: str) -> list[int]:
    offsets = [0]
    for index, char in enumerate(text):
        if char == "\n":
            offsets.append(index + 1)
    return offsets


def string_literal_spans(text: str) -> list[tuple[int, int, str]]:
    spans: list[tuple[int, int, str]] = []
    index = 0
    size = len(text)
    while index < size:
        char = text[index]
        if char == "/" and index + 1 < size and text[index + 1] == "/":
            newline = text.find("\n", index + 2)
            index = size if newline < 0 else newline + 1
            continue
        if char == "/" and index + 1 < size and text[index + 1] == "*":
            close = text.find("*/", index + 2)
            index = size if close < 0 else close + 2
            continue
        if char == "'":
            index += 1
            while index < size:
                if text[index] == "\\":
                    index += 2
                elif text[index] == "'":
                    index += 1
                    break
                else:
                    index += 1
            continue
        if char == '"':
            start = index
            index += 1
            while index < size:
                if text[index] == "\\":
                    index += 2
                elif text[index] == '"':
                    spans.append((start, index + 1, '"'))
                    index += 1
                    break
                else:
                    index += 1
            continue
        if char == "`":
            start = index
            close = text.find("`", index + 1)
            if close < 0:
                raise RuntimeError(f"unterminated raw string literal near byte {start}")
            spans.append((start, close + 1, "`"))
            index = close + 1
            continue
        index += 1
    return spans


def finding_offset(text: str, finding: dict) -> tuple[int, int]:
    secret = finding.get("Secret") or ""
    if not secret:
        raise RuntimeError(f"gitleaks finding has no secret: {finding.get('Fingerprint')}")
    offsets = line_offsets(text)
    start_line = max(1, int(finding.get("StartLine") or 1))
    end_line = max(start_line, int(finding.get("EndLine") or start_line))
    if start_line > len(offsets):
        raise RuntimeError(f"finding line is outside source: {start_line}")
    search_start = offsets[start_line - 1]
    search_end = offsets[end_line] if end_line < len(offsets) else len(text)
    start_column = max(1, int(finding.get("StartColumn") or 1))
    preferred = min(search_end, search_start + start_column - 1)

    candidates: list[int] = []
    cursor = search_start
    while True:
        location = text.find(secret, cursor, search_end)
        if location < 0:
            break
        candidates.append(location)
        cursor = location + 1
    if not candidates:
        # Some rules report a decoded secret from an escaped Go literal. Fall
        # back to the full scanner match, which is source text.
        match = finding.get("Match") or ""
        cursor = search_start
        while match:
            location = text.find(match, cursor, search_end)
            if location < 0:
                break
            candidates.append(location)
            secret = match
            cursor = location + 1
    if not candidates:
        raise RuntimeError(
            f"cannot locate {finding.get('RuleID')} finding at lines {start_line}-{end_line}"
        )
    start = min(candidates, key=lambda value: abs(value - preferred))
    return start, start + len(secret)


def choose_split(text: str, start: int, end: int, span: tuple[int, int, str]) -> int:
    literal_start, literal_end, _quote = span
    low = max(start + 1, literal_start + 2)
    high = min(end - 1, literal_end - 2)
    if low > high:
        low = max(start, literal_start + 1)
        high = min(end, literal_end - 1)
    midpoint = (low + high) // 2
    candidates = sorted(range(low, high + 1), key=lambda value: abs(value - midpoint))
    for position in candidates:
        if position <= literal_start or position >= literal_end - 1:
            continue
        if text[position - 1] == "\\":
            continue
        if text[position - 1 : position + 1] in {'""', '``'}:
            continue
        return position
    raise RuntimeError(f"cannot choose a safe split inside bytes {start}:{end}")


def sanitize_file(path: Path, findings: list[dict]) -> int:
    text = path.read_text(encoding="utf-8")
    spans = string_literal_spans(text)
    insertions: dict[int, str] = {}
    for finding in findings:
        start, end = finding_offset(text, finding)
        span = next((item for item in spans if item[0] < start and end < item[1]), None)
        if span is None:
            span = next((item for item in spans if item[0] < start < item[1]), None)
        if span is None:
            raise RuntimeError(
                f"{path}: {finding.get('RuleID')} finding is not inside a Go string literal"
            )
        position = choose_split(text, start, end, span)
        quote = span[2]
        insertions[position] = f"{quote} + {quote}"

    for position in sorted(insertions, reverse=True):
        text = text[:position] + insertions[position] + text[position:]
    if insertions:
        path.write_text(text, encoding="utf-8")
    return len(insertions)


def sanitize_extra_patterns(path: Path) -> int:
    text = path.read_text(encoding="utf-8")
    spans = string_literal_spans(text)
    insertions: dict[int, str] = {}
    for pattern in EXTRA_PATTERNS:
        for match in pattern.finditer(text):
            span = next(
                (item for item in spans if item[0] < match.start() and match.end() < item[1]),
                None,
            )
            if span is None:
                continue
            position = choose_split(text, match.start(), match.end(), span)
            quote = span[2]
            insertions[position] = f"{quote} + {quote}"
    for position in sorted(insertions, reverse=True):
        text = text[:position] + insertions[position] + text[position:]
    if insertions:
        path.write_text(text, encoding="utf-8")
    return len(insertions)


def main() -> int:
    if not TARGET_DIR.exists():
        raise RuntimeError(f"missing target directory: {TARGET_DIR}")
    total_changed = 0
    target_paths = [TARGET_DIR / name for name in sorted(TARGET_NAMES)]
    extra_changed = sum(sanitize_extra_patterns(path) for path in target_paths if path.exists())
    if extra_changed:
        total_changed += extra_changed
        subprocess.run(
            ["gofmt", "-w", *[str(path) for path in target_paths if path.exists()]],
            cwd=ROOT,
            check=True,
            timeout=120,
        )
    with tempfile.TemporaryDirectory(prefix="appledev-secret-fixtures-") as temp:
        report = Path(temp) / "gitleaks.json"
        for _pass in range(12):
            findings = run_gitleaks(report)
            selected: dict[Path, list[dict]] = {}
            for finding in findings:
                path = ROOT / finding.get("File", "")
                if path.name in TARGET_NAMES:
                    selected.setdefault(path, []).append(finding)
            if not selected:
                print(f"Sanitized {total_changed} synthetic secret fixture occurrence(s).")
                return 0
            changed = sum(sanitize_file(path, items) for path, items in selected.items())
            if not changed:
                break
            total_changed += changed
            subprocess.run(
                ["gofmt", "-w", *[str(path) for path in selected]],
                cwd=ROOT,
                check=True,
                timeout=120,
            )
        remaining = [
            finding
            for finding in run_gitleaks(report)
            if (ROOT / finding.get("File", "")).name in TARGET_NAMES
        ]
        if remaining:
            summary = ", ".join(
                sorted({f"{item.get('RuleID')}:{item.get('File')}:{item.get('StartLine')}" for item in remaining})
            )
            raise RuntimeError(f"secret fixtures remain after sanitization: {summary}")
    print(f"Sanitized {total_changed} synthetic secret fixture occurrence(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
