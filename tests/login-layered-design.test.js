const assert = require('assert');
const fs = require('fs');
const test = require('node:test');

const loginHTML = fs.readFileSync('login.html', 'utf8');
const platformUI = fs.readFileSync('js/platform-ui.js', 'utf8');

test('login page uses HTML/CSS layout with a standalone server visual', () => {
  assert.match(loginHTML, /class="login-bg"/);
  assert.match(loginHTML, /class="server-visual"/);
  assert.match(loginHTML, /src="assets\/images\/login-server-hero\.webp"/);
  assert.match(loginHTML, /class="login-shell"/);
  assert.doesNotMatch(loginHTML, /--login-hero-image/);
});

test('login page uses generated SimpleHPC logo assets', () => {
  assert.match(loginHTML, /assets\/logos\/simplehpc-favicon\.png/);
  assert.match(loginHTML, /assets\/logos\/simplehpc-navbar-horizontal\.png/);
  assert.match(loginHTML, /assets\/logos\/simplehpc-login-vertical\.png/);
});

test('login page keeps exactly four feature entries', () => {
  const body = loginHTML.slice(loginHTML.indexOf('<body>'));
  const matches = body.match(/class="feature-item"/g) || [];
  assert.strictEqual(matches.length, 4);
});

test('platform public login image config updates standalone server visual', () => {
  assert.match(platformUI, /server-visual/);
  assert.match(platformUI, /config\.loginImage/);
  assert.doesNotMatch(platformUI, /--login-hero-image/);
});
