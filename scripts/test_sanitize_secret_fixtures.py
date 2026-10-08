#!/usr/bin/env python3
"""Regression tests for synthetic fixture sanitization (no real credentials)."""
import importlib.util
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "sanitizer", Path(__file__).with_name("sanitize-secret-fixtures.py")
)
assert spec is not None and spec.loader is not None
sanitizer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sanitizer)


class GoogleFixtureTests(unittest.TestCase):
    def test_partner_patterns_preserve_runtime_values(self):
        for suffix in ("a", "-", "_"):
            with self.subTest(suffix=suffix), tempfile.TemporaryDirectory() as tmp:
                fixture = "AIza" + "0123456789abcdefghijklmnopqrstuvwx" + suffix
                self.assertEqual(len(fixture), 39)
                path = Path(tmp) / "fixture_test.go"
                original = 'package test\nvar value = "' + fixture + '"\n'
                path.write_text(original)
                self.assertEqual(sanitizer.sanitize_extra_patterns(path), 1)
                result = path.read_text()
                self.assertNotIn(fixture, result)
                self.assertEqual(result.replace('" + "', ""), original)
                self.assertEqual(sanitizer.sanitize_extra_patterns(path), 0)

    def test_comments_and_non_key_lengths_are_unchanged(self):
        fixture = "AIza" + "a" * 35
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "fixture_test.go"
            original = '// ' + fixture + '\nvar short = "AIza' + 'a' * 34 + '"\nvar long = "' + fixture + 'a"\n'
            path.write_text(original)
            self.assertEqual(sanitizer.sanitize_extra_patterns(path), 0)
            self.assertEqual(path.read_text(), original)


if __name__ == "__main__":
    unittest.main()
