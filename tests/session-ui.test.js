const test = require('node:test');
const assert = require('node:assert/strict');
const SessionUI = require('../js/session-ui.js');

test('builds sidebar identity from the authenticated user', () => {
  assert.deepEqual(
    SessionUI.viewModel({
      username: 'user001',
      displayName: '测试用户',
      role: 'user',
      type: 'user'
    }),
    {
      name: '测试用户',
      account: 'user001',
      role: 'user',
      avatar: '测'
    }
  );
});

test('formats live health services instead of a fixed success message', () => {
  assert.equal(
    SessionUI.healthMessage({
      postgres: {status: 'ok'},
      redis: {status: 'ok'},
      ldap: {status: 'error'},
      slurm: {status: 'ok'}
    }),
    'postgres: ok · redis: ok · ldap: error · slurm: ok'
  );
});
