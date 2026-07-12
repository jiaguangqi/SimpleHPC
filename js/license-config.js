(function () {
  'use strict';

  const STORAGE_APPS = 'simplehpc_license_apps_v1';
  const STORAGE_MANAGERS = 'simplehpc_license_managers_v1';

  const builtinApps = [
    { code: 'ansys', name: 'ANSYS', type: 'CAE', vendor: 'ANSYS, Inc.', iconUrl: '', accent: '#007aff' },
    { code: 'abaqus', name: 'Abaqus', type: 'CAE', vendor: 'Dassault Systemes', iconUrl: '', accent: '#34c759' },
    { code: 'comsol', name: 'COMSOL', type: 'Multiphysics', vendor: 'COMSOL', iconUrl: '', accent: '#af52de' },
    { code: 'matlab', name: 'MATLAB', type: 'Math', vendor: 'MathWorks', iconUrl: '', accent: '#ff9500' },
    { code: 'fluent', name: 'Fluent', type: 'CFD', vendor: 'ANSYS, Inc.', iconUrl: '', accent: '#00a6a6' },
    { code: 'starccm', name: 'STAR-CCM+', type: 'CFD', vendor: 'Siemens', iconUrl: '', accent: '#5856d6' },
    { code: 'gaussian', name: 'Gaussian', type: 'Chemistry', vendor: 'Gaussian, Inc.', iconUrl: '', accent: '#ff3b30' },
    { code: 'materials-studio', name: 'Materials Studio', type: 'Materials', vendor: 'BIOVIA', iconUrl: '', accent: '#64748b' }
  ];

  const builtinManagers = [
    { code: 'lmstat', name: 'FlexNet lmstat', licenseType: 'FlexNet', executablePath: 'lmstat', defaultArgs: '-a -c', collectMethod: 'lmstat' },
    { code: 'lmutil-lmstat', name: 'FlexNet lmutil lmstat', licenseType: 'FlexNet', executablePath: 'lmutil', defaultArgs: 'lmstat -a -c', collectMethod: 'lmutil' },
    { code: 'rlmutil', name: 'RLM rlmutil', licenseType: 'RLM', executablePath: 'rlmutil', defaultArgs: 'rlmstat -a -c', collectMethod: 'rlmutil' }
  ];

  const licenseTabs = new Set(['apps', 'managers', 'licenses', 'logs']);

  function licenseTabFromURL() {
    const tab = new URLSearchParams(location.search).get('tab') || 'apps';
    return licenseTabs.has(tab) ? tab : 'apps';
  }

  const state = {
    activeTab: licenseTabFromURL(),
    selectedLicenseId: '',
    inlineDraft: null,
    inlineTest: null,
    configs: [],
    apps: loadCatalog(STORAGE_APPS, builtinApps),
    managers: loadCatalog(STORAGE_MANAGERS, builtinManagers, true),
    loading: false,
    error: '',
    busy: ''
  };

  const $ = (id) => document.getElementById(id);
  const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));

  function loadCatalog(key, fallback, preserveSavedOrder = false) {
    try {
      const saved = JSON.parse(localStorage.getItem(key) || '[]');
      if (!Array.isArray(saved) || !saved.length) return fallback.map((item) => Object.assign({}, item));
      const fallbackMap = new Map(fallback.map((item) => [item.code, item]));
      const savedMap = new Map(saved.filter((item) => item?.code).map((item) => [item.code, item]));
      if (preserveSavedOrder) {
        const merged = saved.map((item) => Object.assign({}, fallbackMap.get(item.code) || {}, item));
        fallback.forEach((item) => {
          if (!savedMap.has(item.code)) merged.push(Object.assign({}, item));
        });
        return merged;
      }
      const merged = fallback.map((item) => Object.assign({}, item, savedMap.get(item.code) || {}));
      saved.forEach((item) => {
        if (item?.code && !fallbackMap.has(item.code)) merged.push(Object.assign({}, item));
      });
      return merged;
    } catch (_) {
      return fallback.map((item) => Object.assign({}, item));
    }
  }

  function persistCatalog(key, items) {
    localStorage.setItem(key, JSON.stringify(items));
  }

  async function request(path, options = {}) {
    const headers = Object.assign({ 'Content-Type': 'application/json' }, options.headers || {});
    const fetcher = window.App?.apiFetch || fetch;
    const res = await fetcher(path, Object.assign({}, options, { headers }));
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch (_) { data = { raw: text }; }
    if (!res.ok) throw new Error(data?.error || data?.message || res.statusText || '请求失败');
    return data;
  }

  function toast(message, type = 'info') {
    if (window.App?.toast) App.toast(message, type);
  }

  function confirmAction(message, options) {
    if (window.App?.confirm) {
      App.confirm(message, options);
      return;
    }
    if (window.confirm(message) && options?.onConfirm) options.onConfirm();
  }

  function slug(value) {
    return String(value || '').trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'custom';
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
    const preferItemIcon = !!item?.preferItemIcon;
    return Object.assign({}, item || {}, {
      name: item?.name || item?.appName || app?.name || '应用',
      appName: item?.appName || item?.name || app?.name || '应用',
      iconUrl: preferItemIcon ? (item?.iconUrl || app?.iconUrl || '') : (app?.iconUrl || item?.iconUrl || ''),
      accent: app?.accent || item?.accent || '#007aff'
    });
  }

  function appIcon(item) {
    const visual = appVisual(item);
    if (visual.iconUrl) return `<img src="${esc(visual.iconUrl)}" alt="${esc(visual.name || visual.appName || '应用图标')}">`;
    const color = visual.accent || '#007aff';
    const label = (visual.name || visual.appName || 'L').slice(0, 1).toUpperCase();
    return `<span class="license-template-icon" style="--license-accent:${esc(color)}">${esc(label)}</span>`;
  }

  function iconPreviewMarkup(name, iconUrl, accent) {
    return appIcon({ name, appName: name, iconUrl, accent, preferItemIcon: true });
  }

  function validateIconFile(file) {
    if (!file) return;
    const allowedTypes = new Set(['image/png', 'image/jpeg', 'image/svg+xml', 'image/webp', 'image/gif']);
    const allowedExt = /\.(png|jpe?g|svg|webp|gif)$/i;
    if (!allowedTypes.has(file.type) && !allowedExt.test(file.name || '')) {
      throw new Error('图标文件仅支持 PNG、SVG、JPG、JPEG、WebP、GIF 格式');
    }
    if (file.size > 1024 * 1024) {
      throw new Error('图标文件不能超过 1 MB，请先压缩后再上传');
    }
  }

  function readIconFile(file) {
    validateIconFile(file);
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ''));
      reader.onerror = () => reject(new Error('图标文件读取失败，请重新选择'));
      reader.readAsDataURL(file);
    });
  }

  function managerIcon(type) {
    const label = String(type || 'License').toLowerCase();
    if (label.includes('rlm')) return '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 6h9a5 5 0 0 1 0 10H5"></path><path d="M9 10v10"></path><path d="m14 16 5 4"></path></svg>';
    if (label.includes('sentinel')) return '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.2 2.8 7.8 7 10 4.2-2.2 7-5.8 7-10V6l-7-3Z"></path><path d="m9 12 2 2 4-5"></path></svg>';
    return '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="5" width="16" height="14" rx="3"></rect><path d="M8 9h8"></path><path d="M8 13h5"></path><path d="M16 15.5 18 17.5 21 13"></path></svg>';
  }

  function statusPill(status) {
    const value = status || 'unknown';
    const label = { active: '运行中', inactive: '未运行', failed: '失败', unmanaged: '未托管', unknown: '未知' }[value] || value;
    return `<span class="license-pill license-pill-${esc(value)}">${esc(label)}</span>`;
  }

  function setBusy(action) {
    state.busy = action || '';
    document.querySelectorAll('[data-license-action], #licenseRefreshBtn, #licensePrimaryActionBtn, #licenseAddAppBtn, #licenseAddManagerBtn, #licenseLogsRefreshBtn').forEach((button) => {
      if (!button.dataset.defaultText) button.dataset.defaultText = button.textContent;
      button.disabled = !!action;
      if (button.dataset.busyText && button.dataset.actionName === action) {
        button.textContent = button.dataset.busyText;
      } else {
        button.textContent = button.dataset.defaultText;
      }
    });
  }

  async function withBusy(action, task) {
    if (state.busy) return null;
    setBusy(action);
    try {
      return await task();
    } finally {
      setBusy('');
    }
  }

  function activeManager() {
    return state.managers[0] || builtinManagers[0];
  }

  function licenseDefaults() {
    const app = state.apps[0] || builtinApps[0];
    const manager = activeManager();
    return {
      id: 0,
      appName: app.name,
      appCode: app.code,
      appType: app.type,
      iconUrl: app.iconUrl || '',
      vendor: app.vendor || '',
      licenseType: manager.licenseType || 'FlexNet',
      managerName: manager.name,
      serverHost: '',
      port: 1055,
      collectMethod: manager.collectMethod || 'lmstat',
      collectCommand: '',
      serviceName: '',
      collectIntervalSec: 60,
      timeoutSec: 10,
      warningThreshold: 80,
      criticalThreshold: 95,
      expireWarningDays: 30,
      enabled: true
    };
  }

  function buildCommand(manager, host, port) {
    if (!manager) return '';
    const target = port ? `${port}@${host}` : host;
    return `${manager.executablePath || manager.collectMethod || 'lmstat'} ${manager.defaultArgs || '-a -c'} ${target}`.trim();
  }

  function renderMetrics() {
    const total = state.configs.length;
    const enabled = state.configs.filter((item) => item.enabled).length;
    const abnormal = state.configs.filter((item) => ['failed', 'inactive', 'unknown'].includes(item.serviceStatus)).length;
    $('licenseConfigTotal').textContent = total;
    $('licenseConfigEnabled').textContent = enabled;
    $('licenseConfigAbnormal').textContent = abnormal;
  }

  function syncLicenseTabURL(replace) {
    const url = new URL(location.href);
    if (state.activeTab === 'apps') url.searchParams.delete('tab');
    else url.searchParams.set('tab', state.activeTab);
    const next = url.pathname + url.search + url.hash;
    const current = location.pathname + location.search + location.hash;
    if (next === current) return;
    history[replace ? 'replaceState' : 'pushState']({ licenseTab: state.activeTab }, '', next);
  }

  function setTab(tab, options) {
    tab = licenseTabs.has(tab) ? tab : 'apps';
    state.activeTab = tab;
    document.querySelectorAll('.license-tab').forEach((button) => {
      const active = button.dataset.licenseTab === tab;
      button.classList.toggle('active', active);
      button.setAttribute('aria-selected', String(active));
    });
    $('licenseAppsPanel').hidden = tab !== 'apps';
    $('licenseManagersPanel').hidden = tab !== 'managers';
    $('licenseServersPanel').hidden = tab !== 'licenses';
    $('licenseLogsPanel').hidden = tab !== 'logs';
    $('licenseConfigEnabledFilter').style.display = tab === 'licenses' ? '' : 'none';
    const primaryText = tab === 'apps' ? '添加应用' : tab === 'managers' ? '添加管理器' : tab === 'logs' ? '刷新日志' : '新增许可证';
    $('licensePrimaryActionBtn').textContent = primaryText;
    delete $('licensePrimaryActionBtn').dataset.defaultText;
    $('licenseConfigSearch').placeholder = tab === 'apps' ? '搜索应用名称、厂商、分类...' : tab === 'managers' ? '搜索管理器、路径、类型...' : tab === 'logs' ? '搜索应用、采集状态、错误信息...' : '搜索应用、管理器、License Server...';
    if (options?.writeURL) syncLicenseTabURL(!!options.replace);
    renderAll();
  }

  function filteredApps() {
    const q = $('licenseConfigSearch').value.trim().toLowerCase();
    return state.apps.filter((item) => !q || [item.name, item.code, item.type, item.vendor].some((value) => String(value || '').toLowerCase().includes(q)));
  }

  function filteredManagers() {
    const q = $('licenseConfigSearch').value.trim().toLowerCase();
    return state.managers.filter((item) => !q || [item.name, item.code, item.licenseType, item.executablePath, item.defaultArgs].some((value) => String(value || '').toLowerCase().includes(q)));
  }

  function filteredConfigs() {
    const q = $('licenseConfigSearch').value.trim().toLowerCase();
    const enabled = $('licenseConfigEnabledFilter').value;
    return state.configs.filter((item) => {
      if (enabled && String(!!item.enabled) !== enabled) return false;
      if (!q) return true;
      return [item.appName, item.appCode, item.vendor, item.managerName, item.serverHost, item.licenseType].some((value) => String(value || '').toLowerCase().includes(q));
    });
  }

  function renderApps() {
    const items = filteredApps();
    $('licenseAppCatalog').innerHTML = items.length ? items.map((item) => `
      <article class="license-template-card">
        <span class="license-app-icon">${appIcon(item)}</span>
        <div class="license-template-main">
          <strong>${esc(item.name)}</strong>
          <small>${esc(item.type || '应用软件')} · ${esc(item.vendor || '未填写厂商')}</small>
          <span class="license-template-meta">${state.configs.filter((cfg) => String(cfg.appCode || cfg.appName).toLowerCase() === String(item.code || item.name).toLowerCase() || String(cfg.appName || '').toLowerCase() === String(item.name || '').toLowerCase()).length} 个许可证配置</span>
        </div>
        <div class="license-template-actions">
          <button class="btn btn-ghost" type="button" data-license-action="app-edit" data-code="${esc(item.code)}">编辑</button>
          <button class="btn btn-ghost" type="button" data-license-action="license-from-app" data-code="${esc(item.code)}">配置许可证</button>
        </div>
      </article>`).join('') : '<div class="empty-state compact">暂无匹配应用，可点击“添加应用”。</div>';
  }

  function renderManagers() {
    const items = filteredManagers();
    $('licenseManagerCatalog').innerHTML = items.length ? `
      <table class="license-manager-table">
        <thead>
          <tr><th>管理器</th><th>License 类型</th><th>管理器路径</th><th>默认参数</th><th>采集方式</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          ${items.map((item, index) => `
            <tr>
              <td>
                <div class="license-manager-cell">
                  <span class="license-manager-icon" aria-hidden="true">${managerIcon(item.licenseType)}</span>
                  <div><strong>${esc(item.name)}</strong><small>${esc(item.code || 'custom')}</small></div>
                </div>
              </td>
              <td>${esc(item.licenseType || 'License')}</td>
              <td><code>${esc(item.executablePath || '未配置路径')}</code></td>
              <td><code>${esc(item.defaultArgs || '未配置')}</code></td>
              <td>${esc(item.collectMethod || 'custom')}</td>
              <td>${index === 0 ? '<span class="license-pill license-pill-active">常用</span>' : '<span class="license-pill license-pill-unmanaged">备用</span>'}</td>
              <td>
                <div class="license-table-actions">
                  <button class="btn btn-ghost" type="button" data-license-action="manager-edit" data-code="${esc(item.code)}">编辑</button>
                  <button class="btn btn-ghost" type="button" data-license-action="manager-copy" data-code="${esc(item.code)}">设为常用</button>
                </div>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>` : '<div class="empty-state compact">暂无匹配管理器，可点击“添加管理器”。</div>';
  }

  function renderConfigs() {
    const box = $('licenseConfigList');
    if (state.loading) {
      box.innerHTML = '<div class="empty-state compact">正在加载许可证配置...</div>';
      $('licenseInlineEditor').innerHTML = '<div class="empty-state compact">正在等待 License 配置数据...</div>';
      return;
    }
    if (state.error) {
      box.innerHTML = `
        <div class="license-inline-error">
          <strong>许可证配置加载失败</strong>
          <span>${esc(state.error)}</span>
          <button class="btn btn-ghost" type="button" data-license-action="reload">重新加载</button>
        </div>`;
      $('licenseInlineEditor').innerHTML = '<div class="empty-state compact">加载失败后无法编辑许可证，请重新加载。</div>';
      return;
    }
    const items = filteredConfigs();
    if (!items.length) {
      box.innerHTML = '<div class="empty-state compact">暂无匹配许可证，请点击“新增许可证”。</div>';
      if (state.selectedLicenseId !== 'new') {
        $('licenseInlineEditor').innerHTML = `
          <div class="license-editor-empty">
            <strong>暂无可编辑的许可证</strong>
            <span>新增许可证后，可在左侧选择配置，并在右侧测试 License Server 采集结果。</span>
            <button class="btn btn-primary" type="button" data-license-action="license-new-inline">新增许可证</button>
          </div>`;
      } else {
        renderInlineEditor();
      }
      return;
    }
    if (state.selectedLicenseId !== 'new' && !items.some((item) => String(item.id) === String(state.selectedLicenseId))) {
      state.selectedLicenseId = String(items[0].id);
      state.inlineTest = null;
    }
    box.innerHTML = items.map((item) => `
      <button class="license-config-card${String(item.id) === String(state.selectedLicenseId) ? ' active' : ''}" type="button" data-license-action="license-select" data-id="${esc(item.id)}">
        <span class="license-app-icon">${appIcon(item)}</span>
        <span class="license-config-main">
          <strong>${esc(item.appName)}</strong>
          <small>${esc(item.licenseType || 'License')} · ${esc(item.serverHost)}:${esc(item.port || '')} · ${esc(item.managerName || '未选管理器')}</small>
          <span>${esc(item.lastCollectStatus || 'never')} ${item.lastCollectMessage ? '· ' + esc(item.lastCollectMessage) : ''}</span>
        </span>
        <span class="license-config-side">
          ${statusPill(item.serviceStatus)}
          <small>${item.enabled ? '启用' : '禁用'}</small>
        </span>
      </button>`).join('');
    renderInlineEditor();
  }

  function draftForApp(appCode) {
    const base = licenseDefaults();
    const app = state.apps.find((item) => item.code === appCode);
    if (app) Object.assign(base, { appName: app.name, appCode: app.code, appType: app.type, iconUrl: app.iconUrl, vendor: app.vendor });
    const manager = activeManager();
    base.managerName = manager.name;
    base.licenseType = manager.licenseType || base.licenseType;
    base.collectMethod = manager.collectMethod || base.collectMethod;
    return base;
  }

  function currentEditorConfig() {
    if (state.selectedLicenseId === 'new') return Object.assign(licenseDefaults(), state.inlineDraft || {});
    return state.configs.find((item) => String(item.id) === String(state.selectedLicenseId)) || null;
  }

  function renderInlineResult(cfg) {
    const result = state.inlineTest;
    if (result) {
      return `
        <div class="license-test-console-head">
          <strong>License 信息输出</strong>
          <span>${esc(result.status || '测试完成')}</span>
        </div>
        <div class="license-test-summary">${result.summary || '测试完成'}</div>
        <pre>${esc(result.output || '测试完成，但没有原始输出。')}</pre>`;
    }
    const summary = [
      `<span>采集状态：${esc(cfg.lastCollectStatus || '未采集')}</span>`,
      `<span>服务状态：${esc(cfg.serviceStatus || '未知')}</span>`,
      `<span>最后采集：${esc(cfg.lastCollectedAt || '—')}</span>`
    ].join('');
    return `
      <div class="license-test-console-head">
        <strong>License 信息输出</strong>
        <span>${esc(cfg.lastCollectStatus || '等待测试')}</span>
      </div>
      <div class="license-test-summary">${summary}</div>
      <pre>${esc(cfg.lastRawOutput || cfg.lastCollectMessage || '填写左侧配置后点击“测试连接”，这里会展示 Feature 解析结果和原始输出。')}</pre>`;
  }

  function renderInlineEditor() {
    const cfg = currentEditorConfig();
    const box = $('licenseInlineEditor');
    if (!cfg) {
      box.innerHTML = `
        <div class="license-editor-empty">
          <strong>请选择许可证配置</strong>
          <span>从左侧选择一个 License Server，或新增许可证后在这里编辑和测试。</span>
          <button class="btn btn-primary" type="button" data-license-action="license-new-inline">新增许可证</button>
        </div>`;
      return;
    }
    const selectedApp = state.apps.find((app) => app.code === cfg.appCode) || state.apps.find((app) => app.name === cfg.appName) || state.apps[0];
    const selectedManager = state.managers.find((manager) => manager.name === cfg.managerName || manager.collectMethod === cfg.collectMethod) || state.managers[0];
    const isNew = state.selectedLicenseId === 'new' || !cfg.id;
    box.innerHTML = `
      <div class="license-editor-head">
        <div>
          <h3>${isNew ? '新增许可证' : '编辑许可证'}</h3>
          <p>${isNew ? '填写 License Server 连接信息并测试采集结果。' : '修改配置后可立即保存、测试采集或管理 systemd 服务。'}</p>
        </div>
        <span class="license-pill ${cfg.enabled === false ? 'license-pill-disabled' : 'license-pill-active'}">${cfg.enabled === false ? '禁用' : '启用'}</span>
      </div>
      <div class="license-editor-body">
        <form class="license-form license-inline-form" id="licenseInlineForm">
          <input type="hidden" id="editorLicId" value="${esc(cfg.id || '')}">
          <label>应用软件
            <select id="editorAppCode">
              ${state.apps.map((app) => `<option value="${esc(app.code)}" ${selectedApp?.code === app.code ? 'selected' : ''}>${esc(app.name)}</option>`).join('')}
            </select>
          </label>
          <label>License 管理器
            <select id="editorManagerCode">
              ${state.managers.map((manager) => `<option value="${esc(manager.code)}" ${selectedManager?.code === manager.code ? 'selected' : ''}>${esc(manager.name)}</option>`).join('')}
            </select>
          </label>
          <label>License Server IP <input id="editorServerHost" value="${esc(cfg.serverHost || '')}" placeholder="例如 10.10.38.152"></label>
          <label>端口 <input id="editorPort" type="number" min="0" max="65535" value="${esc(cfg.port || 1055)}"></label>
          <label>systemd 服务名 <input id="editorServiceName" value="${esc(cfg.serviceName || '')}" placeholder="例如 ansys-license.service"></label>
          <label>采集命令 <input id="editorCollectCommand" value="${esc(cfg.collectCommand || '')}" placeholder="可自动生成或手动覆盖"></label>
          <div class="license-editor-grid">
            <label>超时秒数 <input id="editorTimeoutSec" type="number" min="1" value="${esc(cfg.timeoutSec || 10)}"></label>
            <label>采集周期 <input id="editorCollectIntervalSec" type="number" min="10" value="${esc(cfg.collectIntervalSec || 60)}"></label>
            <label>预警阈值 <input id="editorWarningThreshold" type="number" min="1" max="100" value="${esc(cfg.warningThreshold || 80)}"></label>
            <label>严重阈值 <input id="editorCriticalThreshold" type="number" min="1" max="100" value="${esc(cfg.criticalThreshold || 95)}"></label>
            <label>到期提醒天数 <input id="editorExpireWarningDays" type="number" min="1" value="${esc(cfg.expireWarningDays || 30)}"></label>
            <label class="switch-row">启用 <input id="editorEnabled" type="checkbox" ${cfg.enabled !== false ? 'checked' : ''}></label>
          </div>
          <div class="license-editor-actions">
            <button class="btn btn-ghost" type="button" id="editorGenerateCommandBtn">生成命令</button>
            <button class="btn btn-primary" type="button" data-license-action="license-save-inline" data-action-name="save-inline" data-busy-text="保存中...">保存配置</button>
            <button class="btn btn-ghost" type="button" data-license-action="license-test-inline" data-action-name="test-inline" data-busy-text="测试中...">测试连接</button>
            <button class="btn btn-ghost" type="button" data-license-action="license-collect-inline" data-action-name="collect-inline" data-busy-text="采集中..." ${isNew ? 'disabled title="请先保存配置"' : ''}>立即采集</button>
            <button class="btn btn-ghost" type="button" data-license-action="service-start" data-action-name="service-start" data-busy-text="启动中..." ${isNew ? 'disabled title="请先保存配置"' : ''}>启动服务</button>
            <button class="btn btn-ghost" type="button" data-license-action="service-stop" data-action-name="service-stop" data-busy-text="停止中..." ${isNew ? 'disabled title="请先保存配置"' : ''}>停止服务</button>
            <button class="btn btn-ghost" type="button" data-license-action="service-restart" data-action-name="service-restart" data-busy-text="重启中..." ${isNew ? 'disabled title="请先保存配置"' : ''}>重启服务</button>
            <button class="btn btn-danger" type="button" data-license-action="license-delete-inline" ${isNew ? 'disabled title="新增配置尚未保存"' : ''}>删除</button>
          </div>
        </form>
        <aside class="license-test-console license-inline-console">${renderInlineResult(cfg)}</aside>
      </div>`;
    wireInlineEditor();
    if (state.busy) setBusy(state.busy);
  }

  function wireInlineEditor() {
    const syncCommand = () => {
      const manager = state.managers.find((item) => item.code === $('editorManagerCode')?.value) || state.managers[0];
      if ($('editorCollectCommand')) {
        $('editorCollectCommand').value = buildCommand(manager, $('editorServerHost')?.value.trim(), Number($('editorPort')?.value || 0));
      }
    };
    $('editorGenerateCommandBtn')?.addEventListener('click', syncCommand);
    $('editorManagerCode')?.addEventListener('change', syncCommand);
    $('editorServerHost')?.addEventListener('blur', () => { if (!$('editorCollectCommand')?.value.trim()) syncCommand(); });
    $('editorPort')?.addEventListener('blur', () => { if (!$('editorCollectCommand')?.value.trim()) syncCommand(); });
  }

  function readInlinePayload() {
    const app = state.apps.find((item) => item.code === $('editorAppCode').value) || state.apps[0];
    const manager = state.managers.find((item) => item.code === $('editorManagerCode').value) || state.managers[0];
    const host = $('editorServerHost').value.trim();
    const port = Number($('editorPort').value || 0);
    const command = $('editorCollectCommand').value.trim() || buildCommand(manager, host, port);
    const payload = {
      appName: app.name,
      appCode: app.code,
      appType: app.type || '',
      iconUrl: app.iconUrl || '',
      vendor: app.vendor || '',
      licenseType: manager.licenseType || 'FlexNet',
      managerName: manager.name,
      serverHost: host,
      port,
      collectMethod: manager.collectMethod || 'lmstat',
      collectCommand: command,
      serviceName: $('editorServiceName').value.trim(),
      collectIntervalSec: Number($('editorCollectIntervalSec').value || 60),
      timeoutSec: Number($('editorTimeoutSec').value || 10),
      warningThreshold: Number($('editorWarningThreshold').value || 80),
      criticalThreshold: Number($('editorCriticalThreshold').value || 95),
      expireWarningDays: Number($('editorExpireWarningDays').value || 30),
      enabled: $('editorEnabled').checked
    };
    if (!payload.appName) throw new Error('请选择应用软件');
    if (!payload.serverHost) throw new Error('请填写 License Server IP');
    if (payload.port < 0 || payload.port > 65535) throw new Error('端口范围必须在 0-65535 之间');
    if (payload.timeoutSec < 1) throw new Error('超时秒数必须大于 0');
    if (payload.collectIntervalSec < 10) throw new Error('采集周期不能小于 10 秒');
    if (payload.warningThreshold > payload.criticalThreshold) throw new Error('预警阈值不能大于严重阈值');
    return payload;
  }

  function summarizeCollectResult(data) {
    const cfg = data.config || data;
    const features = data.features || [];
    state.inlineTest = {
      status: cfg.lastCollectStatus === 'failed' ? '采集失败' : '测试完成',
      summary: `
        <span>Feature：${features.length}</span>
        <span>总点数：${features.reduce((sum, item) => sum + (Number(item.total) || 0), 0)}</span>
        <span>使用中：${features.reduce((sum, item) => sum + (Number(item.used) || 0), 0)}</span>
        <span>状态：${esc(cfg.lastCollectStatus || 'success')}</span>`,
      output: cfg.lastRawOutput || cfg.lastCollectMessage || '测试完成，但没有原始输出。'
    };
  }

  async function saveInlineLicense(showToast = true) {
    const id = $('editorLicId')?.value || '';
    const payload = readInlinePayload();
    const saved = await request('/api/v1/license/configs' + (id ? '/' + encodeURIComponent(id) : ''), {
      method: id ? 'PUT' : 'POST',
      body: JSON.stringify(payload)
    });
    state.selectedLicenseId = String(saved.id);
    state.inlineDraft = null;
    await loadConfigs();
    if (showToast) toast('许可证配置已保存', 'success');
    return saved;
  }

  async function testInlineLicense() {
    await withBusy('test-inline', async () => {
      state.inlineTest = { status: '正在连接', summary: '<span>正在保存配置并执行采集命令...</span>', output: '正在连接 License Server...' };
      renderInlineEditor();
      const saved = await saveInlineLicense(false);
      const data = await request(`/api/v1/license/configs/${encodeURIComponent(saved.id)}/test`, { method: 'POST' });
      summarizeCollectResult(data);
      await loadConfigs();
      toast(data.config?.lastCollectStatus === 'failed' ? '测试完成，但采集失败' : 'License Server 测试完成', data.config?.lastCollectStatus === 'failed' ? 'warn' : 'success');
    });
  }

  async function collectInlineLicense() {
    const cfg = currentEditorConfig();
    if (!cfg?.id) throw new Error('请先保存许可证配置');
    await withBusy('collect-inline', async () => {
      state.inlineTest = { status: '正在采集', summary: '<span>正在执行采集命令...</span>', output: '等待 License 管理器返回结果...' };
      renderInlineEditor();
      const data = await request(`/api/v1/license/configs/${encodeURIComponent(cfg.id)}/collect`, { method: 'POST' });
      summarizeCollectResult(data);
      await loadConfigs();
      toast(data.config?.lastCollectStatus === 'failed' ? '采集完成，但命令失败' : 'License 采集完成', data.config?.lastCollectStatus === 'failed' ? 'warn' : 'success');
    });
  }

  async function serviceInlineAction(action) {
    const cfg = currentEditorConfig();
    if (!cfg?.id) throw new Error('请先保存许可证配置');
    const labels = { start: '启动', stop: '停止', restart: '重启' };
    await withBusy(`service-${action}`, async () => {
      const data = await request(`/api/v1/license/configs/${encodeURIComponent(cfg.id)}/service/${action}`, { method: 'POST' });
      state.inlineTest = {
        status: `${labels[action] || '操作'}完成`,
        summary: `<span>服务状态：${esc(data.config?.serviceStatus || '未知')}</span>`,
        output: `systemd 服务${labels[action] || '操作'}请求已发送。`
      };
      await loadConfigs();
      toast(`License 服务${labels[action] || '操作'}完成`, 'success');
    });
  }

  function startInlineNew(appCode) {
    state.selectedLicenseId = 'new';
    state.inlineDraft = draftForApp(appCode);
    state.inlineTest = null;
    setTab('licenses', { writeURL: true });
    renderConfigs();
    setTimeout(() => $('editorServerHost')?.focus(), 0);
  }

  function renderLogs() {
    const q = $('licenseConfigSearch').value.trim().toLowerCase();
    const items = state.configs
      .filter((item) => !q || [item.appName, item.serverHost, item.managerName, item.lastCollectStatus, item.lastCollectMessage].some((value) => String(value || '').toLowerCase().includes(q)))
      .slice()
      .sort((a, b) => new Date(b.lastCollectedAt || b.updatedAt || 0) - new Date(a.lastCollectedAt || a.updatedAt || 0));
    $('licenseCollectLogList').innerHTML = items.length ? items.map((item) => `
      <article class="license-log-item">
        <span class="license-app-icon">${appIcon(item)}</span>
        <div class="license-log-main">
          <strong>${esc(item.appName)} · ${esc(item.serverHost || '未配置')}:${esc(item.port || '')}</strong>
          <small>${esc(item.managerName || item.collectMethod || 'License 管理器')} · ${esc(item.lastCollectedAt || '尚未采集')}</small>
          <p>${esc(item.lastCollectMessage || '暂无采集消息')}</p>
          ${item.lastRawOutput ? `<pre>${esc(String(item.lastRawOutput).slice(0, 1200))}${String(item.lastRawOutput).length > 1200 ? '\n...' : ''}</pre>` : ''}
        </div>
        <div class="license-log-side">
          <span class="license-pill license-pill-${esc(item.lastCollectStatus || 'unknown')}">${esc(item.lastCollectStatus || '未采集')}</span>
          <button class="btn btn-ghost" type="button" data-license-action="license-select" data-id="${esc(item.id)}">查看配置</button>
        </div>
      </article>`).join('') : '<div class="empty-state compact">暂无匹配采集日志。完成一次测试或立即采集后，这里会显示结果。</div>';
  }

  function renderAll() {
    renderMetrics();
    if (state.activeTab === 'apps') renderApps();
    if (state.activeTab === 'managers') renderManagers();
    if (state.activeTab === 'licenses') renderConfigs();
    if (state.activeTab === 'logs') renderLogs();
  }

  async function loadConfigs() {
    state.loading = true;
    state.error = '';
    renderConfigs();
    try {
      const data = await request('/api/v1/license/configs');
      state.configs = data.items || [];
    } catch (err) {
      state.configs = [];
      state.error = err.message || '请求失败';
      throw err;
    } finally {
      state.loading = false;
      renderAll();
    }
  }

  function openAppModal(code) {
    const existing = state.apps.find((item) => item.code === code) || { code: '', name: '', type: '', vendor: '', iconUrl: '', accent: '#007aff' };
    const modal = App.modal({
      title: existing.code ? '编辑应用' : '添加应用',
      width: '620px',
      confirmText: '保存应用',
      content: `
        <form class="license-form license-template-form">
          <label>应用名称 <input id="appNameInput" value="${esc(existing.name)}" placeholder="例如 ANSYS"></label>
          <label>应用编码 <input id="appCodeInput" value="${esc(existing.code)}" placeholder="例如 ansys"></label>
          <label>应用分类 <input id="appTypeInput" value="${esc(existing.type)}" placeholder="CAE / EDA / Math / CFD"></label>
          <label>厂商 <input id="appVendorInput" value="${esc(existing.vendor)}" placeholder="例如 ANSYS, Inc."></label>
          <div class="license-icon-upload-row">
            <span id="appIconPreview" class="license-app-icon license-app-icon-preview">${iconPreviewMarkup(existing.name || '应用', existing.iconUrl || '', existing.accent || '#007aff')}</span>
            <div class="license-icon-upload-main">
              <strong>应用图标</strong>
              <small>支持 PNG、SVG、JPG、JPEG、WebP、GIF，建议使用正方形透明背景图片，大小不超过 1 MB。</small>
              <div class="license-icon-upload-actions">
                <label class="btn btn-ghost license-upload-btn" for="appIconFileInput">上传图片</label>
                <input id="appIconFileInput" type="file" accept=".png,.jpg,.jpeg,.svg,.webp,.gif,image/png,image/jpeg,image/svg+xml,image/webp,image/gif" hidden>
                <button class="btn btn-ghost" type="button" id="appIconClearBtn">移除图标</button>
              </div>
            </div>
          </div>
          <label class="wide">图标 URL / 上传结果 <input id="appIconInput" value="${esc(existing.iconUrl)}" placeholder="可粘贴 assets/icons/ansys.png、https://...，或点击上传图片自动填充"></label>
          <label>主题色 <input id="appAccentInput" value="${esc(existing.accent || '#007aff')}" placeholder="#007aff"></label>
        </form>`,
      onSubmit: async () => {
        const item = {
          name: $('appNameInput').value.trim(),
          code: slug($('appCodeInput').value || $('appNameInput').value),
          type: $('appTypeInput').value.trim(),
          vendor: $('appVendorInput').value.trim(),
          iconUrl: $('appIconInput').value.trim(),
          accent: $('appAccentInput').value.trim() || '#007aff'
        };
        if (!item.name) throw new Error('请填写应用名称');
        state.apps = state.apps.filter((app) => app.code !== existing.code && app.code !== item.code).concat(item);
        persistCatalog(STORAGE_APPS, state.apps, builtinApps);
        toast('应用模板已保存', 'success');
        renderAll();
      }
    });
    const updatePreview = () => {
      const name = $('appNameInput')?.value.trim() || existing.name || '应用';
      const iconUrl = $('appIconInput')?.value.trim() || '';
      const accent = $('appAccentInput')?.value.trim() || existing.accent || '#007aff';
      const preview = $('appIconPreview');
      if (preview) preview.innerHTML = iconPreviewMarkup(name, iconUrl, accent);
    };
    $('appIconFileInput')?.addEventListener('change', async (event) => {
      const file = event.target.files?.[0];
      if (!file) return;
      try {
        const dataUrl = await readIconFile(file);
        $('appIconInput').value = dataUrl;
        updatePreview();
        toast('应用图标已载入，保存应用后生效', 'success');
      } catch (err) {
        event.target.value = '';
        toast(err.message || '图标上传失败', 'danger');
      }
    });
    $('appIconClearBtn')?.addEventListener('click', () => {
      $('appIconInput').value = '';
      const fileInput = $('appIconFileInput');
      if (fileInput) fileInput.value = '';
      updatePreview();
      toast('已移除应用图标，保存后生效', 'info');
    });
    ['appNameInput', 'appIconInput', 'appAccentInput'].forEach((id) => $(id)?.addEventListener('input', updatePreview));
    modal.el.querySelector('#appNameInput')?.focus();
  }

  function openManagerModal(code) {
    const existing = state.managers.find((item) => item.code === code) || { code: '', name: '', licenseType: 'FlexNet', executablePath: 'lmstat', defaultArgs: '-a -c', collectMethod: 'lmstat' };
    const modal = App.modal({
      title: existing.code ? '编辑 License 管理器' : '添加 License 管理器',
      width: '660px',
      confirmText: '保存管理器',
      content: `
        <form class="license-form license-template-form">
          <label>管理器名称 <input id="managerNameInput" value="${esc(existing.name)}" placeholder="FlexNet lmstat"></label>
          <label>管理器编码 <input id="managerCodeInput" value="${esc(existing.code)}" placeholder="lmstat"></label>
          <label>License 类型
            <select id="managerTypeInput">
              ${['FlexNet', 'RLM', 'Sentinel', 'Custom'].map((value) => `<option value="${value}" ${existing.licenseType === value ? 'selected' : ''}>${value}</option>`).join('')}
            </select>
          </label>
          <label>管理器路径 <input id="managerPathInput" value="${esc(existing.executablePath)}" placeholder="/opt/license/bin/lmstat 或 lmstat"></label>
          <label>默认参数 <input id="managerArgsInput" value="${esc(existing.defaultArgs)}" placeholder="-a -c"></label>
          <label>采集方式 <input id="managerMethodInput" value="${esc(existing.collectMethod)}" placeholder="lmstat / lmutil / rlmutil"></label>
        </form>`,
      onSubmit: async () => {
        const item = {
          name: $('managerNameInput').value.trim(),
          code: slug($('managerCodeInput').value || $('managerNameInput').value),
          licenseType: $('managerTypeInput').value,
          executablePath: $('managerPathInput').value.trim(),
          defaultArgs: $('managerArgsInput').value.trim(),
          collectMethod: $('managerMethodInput').value.trim()
        };
        if (!item.name) throw new Error('请填写管理器名称');
        if (!item.executablePath) throw new Error('请填写管理器路径');
        state.managers = state.managers.filter((manager) => manager.code !== existing.code && manager.code !== item.code).concat(item);
        persistCatalog(STORAGE_MANAGERS, state.managers, builtinManagers);
        toast('License 管理器已保存', 'success');
        renderAll();
      }
    });
    modal.el.querySelector('#managerNameInput')?.focus();
  }

  function licenseModalContent(cfg) {
    const selectedApp = state.apps.find((app) => app.code === cfg.appCode) || state.apps.find((app) => app.name === cfg.appName) || state.apps[0];
    const selectedManager = state.managers.find((manager) => manager.name === cfg.managerName || manager.collectMethod === cfg.collectMethod) || state.managers[0];
    return `
      <div class="license-license-modal">
        <form class="license-form license-modal-form" id="licenseModalForm">
          <input type="hidden" id="modalLicId" value="${esc(cfg.id || '')}">
          <label>应用软件
            <select id="modalAppCode">
              ${state.apps.map((app) => `<option value="${esc(app.code)}" ${selectedApp?.code === app.code ? 'selected' : ''}>${esc(app.name)}</option>`).join('')}
            </select>
          </label>
          <label>License 管理器
            <select id="modalManagerCode">
              ${state.managers.map((manager) => `<option value="${esc(manager.code)}" ${selectedManager?.code === manager.code ? 'selected' : ''}>${esc(manager.name)}</option>`).join('')}
            </select>
          </label>
          <label>License Server IP <input id="modalServerHost" value="${esc(cfg.serverHost)}" placeholder="10.10.38.152"></label>
          <label>端口 <input id="modalPort" type="number" min="0" max="65535" value="${esc(cfg.port || 1055)}"></label>
          <label>systemd 服务名 <input id="modalServiceName" value="${esc(cfg.serviceName)}" placeholder="ansys-license.service"></label>
          <label>采集命令 <input id="modalCollectCommand" value="${esc(cfg.collectCommand)}" placeholder="自动生成或手动覆盖"></label>
          <div class="license-modal-grid-2">
            <label>超时秒数 <input id="modalTimeoutSec" type="number" min="1" value="${esc(cfg.timeoutSec || 10)}"></label>
            <label>采集周期 <input id="modalCollectIntervalSec" type="number" min="10" value="${esc(cfg.collectIntervalSec || 60)}"></label>
            <label>预警阈值 <input id="modalWarningThreshold" type="number" min="1" max="100" value="${esc(cfg.warningThreshold || 80)}"></label>
            <label>严重阈值 <input id="modalCriticalThreshold" type="number" min="1" max="100" value="${esc(cfg.criticalThreshold || 95)}"></label>
          </div>
          <label class="switch-row">启用 <input id="modalEnabled" type="checkbox" ${cfg.enabled !== false ? 'checked' : ''}></label>
          <div class="license-modal-inline-actions">
            <button class="btn btn-ghost" type="button" id="modalGenerateCommandBtn">生成命令</button>
            <button class="btn btn-primary" type="button" id="modalTestBtn">测试连接</button>
          </div>
        </form>
        <aside class="license-test-console">
          <div class="license-test-console-head">
            <strong>License 信息输出</strong>
            <span id="modalTestStatus">等待测试</span>
          </div>
          <div id="modalTestSummary" class="license-test-summary">填写左侧配置后点击“测试连接”，这里会展示 Feature 解析结果和原始输出。</div>
          <pre id="modalTestOutput">暂无输出</pre>
        </aside>
      </div>`;
  }

  function readLicenseModalPayload() {
    const app = state.apps.find((item) => item.code === $('modalAppCode').value) || state.apps[0];
    const manager = state.managers.find((item) => item.code === $('modalManagerCode').value) || state.managers[0];
    const host = $('modalServerHost').value.trim();
    const port = Number($('modalPort').value || 0);
    const command = $('modalCollectCommand').value.trim() || buildCommand(manager, host, port);
    const payload = {
      appName: app.name,
      appCode: app.code,
      appType: app.type || '',
      iconUrl: app.iconUrl || '',
      vendor: app.vendor || '',
      licenseType: manager.licenseType || 'FlexNet',
      managerName: manager.name,
      serverHost: host,
      port,
      collectMethod: manager.collectMethod || 'lmstat',
      collectCommand: command,
      serviceName: $('modalServiceName').value.trim(),
      collectIntervalSec: Number($('modalCollectIntervalSec').value || 60),
      timeoutSec: Number($('modalTimeoutSec').value || 10),
      warningThreshold: Number($('modalWarningThreshold').value || 80),
      criticalThreshold: Number($('modalCriticalThreshold').value || 95),
      expireWarningDays: 30,
      enabled: $('modalEnabled').checked
    };
    if (!payload.appName) throw new Error('请选择应用软件');
    if (!payload.serverHost) throw new Error('请填写 License Server IP');
    if (payload.port < 0 || payload.port > 65535) throw new Error('端口范围必须在 0-65535 之间');
    if (payload.warningThreshold > payload.criticalThreshold) throw new Error('预警阈值不能大于严重阈值');
    return payload;
  }

  function writeModalOutput(data) {
    const cfg = data.config || data;
    const features = data.features || [];
    $('modalTestStatus').textContent = cfg.lastCollectStatus === 'failed' ? '采集失败' : '测试完成';
    $('modalTestSummary').innerHTML = `
      <span>Feature：${features.length}</span>
      <span>总点数：${features.reduce((sum, item) => sum + (Number(item.total) || 0), 0)}</span>
      <span>使用中：${features.reduce((sum, item) => sum + (Number(item.used) || 0), 0)}</span>`;
    $('modalTestOutput').textContent = cfg.lastRawOutput || cfg.lastCollectMessage || '测试完成，但没有原始输出。';
  }

  async function saveLicenseFromModal() {
    const id = $('modalLicId').value;
    const payload = readLicenseModalPayload();
    const saved = await request('/api/v1/license/configs' + (id ? '/' + encodeURIComponent(id) : ''), {
      method: id ? 'PUT' : 'POST',
      body: JSON.stringify(payload)
    });
    $('modalLicId').value = saved.id;
    await loadConfigs();
    return saved;
  }

  function openLicenseModal(config, appCode) {
    const base = Object.assign(licenseDefaults(), config || {});
    if (appCode) {
      const app = state.apps.find((item) => item.code === appCode);
      if (app) Object.assign(base, { appName: app.name, appCode: app.code, appType: app.type, iconUrl: app.iconUrl, vendor: app.vendor });
    }
    const modal = App.modal({
      title: base.id ? '编辑许可证' : '新增许可证',
      width: '1180px',
      confirmText: '保存许可证',
      content: licenseModalContent(base),
      onSubmit: async () => {
        await saveLicenseFromModal();
        toast('许可证配置已保存', 'success');
      }
    });
    const syncCommand = () => {
      const manager = state.managers.find((item) => item.code === $('modalManagerCode').value) || state.managers[0];
      $('modalCollectCommand').value = buildCommand(manager, $('modalServerHost').value.trim(), Number($('modalPort').value || 0));
    };
    $('modalGenerateCommandBtn').addEventListener('click', syncCommand);
    $('modalManagerCode').addEventListener('change', syncCommand);
    $('modalServerHost').addEventListener('blur', () => {
      if (!$('modalCollectCommand').value.trim()) syncCommand();
    });
    $('modalTestBtn').addEventListener('click', async () => {
      await withBusy('modal-test', async () => {
        $('modalTestBtn').disabled = true;
        $('modalTestBtn').textContent = '测试中...';
        $('modalTestStatus').textContent = '正在连接';
        $('modalTestOutput').textContent = '正在保存配置并执行采集命令...';
        try {
          const saved = await saveLicenseFromModal();
          const data = await request(`/api/v1/license/configs/${encodeURIComponent(saved.id)}/test`, { method: 'POST' });
          writeModalOutput(data);
          toast('License Server 测试完成', data.config?.lastCollectStatus === 'failed' ? 'warn' : 'success');
        } catch (err) {
          $('modalTestStatus').textContent = '测试失败';
          $('modalTestSummary').textContent = err.message || '测试失败';
          $('modalTestOutput').textContent = err.message || '测试失败';
          toast('License Server 测试失败：' + (err.message || err), 'danger');
        } finally {
          $('modalTestBtn').disabled = false;
          $('modalTestBtn').textContent = '测试连接';
        }
      });
    });
    if (!base.collectCommand) syncCommand();
    modal.el.querySelector('#modalServerHost')?.focus();
  }

  async function testExistingLicense(id) {
    await withBusy('test', async () => {
      const data = await request(`/api/v1/license/configs/${encodeURIComponent(id)}/test`, { method: 'POST' });
      toast(data.config?.lastCollectStatus === 'failed' ? '测试完成，但采集失败' : '测试采集完成', data.config?.lastCollectStatus === 'failed' ? 'warn' : 'success');
      await loadConfigs();
    });
  }

  async function deleteLicense(id) {
    confirmAction('确认删除该许可证配置？删除后当前监控配置不可恢复。', {
      danger: true,
      onConfirm: async () => {
        await withBusy('delete', async () => {
          await request('/api/v1/license/configs/' + encodeURIComponent(id), { method: 'DELETE' });
          toast('许可证配置已删除', 'success');
          if (String(state.selectedLicenseId) === String(id)) {
            state.selectedLicenseId = '';
            state.inlineDraft = null;
            state.inlineTest = null;
          }
          await loadConfigs();
        }).catch((err) => toast(err.message, 'danger'));
      }
    });
  }

  function handleAction(event) {
    const target = event.target.closest('[data-license-action]');
    if (!target) return;
    const action = target.dataset.licenseAction;
    const code = target.dataset.code;
    const id = target.dataset.id;
    if (action === 'reload') loadConfigs().catch((err) => toast(err.message, 'danger'));
    if (action === 'app-edit') openAppModal(code);
    if (action === 'license-from-app') startInlineNew(code);
    if (action === 'manager-edit') openManagerModal(code);
    if (action === 'manager-copy') {
      const manager = state.managers.find((item) => item.code === code);
      if (manager) {
        state.managers = [manager].concat(state.managers.filter((item) => item.code !== code));
        persistCatalog(STORAGE_MANAGERS, state.managers, builtinManagers);
        toast('已设为常用管理器', 'success');
        renderAll();
      }
    }
    if (action === 'license-new-inline') startInlineNew();
    if (action === 'license-select') {
      state.selectedLicenseId = String(id || '');
      state.inlineDraft = null;
      state.inlineTest = null;
      setTab('licenses', { writeURL: true });
      renderAll();
    }
    if (action === 'license-edit') {
      const config = state.configs.find((item) => String(item.id) === String(id));
      if (config) {
        state.selectedLicenseId = String(config.id);
        state.inlineDraft = null;
        state.inlineTest = null;
        setTab('licenses', { writeURL: true });
      }
    }
    if (action === 'license-test') testExistingLicense(id).catch((err) => toast(err.message, 'danger'));
    if (action === 'license-delete') deleteLicense(id);
    if (action === 'license-save-inline') withBusy('save-inline', () => saveInlineLicense()).catch((err) => toast(err.message, 'danger'));
    if (action === 'license-test-inline') testInlineLicense().catch((err) => {
      state.inlineTest = { status: '测试失败', summary: `<span>${esc(err.message || '测试失败')}</span>`, output: err.message || '测试失败' };
      renderInlineEditor();
      toast('License Server 测试失败：' + (err.message || err), 'danger');
    });
    if (action === 'license-collect-inline') collectInlineLicense().catch((err) => toast(err.message, 'danger'));
    if (action === 'service-start') serviceInlineAction('start').catch((err) => toast(err.message, 'danger'));
    if (action === 'service-stop') serviceInlineAction('stop').catch((err) => toast(err.message, 'danger'));
    if (action === 'service-restart') serviceInlineAction('restart').catch((err) => toast(err.message, 'danger'));
    if (action === 'license-delete-inline') {
      const cfg = currentEditorConfig();
      if (cfg?.id) deleteLicense(cfg.id);
    }
  }

  function bind() {
    document.querySelectorAll('.license-tab').forEach((button) => {
      button.addEventListener('click', () => setTab(button.dataset.licenseTab, { writeURL: true }));
    });
    $('licensePrimaryActionBtn').addEventListener('click', () => {
      if (state.activeTab === 'apps') openAppModal();
      else if (state.activeTab === 'managers') openManagerModal();
      else if (state.activeTab === 'logs') loadConfigs().catch((err) => toast(err.message, 'danger'));
      else startInlineNew();
    });
    $('licenseAddAppBtn').addEventListener('click', () => openAppModal());
    $('licenseAddManagerBtn').addEventListener('click', () => openManagerModal());
    $('licenseLogsRefreshBtn').addEventListener('click', () => loadConfigs().catch((err) => toast(err.message, 'danger')));
    $('licenseRefreshBtn').addEventListener('click', () => loadConfigs().catch((err) => toast(err.message, 'danger')));
    $('licenseConfigSearch').addEventListener('input', renderAll);
    $('licenseConfigEnabledFilter').addEventListener('change', renderAll);
    document.addEventListener('click', handleAction);
    window.addEventListener('popstate', () => setTab(licenseTabFromURL()));
  }

  document.addEventListener('DOMContentLoaded', () => {
    bind();
    setTab(licenseTabFromURL(), { writeURL: true, replace: true });
    loadConfigs().catch((err) => toast('License 配置未获取：' + err.message, 'danger'));
  });
})();
