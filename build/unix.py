"""Native macOS/Linux build, offline inference check and release packaging."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tarfile
import time

spec = importlib.util.spec_from_file_location('prepare_runtime', Path(__file__).with_name('prepare-runtime.py'))
runtime = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runtime)
ROOT = runtime.ROOT
OUT = ROOT / 'build/bin'


def run(*args, env=None):
    print('+ ' + ' '.join(str(x) for x in args), flush=True)
    subprocess.run([str(x) for x in args], cwd=ROOT, env=env, check=True)


def write(path, text, executable=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding='utf-8')
    path.chmod(0o755 if executable else 0o644)


def clean(path):
    if not path.resolve().is_relative_to((ROOT / '.tmp').resolve()):
        raise RuntimeError('Scratch directory must stay inside .tmp.')
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True)


def licenses(destination):
    destination.mkdir(parents=True, exist_ok=True)
    names = ['MODEL-LICENSE', 'MODEL-README.md', 'ORT-LICENSE', 'ORT-ThirdPartyNotices.txt', 'GO-ORT-LICENSE']
    for name in names:
        shutil.copy2(ROOT / 'internal/embedding/assets' / name, destination / name)
    for name in ['ORT-NATIVE-LICENSE', 'ORT-NATIVE-NOTICES.txt']:
        shutil.copy2(ROOT / 'internal/embedding/native' / name, destination / name)
    shutil.copy2(ROOT / 'build/tools/APPIMAGE-RUNTIME-LICENSE', destination / 'APPIMAGE-RUNTIME-LICENSE')
    write(destination / 'THIRD-PARTY-NOTICES.txt',
          'SmartNote uses BAAI/bge-small-zh-v1.5 (MIT), ONNX Runtime CPU 1.22.0 (MIT), '
          'and onnxruntime_go (MIT). See the accompanying licenses.\n'
          'Linux AppImage runtime source: https://github.com/AppImage/type2-runtime/tree/dd6cebedcbddde9c82f89b011e8e1d40b6e43868\n'
          'Native dependencies and installation: https://github.com/amuer2011/SmartNote/blob/main/docs/platforms.md\n')


def diagnostic(binary, directory, env):
    run(binary, '--self-test-embedding', directory, env=env)
    result = json.loads((directory / 'embedding-self-test.json').read_text())
    if not result.get('ok') or result.get('dimensions') != 512:
        raise RuntimeError('Packaged offline CPU inference failed.')


def startup(binary, directory, env):
    env = {**env, 'SMARTNOTE_DATA_DIR': str(directory)}
    log = directory.parent / 'gui-startup.log'
    with log.open('wb') as output:
        process = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
        try:
            time.sleep(8)
            if process.poll() is not None:
                raise RuntimeError('Desktop startup failed: ' + log.read_text(errors='replace')[-5000:])
            print('PASS isolated desktop startup: ' + str(binary), flush=True)
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


def macos(binary, work, version, arch, env):
    app = work / 'SmartNote.app'
    contents = app / 'Contents'
    (contents / 'MacOS').mkdir(parents=True)
    (contents / 'Resources').mkdir()
    packaged = contents / 'MacOS/SmartNote'
    shutil.copy2(binary, packaged)
    info = {'CFBundleName': 'SmartNote', 'CFBundleDisplayName': 'SmartNote', 'CFBundleIdentifier': 'com.amuer2011.smartnote',
            'CFBundleExecutable': 'SmartNote', 'CFBundlePackageType': 'APPL', 'CFBundleShortVersionString': version,
            'CFBundleVersion': version, 'CFBundleIconFile': 'SmartNote.icns', 'LSMinimumSystemVersion': '13.3',
            'NSHighResolutionCapable': True}
    (contents / 'Info.plist').write_bytes(plistlib.dumps(info))
    icons = work / 'SmartNote.iconset'
    icons.mkdir()
    for size in (16, 32, 128, 256, 512):
        for scale in (1, 2):
            name = 'icon_' + str(size) + 'x' + str(size) + ('@2x' if scale == 2 else '') + '.png'
            run('sips', '-z', size * scale, size * scale, ROOT / 'build/windows/icon.png', '--out', icons / name)
    run('iconutil', '-c', 'icns', icons, '-o', contents / 'Resources/SmartNote.icns')
    licenses(contents / 'Resources/licenses')
    run('codesign', '--force', '--deep', '--sign', '-', app)
    run('codesign', '--verify', '--deep', '--strict', app)
    diagnostic(packaged, work / 'packaged-inference', env)
    startup(packaged, work / 'isolated-profile', env)
    base = 'SmartNote-v' + version + '-macos-' + arch
    run('ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', app, OUT / (base + '.zip'))
    dmg = work / 'dmg'
    dmg.mkdir()
    shutil.copytree(app, dmg / app.name, symlinks=True)
    (dmg / 'Applications').symlink_to('/Applications')
    run('hdiutil', 'create', '-volname', 'SmartNote', '-srcfolder', dmg, '-ov', '-format', 'UDZO', OUT / (base + '.dmg'))


def linux(binary, work, version, arch, goarch, env):
    base = 'SmartNote-v' + version + '-linux-' + arch
    portable = work / base
    portable.mkdir()
    shutil.copy2(binary, portable / 'SmartNote')
    licenses(portable / 'licenses')
    shutil.copy2(ROOT / 'docs/platforms.md', portable / 'README.md')
    with tarfile.open(OUT / (base + '.tar.gz'), 'w:gz') as archive:
        archive.add(portable, arcname=base)

    payload = work / 'payload'
    install = payload / 'opt/smartnote'
    shutil.copytree(portable, install)
    write(payload / 'usr/bin/smartnote', '#!/bin/sh\nexec /opt/smartnote/SmartNote "$@"\n', executable=True)
    desktop = ('[Desktop Entry]\nType=Application\nName=SmartNote\nComment=Local Markdown knowledge base\n'
               'Exec=smartnote\nIcon=smartnote\nTerminal=false\nCategories=Office;\n')
    write(payload / 'usr/share/applications/smartnote.desktop', desktop)
    icon = payload / 'usr/share/icons/hicolor/256x256/apps/smartnote.png'
    icon.parent.mkdir(parents=True)
    shutil.copy2(ROOT / 'build/windows/icon.png', icon)
    run('desktop-file-validate', payload / 'usr/share/applications/smartnote.desktop')

    deb = work / 'deb'
    shutil.copytree(payload, deb)
    write(deb / 'DEBIAN/control',
          'Package: smartnote\nVersion: ' + version + '\nArchitecture: ' + goarch + '\n'
          'Maintainer: SmartNote <753507905@qq.com>\nSection: editors\nPriority: optional\n'
          'Depends: libc6 (>= 2.35), libgtk-3-0, libwebkit2gtk-4.1-0, libstdc++6, libgcc-s1\n'
          'Homepage: https://github.com/amuer2011/SmartNote\n'
          'Description: Local Markdown notes with offline semantic search\n')
    run('dpkg-deb', '--build', '--root-owner-group', deb, OUT / (base + '.deb'))
    extracted = work / 'deb-extracted'
    run('dpkg-deb', '--extract', OUT / (base + '.deb'), extracted)
    diagnostic(extracted / 'opt/smartnote/SmartNote', work / 'deb-inference', env)

    rpm = work / 'rpmbuild'
    for directory in ['BUILD', 'BUILDROOT', 'SOURCES', 'SPECS', 'RPMS', 'SRPMS']:
        (rpm / directory).mkdir(parents=True)
    rpm_arch = 'x86_64' if goarch == 'amd64' else 'aarch64'
    write(rpm / 'SPECS/smartnote.spec',
          'Name: smartnote\nVersion: ' + version + '\nRelease: 1\nSummary: Local Markdown knowledge base\n'
          'License: LicenseRef-SmartNote-and-ThirdParty\nURL: https://github.com/amuer2011/SmartNote\n'
          'BuildArch: ' + rpm_arch + '\nRequires: libgtk-3.so.0()(64bit), libwebkit2gtk-4.1.so.0()(64bit)\n'
          'Requires: libstdc++.so.6()(64bit), libgcc_s.so.1()(64bit)\n'
          '%global debug_package %{nil}\n%global __os_install_post %{nil}\n'
          '%description\nLocal Markdown notes with offline semantic search.\n'
          '%install\nmkdir -p %{buildroot}\ncp -a "' + str(payload) + '/." %{buildroot}/\n'
          '%files\n/opt/smartnote\n/usr/bin/smartnote\n/usr/share/applications/smartnote.desktop\n'
          '/usr/share/icons/hicolor/256x256/apps/smartnote.png\n')
    run('rpmbuild', '--define', '_topdir ' + str(rpm), '-bb', rpm / 'SPECS/smartnote.spec')
    packages = list((rpm / 'RPMS').rglob('*.rpm'))
    if len(packages) != 1:
        raise RuntimeError('Expected exactly one RPM package.')
    shutil.copy2(packages[0], OUT / (base + '.rpm'))
    rpm_extracted = work / 'rpm-extracted'
    rpm_extracted.mkdir()
    subprocess.run(['bash', '-o', 'pipefail', '-c', 'rpm2cpio "$1" | cpio -idm --quiet', '--', str(OUT / (base + '.rpm'))],
                   cwd=rpm_extracted, check=True)
    diagnostic(rpm_extracted / 'opt/smartnote/SmartNote', work / 'rpm-inference', env)

    appdir = work / 'SmartNote.AppDir'
    (appdir / 'usr/bin').mkdir(parents=True)
    shutil.copy2(binary, appdir / 'usr/bin/SmartNote')
    licenses(appdir / 'usr/share/doc/smartnote/licenses')
    write(appdir / 'smartnote.desktop', desktop.replace('Exec=smartnote', 'Exec=SmartNote'))
    shutil.copy2(icon, appdir / 'smartnote.png')
    (appdir / '.DirIcon').symlink_to('smartnote.png')
    write(appdir / 'AppRun', '#!/bin/sh\nAPPDIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)\nexec "$APPDIR/usr/bin/SmartNote" "$@"\n', executable=True)
    pins = json.loads((ROOT / 'build/appimage-runtimes.json').read_text())
    app_runtime = runtime.download(pins['platforms'][goarch], ROOT / '.tmp/appimage-downloads')
    squash = work / 'application.squashfs'
    run('mksquashfs', appdir, squash, '-noappend', '-all-root', '-no-xattrs', '-comp', 'zstd', '-processors', '2')
    image = OUT / (base + '.AppImage')
    with image.open('wb') as output:
        for part in (app_runtime, squash):
            with part.open('rb') as source:
                shutil.copyfileobj(source, output)
    image.chmod(0o755)
    run(image, '--appimage-extract-and-run', '--self-test-embedding', work / 'appimage-inference', env=env)
    result = json.loads((work / 'appimage-inference/embedding-self-test.json').read_text())
    if not result.get('ok'):
        raise RuntimeError('AppImage offline inference failed.')
    startup(binary, work / 'isolated-profile', env)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--skip-install', action='store_true')
    args = parser.parse_args()
    os.chdir(ROOT)
    target = runtime.native_target()
    system, goarch = target.split('-')
    arch = 'x64' if goarch == 'amd64' else 'arm64'
    version = json.loads((ROOT / 'frontend/package.json').read_text())['version']
    if not re.fullmatch(r'\d+\.\d+\.\d+', version):
        raise RuntimeError('Invalid package version.')
    if os.environ.get('GITHUB_REF_TYPE') == 'tag' and os.environ['GITHUB_REF_NAME'] != 'v' + version:
        raise RuntimeError('Release tag and package version differ.')
    if not (ROOT / ('docs/release-v' + version + '.md')).is_file():
        raise RuntimeError('Release notes are missing.')
    runtime.prepare(target)
    env = {**os.environ, 'CGO_ENABLED': '1'}
    if system == 'darwin':
        env['MACOSX_DEPLOYMENT_TARGET'] = '13.3'
        # Wails' native file dialogs use UTType on current macOS SDKs.
        env['CGO_LDFLAGS'] = (env.get('CGO_LDFLAGS', '') + ' -framework UniformTypeIdentifiers').strip()
    if not args.skip_install:
        run('npm', 'ci', '--prefix', 'frontend', env=env)
    run('npm', 'run', 'build', '--prefix', 'frontend', env=env)
    tags = 'desktop,production' + (',webkit2_41' if system == 'linux' else '')
    run('go', 'test', '-tags', 'webkit2_41' if system == 'linux' else 'desktop', './...', env=env)
    work = ROOT / '.tmp/packages' / target
    clean(work)
    OUT.mkdir(parents=True, exist_ok=True)
    binary = work / 'SmartNote'
    run('go', 'build', '-tags', tags, '-ldflags', '-s -w', '-o', binary, '.', env=env)
    diagnostic(binary, work / 'binary-inference', env)
    if system == 'darwin':
        macos(binary, work, version, arch, env)
    else:
        linux(binary, work, version, arch, goarch, env)
    files = sorted(OUT.glob('SmartNote-v' + version + '-' + ('macos' if system == 'darwin' else system) + '-' + arch + '.*'))
    expected = 2 if system == 'darwin' else 4
    if len(files) != expected:
        raise RuntimeError('Unexpected platform package count: ' + str(len(files)))
    write(OUT / ('SHA256SUMS-' + target + '.txt'), ''.join(runtime.sha256(p) + '  ' + p.name + '\n' for p in files))
    print('PASS native build, offline inference, packaging and startup: ' + target, flush=True)


if __name__ == '__main__':
    main()
