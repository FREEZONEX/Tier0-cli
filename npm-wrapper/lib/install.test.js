const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const {
  VERSION,
  binaryName,
  buildSkillsInstallArgs,
  install,
  installSkills,
  isNpxPostinstall,
  materializeEmbeddedSkills,
  replaceInstalledBinary,
} = require('./install');

test('builds a local, copied, global, non-interactive skills install command', () => {
  assert.deepEqual(buildSkillsInstallArgs(), [
    '-y',
    '--package=skills',
    '--',
    'skills',
    'add',
    '.',
    '-y',
    '-g',
    '--copy',
  ]);
});

test('runs the skills installer from the local Skill package directory', async (t) => {
  const skillsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0 skills '));
  t.after(() => fs.rmSync(skillsDir, { recursive: true, force: true }));
  fs.writeFileSync(path.join(skillsDir, 'SKILL.md'), '---\nname: tier0\ndescription: test\n---\n');

  let call;
  await installSkills({
    required: true,
    skillsDir,
    runner: (command, args, options) => {
      call = { command, args, options };
    },
  });

  assert.equal(call.command, 'npx');
  assert.deepEqual(call.args, buildSkillsInstallArgs());
  assert.equal(call.options.cwd, skillsDir);
});

test('skips npm postinstall under npx so the explicit installer runs once', () => {
  assert.equal(isNpxPostinstall({ npm_command: 'exec' }), true);
  assert.equal(isNpxPostinstall({ npm_command: 'install' }), false);
  assert.equal(isNpxPostinstall({ npm_command: 'exec', TIER0_CLI_RUN: '1' }), false);
});

test('installs agent skills even when the CLI binary is already current', async (t) => {
  const binDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-install-test-'));
  t.after(() => fs.rmSync(binDir, { recursive: true, force: true }));

  const versionFile = path.join(binDir, '.version');
  fs.writeFileSync(path.join(binDir, binaryName()), 'already installed');
  fs.writeFileSync(versionFile, VERSION);

  let localCall;
  let agentCall;
  await install({
    binDir,
    versionFile,
    requireSkills: true,
    prepareLocalSkills: async (options) => {
      localCall = options;
    },
    installAgentSkills: async (options) => {
      agentCall = options;
    },
  });

  assert.equal(localCall.binaryPath, path.join(binDir, binaryName()));
  assert.deepEqual(agentCall, { required: true });
});

test('materializes the Skill from the installed binary without a release skill directory', () => {
  const skillsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-embedded-skill-'));
  fs.rmSync(skillsDir, { recursive: true, force: true });
  const parent = path.dirname(skillsDir);
  const binaryPath = path.join(parent, binaryName());

  let call;
  materializeEmbeddedSkills({
    binaryPath,
    skillsDir,
    runner: (command, args) => {
      call = { command, args };
      fs.mkdirSync(skillsDir, { recursive: true });
      fs.writeFileSync(path.join(skillsDir, 'SKILL.md'), 'embedded baseline');
    },
  });

  assert.equal(call.command, binaryPath);
  assert.deepEqual(call.args, ['skills', 'install', '--no-sync', '--json']);
  fs.rmSync(skillsDir, { recursive: true, force: true });
});

test('explicit install fails when required agent skills cannot be installed', async (t) => {
  const skillsDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-skills-test-'));
  t.after(() => fs.rmSync(skillsDir, { recursive: true, force: true }));
  fs.writeFileSync(path.join(skillsDir, 'SKILL.md'), '---\nname: tier0\ndescription: test\n---\n');

  await assert.rejects(
    installSkills({
      required: true,
      skillsDir,
      runner: () => {
        throw new Error('skills unavailable');
      },
    }),
    /Tier0 agent skills installation failed: skills unavailable/,
  );

});

test('atomically replaces a Windows binary through a same-directory staged file', (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-windows-replace-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));

  const sourcePath = path.join(dir, 'downloaded.exe');
  const binaryPath = path.join(dir, 'tier0.exe');
  const staleBackupPath = `${binaryPath}.old-previous`;
  fs.writeFileSync(sourcePath, 'new binary');
  fs.writeFileSync(binaryPath, 'old binary');
  fs.writeFileSync(staleBackupPath, 'stale backup');

  replaceInstalledBinary(sourcePath, binaryPath, {
    platform: 'win32',
    processId: 123,
    timestamp: 456,
  });

  assert.equal(fs.readFileSync(binaryPath, 'utf8'), 'new binary');
  assert.equal(fs.existsSync(`${binaryPath}.old-123-456`), false);
  assert.equal(fs.existsSync(path.join(dir, '.tier0.exe.new-123-456')), false);
  assert.equal(fs.existsSync(staleBackupPath), false);
});

test('rolls back the Windows binary when activation fails', (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-windows-rollback-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));

  const sourcePath = path.join(dir, 'downloaded.exe');
  const binaryPath = path.join(dir, 'tier0.exe');
  const stagedPath = path.join(dir, '.tier0.exe.new-123-456');
  fs.writeFileSync(sourcePath, 'new binary');
  fs.writeFileSync(binaryPath, 'old binary');

  const fileSystem = {
    ...fs,
    renameSync(from, to) {
      if (from === stagedPath && to === binaryPath) {
        const err = new Error('simulated activation failure');
        err.code = 'EBUSY';
        throw err;
      }
      fs.renameSync(from, to);
    },
  };

  assert.throws(
    () => replaceInstalledBinary(sourcePath, binaryPath, {
      platform: 'win32',
      fileSystem,
      processId: 123,
      timestamp: 456,
    }),
    /simulated activation failure/,
  );

  assert.equal(fs.readFileSync(binaryPath, 'utf8'), 'old binary');
  assert.equal(fs.existsSync(stagedPath), false);
  assert.equal(fs.existsSync(`${binaryPath}.old-123-456`), false);
});

test('does not fail after activation when the old Windows binary stays locked', (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-windows-locked-backup-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));

  const sourcePath = path.join(dir, 'downloaded.exe');
  const binaryPath = path.join(dir, 'tier0.exe');
  const backupPath = `${binaryPath}.old-123-456`;
  fs.writeFileSync(sourcePath, 'new binary');
  fs.writeFileSync(binaryPath, 'old binary');

  const fileSystem = {
    ...fs,
    rmSync(target, options) {
      if (target === backupPath) {
        const err = new Error('simulated locked backup');
        err.code = 'EBUSY';
        throw err;
      }
      fs.rmSync(target, options);
    },
  };

  replaceInstalledBinary(sourcePath, binaryPath, {
    platform: 'win32',
    fileSystem,
    processId: 123,
    timestamp: 456,
  });

  assert.equal(fs.readFileSync(binaryPath, 'utf8'), 'new binary');
  assert.equal(fs.readFileSync(backupPath, 'utf8'), 'old binary');
});
