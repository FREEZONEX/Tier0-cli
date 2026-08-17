const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const {
  buildSkillsRemoveArgs,
  uninstallAgentSkills,
  uninstallGlobalNpmPackage,
} = require('./uninstall');

test('removes the installed global skill name non-interactively', () => {
  assert.deepEqual(buildSkillsRemoveArgs(), [
    '-y',
    '--package=skills',
    '--',
    'skills',
    'remove',
    'tier0',
    '-y',
    '-g',
  ]);
});

test('verifies that the Agent Skill was actually removed', (t) => {
  const skillDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-agent-skill-'));
  t.after(() => fs.rmSync(skillDir, { recursive: true, force: true }));

  let call;
  const removed = uninstallAgentSkills({
    skillDir,
    runner(command, args) {
      call = { command, args };
      fs.rmSync(skillDir, { recursive: true, force: true });
    },
  });

  assert.equal(removed, true);
  assert.deepEqual(call, { command: 'npx', args: buildSkillsRemoveArgs() });
});

test('rejects a false successful Agent Skill removal', (t) => {
  const skillDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-agent-skill-'));
  t.after(() => fs.rmSync(skillDir, { recursive: true, force: true }));

  assert.throws(
    () => uninstallAgentSkills({ skillDir, runner() {} }),
    /reported success/,
  );
});

test('removes the global npm wrapper without recursive lifecycle cleanup', (t) => {
  const npmRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'tier0-npm-root-'));
  const packageDir = path.join(npmRoot, '@tier0', 'cli');
  fs.mkdirSync(packageDir, { recursive: true });
  fs.writeFileSync(path.join(packageDir, 'package.json'), '{}');
  t.after(() => fs.rmSync(npmRoot, { recursive: true, force: true }));

  const calls = [];
  const removed = uninstallGlobalNpmPackage({
    runner(command, args, options) {
      calls.push({ command, args, options });
      if (args[0] === 'root') return npmRoot;
      fs.rmSync(packageDir, { recursive: true, force: true });
      return '';
    },
  });

  assert.equal(removed, true);
  assert.deepEqual(calls[1].args, ['uninstall', '-g', '@tier0/cli']);
  assert.equal(calls[1].options.env.TIER0_SKIP_UNINSTALL, '1');
});

