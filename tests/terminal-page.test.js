const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');

test('terminal page exposes WebSSH session controls', () => {
  const html = fs.readFileSync(path.join(root, 'terminal.html'), 'utf8');
  assert.match(html, /终端中心/);
  assert.match(html, /id="terminalScreen"/);
  assert.match(html, /id="terminalEmptyState"/);
  assert.match(html, /id="terminalConnectBtn"/);
  assert.match(html, /data-permission="action\.webssh\.terminal\.create"|data-permission="action\.terminal\.connect"/);
  assert.match(html, /js\/terminal\.js/);
  assert.doesNotMatch(html, /login02 \/ bash/);
  assert.doesNotMatch(html, /gpu-login \/ bash/);
});

test('terminal client connects to backend websocket session endpoint', () => {
  const js = fs.readFileSync(path.join(root, 'js', 'terminal.js'), 'utf8');
  assert.match(js, /new WebSocket/);
  assert.match(js, /\/api\/v1\/webssh\/sessions/);
  assert.match(js, /\/api\/v1\/webssh\/sessions\/.*\/ws|wsUrl/);
  assert.match(js, /type: 'resize'/);
  assert.match(js, /Ctrl\+C|\\x03/);
  assert.match(js, /const sessions = new Map\(\)/);
  assert.match(js, /renderSessionTabs/);
  assert.match(js, /closeSession/);
});

test('terminal page file manager uses real storage APIs', () => {
  const html = fs.readFileSync(path.join(root, 'terminal.html'), 'utf8');
  const js = fs.readFileSync(path.join(root, 'js', 'terminal.js'), 'utf8');
  const css = fs.readFileSync(path.join(root, 'css', 'theme.css'), 'utf8');
  assert.match(html, /id="terminalFileTree"/);
  assert.match(html, /id="terminalFileUploadInput"/);
  assert.match(html, /更多文件管理/);
  assert.match(html, /id="terminalParentBtn"/);
  assert.doesNotMatch(html, /id="terminalCurrentPath"/);
  assert.doesNotMatch(html, /id="terminalPathCopyBtn"/);
  assert.doesNotMatch(html, /id="terminalStorageUsage"/);
  assert.doesNotMatch(html, /id="terminalStoragePercent"/);
  assert.doesNotMatch(html, /存储使用情况/);
  assert.doesNotMatch(html, /权限状态/);
  assert.match(js, /\/api\/v1\/webssh\/files\/tree/);
  assert.match(js, /\/api\/v1\/webssh\/files\/list/);
  assert.match(js, /\/api\/v1\/webssh\/files\/upload/);
  assert.match(js, /\/api\/v1\/webssh\/files\/download/);
  assert.match(js, /\/api\/v1\/webssh\/files\/archive/);
  assert.match(js, /uploadConfirmModal/);
  assert.doesNotMatch(js, /confirm\('上传到/);
  assert.match(css, /\.webssh-upload-confirm/);
  assert.match(css, /\.webssh-upload-target/);
  assert.match(js, /selectedPaths = new Set/);
  assert.match(js, /copyEntryPath/);
  assert.match(js, /enterPathInTerminal/);
  assert.match(js, /includes\(q\)|includes\(keyword\)/);
  assert.match(js, /setTimeout\(renderEntries, 200\)/);
  assert.match(css, /body\.shpc-terminal-page[\s\S]*overflow:\s*hidden/);
  assert.match(css, /\.shpc-webssh-entry-scroll[\s\S]*overflow-y:\s*auto/);
  assert.match(css, /\.shpc-webssh-storage,\s*\n\.shpc-webssh-file-meta[\s\S]*display:\s*none\s*!important/);
  assert.match(css, /\.shpc-webssh-entry[\s\S]*height:\s*34px/);
});

test('terminal client reads login node configuration', () => {
  const js = fs.readFileSync(path.join(root, 'js', 'terminal.js'), 'utf8');
  assert.match(js, /\/api\/v1\/webssh\/nodes/);
  assert.match(js, /round_robin|least_sessions/);
  assert.match(js, /hasLoginNodes/);
  assert.match(js, /请到系统设置页面中设置登录节点信息/);
  assert.doesNotMatch(js, /后端默认本机节点/);
});
