(function (root, factory) {
  const api = factory(root);
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) {
    root.App = root.App || {};
    root.App.authz = api;
  }
}(typeof window !== 'undefined' ? window : globalThis, function (root) {
  'use strict';

  let context = null;
  let loading = null;

  function tokenHeaders() {
    const token = root.localStorage?.getItem('simplehpc_token') || '';
    return token ? {Authorization: 'Bearer ' + token} : {};
  }

  function flattenMenus(items) {
    return (items || []).flatMap(item => [item].concat(flattenMenus(item.children)));
  }

  function can(permission) {
    if (!permission || !context) return false;
    const values = new Set(context.permissions || []);
    return values.has('*') || values.has(permission);
  }

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, c => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    })[c]);
  }

  function currentTarget() {
    return (root.location?.pathname || '').split('/').pop() || 'index.html';
  }

  function isActive(path) {
    const target = String(path || '').split('#')[0] || 'index.html';
    return target === currentTarget();
  }

  function menuTone(menu) {
    const key = String([menu.code, menu.path, menu.name].filter(Boolean).join(' ')).toLowerCase();
    if (key.includes('dashboard') || key.includes('index') || key.includes('仪表')) return 'dashboard';
    if (key.includes('account') || key.includes('user') || key.includes('admin') || key.includes('role') || key.includes('team') || key.includes('unit') || key.includes('账户') || key.includes('用户') || key.includes('角色') || key.includes('团队') || key.includes('单位')) return 'account';
    if (key.includes('compute') || key.includes('resource') || key.includes('partition') || key.includes('queue') || key.includes('node') || key.includes('qos') || key.includes('资源') || key.includes('队列') || key.includes('节点')) return 'resource';
    if (key.includes('data') || key.includes('storage') || key.includes('acl') || key.includes('目录') || key.includes('存储') || key.includes('授权')) return 'data';
    if (key.includes('job') || key.includes('template') || key.includes('vnc') || key.includes('作业') || key.includes('桌面') || key.includes('模板')) return 'job';
    if (key.includes('operation') || key.includes('ops') || key.includes('monitor') || key.includes('inspection') || key.includes('运维') || key.includes('监控') || key.includes('巡检')) return 'ops';
    if (key.includes('log') || key.includes('audit') || key.includes('日志') || key.includes('审计')) return 'logs';
    if (key.includes('system') || key.includes('config') || key.includes('setting') || key.includes('ldap') || key.includes('slurm') || key.includes('notify') || key.includes('系统') || key.includes('配置') || key.includes('设置') || key.includes('通知')) return 'system';
    return 'default';
  }

  function navIconSvg(tone) {
    const icons = {
      dashboard: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 13.2 12 6l8 7.2"/><path d="M7 12.4V20h10v-7.6"/><path d="M10 20v-5h4v5"/></svg>',
      account: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="9" cy="8" r="3.2"/><path d="M3.8 19c.8-3.2 2.7-5 5.2-5s4.4 1.8 5.2 5"/><circle cx="17" cy="9" r="2.2"/><path d="M15.6 14.2c2.1.3 3.6 1.8 4.4 4.4"/></svg>',
      resource: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="5" width="16" height="5" rx="2"/><rect x="4" y="14" width="16" height="5" rx="2"/><path d="M8 7.5h.1M8 16.5h.1M13 7.5h4M13 16.5h4"/></svg>',
      data: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7c0-2 3.6-3.5 8-3.5S20 5 20 7s-3.6 3.5-8 3.5S4 9 4 7Z"/><path d="M4 7v5c0 2 3.6 3.5 8 3.5s8-1.5 8-3.5V7"/><path d="M4 12v5c0 2 3.6 3.5 8 3.5s8-1.5 8-3.5v-5"/></svg>',
      job: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="5" width="16" height="14" rx="3"/><path d="M8 9h8M8 13h5"/><path d="m15 14.5 1.8 1.8L21 12"/></svg>',
      ops: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v3M12 18v3M3 12h3M18 12h3"/><circle cx="12" cy="12" r="4"/><path d="m17 7 2-2M5 19l2-2M7 7 5 5M17 17l2 2"/></svg>',
      logs: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 4h9l3 3v13H6z"/><path d="M15 4v4h4"/><path d="M9 11h6M9 15h6M9 18h4"/></svg>',
      system: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M12 3.5v2.2M12 18.3v2.2M5.2 5.2l1.6 1.6M17.2 17.2l1.6 1.6M3.5 12h2.2M18.3 12h2.2M5.2 18.8l1.6-1.6M17.2 6.8l1.6-1.6"/></svg>',
      default: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="5" width="14" height="14" rx="4"/><path d="M9 12h6"/></svg>'
    };
    return icons[tone] || icons.default;
  }

  function appendIconLabel(element, menu, labelClass) {
    const icon = root.document.createElement('span');
    icon.className = labelClass === 'nav-flat-link-label' ? 'nav-flat-link-icon' : 'nav-primary-icon';
    icon.setAttribute('aria-hidden', 'true');
    const tone = menuTone(menu);
    icon.dataset.navTone = tone;
    element.dataset.navTone = tone;
    icon.innerHTML = navIconSvg(tone);
    const label = root.document.createElement('span');
    label.className = labelClass;
    label.textContent = menu.name || menu.code;
    element.append(icon, label);
  }

  function leafLink(menu, flat) {
    const link = root.document.createElement('a');
    link.href = menu.path || '#';
    link.className = (flat ? 'nav-flat-link' : 'nav-submenu-item') + (isActive(menu.path) ? ' active' : '');
    link.dataset.menuCode = menu.code || '';
    if (flat) {
      appendIconLabel(link, menu, 'nav-flat-link-label');
    } else {
      link.textContent = menu.name || menu.code;
    }
    return link;
  }

  function navPrimaryLink(menu) {
    const item = root.document.createElement('li');
    item.className = 'nav-primary-item';
    const link = root.document.createElement('a');
    link.href = menu.path || '#';
    link.className = 'nav-primary-link' + (isActive(menu.path) ? ' active' : '');
    link.dataset.menuCode = menu.code || '';
    appendIconLabel(link, menu, 'nav-primary-label');
    item.appendChild(link);
    return item;
  }

  function renderNavigation() {
    const nav = root.document?.querySelector('.sidebar .nav');
    if (!nav || !context) return;
    nav.innerHTML = '';
    nav.classList.toggle('nav-flat', !!context.flatMenu);
    nav.classList.toggle('nav-dynamic', true);
    (context.menus || []).forEach(menu => {
      if (context.flatMenu) {
        const item = root.document.createElement('li');
        item.className = 'nav-flat-item';
        item.appendChild(leafLink(menu, true));
        nav.appendChild(item);
        return;
      }
      if (!menu.children?.length) {
        nav.appendChild(navPrimaryLink(menu));
        return;
      }
      const toggle = root.document.createElement('li');
      const active = menu.children.some(child => isActive(child.path));
      toggle.className = 'nav-group-toggle' + (active ? ' expanded' : '');
      toggle.dataset.submenu = menu.code;
      appendIconLabel(toggle, menu, 'nav-primary-label');
      const submenu = root.document.createElement('li');
      submenu.className = 'nav-submenu' + (active ? ' expanded' : '');
      submenu.id = 'submenu-' + menu.code;
      menu.children.forEach(child => submenu.appendChild(leafLink(child, false)));
      toggle.addEventListener('click', () => {
        toggle.classList.toggle('expanded');
        submenu.classList.toggle('expanded');
      });
      nav.append(toggle, submenu);
    });
  }

  function applyButtonPermissions(scope) {
    (scope || root.document).querySelectorAll?.('[data-permission]').forEach(element => {
      const allowed = String(element.dataset.permission || '').split(/\s+/).filter(Boolean).some(can);
      if (allowed) {
        element.hidden = false;
        element.disabled = false;
        element.removeAttribute('aria-disabled');
        return;
      }
      if (element.dataset.permissionMode === 'disable') {
        element.disabled = true;
        element.setAttribute('aria-disabled', 'true');
        element.title = element.title || '当前角色无此操作权限';
      } else {
        element.hidden = true;
      }
    });
  }

  function renderForbidden(firstPath) {
    const main = root.document?.querySelector('main.main') || root.document?.querySelector('main');
    if (!main) return;
    main.innerHTML = '<section class="rbac-forbidden" role="alert">' +
      '<div class="rbac-forbidden-code">403</div><h2>当前角色无权访问此页面</h2>' +
      '<p>页面路由已由 RBAC 权限守卫拦截。若工作职责已变更，请联系管理员调整角色。</p>' +
      (firstPath ? '<a class="btn btn-primary" href="' + escapeHTML(firstPath) + '">前往可访问页面</a>' : '') +
      '</section>';
    root.document.dispatchEvent(new CustomEvent('simplehpc:forbidden', {detail: {path: currentTarget()}}));
  }

  function guardRoute() {
    if (!context || can('*')) return true;
    const target = currentTarget();
    if (['login.html', 'inspection-log.html'].includes(target)) return true;
    const pages = flattenMenus(context.menus).filter(item => item.path);
    const allowed = pages.some(item => String(item.path).split('#')[0] === target &&
      (!item.routePermission || can(item.routePermission)));
    if (!allowed) renderForbidden(pages[0]?.path || 'index.html');
    return allowed;
  }

  async function load(force) {
    if (context && !force) return context;
    if (loading && !force) return loading;
    loading = root.fetch('/api/v1/auth/me', {
      credentials: 'same-origin', cache: 'no-store', headers: tokenHeaders()
    }).then(async response => {
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || '权限上下文获取失败');
      context = data;
      renderNavigation();
      guardRoute();
      applyButtonPermissions(root.document);
      root.document.dispatchEvent(new CustomEvent('simplehpc:authz-ready', {detail: context}));
      return context;
    }).finally(() => {
      loading = null;
      root.document?.documentElement?.classList.remove('rbac-pending');
      root.document?.documentElement?.classList.add('shpc-authz-ready');
    });
    return loading;
  }

  function init() {
    if (!root.document || ['login.html', 'login'].includes(currentTarget())) return Promise.resolve(null);
    return load(false).catch(error => {
      root.App?.toast?.('权限信息加载失败：' + error.message, 'danger');
      return null;
    });
  }

  return {
    init, load, refresh: () => load(true), can, context: () => context,
    flattenMenus, renderNavigation, applyButtonPermissions, guardRoute
  };
}));
