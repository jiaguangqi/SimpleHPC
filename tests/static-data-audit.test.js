const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const assert = require('node:assert/strict');

const root = path.resolve(__dirname, '..');
const forbidden = {
  'monitoring.html': ['gpu-021', '72 / 75 在线', '3,264 / 4,800 核'],
  'inspection.html': ['RPT-20260625-0630', '18 项检查正常'],
  'audit.html': ['old-mpi-template', '#1284593'],
  'data-acl.html': ['/project/ai-lab', '/home/zhangsan/share'],
  'job-list.html': ['2026-06-24 09:07:19', '2026-06-24 12:40:00']
};

for (const [file, values] of Object.entries(forbidden)) {
  test(`${file} contains no simulated cluster records`, () => {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    for (const value of values) {
      assert.equal(source.includes(value), false, `${file} still contains static value: ${value}`);
    }
  });
}

test('dashboard replaces recent job detail table with queue trend chart', () => {
  const source = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
  assert.equal(source.includes('最近提交的作业'), false);
  assert.equal(source.includes('dashJobTable'), false);
  assert.match(source, /资源池作业趋势/);
  assert.match(source, /queueTrendSelect/);
  assert.match(source, /queueTrendSvg/);
});

test('dashboard queue trend client uses aggregate trend endpoint only', () => {
  const source = fs.readFileSync(path.join(root, 'js/dashboard.js'), 'utf8');
  assert.equal(source.includes('/api/v1/dashboard/queue-job-trends'), true);
  assert.equal(source.includes('dashJobTable'), false);
  assert.equal(source.includes('renderJobs'), false);
});
