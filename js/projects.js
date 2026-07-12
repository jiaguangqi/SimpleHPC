(function () {
  'use strict';

  const state = {
    projects: [],
    summary: {},
    selectedId: '',
    loading: false,
    busy: '',
    error: '',
    keyword: '',
    status: ''
  };

  const $ = (id) => document.getElementById(id);
  const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
  const num = (value) => Number.isFinite(Number(value)) ? Number(value) : 0;
  const projectIconPaths = {
    project: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"></path><path d="M3 11h18"></path>',
    active: '<path d="M3 12h4l2.5-7 5 14 2.5-7h4"></path>',
    job: '<rect x="3" y="4" width="18" height="16" rx="2"></rect><path d="M7 8h10"></path><path d="M7 12h6"></path><path d="M7 16h4"></path>',
    task: '<path d="m4 7 2 2 3-3"></path><path d="M11 8h9"></path><path d="m4 15 2 2 3-3"></path><path d="M11 16h9"></path>',
    storage: '<ellipse cx="12" cy="5" rx="8" ry="3"></ellipse><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5"></path><path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"></path>',
    list: '<path d="M8 6h13"></path><path d="M8 12h13"></path><path d="M8 18h13"></path><path d="M3 6h.01"></path><path d="M3 12h.01"></path><path d="M3 18h.01"></path>',
    send: '<path d="m22 2-7 20-4-9-9-4Z"></path><path d="M22 2 11 13"></path>',
    sync: '<path d="M20 6v6h-6"></path><path d="M4 18v-6h6"></path><path d="M18.5 9a7 7 0 0 0-11.7-2.6L4 9"></path><path d="m20 15-2.8 2.6A7 7 0 0 1 5.5 15"></path>',
    edit: '<path d="M12 20h9"></path><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"></path>',
    plus: '<path d="M12 5v14"></path><path d="M5 12h14"></path>',
    trash: '<path d="M3 6h18"></path><path d="M8 6V4h8v2"></path><path d="m19 6-1 14H6L5 6"></path><path d="M10 11v5"></path><path d="M14 11v5"></path>'
  };

  function projectIcon(name) {
    return `<svg class="project-icon" viewBox="0 0 24 24" aria-hidden="true">${projectIconPaths[name] || projectIconPaths.project}</svg>`;
  }

  const statusMap = {
    planning: ['规划中', 'planning'],
    active: ['进行中', 'active'],
    paused: ['已暂停', 'paused'],
    completed: ['已完成', 'completed'],
    archived: ['已归档', 'archived']
  };
  const priorityMap = {
    low: '低',
    normal: '普通',
    high: '高',
    critical: '紧急'
  };
  const roleMap = {
    owner: '负责人',
    manager: '项目管理员',
    compute_member: '计算成员',
    data_member: '数据成员',
    viewer: '只读成员',
    external: '外部协作者'
  };
  const permissionMap = { read: '只读', work: '协作', manage: '管理' };
  const taskStatusMap = { todo: '待处理', running: '进行中', blocked: '受阻', done: '已完成', cancelled: '已取消' };
  const slurmSyncStatusMap = {
    pending: '待同步',
    success: '已同步',
    error: '同步失败',
    disabled: '未启用'
  };

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

  function selectedProject() {
    return state.projects.find((item) => String(item.id) === String(state.selectedId)) || null;
  }

  function statusLabel(value) {
    return statusMap[value]?.[0] || value || '未知';
  }

  function slurmStatusLabel(value) {
    return slurmSyncStatusMap[value] || value || '待同步';
  }

  function slurmStatusClass(value) {
    if (value === 'success') return 'project-sync-success';
    if (value === 'error') return 'project-sync-error';
    if (value === 'disabled') return 'project-sync-disabled';
    return 'project-sync-pending';
  }

  function formatDate(value) {
    if (!value) return '未设置';
    return String(value).slice(0, 10);
  }

  function dateTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('zh-CN', { hour12: false });
  }

  function canManage(project) {
    return Boolean(project?.canManage) || ['manage'].includes(project?.currentUserAccess) || ['owner', 'manager'].includes(project?.currentUserRole);
  }

  function renderKpis() {
    const s = state.summary || {};
    const cards = [
      ['项目总数', s.total || 0, '覆盖项目/课题空间', 'project'],
      ['进行中', s.active || 0, '活跃研发任务', 'active'],
      ['运行作业', s.runningJobs || 0, '项目关联作业', 'job'],
      ['未完成任务', s.openTasks || 0, '待推进事项', 'task'],
      ['存储额度', (s.storageQuotaGb || 0).toLocaleString('zh-CN'), 'GB 项目空间', 'storage']
    ];
    $('projectKpis').innerHTML = cards.map((card) => `
      <article class="project-kpi-card project-kpi-${card[3]}">
        <span class="project-kpi-icon" aria-hidden="true">${projectIcon(card[3])}</span>
        <div><strong>${esc(card[1])}</strong><small>${esc(card[0])}</small><em>${esc(card[2])}</em></div>
      </article>
    `).join('');
  }

  function renderProjectList() {
    const box = $('projectList');
    if (state.loading) {
      box.innerHTML = '<div class="project-empty">正在加载项目...</div>';
      return;
    }
    if (!state.projects.length) {
      box.innerHTML = '<div class="project-empty"><strong>暂无项目</strong><span>点击“新建项目”创建第一个课题空间。</span></div>';
      return;
    }
    box.innerHTML = state.projects.map((project) => {
      const metrics = project.metrics || {};
      return `<button type="button" class="project-list-item ${String(project.id) === String(state.selectedId) ? 'active' : ''}" data-project-action="select" data-id="${project.id}">
        <span class="project-avatar">${projectIcon('project')}</span>
        <span class="project-list-main">
          <strong>${esc(project.name)}</strong>
          <small>${esc(project.code)} · Account ${esc(project.slurmAccount || project.code)} · ${esc(project.ownerDisplayName || project.ownerUsername || '未指定负责人')}</small>
          <span class="project-list-meta">
            <b>${metrics.memberCount || 0} 成员</b><b>${metrics.taskCount || 0} 任务</b><b>${metrics.jobCount || 0} 作业</b>
          </span>
        </span>
        <span class="project-list-badges">
          ${project.currentUserDefaultProject ? '<em class="project-default-badge">默认</em>' : ''}
          <span class="project-status project-status-${esc(project.status)}">${esc(statusLabel(project.status))}</span>
        </span>
      </button>`;
    }).join('');
  }

  function renderDetailEmpty() {
    $('projectDetail').innerHTML = '<div class="project-detail-empty"><strong>选择一个项目</strong><span>左侧项目列表会联动展示成员、任务、数据空间、作业和资源账本。</span></div>';
  }

  function renderDetail() {
    const project = selectedProject();
    if (!project) {
      renderDetailEmpty();
      return;
    }
    const metrics = project.metrics || {};
    const progress = Math.max(0, Math.min(100, num(metrics.progressPercent)));
    const manage = canManage(project);
    const canWork = manage || project.currentUserAccess === 'work';
    $('projectDetail').innerHTML = `
      <div class="project-detail-head">
        <div class="project-title-line">
          <span class="project-avatar large">${projectIcon('project')}</span>
          <div>
            <span class="project-eyebrow">${esc(project.code)}</span>
            <h3>${esc(project.name)}</h3>
            <p>${esc(project.summary || '暂无项目说明')}</p>
          </div>
        </div>
        <div class="project-actions">
          <button class="btn btn-ghost" type="button" data-project-action="open-jobs">${projectIcon('list')}查看作业</button>
          ${canWork ? `<button class="btn btn-primary" type="button" data-project-action="submit-job">${projectIcon('send')}提交作业</button>` : ''}
          ${manage ? `<button class="btn btn-ghost" type="button" data-project-action="sync-slurm">${projectIcon('sync')}同步 Account</button><button class="btn btn-ghost" type="button" data-project-action="edit-project">${projectIcon('edit')}编辑</button><button class="btn btn-primary" type="button" data-project-action="add-task">${projectIcon('plus')}新建任务</button><button class="btn btn-ghost project-danger-btn" type="button" data-project-action="delete-project">${projectIcon('trash')}删除</button>` : ''}
        </div>
      </div>
      <div class="project-meta-strip">
        <span class="project-status project-status-${esc(project.status)}">${esc(statusLabel(project.status))}</span>
        <span class="project-sync-pill ${slurmStatusClass(project.slurmSyncStatus)}">${esc(slurmStatusLabel(project.slurmSyncStatus))}</span>
        <span>Account：${esc(project.slurmAccount || project.code)}</span>
        <span>负责人：${esc(project.ownerDisplayName || project.ownerUsername || '未指定')}</span>
        <span>团队：${esc(project.teamName || '未绑定团队')}</span>
        <span>周期：${esc(formatDate(project.startDate))} - ${esc(formatDate(project.endDate))}</span>
      </div>
      <div class="project-slurm-strip">
        <strong>Slurm 记账</strong>
        <span>父级 Account：${esc(project.slurmParentAccount || '未配置')}</span>
        <span>QOS：${esc(project.slurmQos || '继承集群默认')}</span>
        <span>最近同步：${esc(dateTime(project.slurmSyncedAt))}</span>
        <span>${esc(project.slurmSyncMessage || '保存项目或成员后可手动同步 Slurm Account 与用户关联。')}</span>
      </div>
      <section class="project-resource-board">
        <div class="project-progress">
          <div><strong>${progress.toFixed(0)}%</strong><span>任务完成度</span></div>
          <i style="--project-progress:${progress}%"></i>
        </div>
        <div><strong>${metrics.memberCount || 0}</strong><span>项目成员</span></div>
        <div><strong>${metrics.jobCount || 0}</strong><span>项目作业</span></div>
        <div><strong>${num(metrics.allocatedCpuHours).toLocaleString('zh-CN', { maximumFractionDigits: 2 })}</strong><span>CPU 核时</span></div>
        <div><strong>${num(metrics.allocatedGpuHours).toLocaleString('zh-CN', { maximumFractionDigits: 2 })}</strong><span>GPU 卡时</span></div>
        <div><strong>${metrics.failedJobCount || 0}</strong><span>异常作业</span></div>
        <div><strong>${(project.storageQuotaGb || 0).toLocaleString('zh-CN')} GB</strong><span>存储额度</span></div>
        <div><strong>${(project.licenseBudgetPoints || 0).toLocaleString('zh-CN')}</strong><span>License 预算</span></div>
      </section>
      ${manage ? '' : '<div class="project-readonly-note">你当前是项目只读/协作成员，管理类操作会由后端权限保护。</div>'}
      <section class="project-detail-grid">
        ${membersPanel(project.members || [], manage)}
        ${tasksPanel(project.tasks || [], manage)}
        ${directoriesPanel(project.directories || [], manage)}
        ${jobsPanel(project.jobLinks || [], manage)}
        ${activityPanel(project.activities || [])}
      </section>
    `;
  }

  function membersPanel(items, manage) {
    return `<article class="project-panel">
      <div class="project-panel-head"><div><h3>项目成员</h3><p>跨单位/团队协作成员和项目内角色。</p></div><button class="btn btn-ghost" type="button" data-project-action="add-member">${projectIcon('plus')}添加成员</button></div>
      <div class="project-member-list">${items.length ? items.map((item) => `
        <div class="project-member-row">
          <span class="project-avatar small">${esc((item.displayName || item.username || 'U').slice(0, 1).toUpperCase())}</span>
          <div><strong>${esc(item.displayName || item.username)} ${item.defaultProject ? '<em class="project-default-badge">默认项目</em>' : ''}</strong><small>${esc(item.username)} · ${esc(roleMap[item.role] || item.role)} · ${esc(permissionMap[item.permission] || item.permission)}</small></div>
          ${manage && !item.defaultProject ? `<button class="project-row-action" type="button" data-project-action="set-default-member" data-username="${esc(item.username)}">设为默认</button>` : ''}
          ${manage ? `<button class="project-row-action" type="button" data-project-action="remove-member" data-username="${esc(item.username)}">移除</button>` : ''}
        </div>`).join('') : '<div class="project-empty compact">暂无成员。</div>'}</div>
    </article>`;
  }

  function tasksPanel(items, manage) {
    return `<article class="project-panel">
      <div class="project-panel-head"><div><h3>项目任务</h3><p>把计算流程拆成可追踪任务。</p></div>${manage ? `<button class="btn btn-ghost" type="button" data-project-action="add-task">${projectIcon('plus')}新建任务</button>` : ''}</div>
      <div class="project-task-list">${items.length ? items.map((item) => `
        <div class="project-task-row">
          <span class="project-task-status project-task-${esc(item.status)}">${esc(taskStatusMap[item.status] || item.status)}</span>
          <div><strong>${esc(item.title)}</strong><small>${esc(item.assigneeUsername || '未分配')} · 截止 ${esc(formatDate(item.dueDate))}</small></div>
          ${manage ? `<button class="project-row-action" type="button" data-project-action="edit-task" data-task-id="${item.id}">编辑</button><button class="project-row-action danger" type="button" data-project-action="delete-task" data-task-id="${item.id}">删除</button>` : ''}
        </div>`).join('') : '<div class="project-empty compact">暂无任务，建议先创建“数据准备 / 提交计算 / 结果分析”。</div>'}</div>
    </article>`;
  }

  function directoriesPanel(items, manage) {
    return `<article class="project-panel">
      <div class="project-panel-head"><div><h3>数据空间</h3><p>项目目录只允许配置在 /data/projects 或 /data/project 下。</p></div>${manage ? `<button class="btn btn-ghost" type="button" data-project-action="add-directory">${projectIcon('plus')}添加目录</button>` : ''}</div>
      <div class="project-directory-list">${items.length ? items.map((item) => `
        <div class="project-directory-row">
          <div><strong>${esc(item.name)}</strong><small><code>${esc(item.path)}</code></small></div>
          <span>${esc(item.permission)}</span>
          ${manage ? `<button class="project-row-action danger" type="button" data-project-action="delete-directory" data-directory-id="${item.id}">删除</button>` : ''}
        </div>`).join('') : '<div class="project-empty compact">暂无项目目录。</div>'}</div>
    </article>`;
  }

  function jobsPanel(items, manage) {
    return `<article class="project-panel">
      <div class="project-panel-head"><div><h3>项目作业</h3><p>按 Slurm Account 自动归集，也可由项目管理者补充关联。</p></div>${manage ? `<button class="btn btn-ghost" type="button" data-project-action="add-job">${projectIcon('plus')}关联作业</button>` : ''}</div>
      <div class="project-job-table">${items.length ? items.map((item) => `
        <div class="project-job-row">
          <strong>${esc(item.jobId)}</strong><span>${esc(item.jobName || '未填写名称')}</span><span>${esc(item.username || '—')}</span><span>${esc(item.account || '未记录 Account')}</span><span>${esc(item.state || '未知')}</span>
          ${Number(item.id) > 0
            ? (manage
              ? `<button class="project-row-action danger" type="button" data-project-action="delete-job" data-link-id="${item.id}">取消关联</button>`
              : '<em class="project-auto-link">手动关联</em>')
            : '<em class="project-auto-link">Account 自动归集</em>'}
        </div>`).join('') : '<div class="project-empty compact">暂无关联作业，可以从作业列表复制作业 ID 后绑定。</div>'}</div>
    </article>`;
  }

  function activityPanel(items) {
    return `<article class="project-panel project-activity-panel">
      <div class="project-panel-head"><div><h3>项目动态</h3><p>记录项目配置、成员、任务和作业变更。</p></div></div>
      <div class="project-activity-list">${items.length ? items.map((item) => `
        <div><b>${esc(item.message || item.action)}</b><span>${esc(item.actor || 'system')} · ${esc(dateTime(item.createdAt))}</span></div>
      `).join('') : '<div class="project-empty compact">暂无动态。</div>'}</div>
    </article>`;
  }

  function setError(message) {
    state.error = message || '';
    const el = $('projectError');
    el.hidden = !state.error;
    el.textContent = state.error;
  }

  async function loadProjects(keepSelection = false) {
    state.loading = true;
    setError('');
    renderProjectList();
    try {
      const params = new URLSearchParams();
      state.keyword = ($('projectSearch')?.value || $('globalSearch')?.value || '').trim();
      state.status = $('projectStatusFilter')?.value || '';
      if (state.keyword) params.set('q', state.keyword);
      if (state.status) params.set('status', state.status);
      const data = await request('/api/v1/projects' + (params.toString() ? '?' + params.toString() : ''));
      state.projects = data.items || [];
      state.summary = data.summary || {};
      if (!keepSelection || !state.projects.some((item) => String(item.id) === String(state.selectedId))) {
        state.selectedId = state.projects[0] ? String(state.projects[0].id) : '';
      }
      if (state.selectedId) await loadProjectDetail(state.selectedId);
    } catch (err) {
      state.projects = [];
      state.summary = {};
      state.selectedId = '';
      setError('项目数据加载失败：' + err.message);
      renderKpis();
      renderProjectList();
      renderDetail();
    } finally {
      state.loading = false;
      renderKpis();
      renderProjectList();
      renderDetail();
    }
  }

  async function loadProjectDetail(id) {
    const project = await request('/api/v1/projects/' + encodeURIComponent(id));
    const index = state.projects.findIndex((item) => String(item.id) === String(id));
    if (index >= 0) state.projects[index] = project;
    state.selectedId = String(id);
  }

  function projectForm(project) {
    const tags = (project?.tags || []).join('，');
    return `
      <div class="project-form-grid">
        <label><span>项目名称</span><input name="name" required maxlength="80" value="${esc(project?.name || '')}" placeholder="例如 翼型 CFD 优化项目"></label>
        <label><span>项目编码</span><input name="code" required maxlength="48" pattern="[a-z][a-z0-9_-]{2,47}" value="${esc(project?.code || '')}" ${project?.id ? 'readonly' : ''} placeholder="例如 cfd_airfoil_2026"></label>
        <label><span>负责人账号</span><input name="ownerUsername" required value="${esc(project?.ownerUsername || currentUsername())}" placeholder="Linux/LDAP 账号"></label>
        <label><span>Slurm Account</span><input name="slurmAccount" required maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_.-]{0,63}" value="${esc(project?.slurmAccount || project?.code || '')}" placeholder="默认与项目编码一致"></label>
        <label><span>父级 Account</span><input name="slurmParentAccount" maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_.-]{0,63}" value="${esc(project?.slurmParentAccount || '')}" placeholder="例如 root / research"></label>
        <label><span>Slurm QOS</span><input name="slurmQos" maxlength="64" pattern="[A-Za-z0-9][A-Za-z0-9_.-]{0,63}" value="${esc(project?.slurmQos || '')}" placeholder="例如 normal / gpu"></label>
        <label><span>状态</span><select name="status">${selectOptions(statusMap, project?.status || 'planning')}</select></label>
        <label><span>优先级</span><select name="priority">${selectOptions(priorityMap, project?.priority || 'normal')}</select></label>
        <label><span>存储额度 GB</span><input type="number" min="0" name="storageQuotaGb" value="${esc(project?.storageQuotaGb ?? 100)}"></label>
        <label><span>计算额度 小时</span><input type="number" min="0" name="computeQuotaHours" value="${esc(project?.computeQuotaHours ?? 500)}"></label>
        <label><span>License 预算点数</span><input type="number" min="0" name="licenseBudgetPoints" value="${esc(project?.licenseBudgetPoints ?? 100)}"></label>
        <label><span>开始日期</span><input type="date" name="startDate" value="${esc(project?.startDate || '')}"></label>
        <label><span>结束日期</span><input type="date" name="endDate" value="${esc(project?.endDate || '')}"></label>
        <label class="project-check-field"><span>同步到 Slurm</span><input type="checkbox" name="slurmSyncEnabled" ${project?.slurmSyncEnabled === false ? '' : 'checked'}><small>保存后自动尝试创建 Account 并关联成员</small></label>
        <label class="wide"><span>标签</span><input name="tags" value="${esc(tags)}" placeholder="仿真，CFD，重点项目"></label>
        <label class="wide"><span>项目说明</span><textarea name="summary" rows="4" placeholder="说明项目目标、计算流程和交付结果">${esc(project?.summary || '')}</textarea></label>
      </div>`;
  }

  function selectOptions(source, selected) {
    return Object.entries(source).map(([key, value]) => {
      const label = Array.isArray(value) ? value[0] : value;
      return `<option value="${esc(key)}" ${key === selected ? 'selected' : ''}>${esc(label)}</option>`;
    }).join('');
  }

  function currentUsername() {
    try {
      return JSON.parse(localStorage.getItem('simplehpc_user') || '{}')?.username || 'admin';
    } catch (_) {
      return 'admin';
    }
  }

  function readProjectForm(form) {
    const get = (name) => form.querySelector('[name="' + name + '"]')?.value || '';
    return {
      name: get('name'),
      code: get('code'),
      ownerUsername: get('ownerUsername'),
      slurmAccount: get('slurmAccount'),
      slurmParentAccount: get('slurmParentAccount'),
      slurmQos: get('slurmQos'),
      slurmSyncEnabled: !!form.querySelector('[name="slurmSyncEnabled"]')?.checked,
      status: get('status'),
      priority: get('priority'),
      storageQuotaGb: Number(get('storageQuotaGb') || 0),
      computeQuotaHours: Number(get('computeQuotaHours') || 0),
      licenseBudgetPoints: Number(get('licenseBudgetPoints') || 0),
      startDate: get('startDate'),
      endDate: get('endDate'),
      summary: get('summary'),
      tags: get('tags').split(/[，,]/).map((item) => item.trim()).filter(Boolean)
    };
  }

  function openProjectModal(project) {
    const modal = App.modal({
      title: project ? '编辑项目' : '新建项目',
      width: '820px',
      content: `<form id="projectForm">${projectForm(project)}</form>`,
      confirmText: project ? '保存项目' : '创建项目',
      errorPrefix: project ? '保存项目失败' : '创建项目失败',
      onSubmit: async () => {
        const form = document.getElementById('projectForm');
        if (!form.reportValidity()) throw new Error('请先完善必填项');
        const body = readProjectForm(form);
        const path = project ? '/api/v1/projects/' + project.id : '/api/v1/projects';
        const method = project ? 'PUT' : 'POST';
        const saved = await request(path, { method, body: JSON.stringify(body) });
        toast(project ? '项目已保存' : '项目已创建', 'success');
        state.selectedId = String(saved.id);
        await loadProjects(true);
      }
    });
    const form = modal.el.querySelector('#projectForm');
    if (form && !project) {
      const codeInput = form.querySelector('[name="code"]');
      const accountInput = form.querySelector('[name="slurmAccount"]');
      codeInput?.addEventListener('input', () => {
        if (accountInput && !accountInput.dataset.touched) accountInput.value = codeInput.value;
      });
      accountInput?.addEventListener('input', () => { accountInput.dataset.touched = '1'; });
    }
  }

  function openMemberModal() {
    const project = selectedProject();
    if (!project) return;
    App.modal({
      title: '添加项目成员',
      width: '620px',
      content: `<form id="projectMemberForm" class="project-form-grid">
        <label><span>成员账号</span><input name="username" required placeholder="例如 user001"></label>
        <label><span>显示名称</span><input name="displayName" placeholder="例如 张三"></label>
        <label><span>项目角色</span><select name="role">${selectOptions(roleMap, 'compute_member')}</select></label>
        <label><span>项目权限</span><select name="permission">${selectOptions(permissionMap, 'work')}</select></label>
        <label class="project-check-field wide"><span>默认项目</span><input type="checkbox" name="defaultProject"><small>勾选后该用户提交作业时默认使用本项目 Account。</small></label>
      </form>`,
      confirmText: '保存成员',
      errorPrefix: '保存成员失败',
      onSubmit: async () => {
        const form = document.getElementById('projectMemberForm');
        if (!form.reportValidity()) throw new Error('请填写成员账号');
        const body = {
          username: form.username.value,
          displayName: form.displayName.value,
          role: form.role.value,
          permission: form.permission.value,
          status: 'active',
          defaultProject: !!form.defaultProject.checked
        };
        await request('/api/v1/projects/' + project.id + '/members', { method: 'POST', body: JSON.stringify(body) });
        toast('成员已保存', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      }
    });
  }

  function openTaskModal(task) {
    const project = selectedProject();
    if (!project) return;
    App.modal({
      title: task ? '编辑项目任务' : '新建项目任务',
      width: '720px',
      content: `<form id="projectTaskForm" class="project-form-grid">
        <label class="wide"><span>任务名称</span><input name="title" required maxlength="120" value="${esc(task?.title || '')}" placeholder="例如 提交第一轮求解作业"></label>
        <label><span>负责人账号</span><input name="assigneeUsername" value="${esc(task?.assigneeUsername || currentUsername())}"></label>
        <label><span>状态</span><select name="status">${selectOptions(taskStatusMap, task?.status || 'todo')}</select></label>
        <label><span>优先级</span><select name="priority">${selectOptions(priorityMap, task?.priority || 'normal')}</select></label>
        <label><span>截止日期</span><input type="date" name="dueDate" value="${esc(task?.dueDate || '')}"></label>
        <label class="wide"><span>任务说明</span><textarea name="description" rows="4">${esc(task?.description || '')}</textarea></label>
      </form>`,
      confirmText: '保存任务',
      errorPrefix: '保存任务失败',
      onSubmit: async () => {
        const form = document.getElementById('projectTaskForm');
        if (!form.reportValidity()) throw new Error('请填写任务名称');
        const body = {
          id: task?.id || 0,
          title: form.title.value,
          assigneeUsername: form.assigneeUsername.value,
          status: form.status.value,
          priority: form.priority.value,
          dueDate: form.dueDate.value,
          description: form.description.value
        };
        await request('/api/v1/projects/' + project.id + '/tasks', { method: 'POST', body: JSON.stringify(body) });
        toast('任务已保存', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      }
    });
  }

  function openDirectoryModal() {
    const project = selectedProject();
    if (!project) return;
    App.modal({
      title: '添加项目数据空间',
      width: '640px',
      content: `<form id="projectDirectoryForm" class="project-form-grid">
        <label><span>目录名称</span><input name="name" required value="项目数据空间"></label>
        <label><span>权限</span><select name="permission"><option value="r">只读</option><option value="rw" selected>读写</option><option value="rwx">读写执行</option><option value="manage">管理</option></select></label>
        <label class="wide"><span>目录路径</span><input name="path" required value="/data/projects/${esc(project.code)}" placeholder="/data/projects/project_code"></label>
      </form>`,
      confirmText: '保存目录',
      errorPrefix: '保存目录失败',
      onSubmit: async () => {
        const form = document.getElementById('projectDirectoryForm');
        if (!form.reportValidity()) throw new Error('请填写目录名称和路径');
        await request('/api/v1/projects/' + project.id + '/directories', {
          method: 'POST',
          body: JSON.stringify({ name: form.name.value, path: form.path.value, permission: form.permission.value, status: 'active' })
        });
        toast('项目数据空间已保存', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      }
    });
  }

  function openJobModal() {
    const project = selectedProject();
    if (!project) return;
    App.modal({
      title: '关联 Slurm 作业',
      width: '650px',
      content: `<form id="projectJobForm" class="project-form-grid">
        <label><span>作业 ID</span><input name="jobId" required placeholder="例如 12345"></label>
        <label><span>作业名称</span><input name="jobName" placeholder="例如 fluent_airfoil_run01"></label>
        <label><span>提交用户</span><input name="username" value="${esc(currentUsername())}"></label>
        <label><span>作业状态</span><input name="state" placeholder="RUNNING / PENDING / COMPLETED"></label>
        <label><span>分区</span><input name="partition" placeholder="例如 cpu / gpu"></label>
        <label class="wide"><span>Slurm Account</span><input name="account" value="${esc(project.slurmAccount || project.code)}" placeholder="项目 Account"></label>
      </form>`,
      confirmText: '关联作业',
      errorPrefix: '关联作业失败',
      onSubmit: async () => {
        const form = document.getElementById('projectJobForm');
        if (!form.reportValidity()) throw new Error('请填写作业 ID');
        await request('/api/v1/projects/' + project.id + '/job-links', {
          method: 'POST',
          body: JSON.stringify({
            jobId: form.jobId.value,
            jobName: form.jobName.value,
            username: form.username.value,
            account: form.account.value,
            state: form.state.value,
            partition: form.partition.value
          })
        });
        toast('作业已关联到项目', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      }
    });
  }

  function confirmDanger(message, onConfirm) {
    App.confirm(message, {
      danger: true,
      confirmText: '确认',
      onConfirm: async () => {
        App.loading?.show('正在执行操作...');
        try {
          await onConfirm();
        } catch (err) {
          toast(err.message || '操作失败', 'danger');
        } finally {
          App.loading?.hide();
        }
      }
    });
  }

  async function handleAction(action, target) {
    const project = selectedProject();
    if (action === 'select') {
      state.selectedId = target.dataset.id;
      await loadProjectDetail(state.selectedId);
      renderProjectList();
      renderDetail();
      return;
    }
    if (action === 'edit-project') return openProjectModal(project);
    if (action === 'sync-slurm') {
      App.loading?.show('正在同步 Slurm Account...');
      try {
        const updated = await request('/api/v1/projects/' + project.id + '/slurm-sync', { method: 'POST', body: '{}' });
        const index = state.projects.findIndex((item) => String(item.id) === String(project.id));
        if (index >= 0) state.projects[index] = updated;
        toast('Slurm Account 已同步', 'success');
        renderProjectList();
        renderDetail();
      } finally {
        App.loading?.hide();
      }
      return;
    }
    if (action === 'submit-job') {
      const account = encodeURIComponent(project.slurmAccount || project.code || '');
      window.location.href = 'job-templates.html?account=' + account + '&projectId=' + encodeURIComponent(project.id);
      return;
    }
    if (action === 'open-jobs') {
      window.location.href = 'job-list.html?projectId=' + encodeURIComponent(project.id);
      return;
    }
    if (action === 'delete-project') {
      return confirmDanger('确认删除该项目？项目成员、任务、目录和作业关联记录都会删除。', async () => {
        await request('/api/v1/projects/' + project.id, { method: 'DELETE' });
        toast('项目已删除', 'success');
        state.selectedId = '';
        await loadProjects(false);
      });
    }
    if (action === 'add-member') return openMemberModal();
    if (action === 'remove-member') {
      const username = target.dataset.username;
      return confirmDanger('确认移除成员 ' + username + '？', async () => {
        await request('/api/v1/projects/' + project.id + '/members/' + encodeURIComponent(username), { method: 'DELETE' });
        toast('成员已移除', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      });
    }
    if (action === 'set-default-member') {
      const username = target.dataset.username;
      return confirmDanger('确认把 ' + username + ' 的默认项目设置为当前项目？', async () => {
        const updated = await request('/api/v1/projects/' + project.id + '/members/' + encodeURIComponent(username) + '/default', { method: 'POST', body: '{}' });
        const index = state.projects.findIndex((item) => String(item.id) === String(project.id));
        if (index >= 0) state.projects[index] = updated;
        toast('默认项目已更新', 'success');
        renderProjectList();
        renderDetail();
      });
    }
    if (action === 'add-task') return openTaskModal();
    if (action === 'edit-task') {
      const task = (project.tasks || []).find((item) => String(item.id) === String(target.dataset.taskId));
      return openTaskModal(task);
    }
    if (action === 'delete-task') {
      return confirmDanger('确认删除这个任务？', async () => {
        await request('/api/v1/projects/' + project.id + '/tasks/' + target.dataset.taskId, { method: 'DELETE' });
        toast('任务已删除', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      });
    }
    if (action === 'add-directory') return openDirectoryModal();
    if (action === 'delete-directory') {
      return confirmDanger('确认删除这个项目数据空间配置？不会删除真实文件。', async () => {
        await request('/api/v1/projects/' + project.id + '/directories/' + target.dataset.directoryId, { method: 'DELETE' });
        toast('目录配置已删除', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      });
    }
    if (action === 'add-job') return openJobModal();
    if (action === 'delete-job') {
      return confirmDanger('确认取消这个作业关联？', async () => {
        await request('/api/v1/projects/' + project.id + '/job-links/' + target.dataset.linkId, { method: 'DELETE' });
        toast('作业关联已取消', 'success');
        await loadProjectDetail(project.id);
        renderProjectList();
        renderDetail();
      });
    }
  }

  function bindEvents() {
    $('projectCreateBtn')?.addEventListener('click', () => openProjectModal());
    $('projectRefreshBtn')?.addEventListener('click', () => loadProjects(true));
    $('projectRefreshIcon')?.addEventListener('click', () => loadProjects(true));
    $('projectResetFilterBtn')?.addEventListener('click', () => {
      $('projectSearch').value = '';
      $('globalSearch').value = '';
      $('projectStatusFilter').value = '';
      loadProjects(false);
    });
    $('projectSearch')?.addEventListener('input', debounce(() => loadProjects(false), 260));
    $('globalSearch')?.addEventListener('input', debounce(() => {
      if ($('projectSearch')) $('projectSearch').value = $('globalSearch').value;
      loadProjects(false);
    }, 260));
    $('projectStatusFilter')?.addEventListener('change', () => loadProjects(false));
    document.addEventListener('click', (event) => {
      const target = event.target.closest('[data-project-action]');
      if (!target) return;
      event.preventDefault();
      handleAction(target.dataset.projectAction, target).catch((err) => {
        toast(err.message || '操作失败', 'danger');
      });
    });
  }

  function debounce(fn, wait) {
    let timer = null;
    return function () {
      clearTimeout(timer);
      timer = setTimeout(fn, wait);
    };
  }

  function init() {
    bindEvents();
    renderKpis();
    renderProjectList();
    renderDetail();
    loadProjects(false);
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
