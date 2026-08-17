#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');
const { execFileSync, execSync } = require('child_process');

const REPO = 'FREEZONEX/Tier0-cli';
const BIN_DIR = path.join(require('os').homedir(), '.tier0', 'bin');
const VERSION_FILE = path.join(BIN_DIR, '.version');
const SKILLS_DIR = path.join(require('os').homedir(), '.tier0', 'skills');

// Read the version directly from package.json and keep it in sync with the Go release, following the Lark CLI approach.
// Do not call the GitHub API for latest; this avoids stale GitHub Release metadata installing an old version.
const VERSION = `v${require('../package.json').version}`;

function platformName() {
  const os = process.platform;
  const arch = process.arch;
  const map = {
    'linux-x64': 'Linux-x86_64',
    'linux-arm64': 'Linux-aarch64',
    'darwin-x64': 'macOS-x86_64',
    'darwin-arm64': 'macOS-arm64',
    'win32-x64': 'Windows-x86_64',
    'win32-arm64': 'Windows-arm64',
  };
  const key = `${os}-${arch}`;
  if (!map[key]) {
    console.error(`Unsupported platform: ${os} ${arch}`);
    process.exit(1);
  }
  return map[key];
}

function binaryName() {
  return process.platform === 'win32' ? 'tier0.exe' : 'tier0';
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, { timeout: 300000 }, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        file.close();
        fs.unlinkSync(dest);
        return downloadFile(res.headers.location, dest).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        file.close();
        fs.unlinkSync(dest);
        reject(new Error(`Download failed: ${res.statusCode}`));
        return;
      }
      res.pipe(file);
      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      file.close();
      if (fs.existsSync(dest)) fs.unlinkSync(dest);
      reject(err);
    });
  });
}

/** Download text content, used for sha256sums.txt. */
function fetchText(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { timeout: 30000 }, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        return fetchText(res.headers.location).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`HTTP ${res.statusCode} fetching ${url}`));
      }
      let data = '';
      res.on('data', chunk => { data += chunk; });
      res.on('end', () => resolve(data));
    }).on('error', reject);
  });
}

/**
 * Find the expected hash for filename in sha256sum(1) text.
 * Format: <hash>  <filename>, or <hash> *<filename> in binary mode.
 */
function parseChecksum(sumsText, filename) {
  for (const rawLine of sumsText.split('\n')) {
    const line = rawLine.trim();
    if (!line) continue;
    const parts = line.split(/\s+/);
    if (parts.length < 2) continue;
    const name = parts[1].replace(/^\*/, '');  // strip leading * (binary mode)
    if (name === filename || path.basename(name) === filename) {
      return parts[0].toLowerCase();
    }
  }
  return null;
}

/** Compute the SHA256 hex digest for a local file. */
function sha256File(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    const stream = fs.createReadStream(filePath);
    stream.on('error', reject);
    stream.on('data', chunk => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
  });
}

/**
 * Download sha256sums.txt and verify the local file.
 * Reject on verification failure; the caller decides whether to stop installation.
 */
async function verifyChecksum(version, pkgName, localPath) {
  const sumsUrl = `https://github.com/${REPO}/releases/download/${version}/sha256sums.txt`;
  let sumsText;
  try {
    sumsText = await fetchText(sumsUrl);
  } catch (err) {
    throw new Error(`failed to download sha256sums.txt: ${err.message}`);
  }

  const expected = parseChecksum(sumsText, pkgName);
  if (!expected) {
    throw new Error(`checksum for "${pkgName}" not found in sha256sums.txt`);
  }

  const actual = await sha256File(localPath);
  if (actual !== expected) {
    throw new Error(
      `SHA256 verification failed; the file may be corrupted or tampered with:\n  expected: ${expected}\n  actual: ${actual}`
    );
  }
}

function extractTarGz(tarPath, destDir) {
  if (process.platform === 'win32') {
    // Windows: use PowerShell Expand-Archive (zip)
    execSync(`powershell -Command "Expand-Archive -Path '${tarPath}' -DestinationPath '${destDir}' -Force"`, { stdio: 'inherit' });
  } else {
    execSync(`tar -xzf "${tarPath}" -C "${destDir}"`, { stdio: 'inherit' });
  }
}

function runBestEffort(command, args) {
  try {
    execFileSync(command, args, { stdio: 'ignore' });
  } catch (_) {
    // Best effort only. A missing tool or absent attribute should not block install.
  }
}

function runCommand(command, args, options) {
  if (process.platform === 'win32') {
    return execSync([command, ...args].join(' '), options);
  }
  return execFileSync(command, args, options);
}

function buildSkillsInstallArgs() {
  return ['-y', '--package=skills', '--', 'skills', 'add', '.', '-y', '-g', '--copy'];
}

function isNpxPostinstall(env = process.env) {
  return env.npm_command === 'exec' && env.TIER0_CLI_RUN !== '1';
}

function fixMacOSSigning(binaryPath) {
  if (process.platform !== 'darwin') {
    return;
  }
  runBestEffort('xattr', ['-d', 'com.apple.quarantine', binaryPath]);
  runBestEffort('codesign', ['-f', '-s', '-', binaryPath]);
}

function verifyInstalledBinary(binaryPath, expectedVersion) {
  let output;
  try {
    output = execFileSync(binaryPath, ['version'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      timeout: 10000,
    });
  } catch (err) {
    throw new Error(`installed binary is not executable: ${err.message}`);
  }

  const actual = String(output).trim().split(/\s+/).pop().replace(/^v/, '');
  const expected = expectedVersion.replace(/^v/, '');
  if (actual !== expected) {
    throw new Error(`installed binary version mismatch: expected ${expectedVersion}, got ${actual || '(empty)'}`);
  }
}

function replaceInstalledBinary(sourcePath, binaryPath, {
  platform = process.platform,
  fileSystem = fs,
  processId = process.pid,
  timestamp = Date.now(),
} = {}) {
  if (platform !== 'win32') {
    fileSystem.copyFileSync(sourcePath, binaryPath);
    fileSystem.chmodSync(binaryPath, 0o755);
    return;
  }

  // Windows does not allow copyFile to overwrite a running executable. Stage
  // the new binary beside the destination, rename the running binary out of
  // the way, then atomically activate the staged binary. Cleanup of the old
  // executable is best-effort because Windows may keep it locked until the
  // upgrading process exits.
  const suffix = `${processId}-${timestamp}`;
  const binaryDir = path.dirname(binaryPath);
  const binaryBase = path.basename(binaryPath);
  const stagedPath = path.join(binaryDir, `.${binaryBase}.new-${suffix}`);
  const backupPath = `${binaryPath}.old-${suffix}`;
  let movedExisting = false;

  try {
    // Remove backups left locked by earlier upgrade processes once they are no
    // longer in use. A dedicated versioned backup uses a different name and is
    // intentionally preserved.
    for (const entry of fileSystem.readdirSync(binaryDir)) {
      if (entry === `${binaryBase}.old` || entry.startsWith(`${binaryBase}.old-`)) {
        try {
          fileSystem.rmSync(path.join(binaryDir, entry), { force: true });
        } catch (_) {
          // A still-running prior process can keep its own backup locked.
        }
      }
    }
    fileSystem.copyFileSync(sourcePath, stagedPath);
    if (fileSystem.existsSync(binaryPath)) {
      fileSystem.renameSync(binaryPath, backupPath);
      movedExisting = true;
    }
    fileSystem.renameSync(stagedPath, binaryPath);
  } catch (err) {
    try {
      if (movedExisting && !fileSystem.existsSync(binaryPath) && fileSystem.existsSync(backupPath)) {
        fileSystem.renameSync(backupPath, binaryPath);
      }
    } catch (rollbackErr) {
      err.message += `; rollback failed: ${rollbackErr.message}`;
    }
    try {
      fileSystem.rmSync(stagedPath, { force: true });
    } catch (_) {
      // Best-effort cleanup after a failed replacement.
    }
    throw err;
  }

  if (movedExisting) {
    try {
      fileSystem.rmSync(backupPath, { force: true });
    } catch (_) {
      // The old running executable can remain locked until this process exits.
    }
  }
}

function materializeEmbeddedSkills({
  binaryPath,
  runner = execFileSync,
  skillsDir = SKILLS_DIR,
} = {}) {
  console.log('\nPreparing the Tier0 Skill embedded in the CLI...');
  try {
    runner(binaryPath, ['skills', 'install', '--no-sync', '--json'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      timeout: 30000,
    });
  } catch (err) {
    throw new Error(`embedded Tier0 Skill installation failed: ${err.message}`);
  }
  if (!fs.existsSync(path.join(skillsDir, 'SKILL.md'))) {
    throw new Error(`embedded Tier0 Skill did not create ${skillsDir}`);
  }
  console.log('Tier0 Skill baseline is ready.');
}

function printAuthNextStep() {
  console.log('\nNext: verify authentication with `tier0 auth whoami --json`.');
  console.log('If authentication is missing, run `tier0 login --no-wait --json` and open the returned verification_url.');
}

async function install({
  force = false,
  requireSkills = false,
  installAgentSkills = installSkills,
  prepareLocalSkills = materializeEmbeddedSkills,
  binDir = BIN_DIR,
  versionFile = VERSION_FILE,
} = {}) {
  const plat = platformName();
  const version = VERSION;  // always use npm package version — in sync with Go release
  const binPath = path.join(binDir, binaryName());

  // Check if already installed and up to date (skip when force=true)
  if (!force && fs.existsSync(binPath) && fs.existsSync(versionFile)) {
    const current = fs.readFileSync(versionFile, 'utf8').trim();
    if (current === version) {
      console.log(`tier0 ${version} already installed.`);
      await prepareLocalSkills({ binaryPath: binPath });
      await installAgentSkills({ required: requireSkills });
      printAuthNextStep();
      return;
    }
  }

  console.log(`Installing tier0 ${version} for ${plat}...`);

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  const suffix = process.platform === 'win32' ? 'zip' : 'tar.gz';
  const pkgName = `tier0-cli-${version}-${plat}.${suffix}`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/${version}/${pkgName}`;
  const tmpDir = require('os').tmpdir();
  const tmpFile = path.join(tmpDir, pkgName);

  try {
    console.log(`Downloading ${pkgName}...`);
    await downloadFile(downloadUrl, tmpFile);

    // SHA256 verification protects against corrupted downloads or MITM tampering.
    console.log('Verifying checksum...');
    await verifyChecksum(version, pkgName, tmpFile);
    console.log('✓ Checksum verified.');

    console.log('Extracting...');
    const extractDir = path.join(tmpDir, `tier0-extract-${Date.now()}`);
    fs.mkdirSync(extractDir, { recursive: true });

    if (suffix === 'zip') {
      execSync(`powershell -Command "Expand-Archive -Path '${tmpFile}' -DestinationPath '${extractDir}' -Force"`, { stdio: 'pipe' });
    } else {
      execSync(`tar -xzf "${tmpFile}" -C "${extractDir}"`, { stdio: 'pipe' });
    }

    // Find binary
    const findBinary = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const fullPath = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          const found = findBinary(fullPath);
          if (found) return found;
        } else if (entry.name === binaryName()) {
          return fullPath;
        }
      }
      return null;
    };

    const foundBinary = findBinary(extractDir);
    if (!foundBinary) {
      throw new Error('Binary not found in package');
    }

    replaceInstalledBinary(foundBinary, binPath);
    fixMacOSSigning(binPath);
    verifyInstalledBinary(binPath, version);
    fs.writeFileSync(versionFile, version);

    // Cleanup
    fs.unlinkSync(tmpFile);
    fs.rmSync(extractDir, { recursive: true, force: true });

    console.log(`✓ tier0 ${version} installed to ${binPath}`);
  } catch (err) {
    console.error(`Installation failed: ${err.message}`);
    process.exit(1);
  }

  // Materialize the baseline from the verified binary. Release archives no
  // longer need to carry a duplicate Skill package.
  await prepareLocalSkills({ binaryPath: binPath });

  // Install agent skills globally for detected tools such as Codex and Claude.
  await installAgentSkills({ required: requireSkills });
  printAuthNextStep();
}

async function installSkills({
  required = false,
  runner = runCommand,
  skillsDir = SKILLS_DIR,
} = {}) {
  console.log('\nInstalling Tier0 agent skills...');
  try {
    if (!fs.existsSync(path.join(skillsDir, 'SKILL.md'))) {
      throw new Error(`local Skill package not found: ${skillsDir}`);
    }
    runner('npx', buildSkillsInstallArgs(), { stdio: 'inherit', cwd: skillsDir });
    console.log('✓ Tier0 agent skills installed.');
  } catch (err) {
    if (required) {
      throw new Error(`Tier0 agent skills installation failed: ${err.message}`);
    }
    console.warn(`⚠ Skills installation failed (non-fatal): ${err.message}`);
    console.warn(`  You can install manually: npx -y skills add "${skillsDir}" -y -g --copy`);
    return false;
  }
}

// Run install if called directly
if (require.main === module) {
  // Match the Lark CLI pattern: under npx, let bin/tier0.js run the explicit
  // installer once instead of downloading the binary twice in postinstall.
  if (isNpxPostinstall()) {
    process.exit(0);
  } else {
    install().catch(err => {
      console.error(err);
      process.exit(1);
    });
  }
}

module.exports = {
  install,
  installSkills,
  materializeEmbeddedSkills,
  buildSkillsInstallArgs,
  isNpxPostinstall,
  BIN_DIR,
  VERSION_FILE,
  SKILLS_DIR,
  VERSION,
  binaryName,
  replaceInstalledBinary,
};
