"""Prepare a pinned, architecture-specific ONNX CPU runtime for Go embedding."""
import argparse
import hashlib
import json
import os
import platform
from pathlib import Path
import shutil
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def native_target():
    system = {'Darwin': 'darwin', 'Linux': 'linux'}.get(platform.system())
    arch = {'x86_64': 'amd64', 'AMD64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}.get(platform.machine())
    if not system or not arch:
        raise RuntimeError('This script supports native macOS/Linux x64 and ARM64 builds.')
    return system + '-' + arch


def sha256(path):
    with Path(path).open('rb') as source:
        digest = hashlib.file_digest(source, 'sha256') if hasattr(hashlib, 'file_digest') else None
        if digest is not None:
            return digest.hexdigest()
        digest = hashlib.sha256()
        while block := source.read(1024 * 1024):
            digest.update(block)
        return digest.hexdigest()


def download(pin, cache):
    cache.mkdir(parents=True, exist_ok=True)
    path = cache / pin['url'].rsplit('/', 1)[1]
    if path.exists() and sha256(path) == pin['sha256']:
        return path
    partial = path.with_name(path.name + '.part')
    request = urllib.request.Request(pin['url'], headers={'User-Agent': 'SmartNote-build'})
    with urllib.request.urlopen(request, timeout=90) as response, partial.open('wb') as output:
        shutil.copyfileobj(response, output)
    if sha256(partial) != pin['sha256']:
        raise RuntimeError('Downloaded archive failed SHA-256 verification: ' + path.name)
    if 'size' in pin and partial.stat().st_size != pin['size']:
        raise RuntimeError('Downloaded archive has unexpected size: ' + path.name)
    os.replace(partial, path)
    return path


def prepare(target):
    pins = json.loads((ROOT / 'build/native-runtimes.json').read_text(encoding='utf-8'))
    pin = pins['platforms'][target]
    archive = download(pin, ROOT / '.tmp/native-downloads')
    destination = ROOT / 'internal/embedding/native'
    destination.mkdir(parents=True, exist_ok=True)
    # Only remove generated library names; preserve the tracked README.
    for name in ('libonnxruntime.so', 'libonnxruntime.dylib', 'libonnxruntime_providers_shared.so',
                 'ORT-NATIVE-LICENSE', 'ORT-NATIVE-NOTICES.txt', 'runtime.json'):
        (destination / name).unlink(missing_ok=True)
    entries = {}
    with tarfile.open(archive, 'r:gz') as source:
        for name, member_name in pin['files'].items():
            if Path(name).name != name or '/' in name or '\\' in name:
                raise RuntimeError('Invalid native resource name.')
            member = source.getmember(member_name)
            if not member.isfile():
                raise RuntimeError('Runtime archive member must be a regular file: ' + member_name)
            with source.extractfile(member) as member_file, (destination / name).open('wb') as output:
                shutil.copyfileobj(member_file, output)
            (destination / name).chmod(0o600)
            entries[name] = {'sha256': sha256(destination / name), 'size': (destination / name).stat().st_size}
    manifest = {'platform': target, 'runtime': 'ONNX Runtime CPU ' + pins['version'] + ' / ' + target, 'files': entries}
    (destination / 'runtime.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    print('Verified native runtime prepared for ' + target, flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--target', choices=['darwin-amd64', 'darwin-arm64', 'linux-amd64', 'linux-arm64'])
    args = parser.parse_args()
    prepare(args.target or native_target())
