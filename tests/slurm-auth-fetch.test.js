const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..');

test('shared app client exposes authenticated fetch helpers', () => {
  const source = fs.readFileSync(path.join(root, 'js', 'app.js'), 'utf8');
  assert.match(source, /function authHeaders\(/);
  assert.match(source, /function apiFetch\(/);
  assert.match(source, /authHeaders:\s*authHeaders/);
  assert.match(source, /apiFetch:\s*apiFetch/);
});

test('slurm pages use authenticated API fetches', () => {
  for (const file of ['queue-status.html', 'nodes.html', 'partitions.html', 'qos.html', 'job-list.html', 'slurm.html']) {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    assert.doesNotMatch(source, /fetch\('\/api\/v1\/(slurm|config\/slurm)/, `${file} still calls Slurm API without auth helper`);
    assert.match(source, /App\.apiFetch\(/, `${file} should use App.apiFetch for API calls`);
  }
});

test('slurm config page exposes primary standby and optional mysql fields', () => {
  const source = fs.readFileSync(path.join(root, 'slurm.html'), 'utf8');
  assert.match(source, /Controller 主节点/);
  assert.match(source, /Controller 备节点/);
  assert.match(source, /Slurm MySQL 数据库（选填）/);
  assert.match(source, /slurmControllerBackupHost/);
  assert.match(source, /slurmMysqlHost/);
  assert.match(source, /slurmMysqlPort/);
  assert.match(source, /slurmMysqlAdminUser/);
  assert.match(source, /slurmMysqlAdminPassword/);
});

test('ldap config page exposes primary and standby ldap server fields', () => {
  const source = fs.readFileSync(path.join(root, 'ldap.html'), 'utf8');
  assert.doesNotMatch(source, /fetch\(url/);
  assert.match(source, /App\.apiFetch\(url/);
  assert.match(source, /LDAP 主节点地址/);
  assert.match(source, /LDAP 备节点地址/);
  assert.match(source, /ldapBackupUrl/);
  assert.match(source, /passwordConfigured/);
});

test('live node helper uses authenticated fetch when available', () => {
  const source = fs.readFileSync(path.join(root, 'js', 'live.js'), 'utf8');
  assert.match(source, /App\.apiFetch/);
});

test('api data source probe uses authenticated fetches', () => {
  const source = fs.readFileSync(path.join(root, 'js', 'app.js'), 'utf8');
  assert.match(source, /apiFetch\(item\.url/);
});
