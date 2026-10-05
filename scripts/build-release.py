#!/usr/bin/env python3
"""Build the six release archives and SHA-256 manifest."""
import hashlib
import os
import re
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import zipfile

root = Path(__file__).resolve().parent.parent
output = Path(sys.argv[1] if len(sys.argv) > 1 else root / 'dist').resolve()
output.mkdir(parents=True, exist_ok=True)
archives = []
# Reject an unconfigured toolchain rather than silently producing plain binaries.
version = subprocess.check_output(['garble', 'version'], text=True)
if 'v0.17.0' not in version:
    raise SystemExit('Install garble v0.17.0 before building releases.')
endpoints = re.findall(r'"https://([^"/]+)"', (root / 'library.go').read_text())
with tempfile.TemporaryDirectory(prefix='mylib-build-') as temporary:
    for platform in ('darwin', 'linux', 'windows'):
        for architecture in ('arm64', 'amd64'):
            name = 'mylib.exe' if platform == 'windows' else 'mylib'
            binary = Path(temporary) / name
            environment = os.environ | {
                'GOOS': platform, 'GOARCH': architecture, 'CGO_ENABLED': '0'
            }
            subprocess.run(
                ['garble', '-literals', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(binary), '.'],
                cwd=root, env=environment, check=True,
            )
            data = binary.read_bytes()
            if any(host.encode() in data for host in endpoints):
                raise SystemExit('A release binary contains an unprotected endpoint.')
            binary.chmod(0o755)
            extension = 'zip' if platform == 'windows' else 'tar.gz'
            archive = output / f'mylib_{platform}_{architecture}.{extension}'
            if platform == 'windows':
                with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as bundle:
                    bundle.write(binary, name)
            else:
                with tarfile.open(archive, 'w:gz') as bundle:
                    bundle.add(binary, arcname=name)
            archives.append(archive)
            print(archive.name, flush=True)
manifest = ''.join(
    f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n'
    for archive in sorted(archives)
)
(output / 'checksums.txt').write_text(manifest, encoding='utf-8')
