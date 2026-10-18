// Publishes qu to an npm registry: one package per platform holding the binary from
// the release archives, and the quanarch package that picks the right one.
//
//   node npm/publish.mjs <version> <dist dir> <registry>    publish
//   node npm/publish.mjs <version> <dist dir> --pack <dir>  only write the .tgz files
//
// Publishing reads NEXUS_USERNAME and NEXUS_PASSWORD. A version already in the
// registry is skipped, so a release can be published again.

import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const [version, dist, ...rest] = process.argv.slice(2);
const packDir = rest[0] === '--pack' ? path.resolve(rest[1]) : null;
const registry = packDir ? null : rest[0];
if (!version || !dist || (!registry && !packDir)) {
  console.error('usage: node npm/publish.mjs <version> <dist dir> <registry> | --pack <dir>');
  process.exit(2);
}

// Go's names → npm's names, and the archive each binary comes from.
const TARGETS = [
  ['linux', 'amd64', 'linux', 'x64'],
  ['linux', 'arm64', 'linux', 'arm64'],
  ['darwin', 'amd64', 'darwin', 'x64'],
  ['darwin', 'arm64', 'darwin', 'arm64'],
  ['windows', 'amd64', 'win32', 'x64'],
  ['windows', 'arm64', 'win32', 'arm64'],
];

const work = fs.mkdtempSync(path.join(os.tmpdir(), 'quanarch-npm-'));
const npmrc = path.join(work, '.npmrc');

if (registry) {
  const { NEXUS_USERNAME: user, NEXUS_PASSWORD: pass } = process.env;
  if (!user || !pass) {
    console.error('NEXUS_USERNAME and NEXUS_PASSWORD must be set');
    process.exit(1);
  }
  const auth = Buffer.from(`${user}:${pass}`).toString('base64');
  fs.writeFileSync(npmrc, `${registry.replace(/^https?:/, '')}:_auth=${auth}\n`);
}

function published(name) {
  const r = spawnSync('npm', ['view', `${name}@${version}`, 'version', '--registry', registry, '--userconfig', npmrc], { encoding: 'utf8' });
  return r.status === 0 && r.stdout.trim() === version;
}

function release(dir, name) {
  if (packDir) {
    fs.mkdirSync(packDir, { recursive: true });
    execFileSync('npm', ['pack', '--pack-destination', packDir], { cwd: dir, stdio: 'inherit' });
    return;
  }
  if (published(name)) {
    console.log(`${name}@${version} is already published`);
    return;
  }
  execFileSync('npm', ['publish', '--registry', registry, '--userconfig', npmrc], { cwd: dir, stdio: 'inherit' });
}

const platformPackages = {};

for (const [goos, goarch, platform, cpu] of TARGETS) {
  const name = `@quanarch/qu-${platform}-${cpu}`;
  platformPackages[name] = version;
  const exe = goos === 'windows' ? 'qu.exe' : 'qu';
  const archive = path.join(dist, `quanarch_${version}_${goos}_${goarch}.${goos === 'windows' ? 'zip' : 'tar.gz'}`);
  const dir = path.join(work, `${platform}-${cpu}`);
  fs.mkdirSync(path.join(dir, 'bin'), { recursive: true });

  if (goos === 'windows') execFileSync('unzip', ['-o', '-q', archive, exe, '-d', path.join(dir, 'bin')]);
  else execFileSync('tar', ['-xzf', archive, '-C', path.join(dir, 'bin'), exe]);
  fs.chmodSync(path.join(dir, 'bin', exe), 0o755);

  fs.writeFileSync(
    path.join(dir, 'package.json'),
    JSON.stringify(
      {
        name,
        version,
        description: `The qu binary for ${platform} ${cpu}. Install quanarch instead.`,
        homepage: 'https://quanarch.org',
        repository: { type: 'git', url: 'git+https://github.com/quanarch/quanarch.git' },
        license: 'Apache-2.0',
        os: [platform],
        cpu: [cpu],
        files: ['bin'],
        preferUnplugged: true,
      },
      null,
      2,
    ) + '\n',
  );
  release(dir, name);
}

// The quanarch package itself, pinned to the platform packages of the same version.
const main = path.join(work, 'quanarch');
fs.cpSync(path.join(import.meta.dirname, 'quanarch'), main, { recursive: true });
const pkg = JSON.parse(fs.readFileSync(path.join(main, 'package.json'), 'utf8'));
pkg.version = version;
pkg.optionalDependencies = platformPackages;
fs.writeFileSync(path.join(main, 'package.json'), JSON.stringify(pkg, null, 2) + '\n');
release(main, 'quanarch');

fs.rmSync(work, { recursive: true, force: true });
