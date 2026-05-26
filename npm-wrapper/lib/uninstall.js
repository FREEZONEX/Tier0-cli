#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');
const { execSync } = require('child_process');

const TIER0_DIR = path.join(os.homedir(), '.tier0');
const BIN_DIR = path.join(TIER0_DIR, 'bin');
const SKILLS_DIR = path.join(TIER0_DIR, 'skills');
const CONFIG_FILE = path.join(TIER0_DIR, 'config.json');

function binaryName() {
  return process.platform === 'win32' ? 'tier0.exe' : 'tier0';
}

function removeDir(dir, label) {
  if (fs.existsSync(dir)) {
    fs.rmSync(dir, { recursive: true, force: true });
    console.log(`✓ Removed ${label}: ${dir}`);
    return true;
  }
  return false;
}

function removeFile(file, label) {
  if (fs.existsSync(file)) {
    fs.unlinkSync(file);
    console.log(`✓ Removed ${label}: ${file}`);
    return true;
  }
  return false;
}

function uninstallAgentSkills() {
  console.log('\nRemoving Tier0 agent skills...');
  try {
    execSync('npx --yes skills remove FREEZONEX/Tier0-skill', { stdio: 'inherit' });
    console.log('✓ Tier0 agent skills removed.');
  } catch (err) {
    console.warn(`⚠ Agent skills removal failed (non-fatal): ${err.message}`);
    console.warn('  You can remove manually: npx skills remove FREEZONEX/Tier0-skill');
  }
}

/**
 * @param {object} opts
 * @param {boolean} [opts.purge=false]  - also delete config.json (credentials)
 * @param {boolean} [opts.keepSkills=false] - skip agent skills removal
 */
async function uninstall({ purge = false, keepSkills = false } = {}) {
  console.log('Uninstalling tier0 CLI...\n');

  let removed = 0;

  // Remove binary
  const binPath = path.join(BIN_DIR, binaryName());
  const versionFile = path.join(BIN_DIR, '.version');
  if (removeFile(binPath, 'binary')) removed++;
  removeFile(versionFile, 'version record');

  // Remove binary dir if empty
  if (fs.existsSync(BIN_DIR)) {
    const remaining = fs.readdirSync(BIN_DIR);
    if (remaining.length === 0) {
      fs.rmdirSync(BIN_DIR);
    }
  }

  // Remove bundled skills docs (~/.tier0/skills/)
  if (removeDir(SKILLS_DIR, 'bundled skills')) removed++;

  // Remove config if --purge
  if (purge) {
    if (removeFile(CONFIG_FILE, 'config (credentials)')) removed++;
    // Remove the ~/.tier0 dir itself if now empty
    if (fs.existsSync(TIER0_DIR) && fs.readdirSync(TIER0_DIR).length === 0) {
      fs.rmdirSync(TIER0_DIR);
      console.log(`✓ Removed ${TIER0_DIR}`);
    }
  } else {
    console.log(`\n  Config kept: ${CONFIG_FILE}`);
    console.log('  Run with --purge to also remove credentials.');
  }

  // Remove agent skills
  if (!keepSkills) {
    uninstallAgentSkills();
  }

  if (removed === 0) {
    console.log('\ntier0 CLI was not installed (nothing to remove).');
  } else {
    console.log('\ntier0 CLI uninstalled successfully.');
  }
}

// Parse CLI flags when called directly
if (require.main === module) {
  const args = process.argv.slice(2);
  const purge = args.includes('--purge');
  const keepSkills = args.includes('--keep-skills');

  // Skip when invoked by npm/yarn uninstall in CI/non-interactive environments
  // (TIER0_SKIP_UNINSTALL=1 lets automated tooling bypass the hook)
  if (process.env.TIER0_SKIP_UNINSTALL === '1') {
    process.exit(0);
  }

  uninstall({ purge, keepSkills }).catch(err => {
    console.error(err);
    process.exit(1);
  });
}

module.exports = { uninstall };
