const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const html = fs.readFileSync(path.join(__dirname, '..', 'data.html'), 'utf8');

test('file manager consumes backend path boundary metadata', () => {
  assert.match(html, /data\.effectivePath/);
  assert.match(html, /data\.initialPath/);
  assert.match(html, /data\.canGoParent/);
  assert.match(html, /data\.parentPath/);
});

test('parent navigation uses backend parent path instead of constructing one', () => {
  const parentFunction = html.match(/function goParent\(\)\{[^}]+\}/)?.[0] || '';
  assert.match(parentFunction, /currentParentPath/);
  assert.doesNotMatch(parentFunction, /replace\(/);
});
