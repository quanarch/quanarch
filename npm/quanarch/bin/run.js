'use strict';

// Runs the qu binary of this machine's platform package. npm installs only the
// optional dependency whose os and cpu match, so there is nothing to download here.

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const PLATFORMS = {
  'linux x64': '@quanarch/qu-linux-x64',
  'linux arm64': '@quanarch/qu-linux-arm64',
  'darwin x64': '@quanarch/qu-darwin-x64',
  'darwin arm64': '@quanarch/qu-darwin-arm64',
  'win32 x64': '@quanarch/qu-win32-x64',
  'win32 arm64': '@quanarch/qu-win32-arm64',
};

function binary() {
  const name = PLATFORMS[`${process.platform} ${process.arch}`];
  if (!name) {
    throw new Error(`qu has no build for ${process.platform} ${process.arch}`);
  }
  const exe = process.platform === 'win32' ? 'qu.exe' : 'qu';
  try {
    return path.join(path.dirname(require.resolve(`${name}/package.json`)), 'bin', exe);
  } catch {
    throw new Error(`${name} is not installed. Reinstall quanarch without --no-optional / --omit=optional.`);
  }
}

module.exports = function run(programName) {
  let file;
  try {
    file = binary();
  } catch (error) {
    console.error(`quanarch: ${error.message}`);
    process.exit(1);
  }
  // argv0 keeps the name the user typed, so help and errors say qu or quanarch.
  const result = spawnSync(file, process.argv.slice(2), { stdio: 'inherit', argv0: programName });
  if (result.error) {
    console.error(`quanarch: ${result.error.message}`);
    process.exit(1);
  }
  if (result.signal) {
    process.kill(process.pid, result.signal);
    return;
  }
  process.exit(result.status ?? 1);
};
