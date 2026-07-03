(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.DashboardTrendAxis = api;
}(typeof window !== 'undefined' ? window : globalThis, function () {
  const hour = 60 * 60 * 1000;
  const day = 24 * hour;
  const shanghaiOffset = 8 * hour;
  const configs = {
    '24h': { duration: day, step: 2 * hour, unit: 'hour' },
    '7d': { duration: 7 * day, step: day, unit: 'day' },
    '30d': { duration: 30 * day, step: day, unit: 'day' },
    '90d': { duration: 90 * day, step: 10 * day, unit: 'day' },
    '1y': { duration: 365 * day, unit: 'month' }
  };

  function format(timestamp, range) {
    const options = range === '24h'
      ? { hour: '2-digit', minute: '2-digit', hour12: false }
      : range === '1y'
        ? { year: 'numeric', month: '2-digit' }
        : { month: '2-digit', day: '2-digit' };
    return new Intl.DateTimeFormat('zh-CN', Object.assign({
      timeZone: 'Asia/Shanghai'
    }, options)).format(new Date(timestamp));
  }

  function firstFixedTick(start, step) {
    return Math.ceil((start + shanghaiOffset) / step) * step - shanghaiOffset;
  }

  function monthTicks(start, end) {
    const localStart = new Date(start + shanghaiOffset);
    let year = localStart.getUTCFullYear();
    let month = localStart.getUTCMonth();
    let timestamp = Date.UTC(year, month, 1) - shanghaiOffset;
    if (timestamp < start) month += 1;
    const ticks = [];
    while (true) {
      timestamp = Date.UTC(year, month, 1) - shanghaiOffset;
      if (timestamp > end) break;
      ticks.push(timestamp);
      month += 1;
    }
    return ticks;
  }

  function build(range, endValue) {
    const name = configs[range] ? range : '7d';
    const config = configs[name];
    const parsedEnd = new Date(endValue).getTime();
    const end = Number.isFinite(parsedEnd) ? parsedEnd : Date.now();
    const start = end - config.duration;
    const timestamps = config.unit === 'month'
      ? monthTicks(start, end)
      : (() => {
        const ticks = [];
        for (let timestamp = firstFixedTick(start, config.step); timestamp <= end; timestamp += config.step) {
          ticks.push(timestamp);
        }
        return ticks;
      })();
    return {
      range: name,
      start: start,
      end: end,
      ticks: timestamps.map(timestamp => ({
        timestamp: timestamp,
        ratio: (timestamp - start) / (end - start),
        label: format(timestamp, name)
      })),
      ratio: timestamp => (timestamp - start) / (end - start)
    };
  }

  return { build: build };
}));
