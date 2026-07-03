(function () {
  'use strict';

  const token = localStorage.getItem('simplehpc_token') || '';
  const headers = token ? {Authorization: 'Bearer ' + token} : {};

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, char => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[char]));
  }

  async function request(url, options) {
    const response = await fetch(url, Object.assign({cache: 'no-store', headers}, options || {}));
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    return data;
  }

  function renderDashboard(data) {
    const resources = data.resources || {};
    const total = Number(resources.totalNodes) || 0;
    const idle = Number(resources.idleNodes) || 0;
    const healthy = Math.max(0, total - ((data.alerts || []).filter(item => String(item.source || '').startsWith('slurm-node:') && item.status !== 'resolved').length));
    document.getElementById('monitorNodeHealth').textContent = total ? Math.round(healthy / total * 100) + '%' : '无节点';
    document.getElementById('monitorNodeSummary').textContent = `${healthy} / ${total} 正常，${idle} 空闲`;
    const usage = Number(resources.cpuUsagePercent);
    document.getElementById('monitorCpuUsage').textContent = Number.isFinite(usage) ? usage.toFixed(1) + '%' : '数据未获取';
    document.getElementById('monitorCpuSummary').textContent = `已分配 ${resources.allocatedCpus ?? '—'} / 总计 ${resources.totalCpus ?? '—'} 核`;
  }

  function renderAlerts(items) {
    const active = items.filter(item => item.status === 'active');
    const critical = active.filter(item => item.level === 'critical').length;
    const warning = active.filter(item => item.level === 'warning').length;
    document.getElementById('monitorAlertCount').textContent = String(active.length);
    document.getElementById('monitorAlertSummary').textContent = `严重 ${critical} / 警告 ${warning}`;
    const rows = document.getElementById('monitorAlertRows');
    if (!items.length) {
      rows.innerHTML = '<tr><td colspan="5" class="api-data-missing">当前没有告警记录</td></tr>';
      return;
    }
    rows.innerHTML = items.map(item => {
      const badge = item.level === 'critical' ? 'danger' : item.level === 'warning' ? 'warning' : 'info';
      const action = item.status === 'active'
        ? `<button class="small-action" data-ack="${item.id}">确认</button>`
        : `<span class="pill pill-info">${escapeHTML(item.status)}</span>`;
      return `<tr><td><span class="badge ${badge}">${escapeHTML(item.level)}</span></td><td>${escapeHTML(item.source)}</td><td><strong>${escapeHTML(item.title)}</strong><br><span class="muted-small">${escapeHTML(item.message)}</span></td><td>${escapeHTML(item.occurredAt)}</td><td>${action}</td></tr>`;
    }).join('');
  }

  async function load(refresh) {
    try {
      if (refresh) await request('/api/v1/monitoring/refresh', {method: 'POST'});
      const [dashboard, alerts] = await Promise.all([
        request('/api/v1/dashboard'),
        request('/api/v1/monitoring/alerts?limit=100')
      ]);
      dashboard.alerts = alerts.items || [];
      renderDashboard(dashboard);
      renderAlerts(alerts.items || []);
      if (refresh && window.App) App.toast('监控数据已从集群刷新', 'success');
    } catch (error) {
      document.getElementById('monitorAlertRows').innerHTML = `<tr><td colspan="5" class="api-data-missing">数据未获取：${escapeHTML(error.message)}</td></tr>`;
      if (window.App) App.toast('监控数据获取失败：' + error.message, 'danger');
    }
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('refreshMonitoring').addEventListener('click', () => load(true));
    document.getElementById('monitorAlertRows').addEventListener('click', async event => {
      const button = event.target.closest('[data-ack]');
      if (!button) return;
      try {
        await request(`/api/v1/monitoring/alerts/${encodeURIComponent(button.dataset.ack)}/acknowledge`, {method: 'POST'});
        await load(false);
        if (window.App) App.toast('告警已确认', 'success');
      } catch (error) {
        if (window.App) App.toast(error.message, 'danger');
      }
    });
    load(false);
  });
}());
