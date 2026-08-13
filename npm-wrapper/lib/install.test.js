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
