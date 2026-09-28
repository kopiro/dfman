#!/usr/bin/env python3
"""Create reproducible-layout release ZIPs, preserving executable permissions."""
import hashlib
import pathlib
import sys
import zipfile
version = sys.argv[1]
root = pathlib.Path('dist')
for platform in sys.argv[2:]:
    source = root / platform
    target = root / f'dfman_{version}_{platform.replace("-", "_")}.zip'
    with zipfile.ZipFile(target, 'w', zipfile.ZIP_DEFLATED) as archive:
        for item in sorted(source.rglob('*')):
            if item.is_file():
                archive.write(item, item.relative_to(source))
with (root / 'checksums.txt').open('w') as checksums:
    for archive in sorted(root.glob(f'dfman_{version}_*')):
        if not archive.is_file():
            continue
        checksums.write(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
