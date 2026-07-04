const test = require('node:test');
const assert = require('node:assert/strict');
const pagination = require('../js/job-pagination.js');

test('shows first five and last two pages at the beginning', () => {
  assert.deepEqual(pagination.tokens(1, 34), [1, 2, 3, 4, 5, 'ellipsis', 33, 34]);
});

test('shows current page and neighbors in the middle', () => {
  assert.deepEqual(pagination.tokens(18, 34), [1, 'ellipsis', 17, 18, 19, 'ellipsis', 33, 34]);
});

test('does not duplicate pages near the end', () => {
  assert.deepEqual(pagination.tokens(33, 34), [1, 2, 3, 4, 5, 'ellipsis', 32, 33, 34]);
});

test('shows every page when the list is short', () => {
  assert.deepEqual(pagination.tokens(2, 5), [1, 2, 3, 4, 5]);
});
