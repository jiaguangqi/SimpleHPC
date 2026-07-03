const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const appSource = fs.readFileSync(path.join(__dirname, '..', 'js', 'app.js'), 'utf8');
const authSource = fs.readFileSync(path.join(__dirname, '..', 'backend', 'internal', 'httpapi', 'auth.go'), 'utf8');

test('user dropdown opens current profile modal instead of users management page', () => {
  assert.match(appSource, /openProfileModal/);
  assert.match(appSource, /App\.openProfileModal\(\)/);
  assert.doesNotMatch(appSource, /href="users\.html">个人资料/);
});

test('auth me response includes current account profile payload', () => {
  assert.match(authSource, /"profile": profile/);
});
