(function () {
  const missing = '数据未获取';
  let latestSnapshot = null;
  let activeTrendRange = '7d';
  let activeQueueTrendRange = '7d';
  let activeQueue = '';
  let latestQueueTrend = null;
  const visibleQueueSeries = { running: true, pending: true };

  function text(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[ch]));
  }

  function hasNumber(value) {
    return value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value));
  }

  function numberOrMissing(value, suffix) {
    return hasNumber(value) ? String(value) + (suffix || '') : missing;
  }

  function pct(value) {
    return hasNumber(value) ? Number(value).toFixed(1).replace(/\.0$/, '') + '%' : missing;
  }

  function bytes(value) {
    if (!Number.isFinite(Number(value))) return '';
    const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    let n = Number(value);
    let i = 0;
    while (n >= 1024 && i < units.length - 1) {
      n /= 1024;
      i++;
    }
    return (n >= 10 || i === 0 ? n.toFixed(0) : n.toFixed(1)) + ' ' + units[i];
  }

  function widget(id) {
    return document.querySelector('[data-widget-id="' + id + '"]');
  }

  function setStat(id, label, value, note, noteClass) {
    const card = widget(id);
    if (!card) return;
    const labelEl = card.querySelector('.stat-label');
    const valueEl = card.querySelector('.stat-value');
    const changeEl = card.querySelector('.stat-change, .stat-note');
    if (labelEl) labelEl.textContent = label;
    if (valueEl) valueEl.textContent = value;
    if (changeEl) {
      changeEl.className = 'stat-note ' + (noteClass || '');
      changeEl.textContent = note || '';
    }
  }

  function setMetricBar(id, value) {
    const card = widget(id);
    if (!card) return;
    const fill = card.querySelector('.metric-bar-fill');
    if (fill) fill.style.width = hasNumber(value) ? Math.max(0, Math.min(100, Number(value))) + '%' : '0%';
  }

  function formatSubmit(value) {
    if (!value) return missing;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return text(value);
    const pad = n => String(n).padStart(2, '0');
    return pad(date.getMonth() + 1) + '-' + pad(date.getDate()) + ' ' + pad(date.getHours()) + ':' + pad(date.getMinutes());
  }

  function formatTrendTime(value, range) {
    if (!value) return missing;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return text(value);
    const pad = n => String(n).padStart(2, '0');
    if (range === '24h') return pad(date.getMonth() + 1) + '-' + pad(date.getDate()) + ' ' + pad(date.getHours()) + ':00';
    if (range === '1y') return date.getFullYear() + '-' + pad(date.getMonth() + 1);
    return pad(date.getMonth() + 1) + '-' + pad(date.getDate()) + ' ' + pad(date.getHours()) + ':00';
  }

  function renderStats(data) {
    const users = data.users || {};
    const jobs = data.jobs || {};
    const resources = data.resources || {};
    setStat(
      'online-users',
      '在线用户',
      hasNumber(users.online) ? String(users.online) : missing,
      hasNumber(users.online) ? '有效登录用户 ' + users.online + ' / 总用户 ' + users.total : 'Redis 会话统计不可用',
      hasNumber(users.online) ? 'success' : 'warn'
    );
    setStat('running-jobs', '运行中作业', numberOrMissing(jobs.running), '来自 Slurm squeue 实时队列', 'success');
    setStat('pending-jobs', '排队作业', numberOrMissing(jobs.pending), '来自 Slurm squeue 实时队列', 'warn');
    setStat('cpu-usage', 'CPU 利用率', pct(resources.cpuUsagePercent), '来自 Slurm sinfo：已分配 ' + numberOrMissing(resources.allocatedCpus, ' 核') + ' / 总 ' + numberOrMissing(resources.totalCpus, ' 核'), 'success');
    setMetricBar('cpu-usage', resources.cpuUsagePercent);
    const noGPU = Number(resources.totalGpus) === 0 && resources.gpuUsageNote;
    setStat('gpu-usage', 'GPU 利用率', noGPU ? '未配置' : pct(resources.gpuUsagePercent), resources.gpuUsageNote || 'GPU 分配数据未获取', hasNumber(resources.gpuUsagePercent) ? 'success' : 'warn');
    setMetricBar('gpu-usage', resources.gpuUsagePercent);
  }

  function chartY(value) {
    const n = Math.max(0, Math.min(100, Number(value)));
    return 210 - (n / 100) * 180;
  }

  function renderTrend(data) {
    const trends = Array.isArray(data.trends) ? data.trends : [];
    const range = data.trendRange || activeTrendRange;
    const axis = window.DashboardTrendAxis.build(range, data.generatedAt || Date.now());
    const svg = document.getElementById('trendSvg');
    const summary = document.getElementById('trendSummary');
    const windowEl = document.getElementById('trendWindow');
    const rangeLabels = { '24h': '过去 24 小时', '7d': '过去 7 天', '30d': '过去 30 天', '90d': '过去 90 天', '1y': '过去 1 年' };
    const bucketLabels = { '15 minutes': '15 分钟', '1 hour': '1 小时', '6 hours': '6 小时', '1 day': '1 天', '7 days': '7 天' };
    if (summary) summary.textContent = (rangeLabels[range] || rangeLabels['7d']) + ' CPU/GPU 平均利用率，单位：%';
    if (windowEl) windowEl.textContent = trends.length > 1 ? '采样粒度：' + (bucketLabels[data.trendBucket] || data.trendBucket || missing) + ' · ' + trends.length + ' 点' : '历史样本不足';
    if (!svg) return;
    const left = 34;
    const right = 630;
    const points = field => trends.map(item => {
      const value = Number(item[field]);
      if (!Number.isFinite(value)) return null;
      const ratio = axis.ratio(new Date(item.sampledAt).getTime());
      if (!Number.isFinite(ratio) || ratio < 0 || ratio > 1) return null;
      return (left + ratio * (right - left)).toFixed(1) + ',' + chartY(value).toFixed(1);
    }).filter(Boolean).join(' ');
    const cpuPoints = points('cpuUsagePercent');
    const gpuPoints = points('gpuUsagePercent');
    svg.innerHTML = '<g stroke="var(--border)" stroke-width="1" stroke-dasharray="4 4"><line x1="34" y1="30" x2="630" y2="30"/><line x1="34" y1="75" x2="630" y2="75"/><line x1="34" y1="120" x2="630" y2="120"/><line x1="34" y1="165" x2="630" y2="165"/><line x1="34" y1="210" x2="630" y2="210"/></g><g font-size="10" fill="var(--muted)"><text x="4" y="34">100%</text><text x="10" y="79">80%</text><text x="10" y="124">60%</text><text x="10" y="169">40%</text><text x="10" y="214">20%</text></g><polyline fill="none" stroke="#007AFF" stroke-width="3" points="' + cpuPoints + '"/><polyline fill="none" stroke="#AF52DE" stroke-width="3" points="' + gpuPoints + '"/><g id="trendLabels" font-size="' + (range === '30d' ? '8' : '9') + '" fill="var(--muted)"></g>';
    const labels = svg.querySelector('#trendLabels');
    axis.ticks.forEach((tick, index) => {
      const x = left + tick.ratio * (right - left);
      const mark = document.createElementNS('http://www.w3.org/2000/svg', 'line');
      mark.setAttribute('x1', x);
      mark.setAttribute('x2', x);
      mark.setAttribute('y1', '210');
      mark.setAttribute('y2', '216');
      mark.setAttribute('stroke', 'var(--muted)');
      labels.appendChild(mark);
      const t = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      t.setAttribute('x', x);
      t.setAttribute('y', range === '30d' ? '232' : '234');
      t.setAttribute('text-anchor', range === '30d' ? 'end' : 'middle');
      if (range === '30d') t.setAttribute('transform', 'rotate(-45 ' + x + ' 232)');
      t.textContent = tick.label;
      labels.appendChild(t);
    });
    if (trends.length < 2) {
      const message = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      message.setAttribute('x', '332');
      message.setAttribute('y', '125');
      message.setAttribute('text-anchor', 'middle');
      message.setAttribute('fill', 'var(--muted)');
      message.setAttribute('font-size', '14');
      message.textContent = '当前时间范围内历史样本不足';
      svg.appendChild(message);
    }
  }

  function queueChartY(value, maxValue) {
    const top = 28;
    const bottom = 206;
    const n = Math.max(0, Number(value) || 0);
    if (maxValue <= 0) return bottom;
    return bottom - (n / maxValue) * (bottom - top);
  }

  function renderQueueTrend(data) {
    latestQueueTrend = data || {};
    const points = Array.isArray(latestQueueTrend.points) ? latestQueueTrend.points : [];
    const queues = Array.isArray(latestQueueTrend.queues) ? latestQueueTrend.queues : [];
    const queue = latestQueueTrend.queue || activeQueue || '';
    const range = latestQueueTrend.range || activeQueueTrendRange;
    const select = document.getElementById('queueTrendSelect');
    const svg = document.getElementById('queueTrendSvg');
    const empty = document.getElementById('queueTrendEmpty');
    const sample = document.getElementById('queueTrendSample');
    const tooltip = document.getElementById('queueTrendTooltip');
    if (select) {
      const current = select.value;
      select.innerHTML = queues.length
        ? queues.map(item => '<option value="' + text(item) + '">' + text(item) + '</option>').join('')
        : '<option value="">暂无队列</option>';
      select.value = queue || current || (queues[0] || '');
      activeQueue = select.value || queue || '';
    }
    if (sample) {
      sample.textContent = points.length
        ? '采样粒度：' + text(latestQueueTrend.sampleIntervalLabel || latestQueueTrend.sampleInterval || '—') + ' · ' + points.length + ' 点'
        : '采样粒度：' + text(latestQueueTrend.sampleIntervalLabel || latestQueueTrend.sampleInterval || '—');
    }
    if (!svg) return;
    if (tooltip) tooltip.hidden = true;
    svg.innerHTML = '';
    const hasData = points.some(p => Number(p.running) > 0 || Number(p.pending) > 0);
    if (empty) empty.hidden = hasData;
    if (!hasData) {
      drawQueueTrendAxes(svg, range, [], 1);
      return;
    }

    const axis = window.DashboardTrendAxis.build(range, new Date().toISOString());
    const left = 38;
    const right = 630;
    const maxValue = Math.max(1, ...points.map(p => Math.max(Number(p.running) || 0, Number(p.pending) || 0)));
    const yMax = Math.max(1, Math.ceil(maxValue * 1.2));
    drawQueueTrendAxes(svg, range, axis.ticks, yMax);

    const series = [
      { key: 'running', label: '运行作业数', color: '#007AFF' },
      { key: 'pending', label: '排队作业数', color: '#FF9500' }
    ];
    series.forEach(item => {
      if (!visibleQueueSeries[item.key]) return;
      const values = points.map(point => {
        const ratio = axis.ratio(new Date(point.time).getTime());
        if (!Number.isFinite(ratio) || ratio < 0 || ratio > 1) return null;
        return {
          x: left + ratio * (right - left),
          y: queueChartY(point[item.key], yMax),
          raw: point
        };
      }).filter(Boolean);
      const polyline = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
      polyline.setAttribute('fill', 'none');
      polyline.setAttribute('stroke', item.color);
      polyline.setAttribute('stroke-width', '3');
      polyline.setAttribute('stroke-linecap', 'round');
      polyline.setAttribute('stroke-linejoin', 'round');
      polyline.setAttribute('points', values.map(p => p.x.toFixed(1) + ',' + p.y.toFixed(1)).join(' '));
      svg.appendChild(polyline);
      values.forEach(p => {
        const circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        circle.setAttribute('cx', p.x.toFixed(1));
        circle.setAttribute('cy', p.y.toFixed(1));
        circle.setAttribute('r', '5');
        circle.setAttribute('fill', '#fff');
        circle.setAttribute('stroke', item.color);
        circle.setAttribute('stroke-width', '2');
        circle.style.cursor = 'pointer';
        circle.addEventListener('mouseenter', event => showQueueTooltip(event, p.raw, queue, range));
        circle.addEventListener('mousemove', event => showQueueTooltip(event, p.raw, queue, range));
        circle.addEventListener('mouseleave', hideQueueTooltip);
        svg.appendChild(circle);
      });
    });
  }

  function drawQueueTrendAxes(svg, range, ticks, yMax) {
    const left = 38;
    const right = 630;
    const top = 28;
    const bottom = 206;
    const grid = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    grid.setAttribute('stroke', 'var(--border)');
    grid.setAttribute('stroke-width', '1');
    grid.setAttribute('stroke-dasharray', '4 4');
    for (let i = 0; i <= 4; i++) {
      const y = top + ((bottom - top) / 4) * i;
      const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
      line.setAttribute('x1', left);
      line.setAttribute('x2', right);
      line.setAttribute('y1', y.toFixed(1));
      line.setAttribute('y2', y.toFixed(1));
      grid.appendChild(line);
    }
    svg.appendChild(grid);
    const labels = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    labels.setAttribute('font-size', '10');
    labels.setAttribute('fill', 'var(--muted)');
    for (let i = 0; i <= 4; i++) {
      const y = top + ((bottom - top) / 4) * i;
      const value = Math.round(yMax - (yMax / 4) * i);
      const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      label.setAttribute('x', '4');
      label.setAttribute('y', String(y + 4));
      label.textContent = value + '个';
      labels.appendChild(label);
    }
    (ticks || []).forEach(tick => {
      const x = left + tick.ratio * (right - left);
      const mark = document.createElementNS('http://www.w3.org/2000/svg', 'line');
      mark.setAttribute('x1', x);
      mark.setAttribute('x2', x);
      mark.setAttribute('y1', bottom);
      mark.setAttribute('y2', bottom + 6);
      mark.setAttribute('stroke', 'var(--muted)');
      labels.appendChild(mark);
      const t = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      t.setAttribute('x', x);
      t.setAttribute('y', range === '30d' ? '232' : '234');
      t.setAttribute('text-anchor', range === '30d' ? 'end' : 'middle');
      if (range === '30d') t.setAttribute('transform', 'rotate(-45 ' + x + ' 232)');
      t.textContent = tick.label;
      labels.appendChild(t);
    });
    svg.appendChild(labels);
  }

  function showQueueTooltip(event, point, queue, range) {
    const tooltip = document.getElementById('queueTrendTooltip');
    const wrap = document.getElementById('queueTrendChartWrap');
    if (!tooltip || !wrap) return;
    const rect = wrap.getBoundingClientRect();
    tooltip.innerHTML =
      '<b>' + text(formatTrendTime(point.time, range)) + '</b>' +
      '<span><em>队列</em><strong>' + text(queue || missing) + '</strong></span>' +
      '<span><em>运行作业数</em><strong style="color:#007AFF">' + text(point.running || 0) + ' 个</strong></span>' +
      '<span><em>排队作业数</em><strong style="color:#FF9500">' + text(point.pending || 0) + ' 个</strong></span>';
    tooltip.hidden = false;
    const x = Math.min(rect.width - 200, Math.max(8, event.clientX - rect.left + 12));
    const y = Math.min(rect.height - 110, Math.max(8, event.clientY - rect.top - 12));
    tooltip.style.left = x + 'px';
    tooltip.style.top = y + 'px';
  }

  function hideQueueTooltip() {
    const tooltip = document.getElementById('queueTrendTooltip');
    if (tooltip) tooltip.hidden = true;
  }

  async function loadQueueJobTrends(showLoading) {
    const loading = document.getElementById('queueTrendLoading');
    if (showLoading && loading) loading.hidden = false;
    try {
      const query = new URLSearchParams({ range: activeQueueTrendRange });
      if (activeQueue) query.set('queue', activeQueue);
      const fetcher = window.App && App.apiFetch ? App.apiFetch : fetch;
      const res = await fetcher('/api/v1/dashboard/queue-job-trends?' + query.toString(), { cache: 'no-store' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || res.statusText || '接口请求失败');
      renderQueueTrend(data);
    } catch (err) {
      const empty = document.getElementById('queueTrendEmpty');
      if (empty) {
        empty.hidden = false;
        empty.querySelector('strong').textContent = '暂无作业趋势数据';
        empty.querySelector('span').textContent = '资源池作业趋势数据未获取：' + (err.message || err);
      }
      if (window.App) App.toast('资源池作业趋势未获取：' + (err.message || err), 'warn', 4000);
    } finally {
      if (loading) loading.hidden = true;
    }
  }

  function wireQueueTrendControls() {
    const select = document.getElementById('queueTrendSelect');
    if (select) {
      select.addEventListener('change', function () {
        activeQueue = select.value;
        loadQueueJobTrends(true);
      });
    }
    const rangeTabs = document.getElementById('queueTrendRangeTabs');
    if (rangeTabs) {
      rangeTabs.addEventListener('click', function (event) {
        const btn = event.target.closest('button[data-range]');
        if (!btn) return;
        activeQueueTrendRange = btn.dataset.range || '7d';
        rangeTabs.querySelectorAll('button').forEach(item => {
          item.classList.toggle('btn-primary', item === btn);
          item.classList.toggle('btn-ghost', item !== btn);
        });
        loadQueueJobTrends(true);
      });
    }
    document.querySelectorAll('.queue-trend-legend-item[data-series]').forEach(button => {
      button.addEventListener('click', function () {
        const key = button.dataset.series;
        visibleQueueSeries[key] = !visibleQueueSeries[key];
        button.classList.toggle('active', !!visibleQueueSeries[key]);
        if (latestQueueTrend) renderQueueTrend(latestQueueTrend);
      });
    });
    window.addEventListener('resize', function () {
      if (latestQueueTrend) renderQueueTrend(latestQueueTrend);
    });
  }

  function renderStorage(data) {
    const list = document.getElementById('dashboardStorageList');
    const summary = document.getElementById('dashboardStorageSummary');
    const items = Array.isArray(data.storage) ? data.storage : [];
    if (!list) return;
    if (!items.length) {
      list.innerHTML = '<div class="api-data-missing"><strong>数据未获取</strong>：后端未返回项目对接的存储目录。</div>';
      if (summary) summary.innerHTML = '<span>存储配置为空</span>';
      return;
    }
    list.innerHTML = items.map(root => {
      const usage = Number(root.usagePercent);
      const hasUsage = Number.isFinite(usage);
      const className = hasUsage ? (usage >= 85 ? 'danger' : usage >= 75 ? 'warn' : 'success') : '';
      const error = root.usageError ? ' · ' + root.usageError : '';
      return '<div class="storage-row"><div><strong>' + text(root.path || root.name || missing) + '</strong><span>' + text((root.fsType || missing) + ' · ' + (root.purpose || '') + error) + '</span></div><em>' + (hasUsage ? pct(usage) : '用量数据未获取') + '</em><div class="metric-bar"><div class="metric-bar-fill ' + className + '" style="width:' + (hasUsage ? Math.max(0, Math.min(100, usage)) : 0) + '%;"></div></div></div>';
    }).join('');
    if (summary) {
      summary.innerHTML = items.map(root => {
        const bits = [root.totalBytes ? '总容量 ' + bytes(root.totalBytes) : '', root.usedBytes ? '已用 ' + bytes(root.usedBytes) : ''].filter(Boolean);
        return bits.length ? '<span>' + text(root.path) + ' ' + text(bits.join(' · ')) + '</span>' : '';
      }).join('') || '<span>目录容量字段未获取</span>';
    }
  }

  function renderAlerts(data) {
    const card = widget('recent-alerts');
    if (!card) return;
    const container = card.querySelector('.card-title + div');
    const alerts = Array.isArray(data.alerts) ? data.alerts : [];
    if (!container) return;
    if (!alerts.length) {
      container.innerHTML = '<div class="api-data-missing"><strong>暂无告警数据</strong>：dashboard_alerts 当前没有活跃或历史告警。</div>';
      return;
    }
    container.innerHTML = alerts.map(alert => {
      const klass = alert.level === 'critical' || alert.level === 'danger' ? 'pill-danger' : (alert.level === 'warn' || alert.level === 'warning' ? 'pill-warn' : (alert.status === 'resolved' ? 'pill-success' : 'pill-info'));
      return '<div style="background:var(--bg);border-radius:var(--radius-sm);padding:14px 16px;display:flex;flex-direction:column;gap:6px;"><div style="display:flex;justify-content:space-between;align-items:center;"><span class="pill ' + klass + '" style="font-size:11px;">' + text(alert.status || alert.level || '信息') + '</span><span style="font-size:11px;color:var(--muted);">' + formatSubmit(alert.occurredAt) + '</span></div><div style="font-size:14px;font-weight:500;">' + text(alert.title || alert.message || missing) + '</div></div>';
    }).join('');
  }

  function renderErrors(data) {
    const errors = data.errors || {};
    const messages = Object.keys(errors).filter(key => errors[key]).map(key => key + ': ' + errors[key]);
    if (messages.length && window.App) {
      App.toast('仪表盘部分数据未获取：' + messages.join('；'), 'warn', 4000);
    }
  }

  function detailRows(rows) {
    return '<div style="font-size:14px;line-height:1.9;color:var(--text);">' + rows.map(row => (
      '<div style="display:flex;justify-content:space-between;gap:24px;border-bottom:1px solid var(--border);padding:6px 0;"><span style="color:var(--muted);">' + text(row[0]) + '</span><strong>' + text(row[1]) + '</strong></div>'
    )).join('') + '</div>';
  }

  window.showDashboardDetail = function (id) {
    if (typeof dashboardEditMode !== 'undefined' && dashboardEditMode) return;
    if (!window.App) return;
    const data = latestSnapshot || {};
    const users = data.users || {};
    const jobs = data.jobs || {};
    const resources = data.resources || {};
    const maps = {
      'online-users': {
        title: '用户在线情况',
        rows: [['在线用户', hasNumber(users.online) ? users.online : missing], ['用户总数', hasNumber(users.total) ? users.total : missing], ['正常用户', hasNumber(users.active) ? users.active : missing], ['冻结用户', hasNumber(users.frozen) ? users.frozen : missing]]
      },
      'running-jobs': {
        title: '运行中作业',
        rows: [['运行作业', hasNumber(jobs.running) ? jobs.running : missing], ['作业总数', hasNumber(jobs.total) ? jobs.total : missing], ['数据来源', data.source || missing]]
      },
      'pending-jobs': {
        title: '排队作业',
        rows: [['排队作业', hasNumber(jobs.pending) ? jobs.pending : missing], ['作业总数', hasNumber(jobs.total) ? jobs.total : missing], ['数据来源', data.source || missing]]
      },
      'cpu-usage': {
        title: 'CPU 利用率',
        rows: [['利用率', pct(resources.cpuUsagePercent)], ['已分配 CPU', numberOrMissing(resources.allocatedCpus, ' 核')], ['总 CPU', numberOrMissing(resources.totalCpus, ' 核')], ['空闲节点', numberOrMissing(resources.idleNodes, ' 个')]]
      },
      'gpu-usage': {
        title: 'GPU 利用率',
        rows: [['利用率', pct(resources.gpuUsagePercent)], ['已分配 GPU', numberOrMissing(resources.allocatedGpus, ' 卡')], ['总 GPU', numberOrMissing(resources.totalGpus, ' 卡')], ['说明', resources.gpuUsageNote || 'GPU 分配数据未获取']]
      }
    };
    const item = maps[id];
    if (!item) return;
    App.modal({ title: item.title, content: detailRows(item.rows), showFooter: false });
  };

  async function loadDashboard(showToast, range) {
    if (range) activeTrendRange = range;
    if (showToast && window.App) App.loading.show('刷新仪表盘数据中...');
    try {
      const fetcher = window.App && App.apiFetch ? App.apiFetch : fetch;
      const res = await fetcher('/api/v1/dashboard?range=' + encodeURIComponent(activeTrendRange), { cache: 'no-store' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || res.statusText || '接口请求失败');
      latestSnapshot = data;
      renderStats(data);
      renderTrend(data);
      renderStorage(data);
      renderAlerts(data);
      renderErrors(data);
      if (showToast && window.App) App.toast('仪表盘已更新：' + (data.source || 'API'), 'success');
    } catch (err) {
      if (window.App) App.toast('仪表盘数据未获取：' + err.message, 'danger', 5000);
    } finally {
      document.body.classList.add('dashboard-data-ready');
      document.dispatchEvent(new CustomEvent('simplehpc:dashboard-ready', { detail: latestSnapshot || null }));
      if (showToast && window.App) App.loading.hide();
    }
  }

  window.simpleHPCRefreshDashboard = function (range) {
    return loadDashboard(true, range);
  };
  window.simpleHPCDashboardSnapshot = function () {
    return latestSnapshot;
  };

  document.addEventListener('DOMContentLoaded', function () {
    wireQueueTrendControls();
    loadDashboard(false);
    loadQueueJobTrends(false);
  });
}());
