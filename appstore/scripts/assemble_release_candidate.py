#!/usr/bin/env python3
"""Join exact-source, platform-specific intermediates into qualified release assets."""
import argparse
import hashlib
from pathlib import Path
import re
import tarfile


def read_lane(path, expected_names, expected_sha):
    entries = {}
    try:
        with tarfile.open(path) as archive:
            for member in archive:
                if member.name == 'release' and member.isdir():
                    continue
                if not member.isfile() or member.name in entries:
                    raise ValueError(f'{path}: nonregular or duplicate member {member.name}')
                if member.name not in {'commit', *('release/' + name for name in expected_names)}:
                    raise ValueError(f'{path}: unexpected member {member.name}')
                if member.mode & ~0o777 or (member.name != 'commit' and not member.mode & 0o111):
                    raise ValueError(f'{path}: invalid executable mode for {member.name}')
                data = archive.extractfile(member).read()
                if not data:
                    raise ValueError(f'{path}: empty member {member.name}')
                entries[member.name] = (data, member.mode)
    except (tarfile.TarError, OSError) as error:
        raise ValueError(f'{path}: unreadable archive: {error}') from error
    expected = {'commit', *('release/' + name for name in expected_names)}
    if set(entries) != expected:
        raise ValueError(f'{path}: incomplete platform asset set')
    if entries['commit'][0].strip() != expected_sha.encode():
        raise ValueError(f'{path}: source commit does not match {expected_sha}')
    return {name: entries['release/' + name] for name in expected_names}


def assemble(macos, portable, output, version, expected_sha):
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', version):
        raise ValueError('version must match x.y.z')
    if not re.fullmatch(r'[0-9a-f]{40}', expected_sha):
        raise ValueError('expected source must be a full commit SHA')
    output = Path(output)
    if output.exists() or output.is_symlink():
        raise ValueError('output must not already exist')
    mac_names = [f'asc_{version}_macOS_{arch}' for arch in ('amd64', 'arm64')]
    portable_names = [f'asc_{version}_linux_{arch}' for arch in ('amd64', 'arm64')] + [f'asc_{version}_windows_amd64.exe']
    assets = read_lane(macos, mac_names, expected_sha)
    assets.update(read_lane(portable, portable_names, expected_sha))
    # Do not create any output until both tar files pass the provenance and shape checks.
    release = output / 'release'
    release.mkdir(parents=True)
    lines = []
    for name in sorted(assets):
        data, mode = assets[name]
        target = release / name
        target.write_bytes(data)
        target.chmod(mode)
        lines.append(f'{hashlib.sha256(data).hexdigest()}  {name}')
    (release / f'asc_{version}_checksums.txt').write_text('\n'.join(lines) + '\n')
    (output / 'commit').write_text(expected_sha + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--macos', required=True, type=Path)
    parser.add_argument('--portable', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--version', required=True)
    parser.add_argument('--expected-sha', required=True)
    args = parser.parse_args()
    assemble(args.macos, args.portable, args.output, args.version, args.expected_sha)


if __name__ == '__main__':
    main()
