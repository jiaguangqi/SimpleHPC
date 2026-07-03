(function () {
  'use strict';

  function ready(fn) {
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', fn);
    else fn();
  }

  async function getJSON(path) {
    const fetcher = window.App && window.App.apiFetch ? window.App.apiFetch : fetch;
    const res = await fetcher(path, { cache: 'no-store' });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || res.statusText || 'request failed');
    return data;
  }

  function findStat(label) {
    return Array.from(document.querySelectorAll('.stat-card')).find(function (card) {
      return (card.querySelector('.stat-label')?.textContent || '').trim() === label;
    });
  }

  function setStat(label, value, note, noteClass) {
    const card = findStat(label);
    if (!card) return;
    const valueEl = card.querySelector('.stat-value');
    if (valueEl && value !== undefined && value !== null) valueEl.textContent = String(value);
    let noteEl = card.querySelector('.stat-note, .stat-change');
    if (!noteEl && note) {
      noteEl = document.createElement('div');
      noteEl.className = 'stat-note';
      card.appendChild(noteEl);
    }
    if (noteEl && note) {
      noteEl.textContent = note;
      noteEl.className = 'stat-note' + (noteClass ? ' ' + noteClass : '');
    }
  }

  function ensureStatusBadge() {
    const actions = document.querySelector('.header-actions, .topbar-actions');
    if (!actions || actions.querySelector('.live-status')) return null;
    const badge = document.createElement('div');
    badge.className = 'live-status pending';
    badge.textContent = 'API 检测中';
    actions.prepend(badge);
    return badge;
  }

  function setStatusBadge(health) {
    const badge = document.querySelector('.live-status') || ensureStatusBadge();
    if (!badge) return;
    const ok = health && health.status === 'ok';
    badge.className = 'live-status ' + (ok ? 'ok' : 'warn');
    badge.textContent = ok ? 'API 在线' : 'API 异常';
    badge.title = health && health.services
      ? Object.entries(health.services).map(function (entry) {
        return entry[0] + ': ' + entry[1].status;
      }).join('\n')
      : '';
  }

  async function loadHealth() {
    ensureStatusBadge();
    try {
      const health = await getJSON('/api/health');
      setStatusBadge(health);
      return health;
    } catch (err) {
      setStatusBadge({ status: 'degraded' });
      return null;
    }
  }

  async function loadOverview() {
    const overview = await getJSON('/api/v1/overview');
    if (overview.jobs) {
      setStat('运行中作业', overview.jobs.running, '来自 Slurm 实时队列', 'success');
      setStat('排队作业', overview.jobs.pending, '来自 Slurm 实时队列', overview.jobs.pending > 0 ? 'warning' : 'success');
    }
    if (overview.nodes) {
      const states = overview.nodes.states || {};
      const idle = states.idle || states.IDLE || 0;
      const total = overview.nodes.count || 0;
      setStat('CPU 利用率', total ? overview.nodes.cpus + '核' : '-', '节点 ' + total + ' 台，空闲 ' + idle + ' 台');
    }
    return overview;
  }

  async function loadNodes() {
    const data = await getJSON('/api/v1/slurm/nodes');
    const nodes = data.items || [];
    if (!nodes.length) return nodes;
    const healthy = nodes.filter(function (node) {
      return !/down|drain|fail|unknown/i.test(node.state || '');
    }).length;
    const percent = Math.round((healthy / nodes.length) * 100);
    setStat('节点健康率', percent + '%', healthy + ' / ' + nodes.length + ' 在线', percent === 100 ? 'success' : 'warning');

    const groups = {};
    nodes.forEach(function (node) {
      const key = (node.state || 'unknown').toLowerCase().replace(/[~*+]/g, '');
      groups[key] = (groups[key] || 0) + 1;
    });
    const chips = document.querySelectorAll('.node-chip');
    chips.forEach(function (chip) {
      const key = (chip.querySelector('strong')?.textContent || '').trim().toLowerCase();
      const value = groups[key] || 0;
      const tail = chip.querySelector('span:last-child');
      if (tail) tail.textContent = value + ' 节点';
    });
    return nodes;
  }

  async function loadStorageRoots() {
    const data = await getJSON('/api/v1/storage/roots');
    const roots = data.items || [];
    const currentPath = document.getElementById('currentPath');
    const treeRows = document.querySelectorAll('.tree-row');
    if (!roots.length || !treeRows.length) return roots;
    const parent = treeRows[0].parentElement;
    parent.innerHTML = '';
    roots.forEach(function (root, index) {
      const btn = document.createElement('button');
      btn.className = 'tree-row' + (index === 0 ? ' active' : '');
      btn.type = 'button';
      btn.innerHTML = '<span>' + (index === 0 ? '⌂' : '▣') + '</span><span>' + root.path + '</span>';
      btn.addEventListener('click', function () {
        parent.querySelectorAll('.tree-row').forEach(function (row) { row.classList.remove('active'); });
        btn.classList.add('active');
        if (currentPath) currentPath.textContent = root.path;
        if (window.App) App.toast('已切换到 ' + root.path, 'info');
      });
      parent.appendChild(btn);
    });
    if (currentPath) currentPath.textContent = roots[0].path;
    return roots;
  }

  ready(function () {
    loadHealth();
    const page = location.pathname.split('/').pop() || 'index.html';
    if (page === 'index.html') {
      loadOverview().catch(function () {});
    }
    if (page === 'monitoring.html') {
      loadOverview().catch(function () {});
      loadNodes().catch(function () {});
    }
    if (page === 'data.html') {
      loadStorageRoots().catch(function () {});
    }
  });
})();
