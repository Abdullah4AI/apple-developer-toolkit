#!/usr/bin/env python3
"""Exercise the intermediate-to-qualified release boundary with real tar files."""
import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('assembler', Path(__file__).with_name('assemble_release_candidate.py'))
assembler = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(assembler)
SHA = 'a' * 40
VERSION = '1.2.3'
MAC = [f'asc_{VERSION}_macOS_{arch}' for arch in ('amd64', 'arm64')]
PORTABLE = [f'asc_{VERSION}_linux_{arch}' for arch in ('amd64', 'arm64')] + [f'asc_{VERSION}_windows_amd64.exe']

class AssembleTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def archive(self, lane, names, sha=SHA, special=None):
        path = self.root / f'{lane}.tar'
        with tarfile.open(path, 'w') as archive:
            entries = [('commit', sha.encode())] + [('release/' + name, name.encode()) for name in names]
            for name, data in entries:
                info = tarfile.TarInfo(name)
                info.mode = 0o755 if name.startswith('release/') else 0o644
                info.size = len(data)
                archive.addfile(info, io.BytesIO(data))
            if special:
                archive.addfile(special)
        return path

    def assemble(self, mac, portable):
        return assembler.assemble(mac, portable, self.root / 'output', VERSION, SHA)

    def test_complete_join_preserves_bytes_modes_and_provenance(self):
        self.assemble(self.archive('mac', MAC), self.archive('portable', PORTABLE))
        output = self.root / 'output'
        self.assertEqual((output / 'commit').read_text().strip(), SHA)
        self.assertEqual(sorted(p.name for p in (output / 'release').iterdir()), sorted(MAC + PORTABLE + [f'asc_{VERSION}_checksums.txt']))
        for name in MAC + PORTABLE:
            asset = output / 'release' / name
            self.assertEqual(asset.read_bytes(), name.encode())
            self.assertEqual(asset.stat().st_mode & 0o777, 0o755)

    def test_rejects_missing_mixed_sha_extra_and_wrong_lane(self):
        cases = [('missing', MAC[:1], PORTABLE, SHA), ('mixed', MAC, PORTABLE, 'b' * 40), ('extra', MAC + ['unexpected'], PORTABLE, SHA), ('wrong-lane', MAC + PORTABLE[:1], PORTABLE[1:], SHA)]
        for name, mac_names, portable_names, portable_sha in cases:
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    self.assemble(self.archive('mac', mac_names), self.archive('portable', portable_names, portable_sha))
                self.assertFalse((self.root / 'output').exists())

    def test_rejects_malformed_tar_and_nonexecutable_assets(self):
        malformed = self.root / 'malformed.tar'
        malformed.write_bytes(b'not an archive')
        with self.assertRaises(ValueError):
            self.assemble(malformed, self.archive('portable', PORTABLE))
        mac = self.archive('mac', MAC)
        with tarfile.open(mac) as archive:
            entries = [(item, archive.extractfile(item).read()) for item in archive if item.isfile()]
        with tarfile.open(mac, 'w') as archive:
            for info, data in entries:
                info.mode = 0o644
                archive.addfile(info, io.BytesIO(data))
        with self.assertRaises(ValueError):
            self.assemble(mac, self.archive('portable', PORTABLE))
        self.assertFalse((self.root / 'output').exists())

    def test_existing_output_is_preserved(self):
        output = self.root / 'output'
        output.mkdir()
        marker = output / 'keep'
        marker.write_text('existing output')
        with self.assertRaises(ValueError):
            self.assemble(self.archive('mac', MAC), self.archive('portable', PORTABLE))
        self.assertEqual(marker.read_text(), 'existing output')

    def test_rejects_unsafe_tar_members_and_duplicate_provenance(self):
        for name, kind in [('release/link', tarfile.SYMTYPE), ('../escaped', tarfile.REGTYPE), ('commit', tarfile.REGTYPE)]:
            info = tarfile.TarInfo(name)
            info.type = kind
            info.linkname = '/etc/passwd'
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    self.assemble(self.archive('mac', MAC, special=info), self.archive('portable', PORTABLE))
                self.assertFalse((self.root / 'output').exists())

if __name__ == '__main__':
    unittest.main()
