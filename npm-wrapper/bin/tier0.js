#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const { install, BIN_DIR, binaryName } = require('../lib/install');
const { uninstall } = require('../lib/uninstall');

async function main() {
  const args = process.argv.slice(2);
  const binPath = path.join(BIN_DIR, binaryName());

  // `npx @tier0/cli@latest install` — explicit install/upgrade command
  if (args[0] === 'install') {
    await install({ force: true, requireSkills: true });
    process.exit(0);
  }

  // `npx @tier0/cli@latest uninstall [--purge] [--keep-skills]`
  if (args[0] === 'uninstall') {
    const purge = args.includes('--purge');
    const keepSkills = args.includes('--keep-skills');
    await uninstall({ purge, keepSkills });
    process.exit(0);
  }

  // Check if binary exists, if not install it
  if (!fs.existsSync(binPath)) {
    console.log('tier0 CLI not found, installing...');
    await install({ requireSkills: true });
  }

  // Spawn the actual binary with all arguments
  const child = spawn(binPath, args, {
    stdio: 'inherit',
    shell: false,
  });

  child.on('exit', (code) => {
    process.exit(code ?? 0);
  });

  child.on('error', (err) => {
    console.error(`Failed to run tier0: ${err.message}`);
    process.exit(1);
  });
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
