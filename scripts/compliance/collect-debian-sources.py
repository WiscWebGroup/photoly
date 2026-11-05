#!/usr/bin/env python3
"""Collect exact installed sources in a disposable image; never touch a live service."""
import concurrent.futures
import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import time
import urllib.parse
import urllib.request

OUT = pathlib.Path('/compliance-sources')
CACHE = pathlib.Path('/source-cache')


def fetch(url):
    for attempt in range(3):
        try:
            request = urllib.request.Request(url, headers={'User-Agent': 'Photoly-source-archive'})
            with urllib.request.urlopen(request, timeout=60) as response:
                return response.read()
        except Exception:
            if attempt == 2:
                raise
            time.sleep(attempt + 1)


def checksums(control):
    match = re.search(r'^Checksums-Sha256:\n((?: .+\n)+)', control, re.M)
    if not match:
        raise RuntimeError('Missing source SHA256 records')
    result = []
    for line in match[1].splitlines():
        digest, size, filename = line.split()
        if pathlib.Path(filename).name != filename:
            raise RuntimeError('Invalid source filename')
        result.append((digest, int(size), filename))
    return result


def verify(directory):
    controls = list(directory.glob('*.dsc'))
    if len(controls) != 1:
        raise RuntimeError('Missing or ambiguous source control file')
    for digest, size, filename in checksums(controls[0].read_text()):
        content = (directory / filename).read_bytes()
        if len(content) != size or hashlib.sha256(content).hexdigest() != digest:
            raise RuntimeError('Source checksum mismatch: ' + filename)


def collect(pair):
    name, version = pair
    folder = name + '_' + version.replace(':', '_')
    destination = CACHE / folder
    destination.mkdir(parents=True, exist_ok=True)
    provenance = destination / '_retrieval.json'
    print('Collecting ' + name + '=' + version, flush=True)
    try:
        verify(destination)
        method = json.loads(provenance.read_text()) if provenance.exists() else 'Exact cached source package, reverified against its .dsc SHA256 records'
    except (OSError, ValueError, RuntimeError):
        result = subprocess.run(
            ['apt-get', 'source', '--download-only', '-o', 'Acquire::Retries=3', name + '=' + version],
            cwd=destination, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        )
        method = 'APT signed source index for ' + name + '=' + version
        if result.returncode and (name == 'nginx' or name.startswith('nginx-module-')):
            # Official nginx images remove the nginx.org APT source list after building.
            pool = 'https://nginx.org/packages/mainline/debian/pool/nginx/n/' + name + '/'
            filename = name + '_' + version.split(':')[-1] + '.dsc'
            control = fetch(pool + filename)
            (destination / filename).write_bytes(control)
            for digest, size, filename in checksums(control.decode()):
                content = fetch(pool + filename)
                if len(content) != size or hashlib.sha256(content).hexdigest() != digest:
                    raise RuntimeError('nginx source checksum mismatch: ' + filename)
                (destination / filename).write_bytes(content)
            method = pool
        elif result.returncode:
            # Debian snapshot retains exact old versions after mirror rotation.
            base = 'https://snapshot.debian.org/mr/package/' + urllib.parse.quote(name, safe='') + '/' + urllib.parse.quote(version, safe='') + '/srcfiles'
            print('Using exact-version snapshot for ' + name + '=' + version, flush=True)
            hashes = [item['hash'] for item in json.loads(fetch(base))['result']]
            if not hashes:
                raise RuntimeError('No exact source available')
            for digest in hashes:
                info = json.loads(fetch('https://snapshot.debian.org/mr/file/' + digest + '/info'))['result']
                filename = info[0]['name']
                if pathlib.Path(filename).name != filename:
                    raise RuntimeError('Invalid snapshot filename')
                content = fetch('https://snapshot.debian.org/file/' + digest)
                if hashlib.sha1(content).hexdigest() != digest:
                    raise RuntimeError('Snapshot content-address mismatch')
                (destination / filename).write_bytes(content)
            method = base
        verify(destination)
        provenance.write_text(json.dumps(method) + '\n')
    shutil.copytree(destination, OUT / folder, dirs_exist_ok=True)
    return {
        'package': name, 'version': version, 'retrieval': method,
        'files': [
            {'file': folder + '/' + path.name,
             'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
             'bytes': path.stat().st_size}
            for path in sorted(destination.iterdir()) if path.is_file()
        ],
    }


def checked(pair):
    try:
        return collect(pair)
    except Exception as error:
        raise RuntimeError('Could not collect %s=%s: %s' % (*pair, error)) from error


def main():
    OUT.mkdir(exist_ok=True)
    # Captured by the Dockerfile before installing any collection tools.
    raw = pathlib.Path('/tmp/runtime-packages.tsv').read_text()
    (OUT / 'system-packages.tsv').write_text(raw)
    for path in pathlib.Path('/etc/apt/sources.list.d').glob('*.sources'):
        path.write_text(re.sub(r'^Types: deb$', 'Types: deb deb-src', path.read_text(), flags=re.M))
    for path in [pathlib.Path('/etc/apt/sources.list'), *pathlib.Path('/etc/apt/sources.list.d').glob('*.list')]:
        if path.exists():
            text = path.read_text()
            path.write_text(text + '\n' + '\n'.join(re.sub(r'^deb ', 'deb-src ', line) for line in text.splitlines() if line.startswith('deb ')) + '\n')
    subprocess.run(['apt-get', 'update'], check=True)
    subprocess.run(['apt-get', 'install', '-y', '--no-install-recommends', 'dpkg-dev', 'ca-certificates', 'xz-utils'], check=True)
    packages = set()
    for row in raw.splitlines():
        binary, version, source, source_version = row.split('\t')
        packages.add((source or binary.split(':')[0], source_version or version))
    # Download-only source operations do not change the package database.
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        manifest = list(pool.map(checked, sorted(packages)))
    (OUT / 'sources.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (OUT / 'COMPLETE').write_text('All original installed source packages retrieved and checked against their .dsc SHA256 records.\n')


if __name__ == '__main__':
    main()
