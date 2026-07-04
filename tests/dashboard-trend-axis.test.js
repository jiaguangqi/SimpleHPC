const test = require('node:test');
const assert = require('node:assert/strict');
const axis = require('../js/dashboard-trend-axis.js');

const end = '2026-06-27T19:15:00+08:00';

test('dashboard trend ranges use the requested calendar tick intervals', () => {
  const cases = [
    ['24h', 12, 2 * 60 * 60 * 1000],
    ['7d', 7, 24 * 60 * 60 * 1000],
    ['30d', 30, 24 * 60 * 60 * 1000],
    ['90d', 9, 10 * 24 * 60 * 60 * 1000]
  ];
  for (const [range, count, interval] of cases) {
    const result = axis.build(range, end);
    assert.equal(result.ticks.length, count, range);
    assert.equal(result.ticks[1].timestamp - result.ticks[0].timestamp, interval, range);
  }
});

test('one year range uses one tick per calendar month', () => {
  const result = axis.build('1y', end);
  assert.equal(result.ticks.length, 12);
  assert.deepEqual(
    result.ticks.slice(0, 3).map(tick => tick.label),
    ['2025年7月', '2025年8月', '2025年9月']
  );
});

test('sample positions are based on the complete selected window', () => {
  const result = axis.build('7d', end);
  const oneDayBeforeEnd = new Date(end).getTime() - 24 * 60 * 60 * 1000;
  assert.ok(Math.abs(result.ratio(oneDayBeforeEnd) - (6 / 7)) < 0.0001);
});
