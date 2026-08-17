#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync, execSync } = require('child_process');

const TIER0_DIR = path.join(os.homedir(), '.tier0');
const BIN_DIR = path.join(TIER0_DIR, 'bin');
const SKILLS_DIR = path.join(TIER0_DIR, 'skills');
const CONFIG_FILE = path.join(TIER0_DIR, 'config.json');
const AGENT_SKILL_DIR = path.join(os.homedir(), '.agents', 'skills', 'tier0');
const NPM_PACKAGE = '@tier0/cli';

function binaryName() {
  return process.platform === 'win32' ? 'tier0.exe' : 'tier0';
}

function runCommand(command, args, options) {
  if (process.platform === 'win32') {
    return execSync([command, ...args].join(' '), options);
  }
  return execFileSync(command, args, options);
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

function buildSkillsRemoveArgs() {
  return ['-y', '--package=skills', '--', 'skills', 'remove', 'tier0', '-y', '-g'];
}

function uninstallAgentSkills({ runner = runCommand, skillDir = AGENT_SKILL_DIR } = {}) {
  if (!fs.existsSync(skillDir)) {
    console.log('  Tier0 Agent Skill is not installed.');
    return false;
  }

  console.log('\nRemoving Tier0 agent skills...');
  try {
    runner('npx', buildSkillsRemoveArgs(), { stdio: 'inherit' });
    if (fs.existsSync(skillDir)) {
      throw new Error(`skills remove reported success but ${skillDir} still exists`);
    }
    console.log('✓ Tier0 agent skills removed.');
    return true;
  } catch (err) {
    throw new Error(
      `Agent Skills removal failed: ${err.message}\n` +
      'Run manually: npx -y --package=skills -- skills remove tier0 -y -g'
    );
  }
}

function uninstallGlobalNpmPackage({ runner = runCommand } = {}) {
  let npmRoot;
  try {
    npmRoot = String(runner('npm', ['root', '-g'], {
      stdio: ['ignore', 'pipe', 'pipe'],
      encoding: 'utf8',
    })).trim();
  } catch (_) {
    return false;
  }

  const packageFile = path.join(npmRoot, '@tier0', 'cli', 'package.json');
  if (!fs.existsSync(packageFile)) return false;

  runner('npm', ['uninstall', '-g', NPM_PACKAGE], {
    stdio: 'inherit',
    env: { ...process.env, TIER0_SKIP_UNINSTALL: '1' },
  });
  if (fs.existsSync(packageFile)) {
    throw new Error(`npm reported success but ${path.dirname(packageFile)} still exists`);
  }
  console.log(`✓ Removed global npm package: ${NPM_PACKAGE}`);
  return true;
}

/**
 * @param {object} opts
 * @param {boolean} [opts.purge=false]  - also delete config.json (credentials)
 * @param {boolean} [opts.removeSkills=false] - also remove agent skills
 * @param {boolean} [opts.removeNpmPackage=true] - remove a global npm wrapper
 */
async function uninstall({
  purge = false,
  removeSkills = false,
  removeNpmPackage = true,
} = {}) {
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

  // Agent Skills have their own lifecycle and are preserved by default.
  if (removeSkills) {
    uninstallAgentSkills();
  } else {
    console.log('\n  Agent Skill kept. Use --remove-skills to delete it.');
  }

  // Explicit npx/CLI uninstall should also remove a wrapper left by an npm
  // upgrade. The npm preuninstall lifecycle disables this to avoid recursion.
  if (removeNpmPackage && uninstallGlobalNpmPackage()) {
    removed++;
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
  const removeSkills = args.includes('--remove-skills');

  // Skip when invoked by npm/yarn uninstall in CI/non-interactive environments
  // (TIER0_SKIP_UNINSTALL=1 lets automated tooling bypass the hook)
  if (process.env.TIER0_SKIP_UNINSTALL === '1') {
    process.exit(0);
  }

  uninstall({
    purge,
    removeSkills,
    removeNpmPackage: process.env.npm_lifecycle_event !== 'preuninstall',
  }).catch(err => {
    console.error(err);
    process.exit(1);
  });
}

module.exports = {
  buildSkillsRemoveArgs,
  uninstall,
  uninstallAgentSkills,
  uninstallGlobalNpmPackage,
};
