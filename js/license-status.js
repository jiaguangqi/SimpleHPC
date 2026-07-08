(function () {
  'use strict';

  const STORAGE_APPS = 'simplehpc_license_apps_v1';
  const builtinApps = [
    { code: 'ansys', name: 'ANSYS', accent: '#007aff' },
    { code: 'abaqus', name: 'Abaqus', accent: '#34c759' },
    { code: 'comsol', name: 'COMSOL', accent: '#af52de' },
    { code: 'matlab', name: 'MATLAB', accent: '#ff9500' },
    { code: 'fluent', name: 'Fluent', accent: '#00a6a6' },
    { code: 'starccm', name: 'STAR-CCM+', accent: '#5856d6' },
    { code: 'gaussian', name: 'Gaussian', accent: '#ff3b30' },
    { code: 'materials-studio', name: 'Materials Studio', accent: '#64748b' }
  ];

  const state = {
    data: null,
    apps: loadAppCatalog(),
    selectedConfigId: '',
    selectedFeature: '',
    appFilter: '',
    featureFilter: '',
    statusFilter: '',
    userFilter: '',
    jobFilter: '',
    trendRange: '24h',
    detailFeatureFilter: '',
    loading: false,
    error: ''
  };

  const $ = (id) => document.getElementById(id);
  const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (ch) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[ch]));

  function loadAppCatalog() {
    try {
      const saved = JSON.parse(localStorage.getItem(STORAGE_APPS) || '[]');
      if (!Array.isArray(saved) || !saved.length) return builtinApps.map((item) => Object.assign({}, item));
      const savedMap = new Map(saved.filter((item) => item?.code).map((item) => [item.code, item]));
      const merged = builtinApps.map((item) => Object.assign({}, item, savedMap.get(item.code) || {}));
      saved.forEach((item) => {
        if (item?.code && !builtinApps.some((app) => app.code === item.code)) merged.push(Object.assign({}, item));
      });
      return merged;
    } catch (_) {
      return builtinApps.map((item) => Object.assign({}, item));
    }
  }

  async function request(path) {
    const fetcher = window.App?.apiFetch || fetch;
    const res = await fetcher(path);
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch (_) { data = { raw: text }; }
    if (!res.ok) throw new Error(data?.error || data?.message || res.statusText || '请求失败');
    return data;
  }

  function toast(message, type = 'info') {
    if (window.App?.toast) App.toast(message, type);
  }

  function num(value) {
    const parsed = Number(value || 0);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function percentText(used, total) {
    if (!total) return '0.0%';
    return `${(used * 100 / total).toFixed(1)}%`;
  }

  function percentValue(value) {
    if (typeof value === 'string' && value.endsWith('%')) return Number(value.slice(0, -1)) || 0;
    return Number(value || 0);
  }

  function formatTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', { hour12: false });
  }

  function formatShortTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false });
  }

  function durationFrom(value) {
    if (!value) return '—';
    const start = new Date(value).getTime();
    if (!start) return '—';
    const diff = Math.max(0, Date.now() - start);
    const minutes = Math.floor(diff / 60000);
    if (minutes < 1) return '< 1 分钟';
    const hours = Math.floor(minutes / 60);
    const rest = minutes % 60;
    if (!hours) return `${minutes} 分钟`;
    return `${hours} 小时 ${rest} 分钟`;
  }

  function rangeCutoff(range) {
    const hours = { '24h': 24, '7d': 24 * 7, '30d': 24 * 30, '90d': 24 * 90, '1y': 24 * 365 }[range] || 24;
    return Date.now() - hours * 60 * 60 * 1000;
  }

  function daysUntil(value) {
    if (!value) return null;
    const time = new Date(value).getTime();
    if (!time) return null;
    return Math.ceil((time - Date.now()) / 86400000);
  }

  function isFeatureExpiring(feature, cfg) {
    const days = daysUntil(feature.expiresAt);
    if (days === null) return false;
    return days <= num(cfg?.expireWarningDays || 30);
  }

  function featureStatus(feature, cfg) {
    const days = daysUntil(feature.expiresAt);
    if (days !== null && days < 0) return { value: 'expired', label: '已过期' };
    if (days !== null && days <= num(cfg?.expireWarningDays || 30)) return { value: 'expired', label: '即将到期' };
    const rate = percentValue(feature.usageRate);
    if (rate >= num(cfg?.criticalThreshold || 95)) return { value: 'full', label: '已满载' };
    if (rate >= num(cfg?.warningThreshold || 80)) return { value: 'warning', label: '高负载' };
    return { value: 'normal', label: '正常' };
  }

  function normalizeKey(value) {
    return String(value || '').trim().toLowerCase();
  }

  function appForConfig(item) {
    const appCode = normalizeKey(item?.appCode || item?.code);
    const appName = normalizeKey(item?.appName || item?.name);
    return state.apps.find((app) => {
      const code = normalizeKey(app.code);
      const name = normalizeKey(app.name);
      return (appCode && appCode === code) || (appName && appName === name);
    }) || null;
  }

  function appVisual(item) {
    const app = appForConfig(item);
    return Object.assign({}, item || {}, {
      appName: item?.appName || item?.name || app?.name || '应用',
      name: item?.name || item?.appName || app?.name || '应用',
      iconUrl: app?.iconUrl || item?.iconUrl || '',
      accent: app?.accent || item?.accent || '#007aff'
    });
  }

  function icon(item) {
    const visual = appVisual(item);
    if (visual.iconUrl) return `<img src="${esc(visual.iconUrl)}" alt="${esc(visual.appName || visual.name || '应用图标')}">`;
    return `<span>${esc((visual.appName || visual.name || 'L').slice(0, 1).toUpperCase())}</span>`;
  }

  function statusLabel(value) {
    return {
      active: '运行中',
      inactive: '未运行',
      failed: '失败',
      unmanaged: '未托管',
      unknown: '未知',
      running: '使用中',
      full: '已满载',
      warning: '高负载',
      normal: '正常',
      critical: '严重',
      info: '提示',
      expired: '即将到期',
      abnormal: '服务异常'
    }[value] || value || '未知';
  }

  function pill(value, label) {
    return `<span class="license-pill license-pill-${esc(value || 'unknown')}">${esc(label || statusLabel(value))}</span>`;
  }

  function configs() {
    return state.data?.configs || [];
  }

  function configById(id) {
    return configs().find((item) => String(item.id) === String(id));
  }

  function selectedConfig() {
    return configById(state.selectedConfigId);
  }

  function featuresForConfig(configId) {
    return (state.data?.features || []).filter((item) => String(item.configId) === String(configId));
  }

  function sessionsForConfig(configId) {
    return (state.data?.sessions || []).filter((item) => String(item.configId) === String(configId));
  }

  function alertsForConfig(configId) {
    const cfg = configById(configId);
    const appName = String(cfg?.appName || '').toLowerCase();
    return (state.data?.alerts || []).filter((alert) => (
      String(alert.configId || '') === String(configId) ||
      (appName && String(alert.appName || alert.title || alert.message || '').toLowerCase().includes(appName))
    ));
  }

  function appUsage(config) {
    const features = featuresForConfig(config.id);
    const total = features.reduce((sum, item) => sum + num(item.total), 0);
    const used = features.reduce((sum, item) => sum + num(item.used), 0);
    const free = features.reduce((sum, item) => sum + num(item.free), 0);
    const queued = features.reduce((sum, item) => sum + num(item.queued), 0);
    return { total, used, free: free || Math.max(0, total - used), queued, rate: total ? used * 100 / total : 0 };
  }

  function appHealth(config, usage) {
    if (['inactive', 'failed'].includes(config.serviceStatus)) return 'abnormal';
    if (featuresForConfig(config.id).some((feature) => isFeatureExpiring(feature, config))) return 'expired';
    if (usage.rate >= num(config.criticalThreshold || 95)) return 'full';
    if (usage.rate >= num(config.warningThreshold || 80)) return 'warning';
    return 'normal';
  }

  function ensureSelection() {
    const list = configs();
    if (!list.length) {
      state.selectedConfigId = '';
      return;
    }
    if (state.appFilter && configById(state.appFilter)) {
      state.selectedConfigId = String(state.appFilter);
      return;
    }
    if (!state.selectedConfigId || !configById(state.selectedConfigId)) {
      state.selectedConfigId = String(list[0].id);
    }
  }

  function selectedFeatureRows() {
    const cfg = selectedConfig();
    if (!cfg) return [];
    return featuresForConfig(cfg.id).filter((feature) => {
      const rate = percentValue(feature.usageRate);
      if (state.featureFilter && feature.featureName !== state.featureFilter) return false;
      if (state.selectedFeature && feature.featureName !== state.selectedFeature) return false;
      if (state.statusFilter === 'full' && rate < num(cfg.criticalThreshold || 95)) return false;
      if (state.statusFilter === 'running' && num(feature.used) <= 0) return false;
      if (state.statusFilter === 'abnormal' && !['inactive', 'failed'].includes(cfg.serviceStatus)) return false;
      if (state.statusFilter === 'expired' && !isFeatureExpiring(feature, cfg)) return false;
      return true;
    });
  }

  function filteredSessions() {
    const cfg = selectedConfig();
    const source = cfg ? sessionsForConfig(cfg.id) : (state.data?.sessions || []);
    const userQ = state.userFilter.toLowerCase();
    const jobQ = state.jobFilter.toLowerCase();
    return source.filter((session) => {
      if (state.appFilter && String(session.configId) !== state.appFilter) return false;
      if (state.featureFilter && session.featureName !== state.featureFilter) return false;
      if (state.selectedFeature && session.featureName !== state.selectedFeature) return false;
      if (userQ && !String(session.username || '').toLowerCase().includes(userQ)) return false;
      if (jobQ && !String(session.jobId || '').toLowerCase().includes(jobQ)) return false;
      return true;
    });
  }

  function rangeLabel(range) {
    return { '24h': '24 小时', '7d': '7 天', '30d': '30 天', '90d': '90 天', '1y': '1 年' }[range] || range;
  }

  function extractTrendPoints() {
    const cfg = selectedConfig();
    if (!cfg) return [];
    const pools = [
      state.data?.trends,
      state.data?.trendSamples,
      state.data?.samples,
      state.data?.history,
      state.data?.queueJobTrends,
      state.data?.licenseTrends
    ].filter(Array.isArray);
    const raw = pools.flat();
    if (!raw.length) return [];
    return raw.filter((point) => {
      const matchesConfig = String(point.configId || point.licenseConfigId || '') === String(cfg.id);
      const matchesApp = String(point.appName || point.app || '').toLowerCase() === String(cfg.appName || '').toLowerCase();
      const matchesFeature = !state.selectedFeature || String(point.featureName || point.feature || '') === state.selectedFeature;
      const pointTime = new Date(point.time || point.sampleTime || point.createdAt || point.collectedAt).getTime();
      const matchesRange = Number.isFinite(pointTime) && pointTime >= rangeCutoff(state.trendRange);
      return (matchesConfig || matchesApp) && matchesFeature && matchesRange;
    }).map((point) => ({
      time: point.time || point.sampleTime || point.createdAt || point.collectedAt,
      used: num(point.used ?? point.usedCount ?? point.usedLicenses),
      free: num(point.free ?? point.freeCount ?? point.freeLicenses),
      queued: num(point.queued ?? point.queuedCount ?? point.queueCount),
      total: num(point.total ?? point.totalCount ?? point.totalLicenses)
    })).filter((point) => point.time).sort((a, b) => new Date(a.time) - new Date(b.time));
  }

  function linePath(points, key, x, y) {
    return points.map((point, index) => `${index ? 'L' : 'M'}${x(index).toFixed(1)},${y(point[key]).toFixed(1)}`).join(' ');
  }

  function overviewIcon(name) {
    return {
      apps: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="4" width="7" height="7" rx="2"></rect><rect x="13" y="4" width="7" height="7" rx="2"></rect><rect x="4" y="13" width="7" height="7" rx="2"></rect><rect x="13" y="13" width="7" height="7" rx="2"></rect></svg>',
      total: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16"></path><path d="M4 12h16"></path><path d="M4 17h10"></path><circle cx="18" cy="17" r="2"></circle></svg>',
      used: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 20V10"></path><path d="M12 20V4"></path><path d="M17 20v-7"></path></svg>',
      rate: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M19 5 5 19"></path><circle cx="7" cy="7" r="2"></circle><circle cx="17" cy="17" r="2"></circle></svg>',
      hot: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3c3 3 5 5.6 5 9a5 5 0 0 1-10 0c0-2 1-3.7 3-5-.2 2.2.6 3.6 2 4.4C13.4 9.6 13.7 6.7 12 3Z"></path></svg>',
      abnormal: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 8v5"></path><path d="M12 17h.01"></path><path d="M10.3 4.2 2.8 17.4A2 2 0 0 0 4.5 20h15a2 2 0 0 0 1.7-2.6L13.7 4.2a2 2 0 0 0-3.4 0Z"></path></svg>'
    }[name] || '';
  }

  function zeroTrendPoints(range) {
    const now = Date.now();
    const start = rangeCutoff(range);
    return [0, 0.25, 0.5, 0.75, 1].map((ratio) => ({
      time: new Date(start + (now - start) * ratio).toISOString(),
      total: 0,
      used: 0,
      free: 0,
      queued: 0
    }));
  }

  function renderTrendSvg(points, cfg, empty) {
    const width = 900;
    const height = 320;
    const pad = { left: 54, right: 24, top: 28, bottom: 42 };
    const maxY = Math.max(1, ...points.flatMap((point) => [point.total, point.used, point.free, point.queued]));
    const x = (index) => pad.left + (points.length === 1 ? 0 : index * (width - pad.left - pad.right) / (points.length - 1));
    const y = (value) => height - pad.bottom - (num(value) / maxY) * (height - pad.top - pad.bottom);
    const ticks = [0, 0.25, 0.5, 0.75, 1].map((ratio) => Math.round(maxY * ratio));
    const timeLabels = points.length > 1 ? [points[0], points[Math.floor(points.length / 2)], points[points.length - 1]] : points;
    return `
      <div class="license-trend-legend">
        <span><i class="used"></i>已用 License</span>
        <span><i class="free"></i>空闲 License</span>
        <span><i class="queued"></i>排队数量</span>
        <span><i class="total"></i>总点数</span>
        ${empty ? '<em>暂无采样记录，当前显示 0 值基线</em>' : ''}
      </div>
      <svg class="license-trend-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="${esc(cfg.appName)} License 趋势">
        ${ticks.map((tick) => `<line x1="${pad.left}" y1="${y(tick).toFixed(1)}" x2="${width - pad.right}" y2="${y(tick).toFixed(1)}" class="grid"></line><text x="${pad.left - 12}" y="${(y(tick) + 4).toFixed(1)}" class="axis" text-anchor="end">${tick}</text>`).join('')}
        ${timeLabels.map((point) => `<text x="${x(points.indexOf(point)).toFixed(1)}" y="${height - 10}" class="axis" text-anchor="middle">${esc(formatShortTime(point.time))}</text>`).join('')}
        <path d="${linePath(points, 'total', x, y)}" class="line total"></path>
        <path d="${linePath(points, 'used', x, y)}" class="line used"></path>
        <path d="${linePath(points, 'free', x, y)}" class="line free"></path>
        <path d="${linePath(points, 'queued', x, y)}" class="line queued"></path>
        ${points.map((point, index) => `<circle cx="${x(index).toFixed(1)}" cy="${y(point.used).toFixed(1)}" r="3.5" class="point used"><title>${esc(formatTime(point.time))} · 已用 ${point.used} · 空闲 ${point.free} · 排队 ${point.queued}</title></circle>`).join('')}
      </svg>`;
  }

  function renderTrend() {
    const cfg = selectedConfig();
    const title = $('licenseTrendTitle');
    const meta = $('licenseTrendMeta');
    const sample = $('licenseTrendSample');
    const chart = $('licenseTrendChart');
    document.querySelectorAll('.license-range-btn').forEach((btn) => btn.classList.toggle('active', btn.dataset.licenseRange === state.trendRange));

    if (!cfg) {
      title.textContent = 'License 使用趋势';
      meta.textContent = '暂无 License 配置。';
      sample.textContent = '采样：—';
      chart.innerHTML = '<div class="license-trend-empty"><strong>暂无 License 趋势数据</strong><span>请先配置 License Server 并完成采集。</span></div>';
      return;
    }

    title.textContent = `${cfg.appName} License 使用趋势`;
    meta.textContent = state.selectedFeature ? `当前 Feature：${state.selectedFeature}` : `${cfg.licenseType || cfg.managerName || 'License'} · ${cfg.serverHost || cfg.serverAddress || '—'}:${cfg.serverPort || cfg.port || '—'}`;
    const points = extractTrendPoints();
    sample.textContent = points.length ? `采样粒度：${rangeLabel(state.trendRange)} · ${points.length} 点` : `采样粒度：${rangeLabel(state.trendRange)} · 0 点`;

    if (!points.length) {
      chart.innerHTML = renderTrendSvg(zeroTrendPoints(state.trendRange), cfg, true);
      return;
    }
    chart.innerHTML = renderTrendSvg(points, cfg, false);
  }

  function renderOverview() {
    if (state.error) {
      $('licenseOverviewCards').innerHTML = `
        <div class="license-inline-error license-status-error">
          <strong>License 状态加载失败</strong>
          <span>${esc(state.error)}</span>
          <button class="btn btn-ghost" type="button" id="licenseStatusRetry">重新加载</button>
        </div>`;
      $('licenseStatusRetry')?.addEventListener('click', () => loadStatus().catch((err) => toast('License 状态未获取：' + err.message, 'danger')));
      return;
    }
    const overview = state.data?.overview || {};
    const cards = [
      ['apps', '已接入软件', num(overview.appCount), 'License Server 数', '#007AFF'],
      ['total', 'License 总点数', num(overview.totalLicenses).toLocaleString(), `空闲 ${num(overview.freeLicenses).toLocaleString()}`, '#AF52DE'],
      ['used', '当前使用中', num(overview.usedLicenses).toLocaleString(), `排队 ${num(overview.queuedCount)}`, '#34C759'],
      ['rate', '当前使用率', overview.usageRate || percentText(num(overview.usedLicenses), num(overview.totalLicenses)), '全部 Feature 聚合', '#FF9500'],
      ['hot', '高负载软件', num(overview.highLoadApps), '使用率 ≥ 80%', '#FF3B30'],
      ['abnormal', '异常服务', num(overview.abnormalServer), '服务非 active', '#FF3B30']
    ];
    $('licenseOverviewCards').innerHTML = cards.map(([iconName, title, value, desc, color]) => `
      <article class="license-overview-card" style="--license-color:${color}">
        <span class="license-card-icon">${overviewIcon(iconName)}</span>
        <div><p>${esc(title)}</p><strong>${esc(value)}</strong><small>${esc(desc)}</small></div>
      </article>`).join('');
  }

  function populateFilters() {
    const appOptions = ['<option value="">全部软件</option>'].concat(configs().map((cfg) => `<option value="${esc(cfg.id)}">${esc(cfg.appName)}</option>`));
    $('licenseStatusAppFilter').innerHTML = appOptions.join('');
    $('licenseStatusAppFilter').value = state.appFilter;

    const features = Array.from(new Set((state.data?.features || []).map((item) => item.featureName).filter(Boolean))).sort();
    $('licenseStatusFeatureFilter').innerHTML = ['<option value="">全部 Feature</option>'].concat(features.map((name) => `<option value="${esc(name)}">${esc(name)}</option>`)).join('');
    $('licenseStatusFeatureFilter').value = state.featureFilter;
  }

  function selectConfig(configId, syncFilter = true) {
    state.selectedConfigId = String(configId || '');
    state.selectedFeature = '';
    if (syncFilter) {
      state.appFilter = state.selectedConfigId;
      const appFilter = $('licenseStatusAppFilter');
      if (appFilter) appFilter.value = state.appFilter;
    }
    renderAll(false);
  }

  function renderAppList() {
    const items = configs();
    if (!items.length) {
      $('licenseAppList').innerHTML = '<div class="empty-state compact">暂无 License 配置。请先到“应用许可配置”中添加 License Server。</div>';
      return;
    }
    $('licenseAppList').innerHTML = items.map((cfg) => {
      const usage = appUsage(cfg);
      const health = appHealth(cfg, usage);
      const featureCount = featuresForConfig(cfg.id).length;
      const sessionCount = sessionsForConfig(cfg.id).length;
      const active = String(cfg.id) === String(state.selectedConfigId) ? ' active' : '';
      return `
        <article class="license-app-card${active}" data-id="${esc(cfg.id)}">
          <span class="license-app-icon">${icon(cfg)}</span>
          <span class="license-app-text">
            <strong>${esc(cfg.appName)}</strong>
            <small>${esc(cfg.licenseType || cfg.managerName || 'License')} · Feature ${featureCount} · 占用会话 ${sessionCount}</small>
            <span class="license-progress"><i style="width:${Math.min(100, usage.rate).toFixed(1)}%"></i></span>
            <span class="license-app-metrics">
              <em>总 ${usage.total}</em><em>用 ${usage.used}</em><em>闲 ${usage.free}</em><em>排 ${usage.queued}</em>
            </span>
          </span>
          <span class="license-app-side">
            <b>${usage.rate.toFixed(1)}%</b>
            ${pill(health)}
            <button class="license-detail-link" type="button" data-detail-id="${esc(cfg.id)}">详情</button>
          </span>
        </article>`;
    }).join('');
    $('licenseAppList').querySelectorAll('.license-app-card').forEach((card) => {
      card.addEventListener('click', () => selectConfig(card.dataset.id));
    });
    $('licenseAppList').querySelectorAll('[data-detail-id]').forEach((button) => {
      button.addEventListener('click', (event) => {
        event.stopPropagation();
        openDetail(button.dataset.detailId);
      });
    });
  }

  function renderFeatureTable() {
    const cfg = selectedConfig();
    const rows = selectedFeatureRows();
    if (!cfg) {
      $('licenseFeatureSummary').textContent = '暂无已选软件。';
      $('licenseFeatureRows').innerHTML = '<tr><td colspan="7" class="empty-cell">暂无 Feature 数据</td></tr>';
      return;
    }
    $('licenseFeatureSummary').textContent = `${cfg.appName} 共 ${featuresForConfig(cfg.id).length} 个 Feature，点击行可按 Feature 聚焦趋势和占用明细。`;
    if (!rows.length) {
      $('licenseFeatureRows').innerHTML = '<tr><td colspan="7" class="empty-cell">当前筛选条件下暂无 Feature 数据</td></tr>';
      return;
    }
    $('licenseFeatureRows').innerHTML = rows.map((item) => {
      const rate = percentValue(item.usageRate);
      const status = featureStatus(item, cfg);
      const active = state.selectedFeature === item.featureName ? ' class="active"' : '';
      return `<tr${active} data-feature-name="${esc(item.featureName)}">
        <td><strong>${esc(item.featureName)}</strong></td>
        <td>${num(item.total)}</td>
        <td>${num(item.used)}</td>
        <td>${num(item.free)}</td>
        <td>${esc(item.usageRate || percentText(num(item.used), num(item.total)))}</td>
        <td>${num(item.queued)}</td>
        <td>${pill(status.value, status.label)}</td>
      </tr>`;
    }).join('');
    $('licenseFeatureRows').querySelectorAll('[data-feature-name]').forEach((row) => {
      row.addEventListener('click', () => {
        state.selectedFeature = state.selectedFeature === row.dataset.featureName ? '' : row.dataset.featureName;
        renderTrend();
        renderFeatureTable();
        renderSessions();
      });
    });
  }

  function renderSessions() {
    const rows = filteredSessions();
    if (!rows.length) {
      $('licenseSessionRows').innerHTML = '<tr><td colspan="8" class="empty-cell">暂无当前占用明细</td></tr>';
      return;
    }
    $('licenseSessionRows').innerHTML = rows.slice(0, 120).map((item) => `<tr>
      <td>${esc(item.appName)}</td>
      <td>${esc(item.featureName)}</td>
      <td>${esc(item.username || '—')}</td>
      <td>${esc(item.jobId || '—')}</td>
      <td>${esc(item.nodeName || item.hostName || '—')}</td>
      <td>${num(item.checkoutCount) || 1}</td>
      <td>${durationFrom(item.startedAt || item.lastSeenAt)}</td>
      <td>${pill(item.status || 'running')}</td>
    </tr>`).join('');
  }

  function renderAlerts() {
    const cfg = selectedConfig();
    const alerts = cfg ? alertsForConfig(cfg.id) : (state.data?.alerts || []);
    if (!alerts.length) {
      $('licenseAlertRows').innerHTML = '<div class="empty-state compact">当前暂无 License 告警。</div>';
      return;
    }
    $('licenseAlertRows').innerHTML = alerts.slice(0, 10).map((alert) => `
      <article class="license-alert-item ${esc(alert.level || 'info')}">
        <span class="level">${esc(alert.level || 'info')}</span>
        <div><strong>${esc(alert.title || 'License 告警')}</strong><p>${esc(alert.message || '')}</p><small>${formatTime(alert.occurredAt || alert.createdAt)}</small></div>
      </article>`).join('');
  }

  function renderDetailChart(features) {
    if (!features.length) return '<div class="empty-state compact">暂无 Feature 分布数据</div>';
    const max = Math.max(1, ...features.map((item) => Math.max(num(item.total), num(item.used) + num(item.free) + num(item.queued))));
    return `
      <div class="license-chart-legend">
        <span><i class="used"></i>使用中</span><span><i class="free"></i>空闲</span><span><i class="queued"></i>排队</span>
      </div>
      <div class="license-chart-bars">
        ${features.slice(0, 10).map((item) => {
          const used = num(item.used);
          const free = num(item.free);
          const queued = num(item.queued);
          return `
            <div class="license-chart-row" title="${esc(item.appName)} · ${esc(item.featureName)}">
              <label class="license-chart-name">${esc(item.featureName)}</label>
              <span class="license-chart-stack">
                <span class="used" style="width:${(used / max * 100).toFixed(1)}%"></span>
                <span class="free" style="width:${(free / max * 100).toFixed(1)}%"></span>
                <span class="queued" style="width:${(queued / max * 100).toFixed(1)}%"></span>
              </span>
              <b>${used}/${num(item.total)}</b>
            </div>`;
        }).join('')}
      </div>`;
  }

  function renderDetail() {
    const cfg = selectedConfig();
    if (!cfg) return;
    const usage = appUsage(cfg);
    const allFeatures = featuresForConfig(cfg.id);
    const q = state.detailFeatureFilter.toLowerCase();
    const matchedFeatures = allFeatures.filter((item) => !q || String(item.featureName || '').toLowerCase().includes(q));
    const visibleFeatures = matchedFeatures.slice(0, q ? 500 : 240);
    const sessions = sessionsForConfig(cfg.id);
    const alerts = alertsForConfig(cfg.id);

    $('licenseDetailIcon').innerHTML = icon(cfg);
    $('licenseDetailTitle').textContent = `${cfg.appName} License 详情`;
    $('licenseDetailMeta').textContent = `${cfg.licenseType || cfg.managerName || 'License'} · ${cfg.serverHost || cfg.serverAddress || '—'}:${cfg.serverPort || cfg.port || '—'} · 最后采集 ${formatTime(cfg.lastCollectedAt || state.data?.overview?.lastUpdated)}`;
    $('licenseDetailMetrics').innerHTML = [
      ['总点数', usage.total],
      ['使用中', usage.used],
      ['空闲', usage.free],
      ['排队', usage.queued],
      ['使用率', `${usage.rate.toFixed(1)}%`]
    ].map(([label, value]) => `<div><span>${esc(label)}</span><strong>${esc(value)}</strong></div>`).join('');
    $('licenseDetailFeatureHint').textContent = `Feature 共 ${allFeatures.length} 项，当前展示 ${visibleFeatures.length} / ${matchedFeatures.length} 项；可输入 Feature 名称快速筛选。`;
    $('licenseDetailChart').innerHTML = renderDetailChart(matchedFeatures);
    $('licenseDetailFeatureRows').innerHTML = visibleFeatures.length ? visibleFeatures.map((item) => {
      const status = featureStatus(item, cfg);
      return `<tr>
        <td><strong>${esc(item.featureName)}</strong></td>
        <td>${num(item.total)}</td>
        <td>${num(item.used)}</td>
        <td>${num(item.free)}</td>
        <td>${esc(item.usageRate || percentText(num(item.used), num(item.total)))}</td>
        <td>${num(item.queued)}</td>
        <td>${pill(status.value, status.label)}</td>
      </tr>`;
    }).join('') : '<tr><td colspan="7" class="empty-cell">暂无匹配 Feature</td></tr>';
    $('licenseDetailSessionRows').innerHTML = sessions.length ? sessions.slice(0, 120).map((item) => `<tr>
      <td>${esc(item.username || '—')}</td>
      <td>${esc(item.jobId || '—')}</td>
      <td>${esc(item.featureName || '—')}</td>
      <td>${esc(item.nodeName || item.hostName || '—')}</td>
      <td>${num(item.checkoutCount) || 1}</td>
      <td>${durationFrom(item.startedAt || item.lastSeenAt)}</td>
    </tr>`).join('') : '<tr><td colspan="6" class="empty-cell">当前暂无占用会话</td></tr>';
    $('licenseDetailAlerts').innerHTML = alerts.length ? alerts.slice(0, 6).map((alert) => `
      <article class="license-alert-item ${esc(alert.level || 'info')}">
        <span class="level">${esc(alert.level || 'info')}</span>
        <div><strong>${esc(alert.title || 'License 告警')}</strong><p>${esc(alert.message || '')}</p><small>${formatTime(alert.occurredAt || alert.createdAt)}</small></div>
      </article>`).join('') : '<div class="empty-state compact">当前软件暂无告警。</div>';
  }

  function openDetail(configId) {
    state.selectedConfigId = String(configId);
    state.detailFeatureFilter = '';
    $('licenseDetailFeatureSearch').value = '';
    renderDetail();
    const modal = $('licenseDetailModal');
    modal.hidden = false;
    modal.setAttribute('aria-hidden', 'false');
    document.body.classList.add('modal-open');
  }

  function closeDetail() {
    const modal = $('licenseDetailModal');
    modal.hidden = true;
    modal.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('modal-open');
  }

  function renderAll(rebuildFilters = true) {
    if (state.error) {
      renderOverview();
      $('licenseAppList').innerHTML = '<div class="empty-state compact">状态加载失败，无法展示监控软件。</div>';
      $('licenseTrendChart').innerHTML = '<div class="license-trend-empty"><strong>暂无趋势</strong><span>请重新加载 License 状态。</span></div>';
      $('licenseFeatureRows').innerHTML = '<tr><td colspan="7" class="empty-cell">状态加载失败</td></tr>';
      $('licenseSessionRows').innerHTML = '<tr><td colspan="8" class="empty-cell">状态加载失败</td></tr>';
      $('licenseAlertRows').innerHTML = '<div class="empty-state compact">状态加载失败，无法展示告警。</div>';
      $('licenseStatusUpdated').textContent = '最后更新：加载失败';
      return;
    }
    ensureSelection();
    renderOverview();
    if (rebuildFilters) populateFilters();
    renderAppList();
    renderTrend();
    renderFeatureTable();
    renderSessions();
    renderAlerts();
    if (!$('licenseDetailModal').hidden) renderDetail();
    $('licenseStatusUpdated').textContent = `最后更新：${formatTime(state.data?.overview?.lastUpdated || new Date().toISOString())}`;
  }

  async function loadStatus() {
    if (state.loading) return;
    state.loading = true;
    state.error = '';
    const refreshButtons = [$('licenseStatusRefresh'), $('licenseStatusRefreshInline')].filter(Boolean);
    refreshButtons.forEach((button) => {
      if (!button.dataset.defaultText) button.dataset.defaultText = button.textContent;
      button.disabled = true;
      button.textContent = button.id === 'licenseStatusRefresh' ? '⟳' : '刷新中...';
    });
    $('licenseOverviewCards').innerHTML = '<div class="empty-state compact">正在加载 License 状态...</div>';
    try {
      state.data = await request('/api/v1/license/status');
      renderAll(true);
    } catch (err) {
      state.data = null;
      state.error = err.message || '请求失败';
      renderAll(false);
      throw err;
    } finally {
      state.loading = false;
      refreshButtons.forEach((button) => {
        button.disabled = false;
        button.textContent = button.dataset.defaultText;
      });
    }
  }

  function bind() {
    const reload = () => loadStatus().catch((err) => toast('License 状态未获取：' + err.message, 'danger'));
    $('licenseStatusRefresh').addEventListener('click', reload);
    $('licenseStatusRefreshInline').addEventListener('click', reload);
    $('licenseStatusAppFilter').addEventListener('change', (event) => {
      state.appFilter = event.target.value;
      state.selectedFeature = '';
      if (state.appFilter) state.selectedConfigId = state.appFilter;
      renderAll(false);
    });
    $('licenseStatusFeatureFilter').addEventListener('change', (event) => {
      state.featureFilter = event.target.value;
      state.selectedFeature = event.target.value;
      renderAll(false);
    });
    $('licenseStatusStateFilter').addEventListener('change', (event) => { state.statusFilter = event.target.value; renderAll(false); });
    $('licenseStatusUserFilter').addEventListener('input', (event) => { state.userFilter = event.target.value.trim(); renderSessions(); });
    $('licenseStatusJobFilter').addEventListener('input', (event) => { state.jobFilter = event.target.value.trim(); renderSessions(); });
    document.querySelectorAll('.license-range-btn').forEach((button) => {
      button.addEventListener('click', () => {
        state.trendRange = button.dataset.licenseRange;
        renderTrend();
      });
    });
    $('licenseDetailFeatureSearch').addEventListener('input', (event) => {
      state.detailFeatureFilter = event.target.value.trim();
      renderDetail();
    });
    document.querySelectorAll('[data-license-detail-close]').forEach((el) => el.addEventListener('click', closeDetail));
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && !$('licenseDetailModal').hidden) closeDetail();
    });
    window.addEventListener('storage', (event) => {
      if (event.key !== STORAGE_APPS) return;
      state.apps = loadAppCatalog();
      renderAll(false);
    });
  }

  document.addEventListener('DOMContentLoaded', () => {
    bind();
    loadStatus().catch((err) => toast('License 状态未获取：' + err.message, 'danger'));
  });
})();
