const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const html = fs.readFileSync('teams.html', 'utf8');

test('team creation uses the two-step group leader workflow', () => {
  assert.match(html, /新建用户组向导/);
  assert.match(html, /创建组长首用户/);
  assert.match(html, /account\/teams\/create-with-leader/);
  assert.match(html, /leaderUsername:value\('wizardLeaderUsername'\)/);
  assert.match(html, /wizardLeaderTeamLocked/);
  assert.match(html, /组长 \/ team_admin/);
  assert.match(html, /创建用户组和组长/);
  assert.match(html, /组存储目录授权/);
  assert.match(html, /资源策略默认绑定/);
  assert.match(html, /storageGrants:storageGrants\(\)/);
  assert.match(html, /\/api\/v1\/storage\/roots/);
  assert.match(html, /<select id="wizardUnit">/);
  assert.match(html, /<select id="wizardPolicy">/);
  assert.match(html, /\/api\/v1\/account\/units/);
  assert.match(html, /\/api\/v1\/slurm\/qos/);
  assert.doesNotMatch(html, /可选，例如 ogsp/);
  assert.doesNotMatch(html, /确定并创建组长/);
  assert.doesNotMatch(html, /确定并创建用户组/);
});

test('team edit and member creation legacy endpoints remain available', () => {
  assert.match(html, /account\/teams\/'\+encodeURIComponent\(teamName\)/);
  assert.match(html, /account\/users/);
});

test('team creation backend rejects orphan team creation endpoint', () => {
  const router = fs.readFileSync('backend/internal/httpapi/router.go', 'utf8');
  assert.match(router, /新建用户组必须同时创建组长首用户/);
  assert.match(router, /account\/teams\/create-with-leader/);
});
