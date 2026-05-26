#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync, spawn } = require('child_process');

const REPO = 'FREEZONEX/Tier0-cli';
const BIN_DIR = path.join(require('os').homedir(), '.tier0', 'bin');
const VERSION_FILE = path.join(BIN_DIR, '.version');

// 版本直接取自 package.json，与 Go Release 强制同步（参考 Lark CLI 做法）
// 不再调 GitHub API 取 latest，避免 GitHub Release 滞后导致安装旧版本
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

function findSkillDir(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'skill' && fs.existsSync(path.join(fullPath, 'SKILL.md'))) {
        return fullPath;
      }
      const found = findSkillDir(fullPath);
      if (found) return found;
    }
  }
  return null;
}

function copyDirSync(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const srcPath = path.join(src, entry.name);
    const dstPath = path.join(dst, entry.name);
    if (entry.isDirectory()) {
      copyDirSync(srcPath, dstPath);
    } else {
      fs.copyFileSync(srcPath, dstPath);
    }
  }
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

function extractTarGz(tarPath, destDir) {
  if (process.platform === 'win32') {
    // Windows: use PowerShell Expand-Archive (zip)
    execSync(`powershell -Command "Expand-Archive -Path '${tarPath}' -DestinationPath '${destDir}' -Force"`, { stdio: 'inherit' });
  } else {
    execSync(`tar -xzf "${tarPath}" -C "${destDir}"`, { stdio: 'inherit' });
  }
}

async function install({ force = false } = {}) {
  const plat = platformName();
  const version = VERSION;  // always use npm package version — in sync with Go release
  const binPath = path.join(BIN_DIR, binaryName());

  // Check if already installed and up to date (skip when force=true)
  if (!force && fs.existsSync(binPath) && fs.existsSync(VERSION_FILE)) {
    const current = fs.readFileSync(VERSION_FILE, 'utf8').trim();
    if (current === version) {
      console.log(`tier0 ${version} already installed.`);
      return;
    }
  }

  console.log(`Installing tier0 ${version} for ${plat}...`);

  if (!fs.existsSync(BIN_DIR)) {
    fs.mkdirSync(BIN_DIR, { recursive: true });
  }

  const suffix = process.platform === 'win32' ? 'zip' : 'tar.gz';
  const pkgName = `tier0-cli-${version}-${plat}.${suffix}`;
  const downloadUrl = `https://github.com/${REPO}/releases/download/${version}/${pkgName}`;
  const tmpDir = require('os').tmpdir();
  const tmpFile = path.join(tmpDir, pkgName);

  try {
    console.log(`Downloading ${pkgName}...`);
    await downloadFile(downloadUrl, tmpFile);

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

    fs.copyFileSync(foundBinary, binPath);
    if (process.platform !== 'win32') {
      fs.chmodSync(binPath, 0o755);
    }
    fs.writeFileSync(VERSION_FILE, version);

    // 同步安装 skills 文档
    const foundSkillDir = findSkillDir(extractDir);
    if (foundSkillDir) {
      const skillsDest = path.join(require('os').homedir(), '.tier0', 'skills');
      if (fs.existsSync(skillsDest)) {
        fs.rmSync(skillsDest, { recursive: true, force: true });
      }
      copyDirSync(foundSkillDir, skillsDest);
    }

    // Cleanup
    fs.unlinkSync(tmpFile);
    fs.rmSync(extractDir, { recursive: true, force: true });

    console.log(`✓ tier0 ${version} installed to ${binPath}`);
  } catch (err) {
    console.error(`Installation failed: ${err.message}`);
    process.exit(1);
  }

  // Install Cursor/Claude agent skills
  await installSkills();
}

async function installSkills() {
  console.log('\nInstalling Tier0 agent skills...');
  try {
    execSync('npx --yes skills add FREEZONEX/Tier0-skill', { stdio: 'inherit' });
    console.log('✓ Tier0 agent skills installed.');
  } catch (err) {
    console.warn(`⚠ Skills installation failed (non-fatal): ${err.message}`);
    console.warn('  You can install manually: npx skills add FREEZONEX/Tier0-skill');
  }
}

// Run install if called directly
if (require.main === module) {
  install().catch(err => {
    console.error(err);
    process.exit(1);
  });
}

module.exports = { install, BIN_DIR, binaryName };
