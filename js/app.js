/**
 * simpleHPC 前端交互引擎 v2
 * Toast / Modal / Drawer / Loading / Confirm / Dropdown +
 * SearchFilter / TableSort / BatchSelect / StateToggle / ExpandModal
 */
(function () {
  'use strict';
  const $el = (sel, root) => (root || document).querySelector(sel);
  const $els = (sel, root) => Array.from((root || document).querySelectorAll(sel));

  function authHeaders(extra) {
    const headers = Object.assign({}, extra || {});
    const token = localStorage.getItem('simplehpc_token') || '';
    if (token && !headers.Authorization) headers.Authorization = 'Bearer ' + token;
    return headers;
  }

  function apiFetch(url, options) {
    const opts = Object.assign({ cache: 'no-store' }, options || {});
    opts.headers = authHeaders(opts.headers || {});
    return fetch(url, opts);
  }

  /* ===== Toast ===== */
  const Toast = {
    _container: null,
    _init() {
      if (this._container) return;
      this._container = document.createElement('div');
      Object.assign(this._container.style, {
        position: 'fixed', top: '20px', right: '20px', zIndex: '10000',
        display: 'flex', flexDirection: 'column', gap: '10px', pointerEvents: 'none'
      });
      document.body.appendChild(this._container);
    },
    show(msg, type = 'info', duration = 3000) {
      this._init();
      const icons = {
        success: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>',
        warn: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
        danger: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>',
        info: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>'
      };
      const el = document.createElement('div');
      el.className = '_shpc-toast _shpc-toast--' + type;
      el.innerHTML = (icons[type] || icons.info) + '<span>' + msg + '</span>';
      this._container.appendChild(el);
      requestAnimationFrame(() => el.classList.add('_shpc-toast--in'));
      setTimeout(() => {
        el.classList.remove('_shpc-toast--in');
        el.classList.add('_shpc-toast--out');
        el.addEventListener('transitionend', () => el.remove(), { once: true });
      }, duration);
    }
  };

  /* ===== Loading ===== */
  const Loading = {
    _el: null,
    show(text) {
      if (!this._el) {
        this._el = document.createElement('div');
        this._el.className = '_shpc-loading-overlay';
        this._el.innerHTML = '<div class="_shpc-loading-box"><div class="_shpc-loading-spin"></div><div class="_shpc-loading-text">' + text + '</div></div>';
        document.body.appendChild(this._el);
      }
      this._el.querySelector('._shpc-loading-text').textContent = text;
      this._el.style.display = 'flex';
      requestAnimationFrame(() => this._el.classList.add('_shpc-loading-overlay--show'));
    },
    hide() {
      if (!this._el) return;
      this._el.classList.remove('_shpc-loading-overlay--show');
      setTimeout(() => { this._el.style.display = 'none'; }, 250);
    }
  };

  /* ===== Confirm ===== */
  const Confirm = {
    show(msg, opts) {
      opts = opts || {};
      const onConfirm = opts.onConfirm, onCancel = opts.onCancel, okText = opts.confirmText || '确认', danger = !!opts.danger;
      const mask = document.createElement('div');
      mask.className = '_shpc-modal _shpc-modal--confirm';
      mask.style.display = 'flex';
      mask.innerHTML = '<div class="_shpc-modal-panel" style="max-width:400px">' +
        '<div style="padding:24px 24px 8px;"><div style="font-size:16px;font-weight:600;margin-bottom:8px">确认操作</div>' +
        '<div style="font-size:14px;color:var(--muted);line-height:1.5">' + msg + '</div></div>' +
        '<div class="_shpc-modal-footer"><button class="btn btn-ghost _shpc-btn-cancel">' + (opts.cancelText || '取消') + '</button>' +
        '<button class="btn ' + (danger ? '_shpc-btn-danger' : 'btn-primary') + ' _shpc-btn-ok">' + okText + '</button></div></div>';
      document.body.appendChild(mask);
      const close = () => { mask.classList.remove('_shpc-modal--in'); mask.addEventListener('transitionend', () => mask.remove(), { once: true }); };
      mask.querySelector('._shpc-btn-cancel').addEventListener('click', () => { close(); if (onCancel) onCancel(); });
      mask.querySelector('._shpc-btn-ok').addEventListener('click', () => { close(); if (onConfirm) onConfirm(); });
      mask.addEventListener('click', (e) => { if (e.target === mask) { close(); if (onCancel) onCancel(); } });
      requestAnimationFrame(() => mask.classList.add('_shpc-modal--in'));
    }
  };

  /* ===== Modal ===== */
  const Modal = {
    open(opts) {
      opts = opts || {};
      const title = opts.title || '弹窗', content = opts.content || '', width = opts.width || '560px', onSubmit = opts.onSubmit, onClose = opts.onClose, showFooter = opts.showFooter !== false;
      const confirmText = opts.confirmText || '保存', cancelText = opts.cancelText || '取消', errorPrefix = opts.errorPrefix || '保存失败';
      const mask = document.createElement('div');
      mask.className = '_shpc-modal';
      mask.style.display = 'flex';
      const footer = showFooter ? '<div class="_shpc-modal-footer"><button class="btn btn-ghost _shpc-modal-btn-close">' + cancelText + '</button><button class="btn btn-primary _shpc-modal-btn-ok">' + confirmText + '</button></div>' : '';
      mask.innerHTML = '<div class="_shpc-modal-panel" style="max-width:' + width + '"><div class="_shpc-modal-header"><span>' + title + '</span><button class="_shpc-modal-x">×</button></div><div class="_shpc-modal-body">' + content + '</div>' + footer + '</div>';
      document.body.appendChild(mask);
      const close = () => { mask.classList.remove('_shpc-modal--in'); mask.addEventListener('transitionend', () => { mask.remove(); if (onClose) onClose(); }, { once: true }); };
      mask.querySelector('._shpc-modal-x').addEventListener('click', close);
      if (showFooter) {
        mask.querySelector('._shpc-modal-btn-close').addEventListener('click', close);
        mask.querySelector('._shpc-modal-btn-ok').addEventListener('click', async () => {
          const button = mask.querySelector('._shpc-modal-btn-ok');
          try {
            button.disabled = true;
            if (onSubmit) await onSubmit();
            close();
          } catch (err) {
            button.disabled = false;
            Toast.show(errorPrefix + '：' + (err?.message || err), 'danger', 5000);
          }
        });
      }
      mask.addEventListener('click', (e) => { if (e.target === mask) close(); });
      requestAnimationFrame(() => mask.classList.add('_shpc-modal--in'));
      return { close, el: mask };
    }
  };

  /* ===== Drawer (modal-style, kept for existing page calls) ===== */
  const Drawer = {
    open(opts) {
      opts = opts || {};
      const title = opts.title || '', content = opts.content || '', width = opts.width || '520px', onClose = opts.onClose, onSubmit = opts.onSubmit;
      const footerHtml = opts.footerHtml;
      const mask = document.createElement('div');
      mask.className = '_shpc-drawer _shpc-drawer--modal';
      mask.style.display = 'flex';
      let footer = footerHtml ? '<div class="_shpc-drawer-footer">' + footerHtml + '</div>' : '<div class="_shpc-drawer-footer"><button class="btn btn-ghost _shpc-drawer-btn-close">取消</button><button class="btn btn-primary _shpc-drawer-btn-ok">保存</button></div>';
      mask.innerHTML = '<div class="_shpc-drawer-back" style="flex:1"></div><div class="_shpc-drawer-panel" style="width:' + width + '">' +
        '<div class="_shpc-drawer-header"><span>' + title + '</span><button class="_shpc-drawer-x">×</button></div>' +
        '<div class="_shpc-drawer-body">' + content + '</div>' + footer + '</div>';
      document.body.appendChild(mask);
      const close = () => { mask.classList.remove('_shpc-drawer--in'); mask.addEventListener('transitionend', () => { mask.remove(); if (onClose) onClose(); }, { once: true }); };
      mask.querySelector('._shpc-drawer-x').addEventListener('click', close);
      const closeBtn = mask.querySelector('._shpc-drawer-btn-close');
      if (closeBtn) closeBtn.addEventListener('click', close);
      const okBtn = mask.querySelector('._shpc-drawer-btn-ok');
      if (okBtn) okBtn.addEventListener('click', () => { if (onSubmit) onSubmit(); close(); });
      mask.querySelector('._shpc-drawer-back').addEventListener('click', close);
      requestAnimationFrame(() => mask.classList.add('_shpc-drawer--in'));
      return { close, el: mask };
    }
  };

  /* ===== Dropdown ===== */
  const Dropdown = {
    open(triggerEl, contentHTML, opts) {
      opts = opts || {};
      const placement = opts.placement || 'bottom-end', w = opts.width || '200px';
      if (triggerEl._shpcDrop) { this.close(triggerEl); return; }
      const pop = document.createElement('div');
      pop.className = '_shpc-dropdown'; pop.innerHTML = contentHTML;
      pop.style.cssText = 'position:fixed;width:' + w + ';z-index:9999;';
      document.body.appendChild(pop);
      const rect = triggerEl.getBoundingClientRect();
      if (placement === 'bottom-end') { pop.style.top = (rect.bottom + 8) + 'px'; pop.style.left = (rect.right - parseInt(w)) + 'px'; }
      else if (placement === 'bottom-start') { pop.style.top = (rect.bottom + 8) + 'px'; pop.style.left = rect.left + 'px'; }
      requestAnimationFrame(() => pop.classList.add('_shpc-dropdown--in'));
      triggerEl._shpcDrop = pop;
      const docClick = (e) => { if (!pop.contains(e.target) && e.target !== triggerEl && !triggerEl.contains(e.target)) { Dropdown.close(triggerEl); } };
      document.addEventListener('mousedown', docClick);
      pop._docClick = docClick;
    },
    close(triggerEl) {
      const pop = triggerEl._shpcDrop;
      if (!pop) return;
      pop.classList.remove('_shpc-dropdown--in');
      document.removeEventListener('mousedown', pop._docClick);
      pop.addEventListener('transitionend', () => { if (pop.parentNode) pop.remove(); }, { once: true });
      triggerEl._shpcDrop = null;
    }
  };

  /* ===== Search Filter ===== */
  function wireSearchFilter(input) {
    const targetSel = input.dataset.searchTarget;
    if (!targetSel) return;
    const table = $el(targetSel);
    if (!table) return;
    const rows = $els('tbody tr', table);
    input.addEventListener('input', function () {
      const kw = this.value.trim().toLowerCase();
      rows.forEach(row => { row.style.display = row.textContent.toLowerCase().includes(kw) ? '' : 'none'; });
    });
  }

  /* ===== Table Sort ===== */
  function wireTableSort(th) {
    const colIndex = parseInt(th.dataset.sort, 10); if (isNaN(colIndex)) return;
    const table = th.closest('table');
    if (!table) return;
    let dir = 1;
    th.style.cursor = 'pointer';
    th.addEventListener('click', function () {
      const tbody = table.querySelector('tbody');
      const rows = Array.from(tbody.querySelectorAll('tr'));
      rows.sort((a, b) => {
        const ta = (a.children[colIndex]?.textContent || '').trim().toLowerCase();
        const tb = (b.children[colIndex]?.textContent || '').trim().toLowerCase();
        const na = parseFloat(ta.replace(/,/g, '')); const nb = parseFloat(tb.replace(/,/g, ''));
        if (!isNaN(na) && !isNaN(nb)) return (na - nb) * dir;
        return ta.localeCompare(tb) * dir;
      });
      rows.forEach(r => tbody.appendChild(r));
      dir *= -1;
      table.querySelectorAll('th').forEach(h => { h.querySelector('._sort-marker')?.remove(); });
      const marker = document.createElement('span'); marker.className = '_sort-marker';
      marker.textContent = dir === -1 ? ' ↑' : ' ↓'; marker.style.color = 'var(--accent)';
      th.appendChild(marker);
    });
  }

  /* ===== Select Filter ===== */
  function wireSelectFilter(select) {
    const targetSel = select.dataset.filterTarget; const col = parseInt(select.dataset.filterCol, 10);
    if (!targetSel || isNaN(col)) return;
    const table = $el(targetSel); if (!table) return;
    const rows = $els('tbody tr', table);
    select.addEventListener('change', function () {
      const val = this.value;
      rows.forEach(row => {
        const cellText = (row.children[col]?.textContent || '').trim();
        if (val === '全部状态' || val === '全部' || val === '') { row.style.display = ''; }
        else { row.style.display = cellText.includes(val) ? '' : 'none'; }
      });
    });
  }

  /* ===== State Toggle ===== */
  function wireStateToggle(btn) {
    btn.addEventListener('click', function () {
      const targetSel = btn.dataset.toggleTarget;
      const from = btn.dataset.toggleFrom;
      const to = btn.dataset.toggleTo;
      const toLabel = btn.dataset.toggleToLabel;
      const target = targetSel ? ($el(targetSel, btn.closest('tr') || document) || btn.closest('tr')) : btn.closest('tr');
      if (!target) return;
      const pill = target.querySelector('.pill' + (from ? '.' + from : ''));
      if (!pill) return;
      // swap classes
      if (from) { pill.classList.remove(from); }
      if (to) { pill.classList.add(to); }
      if (toLabel) { pill.textContent = toLabel; }
      Toast.show('状态已更新: ' + toLabel, 'success');
      // hide button
      btn.style.display = 'none';
    });
  }

  /* ===== Expand Modal ===== */
  function wireExpandModal(trigger) {
    trigger.addEventListener('click', function (e) {
      e.preventDefault();
      const title = trigger.dataset.expandTitle || '全部内容';
      const sourceSel = trigger.dataset.expandSource;
      const html = trigger.dataset.expandHtml;
      let content = '';
      if (html) { content = html; }
      else if (sourceSel) {
        const sourceEl = $el(sourceSel);
        if (sourceEl) content = sourceEl.outerHTML;
      }
      Modal.open({ title, content: '<div class="table-wrap">' + content + '</div>', showFooter: false });
    });
  }

  /* ===== Batch Select ===== */
  function wireBatchSelect(table) {
    if (table.dataset.batch !== 'true') return;
    const header = table.querySelector('thead tr');
    if (!header || header.querySelector('th._batch-th')) return;
    const th = document.createElement('th'); th.className = '_batch-th'; th.style.width = '40px';
    th.innerHTML = '<input type="checkbox" class="_batch-master" style="cursor:pointer;width:16px;height:16px;">';
    header.insertBefore(th, header.firstChild);
    const rows = $els('tbody tr', table);
    rows.forEach(row => {
      const td = document.createElement('td'); td.style.width = '40px';
      td.innerHTML = '<input type="checkbox" class="_batch-row" style="cursor:pointer;width:16px;height:16px;">';
      row.insertBefore(td, row.firstChild);
    });
    const master = table.querySelector('._batch-master');
    master.addEventListener('change', function () { $els('._batch-row', table).forEach(c => { c.checked = this.checked; }); });
  }

  
  /* ===== Subnav toggle ===== */
  function wireSubnav() {
    $els('.nav-group-toggle').forEach(t => {
      t.addEventListener('click', function() {
        var sm = document.getElementById('submenu-' + this.dataset.submenu);
        if (sm) { this.classList.toggle('expanded'); sm.classList.toggle('expanded'); }
      });
    });
    $els('.nav-submenu-item').forEach(item => {
      if (item.classList.contains('active')) {
        var sm = item.closest('.nav-submenu');
        if (sm) {
          sm.classList.add('expanded');
          var toggle = document.querySelector('.nav-group-toggle[data-submenu="' + sm.id.replace('submenu-','') + '"]');
          if (toggle) toggle.classList.add('expanded');
        }
      }
    });
  }

/* ===== Inline dirty tracking for forms inside drawers ===== */
  function wireDirtyForms() {
    document.addEventListener('input', function (e) {
      const drawer = e.target.closest('._shpc-drawer-panel');
      if (!drawer) return;
      const okBtn = drawer.querySelector('._shpc-drawer-btn-ok');
      if (okBtn && !okBtn.dataset.dirtyWired) {
        okBtn.dataset.dirtyWired = '1';
        okBtn.textContent = '保存 *';
      }
    });
  }

  /* ===== Tooltip on hover ===== */
  function wireTooltips() {
    $els('[data-tooltip]').forEach(el => {
      el.addEventListener('mouseenter', function () {
        const text = this.dataset.tooltip;
        const tip = document.createElement('div');
        tip.className = '_shpc-tooltip';
        tip.textContent = text;
        document.body.appendChild(tip);
        const rect = this.getBoundingClientRect();
        tip.style.left = (rect.left + rect.width / 2 - tip.offsetWidth / 2) + 'px';
        tip.style.top = (rect.top - tip.offsetHeight - 8) + 'px';
        this._tip = tip;
      });
      el.addEventListener('mouseleave', function () { if (this._tip) { this._tip.remove(); this._tip = null; } });
    });
  }

  /* ===== Nav auto-active ===== */
  function wireNav() {
    const current = location.pathname.split('/').pop() || 'index.html';
    $els('.nav-link').forEach(l => {
      const href = l.getAttribute('href');
      l.classList.toggle('active', href === current || (current === '' && href === 'index.html'));
    });
  }

  /* ===== Dropdown wiring via data-dropdown ===== */
  function decodeEntities(str) { const ta = document.createElement('textarea'); ta.innerHTML = str; return ta.value; }
  function escapeHTML(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }
  function formatAccountType(value) {
    const map = { admin: '管理员账号', ldap: 'LDAP 用户', user: '普通用户' };
    return map[value] || value || '—';
  }
  function formatRole(value, roles) {
    const names = {
      cluster_admin: '集群管理员',
      config_admin: '配置管理员',
      unit_admin: '单位管理员',
      team_admin: '团队管理员',
      user: '普通用户'
    };
    const values = Array.isArray(roles) && roles.length ? roles : (value ? [value] : []);
    return values.map(item => names[item] || item).filter(Boolean).join(' / ') || '—';
  }
  function profileField(label, value) {
    const text = value == null || value === '' ? '—' : value;
    return '<div class="_shpc-profile-field"><span>' + escapeHTML(label) + '</span><strong>' + escapeHTML(text) + '</strong></div>';
  }
  async function fetchProfileContext() {
    try {
      const res = await apiFetch('/api/v1/auth/me', { cache: 'no-store' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'HTTP ' + res.status);
      return data;
    } catch (err) {
      let user = {};
      try { user = JSON.parse(localStorage.getItem('simplehpc_user') || '{}') || {}; } catch (_) {}
      return { user, profile: user, roles: user.role ? [user.role] : [] };
    }
  }
  async function openProfileModal() {
    $els('.header-actions .avatar').forEach(el => { if (el._shpcDrop) Dropdown.close(el); });
    const loadingModal = Modal.open({
      title: '个人资料',
      width: '720px',
      showFooter: false,
      content: '<div class="_shpc-profile-loading">正在读取当前账号资料...</div>'
    });
    try {
      const context = await fetchProfileContext();
      const user = context.user || {};
      const profile = Object.assign({}, user, context.profile || {});
      const displayName = profile.displayName || profile.username || '当前用户';
      const roleText = formatRole(profile.role || user.role, context.roles);
      const accountType = profile.accountType || user.type || profile.type;
      const avatar = String(displayName).slice(0, 1) || '用';
      const content =
        '<div class="_shpc-profile-card">' +
          '<div class="_shpc-profile-hero">' +
            '<div class="_shpc-profile-avatar">' + escapeHTML(avatar) + '</div>' +
            '<div><h3>' + escapeHTML(displayName) + '</h3>' +
            '<p>' + escapeHTML(profile.username || '—') + ' · ' + escapeHTML(formatAccountType(accountType)) + '</p></div>' +
            '<span class="_shpc-profile-status">' + escapeHTML(profile.status || 'active') + '</span>' +
          '</div>' +
          '<div class="_shpc-profile-section"><h4>账号信息</h4><div class="_shpc-profile-grid">' +
            profileField('账号', profile.username) +
            profileField('姓名', displayName) +
            profileField('账号类型', formatAccountType(accountType)) +
            profileField('角色', roleText) +
            profileField('邮箱', profile.email) +
            profileField('手机号码', profile.phone) +
          '</div></div>' +
          '<div class="_shpc-profile-section"><h4>组织与资源</h4><div class="_shpc-profile-grid">' +
            profileField('组织单位', profile.unit) +
            profileField('用户组', profile.team) +
            profileField('组长', profile.leaderName) +
            profileField('UID / GID', [profile.uidNumber, profile.gidNumber].filter(Boolean).join(' / ')) +
            profileField('主目录', profile.homeDir || profile.homeDirectory) +
            profileField('同步时间', profile.syncedAt) +
          '</div></div>' +
          '<div class="_shpc-profile-section"><h4>登录与维护</h4><div class="_shpc-profile-grid">' +
            profileField('最近登录', profile.lastLogin) +
            profileField('创建时间', profile.createdAt) +
            profileField('更新时间', profile.updatedAt) +
            profileField('创建来源', profile.createdBy || profile.source) +
          '</div></div>' +
        '</div>';
      loadingModal.el.querySelector('._shpc-modal-body').innerHTML = content;
    } catch (err) {
      loadingModal.el.querySelector('._shpc-modal-body').innerHTML =
        '<div class="api-data-missing">个人资料读取失败：' + escapeHTML(err.message || err) + '</div>';
    }
  }
  function ensureUserDropdowns(root) {
    const menu = {
      html: '<button type="button" class="_shpc-dropdown-item" style="width:100%;border:0;background:transparent;text-align:left;font:inherit" onclick="App.openProfileModal()">个人资料</button>' +
        '<button type="button" class="_shpc-dropdown-item" style="width:100%;border:0;background:transparent;text-align:left;font:inherit" onclick="App.openPasswordDrawer()">修改密码</button>' +
        '<div class="_shpc-dropdown-divider"></div>' +
        '<button type="button" class="_shpc-dropdown-item" style="width:100%;border:0;background:transparent;text-align:left;font:inherit;color:var(--danger)" onclick="App.logout()">退出登录</button>',
      width: '180px',
      placement: 'bottom-end'
    };
    $els('.header-actions .avatar', root).forEach(el => {
      el.dataset.dropdown = JSON.stringify(menu);
      el.style.cursor = 'pointer';
      el.setAttribute('role', 'button');
      el.setAttribute('tabindex', '0');
      el.setAttribute('aria-label', '用户菜单');
    });
  }
  function wireDropdowns(root) {
    ensureUserDropdowns(root);
    $els('[data-dropdown]', root).forEach(el => {
      if (el.dataset.dropdownWired === '1') return;
      el.dataset.dropdownWired = '1';
      el.addEventListener('click', function () {
        try {
          const raw = decodeEntities(this.dataset.dropdown || '');
          const cfg = JSON.parse(raw);
          Dropdown.open(this, cfg.html || cfg.content || '', { placement: cfg.placement || 'bottom-end', width: cfg.width || '200px' });
        } catch (e) { console.warn('Dropdown parse error', e); }
      });
      el.addEventListener('keydown', function (event) {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          this.click();
        }
      });
    });
  }

  /* ===== API data status ===== */
  const ApiStatus = {
    expectations: {
      'index.html': [
        { label: '仪表盘聚合数据', url: '/api/v1/dashboard' }
      ],
      'storage.html': [{ label: '存储路径配置', url: '/api/v1/storage/roots' }],
      'users.html': [{ label: '账户用户列表', url: '/api/v1/account/users' }],
      'teams.html': [{ label: '团队列表', url: '/api/v1/account/teams' }],
      'units.html': [{ label: '单位列表', url: '/api/v1/account/units' }],
      'admins.html': [{ label: '管理员账号列表', url: '/api/v1/account/admins' }],
      'roles.html': [{ label: '角色列表', url: '/api/v1/account/roles' }],
      'job-list.html': [{ label: 'Slurm 作业列表', url: '/api/v1/slurm/jobs' }],
      'vnc-desktop.html': [{ label: 'VNC 桌面作业', url: '/api/v1/job-template-runs' }],
      'nodes.html': [{ label: 'Slurm 节点状态', url: '/api/v1/slurm/nodes' }],
      'partitions.html': [{ label: 'Slurm 分区配置', url: '/api/v1/slurm/partitions' }],
      'qos.html': [{ label: 'Slurm QOS 策略', url: '/api/v1/slurm/qos' }],
      'slurm.html': [{ label: 'Slurm 配置', url: '/api/v1/config/slurm' }],
      'ldap.html': [{ label: 'LDAP 配置', url: '/api/v1/config/ldap' }],
      'notify.html': [{ label: '通知配置', url: '/api/v1/config/notify' }],
      'monitoring.html': [
        { label: '监控概览', url: '/api/v1/overview' },
        { label: '节点状态', url: '/api/v1/slurm/nodes' }
      ],
      'data.html': [{ label: '存储目录', url: '/api/v1/storage/roots' }],
      'inspection.html': [{ label: '巡检服务', url: '/api/health' }],
      'login-logs.html': [{ label: '用户登录日志', url: '/api/v1/logs/auth-events?pageSize=1' }],
      'system-logs.html': [{ label: '平台系统日志', url: '/api/v1/logs/system?source=simplehpc-backend&since=1h&limit=1' }],
      'audit.html': [{ label: '用户操作审计', url: '/api/v1/audit/logs?pageSize=1' }],
      'queue-status.html': [{ label: 'Slurm 队列状态', url: '/api/v1/slurm/queue-status' }]
    },
    staticNotices: {
    },
    currentPage() {
      return location.pathname.split('/').pop() || 'index.html';
    },
    bannerId: '_shpc-api-status',
    ensureBanner() {
      let banner = document.getElementById(this.bannerId);
      if (banner) return banner;
      const main = $el('main.main') || document.body;
      banner = document.createElement('div');
      banner.id = this.bannerId;
      banner.className = '_shpc-api-status';
      main.insertBefore(banner, main.firstChild);
      return banner;
    },
    render(messages, level) {
      if (!messages.length) return;
      const banner = this.ensureBanner();
      banner.className = '_shpc-api-status _shpc-api-status--' + (level || 'warn');
      banner.innerHTML = messages.map(m => '<div>' + m + '</div>').join('');
    },
    async probe() {
      const page = this.currentPage();
      const expectations = this.expectations[page] || [];
      const messages = [];
      if (this.staticNotices[page]) messages.push('<strong>数据提示</strong>：' + this.staticNotices[page]);
      if (!expectations.length && !$el('#' + this.bannerId) && ($els('table').length || $els('.card').length > 1)) {
        messages.push('<strong>数据未获取</strong>：本页未声明后端 API 数据源，页面内容可能仍为前端占位数据。');
      }
      const failures = [];
      await Promise.all(expectations.map(async item => {
        try {
          const res = await apiFetch(item.url, { cache: 'no-store' });
          if (!res.ok) failures.push(item.label + '（' + item.url + '，HTTP ' + res.status + '）');
        } catch (err) {
          failures.push(item.label + '（' + item.url + '，' + err.message + '）');
        }
      }));
      failures.forEach(f => messages.push('<strong>数据未获取</strong>：' + f));
      this.render(messages, failures.length ? 'danger' : 'warn');
    }
  };

  /* ===== Global Init ===== */
  function initSessionUI() {
    function run() {
      if (window.SessionUI) {
        window.SessionUI.init(document, localStorage, window.fetch.bind(window));
      }
    }
    if (window.SessionUI) {
      run();
      return;
    }
    const script = document.createElement('script');
    script.src = 'js/session-ui.js?v=20260630';
    script.onload = run;
    document.head.appendChild(script);
  }

  function initPlatformUI() {
    function run() {
      if (window.PlatformUI) window.PlatformUI.loadAndApply().catch(() => {});
    }
    if (window.PlatformUI) return run();
    const script = document.createElement('script');
    script.src = 'js/platform-ui.js?v=20260703login';
    script.onload = run;
    document.head.appendChild(script);
  }

  function injectLogCenterNav() {
    const nav = document.querySelector('.sidebar .nav');
    if (!nav || nav.querySelector('[data-submenu="logs"]')) return;
    const systemToggle = nav.querySelector('[data-submenu="system"]');
    const toggle = document.createElement('li');
    toggle.className = 'nav-group-toggle';
    toggle.dataset.submenu = 'logs';
    toggle.textContent = '日志中心';
    const submenu = document.createElement('li');
    submenu.className = 'nav-submenu';
    submenu.id = 'submenu-logs';
    const page = location.pathname.split('/').pop() || '';
    const links = [
      ['login-logs.html', '用户登录日志'],
      ['system-logs.html', '系统日志'],
      ['audit.html', '审计日志']
    ];
    submenu.innerHTML = links.map(item => '<a class="nav-submenu-item'+(page===item[0]?' active':'')+'" href="'+item[0]+'">'+item[1]+'</a>').join('');
    if (systemToggle) {
      nav.insertBefore(toggle, systemToggle);
      nav.insertBefore(submenu, systemToggle);
    } else {
      nav.append(toggle, submenu);
    }
    nav.querySelectorAll('#submenu-system a[href="audit.html"]').forEach(link => link.remove());
    if (links.some(item => item[0] === page)) {
      toggle.classList.add('expanded');
      submenu.classList.add('expanded');
    }
  }

  function init() {
    initPlatformUI();
    initSessionUI();
    initRBAC();
    wireNav();
    wireSubnav();
    wireDropdowns(document);
    $els('input[data-search-target]').forEach(wireSearchFilter);
    $els('th[data-sort]').forEach(wireTableSort);
    $els('select[data-filter-target]').forEach(wireSelectFilter);
    $els('[data-toggle-target]').forEach(wireStateToggle);
    $els('[data-expand-target],[data-expand-html],[data-expand-source]').forEach(wireExpandModal);
    $els('table[data-batch="true"]').forEach(wireBatchSelect);
    wireDirtyForms();
    wireTooltips();
    ApiStatus.probe();
  }

  function initRBAC() {
    function run() {
      if (window.App?.authz) window.App.authz.init();
    }
    if (window.App?.authz) return run();
    document.documentElement.classList.add('rbac-pending');
    const script = document.createElement('script');
    script.src = 'js/rbac.js?v=20260704favnav';
    script.onload = run;
    script.onerror = () => {
      document.documentElement.classList.remove('rbac-pending');
      document.documentElement.classList.add('shpc-authz-ready');
      Toast.show('权限客户端加载失败', 'danger');
    };
    document.head.appendChild(script);
  }

  // 登录检查：非登录页面且未登录则跳转
  (function checkAuth() {
    var publicPages = ["/login.html", "/login"];
    var currentPath = location.pathname;
    var isPublic = publicPages.some(function(p) { return currentPath.indexOf(p) !== -1; });
    var token = localStorage.getItem("simplehpc_token");
    if (!isPublic && !token) {
      location.href = "login.html";
      return;
    }
  })();

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();

  window.App = {
    toast: (msg, type) => Toast.show(msg, type),
    loading: { show: (t) => Loading.show(t), hide: () => Loading.hide() },
    confirm: (msg, opts) => Confirm.show(msg, opts),
    modal: (opts) => Modal.open(opts),
    drawer: (opts) => Drawer.open(opts),
    dropdown: (trigger, html, opts) => Dropdown.open(trigger, html, opts),
    authHeaders: authHeaders,
    apiFetch: apiFetch,
    wire: () => { wireDropdowns(document); $els('input[data-search-target]').forEach(wireSearchFilter); },
    openProfileModal: openProfileModal,
    openPasswordDrawer: () => Drawer.open({ title: '修改密码', content: '<div class="_shpc-form-grid--single" style="display:flex;flex-direction:column;gap:16px"><div class="_shpc-field"><label>当前密码</label><input type="password"></div><div class="_shpc-field"><label>新密码</label><input type="password"></div><div class="_shpc-field"><label>确认新密码</label><input type="password"></div></div>', onSubmit: () => Toast.show('密码已修改', 'success') }),
    logout: () => Confirm.show('确认退出登录？', { danger: true, onConfirm: async () => {
      const token = localStorage.getItem("simplehpc_token") || "";
      try {
        await fetch('/api/v1/auth/logout', { method: 'POST', headers: token ? { Authorization: 'Bearer ' + token } : {} });
      } catch (_) {}
      Toast.show('已安全退出', 'info');
      localStorage.removeItem("simplehpc_token");
      localStorage.removeItem("simplehpc_user");
      localStorage.removeItem("simplehpc_auth_type");
      setTimeout(() => location.href = "login.html", 800);
    } })
  };
})();
