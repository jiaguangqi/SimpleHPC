const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const rbac = fs.readFileSync(path.join(root, 'js', 'rbac.js'), 'utf8');
const roles = fs.readFileSync(path.join(root, 'js', 'roles.js'), 'utf8');
const rolesHTML = fs.readFileSync(path.join(root, 'roles.html'), 'utf8');
const theme = fs.readFileSync(path.join(root, 'css', 'theme.css'), 'utf8');

test('permission client uses auth/me as its single runtime source', () => {
  assert.match(rbac, /\/api\/v1\/auth\/me/);
  assert.doesNotMatch(rbac, /cluster_admin.*\?.*menu|accountType\s*===\s*['"]ldap/);
});

test('dynamic navigation supports backend flat and grouped menus', () => {
  assert.match(rbac, /flatMenu/);
  assert.match(rbac, /renderNavigation/);
  assert.match(rbac, /item\.children/);
  assert.match(rbac, /navIcon/);
  assert.match(rbac, /flattenRenderableMenus/);
  assert.match(rbac, /favoriteKeysFor/);
  assert.match(rbac, /openMenuOverview/);
  assert.doesNotMatch(rbac, /const\s+NAV_TREE/);
});

test('dynamic navigation renders first-level menus into the global top navigation shell', () => {
  assert.match(rbac, /ensureTopbar/);
  assert.match(rbac, /topLevelMenus/);
  assert.match(rbac, /topnavPrimaryMenu/);
  assert.match(rbac, /topnav-primary-list/);
  assert.match(rbac, /topbar-brand-logo/);
  assert.match(rbac, /menu-overview-overlay/);
  assert.match(rbac, /layout--topnav/);
  assert.match(rbac, /shpc-topnav-enabled/);
  assert.match(rbac, /groupLabel/);
  assert.doesNotMatch(rbac, /topnavGroup\(/);
  assert.doesNotMatch(rbac, /querySelector\(['"]\\.sidebar \\.nav['"]\);\s*if \(!nav \|\| !context\) return;\s*nav\.innerHTML/s);
});

test('menu overview keeps categories, favorites and drag layout support', () => {
  assert.match(rbac, /MENU_CATEGORY_LABELS/);
  for (const label of ['系统管理', '资源管理', '用户管理', '作业管理', '运维管理', '运营管理']) {
    assert.match(rbac, new RegExp(label));
  }
  assert.match(rbac, /simplehpc:navigation:favorites/);
  assert.match(rbac, /overviewLayoutKey/);
  assert.match(rbac, /:navigation:overview-layout:/);
  assert.match(rbac, /moveOverviewItem/);
  assert.match(rbac, /button\.draggable\s*=\s*true/);
  assert.match(rbac, /application\/x-simplehpc-menu-key/);
  assert.match(rbac, /overviewDisplayName\(menu\)/);
  assert.match(rbac, /localStorage/);
});

test('menu overview uses compact six-column desktop layout', () => {
  assert.match(theme, /\.menu-overview-modal\s*\{[\s\S]*width:\s*min\(1480px,\s*calc\(100vw - 48px\)\)/);
  assert.match(theme, /\.menu-overview-body\s*\{[\s\S]*grid-template-columns:\s*repeat\(6,\s*minmax\(0,\s*1fr\)\)/);
  assert.match(theme, /\.menu-overview-item\s*\{[\s\S]*height:\s*38px/);
  assert.match(theme, /\.menu-overview-icon\s*\{[\s\S]*width:\s*24px[\s\S]*height:\s*24px/);
  assert.match(theme, /\.menu-overview-copy small\s*\{[\s\S]*display:\s*none/);
  const desktopMedia = theme.slice(
    theme.indexOf('@media (max-width: 1560px)'),
    theme.indexOf('@media (max-width: 1180px)')
  );
  assert.doesNotMatch(desktopMedia, /\.menu-overview-body/);
  assert.match(theme, /\.menu-overview-category--drop/);
  assert.match(theme, /\.menu-overview-category-empty/);
  assert.match(theme, /\.topnav-primary-list/);
  assert.match(rbac, /一级：/);
  assert.match(rbac, /二级：/);
  assert.match(rbac, /未找到匹配菜单/);
});

test('route and button guards are permission driven', () => {
  assert.match(rbac, /guardRoute/);
  assert.match(rbac, /\[data-permission\]/);
  assert.match(rbac, /simplehpc:forbidden/);
});

test('role editor exposes all six required tabs', () => {
  for (const label of ['基础信息', '菜单权限', '操作权限', '数据范围', '文件目录权限', '绑定用户']) {
    assert.match(roles, new RegExp(label));
  }
});

test('role workbench supports lifecycle, bindings and matrix', () => {
  for (const endpoint of [
    '/api/v1/rbac/roles',
    '/copy',
    '/status',
    '/permissions',
    '/data-scopes',
    '/file-policies',
    '/users',
    '/api/v1/rbac/matrix'
  ]) {
    assert.match(roles, new RegExp(endpoint.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  assert.match(rolesHTML, /roles\.js/);
  assert.doesNotMatch(rolesHTML, /account-pages\.js/);
});
