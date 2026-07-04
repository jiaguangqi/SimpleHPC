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

test('live node helper uses authenticated fetch when available', () => {
  const source = fs.readFileSync(path.join(root, 'js', 'live.js'), 'utf8');
  assert.match(source, /App\.apiFetch/);
});

test('api data source probe uses authenticated fetches', () => {
  const source = fs.readFileSync(path.join(root, 'js', 'app.js'), 'utf8');
  assert.match(source, /apiFetch\(item\.url/);
});
