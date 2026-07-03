const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const html = fs.readFileSync('system-logs.html', 'utf8');
const script = fs.readFileSync('js/log-center.js', 'utf8');
const handler = fs.readFileSync('backend/internal/httpapi/logcenter.go', 'utf8');
const service = fs.readFileSync('backend/internal/service/logcenter.go', 'utf8');

test('system logs page exposes log level filter', () => {
  assert.match(html, /id="systemLevel"/);
  assert.match(html, /全部级别/);
  assert.match(html, /错误 error/);
  assert.match(html, /警告 warning/);
  assert.match(html, /信息 info/);
  assert.match(html, /调试 debug/);
});

test('system log requests and backend support level filtering', () => {
  assert.match(script, /level:document\.getElementById\('systemLevel'\)\.value/);
  assert.match(handler, /c\.Query\("level"\)/);
  assert.match(service, /item\.Level != level/);
});
