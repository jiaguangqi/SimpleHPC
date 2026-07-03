(function () {
  'use strict';
  const token = localStorage.getItem('simplehpc_token') || '';
  const headers = Object.assign({'Content-Type': 'application/json'}, token ? {Authorization: 'Bearer ' + token} : {});

  function esc(value) {
    return String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  }

  async function request(url, options) {
    const response = await fetch(url, Object.assign({cache: 'no-store', headers}, options || {}));
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    return data;
  }

  function renderRuns(items) {
    const latest = items[0];
    document.getElementById('inspectionLatest').textContent = latest ? (latest.status === 'passed' ? '通过' : '警告') : '暂无记录';
    document.getElementById('inspectionProblems').textContent = latest ? String(latest.problemCount) : '0';
    document.getElementById('inspectionLatestNote').textContent = latest ? `${latest.checks.length - latest.problemCount} / ${latest.checks.length} 项正常` : '执行一次巡检后生成记录';
    const rows = document.getElementById('inspectionRows');
    if (!items.length) {
      rows.innerHTML = '<tr><td colspan="7" class="api-data-missing">当前没有巡检记录</td></tr>';
      return;
    }
    rows.innerHTML = items.map(item => `<tr><td>${esc(item.runId)}</td><td>${esc(item.startedAt || item.createdAt)}</td><td>${((Number(item.durationMs)||0)/1000).toFixed(2)} 秒</td><td><span class="pill ${item.status === 'passed' ? 'pill-success' : 'pill-warn'}">${item.status === 'passed' ? '通过' : '警告'}</span></td><td>${item.passedCount || 0} / ${(item.checks || []).length}</td><td>${item.problemCount}${item.skippedCount ? `（跳过 ${item.skippedCount}）` : ''}</td><td><div style="display:flex;gap:8px;flex-wrap:wrap"><a class="small-action" target="_blank" rel="noopener" href="/api/v1/inspection/runs/${item.id}/report">总结报告</a><a class="small-action" target="_blank" rel="noopener" href="inspection-log.html?id=${item.id}&run=${encodeURIComponent(item.runId)}">详细日志</a><button class="small-action" type="button" data-notify="${item.id}" data-run="${esc(item.runId)}">发送通知</button></div></td></tr>`).join('');
  }

  async function load() {
    try {
      const [runs, config] = await Promise.all([
        request('/api/v1/inspection/runs?limit=100'),
        request('/api/v1/inspection/config')
      ]);
      const values = config.config || {};
      document.getElementById('inspectionSchedule').textContent = values.schedule || '数据未获取';
      document.getElementById('inspectionScheduleInput').value = values.schedule || '';
      document.getElementById('inspectionRetentionInput').value = values.retentionDays || '';
      renderRuns(runs.items || []);
    } catch (error) {
      document.getElementById('inspectionRows').innerHTML = `<tr><td colspan="7" class="api-data-missing">数据未获取：${esc(error.message)}</td></tr>`;
    }
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('inspectionRows').addEventListener('click', async event => {
      const button = event.target.closest('[data-notify]');
      if (!button || button.disabled) return;
      const original = button.textContent;
      button.disabled = true;
      button.textContent = '发送中...';
      try {
        const result = await request(`/api/v1/inspection/runs/${button.dataset.notify}/notify`, {method: 'POST'});
        if (window.App) App.toast(result.message || `报告 ${button.dataset.run} 已发送到飞书`, 'success');
      } catch (error) {
        if (window.App) App.toast('飞书发送失败：' + error.message, 'danger');
      } finally {
        button.disabled = false;
        button.textContent = original;
      }
    });
    document.getElementById('runInspection').addEventListener('click', async () => {
      if (window.App) App.loading.show('正在执行真实集群巡检...');
      try {
        await request('/api/v1/inspection/runs', {method: 'POST'});
        await load();
        if (window.App) App.toast('巡检完成，已生成总结报告和详细日志', 'success');
      } catch (error) {
        if (window.App) App.toast('巡检失败：' + error.message, 'danger');
      } finally {
        if (window.App) App.loading.hide();
      }
    });
    document.getElementById('saveInspectionConfig').addEventListener('click', async () => {
      try {
        await request('/api/v1/inspection/config', {
          method: 'PUT',
          body: JSON.stringify({
            schedule: document.getElementById('inspectionScheduleInput').value,
            retentionDays: Number(document.getElementById('inspectionRetentionInput').value)
          })
        });
        await load();
        if (window.App) App.toast('巡检配置已保存', 'success');
      } catch (error) {
        if (window.App) App.toast('保存失败：' + error.message, 'danger');
      }
    });
    load();
  });
}());
