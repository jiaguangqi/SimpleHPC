const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..', 'backend', 'internal');
const expected = {
  'httpapi/router.go': 25,
  'httpapi/rbac_admin.go': 14,
  'httpapi/rbac.go': 2,
  'httpapi/terminal_config.go': 1,
  'httpapi/templates.go': 2,
  'httpapi/storage_access.go': 3,
  'httpapi/monitoring.go': 2,
  'httpapi/logcenter.go': 2,
  'httpapi/audit.go': 2,
  'httpapi/acl.go': 2,
  'httpapi/platform_assets.go': 1,
  'service/service.go': 1,
  'service/templates.go': 4
};
const pattern = /user\.Type\s*[!=]=|requireAdmin\(|\.Role\s*==/g;

function walk(directory) {
  return fs.readdirSync(directory, {withFileTypes:true}).flatMap(entry => {
    const target = path.join(directory, entry.name);
    return entry.isDirectory() ? walk(target) : entry.name.endsWith('.go') ? [target] : [];
  });
}

test('legacy authorization checks stay inside the reviewed compatibility allowlist', () => {
  const actual = {};
  for (const file of walk(root)) {
    const count = (fs.readFileSync(file, 'utf8').match(pattern) || []).length;
    if (count) actual[path.relative(root, file)] = count;
  }
  assert.deepEqual(actual, expected,
    'legacy authorization footprint changed; classify and document every new or removed check');
});
