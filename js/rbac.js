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

  function groupLabel(menu) {
    const code = String(menu.code || '').toLowerCase();
    const name = menu.name || menu.code || '';
    const labels = {
      overview: '仪表盘',
      compute: '资源管理',
      resource: '资源管理',
      jobs: '作业管理',
      job: '作业管理',
      ops: '运维管理',
      operation: '运维管理',
      logs: '日志管理',
      log: '日志管理',
      system: '系统配置',
      config: '系统配置'
    };
    return labels[code] || name
      .replace('计算管理', '资源管理')
      .replace('作业中心', '作业管理')
      .replace('监控运维', '运维管理')
      .replace('日志中心', '日志管理')
      .replace(/^系统$/, '系统配置');
  }

  function menuTone(menu) {
    const key = String([menu.code, menu.path, menu.name].filter(Boolean).join(' ')).toLowerCase();
    if (key.includes('dashboard') || key.includes('index') || key.includes('仪表')) return 'dashboard';
    if (key.includes('account') || key.includes('user') || key.includes('admin') || key.includes('role') || key.includes('team') || key.includes('unit') || key.includes('账户') || key.includes('用户') || key.includes('角色') || key.includes('团队') || key.includes('单位')) return 'account';
    if (key.includes('compute') || key.includes('resource') || key.includes('partition') || key.includes('queue') || key.includes('node') || key.includes('qos') || key.includes('资源') || key.includes('队列') || key.includes('节点')) return 'resource';
    if (key.includes('data') || key.includes('storage') || key.includes('acl') || key.includes('目录') || key.includes('存储') || key.includes('授权')) return 'data';
    if (key.includes('job') || key.includes('template') || key.includes('vnc') || key.includes('作业') || key.includes('桌面') || key.includes('模板')) return 'job';
    if (key.includes('terminal') || key.includes('webssh') || key.includes('终端') || key.includes('ssh')) return 'terminal';
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
      terminal: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="5" width="16" height="14" rx="3"/><path d="m8 10 3 2-3 2"/><path d="M13 15h4"/></svg>',
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
    link.className = (flat ? 'nav-flat-link topnav-link' : 'nav-submenu-item topnav-dropdown-link') + (isActive(menu.path) ? ' active' : '');
    link.dataset.menuCode = menu.code || '';
    if (flat) {
      appendIconLabel(link, menu, 'nav-flat-link-label');
    } else {
      link.textContent = menu.name || menu.code;
    }
    return link;
  }

  function menuKey(menu) {
    if (menu?.custom && menu.code) return String(menu.code).trim();
    return String(menu.path || menu.code || menu.name || '').trim();
  }

  function menuText(menu) {
    return String([menu.name, menu.code, menu.path, menu.description].filter(Boolean).join(' ')).toLowerCase();
  }

  function menuDescription(menu) {
    const text = String([menu.code, menu.path, menu.name].filter(Boolean).join(' ')).toLowerCase();
    if (menu.description) return menu.description;
    if (text.includes('dashboard') || text.includes('index') || text.includes('仪表')) return '查看集群关键指标与运营概览';
    if (text.includes('queue-status') || text.includes('队列状态')) return '查看 Slurm 队列和资源池状态';
    if (text.includes('data') || text.includes('目录')) return '管理个人和授权数据目录';
    if (text.includes('template') || text.includes('模板')) return '使用和维护作业提交模板';
    if (text.includes('job-list') || text.includes('作业列表')) return '查看和管理可见范围内作业';
    if (text.includes('vnc')) return '申请和访问图形桌面作业';
    if (text.includes('terminal') || text.includes('webssh') || text.includes('终端')) return '通过浏览器访问授权登录节点';
    if (text.includes('role') || text.includes('角色')) return '查看和配置 RBAC 角色权限';
    if (text.includes('user') || text.includes('用户')) return '管理用户账号和用户资料';
    if (text.includes('team') || text.includes('团队')) return '管理团队和组长用户';
    if (text.includes('unit') || text.includes('单位')) return '管理组织单位信息';
    if (text.includes('node') || text.includes('节点')) return '查看计算节点运行状态';
    if (text.includes('qos')) return '查看和配置调度 QOS 策略';
    if (text.includes('log') || text.includes('日志')) return '查看平台日志与审计记录';
    if (text.includes('setting') || text.includes('config') || text.includes('配置')) return '维护平台基础配置';
    return '';
  }

  function menuCategory(menu, parent) {
    const key = String([menu.code, menu.path, menu.name, parent?.code, parent?.name].filter(Boolean).join(' ')).toLowerCase();
    if (key.includes('setting') || key.includes('config') || key.includes('ldap') || key.includes('slurm') || key.includes('notify') || key.includes('system') || key.includes('role') || key.includes('permission') || key.includes('matrix') || key.includes('平台设置') || key.includes('系统') || key.includes('配置') || key.includes('角色') || key.includes('权限')) return 'system';
    if (key.includes('partition') || key.includes('queue') || key.includes('node') || key.includes('qos') || key.includes('resource') || key.includes('data-acl') || key.includes('storage') || key.includes('资源') || key.includes('队列') || key.includes('节点') || key.includes('存储') || key.includes('访问授权')) return 'resource';
    if (key.includes('unit') || key.includes('team') || key.includes('user') || key.includes('admin') || key.includes('单位') || key.includes('团队') || key.includes('用户') || key.includes('管理员账号')) return 'user';
    if (key.includes('job') || key.includes('template') || key.includes('vnc') || key.includes('terminal') || key.includes('webssh') || key.includes('作业') || key.includes('模板') || key.includes('桌面') || key.includes('终端')) return 'job';
    if (key.includes('monitor') || key.includes('inspection') || key.includes('log') || key.includes('audit') || key.includes('ops') || key.includes('监控') || key.includes('巡检') || key.includes('日志') || key.includes('审计')) return 'ops';
    return 'operation';
  }

  const MENU_CATEGORY_LABELS = {
    system: '系统管理',
    resource: '资源管理',
    user: '用户管理',
    job: '作业管理',
    ops: '运维管理',
    operation: '运营管理'
  };

  const MENU_CATEGORY_ORDER = ['system', 'resource', 'user', 'job', 'ops', 'operation'];
  const MENU_CATEGORY_SET = new Set(MENU_CATEGORY_ORDER);
  const MENU_OVERVIEW_STRUCTURE = [
    {category: 'system', groups: [
      {name: '系统配置', tests: ['平台设置', 'platform', 'settings.html', 'ldap', 'slurm.html', 'storage', 'notify', '通知配置']},
      {name: '权限管理', tests: ['角色管理', 'roles', 'rbac', 'permission', 'matrix', '权限矩阵']},
      {name: '管理员管理', tests: ['管理员账号管理', '管理员账号', 'admin']}
    ]},
    {category: 'resource', groups: [
      {name: '资源状态', tests: ['队列状态', 'queue-status', '节点状态', 'nodes.html']},
      {name: '资源策略', tests: ['资源队列配置', 'partitions', 'partition', 'qos', 'QOS 策略', 'QOS策略']},
      {name: '数据授权', tests: ['访问授权', 'data-acl', 'acl']}
    ]},
    {category: 'user', groups: [
      {name: '组织管理', tests: ['单位管理', 'units', '团队管理', 'teams']},
      {name: '用户账号', tests: ['用户管理', 'users.html', '用户登录日志', 'login-logs']}
    ]},
    {category: 'job', groups: [
      {name: '作业提交', tests: ['作业模板', 'job-template', 'template']},
      {name: '作业运行', tests: ['作业列表', 'job-list', 'jobs.html', 'VNC 桌面', 'vnc', '终端中心', 'terminal', 'webssh']}
    ]},
    {category: 'ops', groups: [
      {name: '监控巡检', tests: ['监控告警', 'monitor', '巡检报告', 'inspection']},
      {name: '日志审计', tests: ['系统日志', 'system-logs', '审计日志', 'audit', '日志中心', 'logs']}
    ]},
    {category: 'operation', groups: [
      {name: '运营概览', tests: ['仪表盘', 'dashboard', 'index.html']},
      {name: '数据目录', tests: ['数据目录', 'data.html', 'directory']}
    ]}
  ];
  const MENU_OVERVIEW_GROUP_DEFAULT = {
    system: '系统配置',
    resource: '资源状态',
    user: '组织管理',
    job: '作业运行',
    ops: '监控巡检',
    operation: '运营概览'
  };

  function overviewMatchText(menu) {
    return String([
      menu.name,
      menu.code,
      menu.path,
      menu.description,
      menu.parentName,
      menu.parentCode
    ].filter(Boolean).join(' ')).toLowerCase();
  }

  function overviewMatches(menu, tests) {
    const text = overviewMatchText(menu);
    return (tests || []).some(test => text.includes(String(test || '').toLowerCase()));
  }

  function overviewStructureCategory(category) {
    return MENU_OVERVIEW_STRUCTURE.find(item => item.category === category);
  }

  function overviewGroupNames(category) {
    const node = overviewStructureCategory(category);
    return (node?.groups || []).map(group => group.name);
  }

  function mappedOverviewCategory(menu) {
    for (const section of MENU_OVERVIEW_STRUCTURE) {
      for (const group of section.groups || []) {
        if (overviewMatches(menu, group.tests)) return section.category;
      }
    }
    return MENU_CATEGORY_SET.has(menu.category) ? menu.category : menuCategory(menu);
  }

  function mappedOverviewGroup(menu, category) {
    const section = overviewStructureCategory(category);
    const matched = (section?.groups || []).find(group => overviewMatches(menu, group.tests));
    if (matched) return matched.name;
    if (menu.parentName) return menu.parentName;
    return MENU_OVERVIEW_GROUP_DEFAULT[category] || '其他';
  }

  function flattenRenderableMenus(items, parent) {
    const rows = [];
    (items || []).forEach(item => {
      if (!item) return;
      if (item.path) {
        rows.push(Object.assign({}, item, {
          parentCode: parent?.code || '',
          parentName: parent ? groupLabel(parent) : '',
          category: menuCategory(item, parent)
        }));
      }
      rows.push(...flattenRenderableMenus(item.children || [], item));
    });
    const seen = new Set();
    return rows.filter(item => {
      const key = menuKey(item);
      if (!key || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  function overviewLayoutKey() {
    return storageKey().replace(':navigation:favorites:', ':navigation:overview-layout:');
  }

  function readOverviewLayout() {
    try {
      const raw = root.localStorage?.getItem(overviewLayoutKey());
      const parsed = raw ? JSON.parse(raw) : null;
      return parsed && typeof parsed === 'object' && parsed.items && typeof parsed.items === 'object'
        ? parsed
        : {version: 1, items: {}};
    } catch (_) {
      return {version: 1, items: {}};
    }
  }

  function writeOverviewLayout(layout) {
    try {
      root.localStorage?.setItem(overviewLayoutKey(), JSON.stringify(layout));
      return true;
    } catch (_) {
      return false;
    }
  }

  function cleanOverviewLayout(layout) {
    const next = layout && typeof layout === 'object' ? layout : {};
    next.version = 2;
    next.items = next.items && typeof next.items === 'object' ? next.items : {};
    next.groups = next.groups && typeof next.groups === 'object' ? next.groups : {};
    next.customMenus = Array.isArray(next.customMenus) ? next.customMenus : [];
    return next;
  }

  function overviewGroupId(category, name) {
    return String(category || 'operation') + '::' + String(name || '未分组');
  }

  function overviewGroupEntry(layout, category, name) {
    const id = overviewGroupId(category, name);
    const entry = layout?.groups?.[id];
    if (entry && typeof entry === 'object') {
      return Object.assign({id, category, name}, entry);
    }
    return {id, category, name, order: 100000};
  }

  function overviewCustomMenus(layout) {
    return (layout?.customMenus || []).map((item, index) => {
      const category = MENU_CATEGORY_SET.has(item.category) ? item.category : 'operation';
      const group = String(item.group || MENU_OVERVIEW_GROUP_DEFAULT[category] || '其他');
      return {
        custom: true,
        code: item.id || ('custom-menu-' + index),
        name: item.name || '未命名菜单',
        path: item.path || '#',
        description: item.description || '自定义菜单入口',
        parentName: group,
        category,
        customGroup: group
      };
    });
  }

  function overviewMenusWithCustom(menus, layout) {
    return (menus || []).concat(overviewCustomMenus(cleanOverviewLayout(layout)));
  }

  function saveOverviewItemName(key, name) {
    const title = String(name || '').trim();
    if (!title) return false;
    const layout = cleanOverviewLayout(readOverviewLayout());
    const custom = layout.customMenus.find(item => item.id === key);
    if (custom) {
      custom.name = title;
    } else {
      layout.items[key] = Object.assign({}, layout.items[key] || {}, {name: title});
    }
    return writeOverviewLayout(layout);
  }

  function saveOverviewGroupName(category, oldName, nextName) {
    const title = String(nextName || '').trim();
    if (!title) return false;
    const layout = cleanOverviewLayout(readOverviewLayout());
    const oldId = overviewGroupId(category, oldName);
    const entry = overviewGroupEntry(layout, category, oldName);
    entry.name = title;
    layout.groups[oldId] = entry;
    Object.keys(layout.items).forEach(key => {
      const item = layout.items[key] || {};
      if ((item.group || item.groupName) === oldName || item.groupId === oldId) {
        layout.items[key] = Object.assign({}, item, {group: title, groupId: oldId});
      }
    });
    layout.customMenus.forEach(item => {
      if (item.category === category && item.group === oldName) item.group = title;
    });
    return writeOverviewLayout(layout);
  }

  function addOverviewGroup(category) {
    const name = root.prompt?.('请输入一级菜单名称');
    const title = String(name || '').trim();
    if (!title || !MENU_CATEGORY_SET.has(category)) return false;
    const layout = cleanOverviewLayout(readOverviewLayout());
    const id = overviewGroupId(category, title);
    layout.groups[id] = {
      id,
      category,
      name: title,
      order: Date.now()
    };
    return writeOverviewLayout(layout);
  }

  function addOverviewMenu(category, groupName) {
    if (!MENU_CATEGORY_SET.has(category)) return false;
    const name = root.prompt?.('请输入二级菜单名称');
    const title = String(name || '').trim();
    if (!title) return false;
    const pathInput = root.prompt?.('请输入菜单链接，例如 terminal.html；留空则作为占位入口');
    const path = String(pathInput || '').trim() || '#';
    const layout = cleanOverviewLayout(readOverviewLayout());
    const id = 'custom-menu-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 7);
    layout.customMenus.push({
      id,
      name: title,
      path,
      category,
      group: groupName || MENU_OVERVIEW_GROUP_DEFAULT[category] || '其他',
      order: Date.now()
    });
    return writeOverviewLayout(layout);
  }

  function overviewRowsFor(menus, layout) {
    layout = cleanOverviewLayout(layout);
    const items = layout.items || {};
    return (menus || []).map((menu, index) => {
      const key = menuKey(menu);
      const entry = items[key] || {};
      const mappedCategory = mappedOverviewCategory(menu);
      const category = MENU_CATEGORY_SET.has(entry.category) ? entry.category : mappedCategory;
      const defaultGroup = menu.customGroup || mappedOverviewGroup(menu, category);
      const groupId = entry.groupId || overviewGroupId(category, String(entry.group || defaultGroup));
      const groupEntry = layout.groups[groupId];
      const group = String(entry.group || groupEntry?.name || defaultGroup).trim();
      const order = Number.isFinite(Number(entry.order)) ? Number(entry.order) : 100000 + index;
      const displayName = String(entry.name || '').trim() || menu.name || menu.code || '';
      return Object.assign({}, menu, {name: displayName, category, overviewGroup: group, overviewGroupId: groupId, overviewOrder: order, overviewIndex: index});
    });
  }

  function sortOverviewRows(rows) {
    return rows.slice().sort((a, b) =>
      (a.overviewOrder - b.overviewOrder) ||
      (a.overviewIndex - b.overviewIndex) ||
      String(a.name || a.code || '').localeCompare(String(b.name || b.code || ''), 'zh-Hans-CN')
    );
  }

  function overviewDisplayName(menu) {
    return menu.name || menu.code || '';
  }

  function moveOverviewItem(menus, key, toCategory, beforeKey, toGroup) {
    if (!MENU_CATEGORY_SET.has(toCategory)) return false;
    const layout = cleanOverviewLayout(readOverviewLayout());
    const rows = overviewRowsFor(menus, layout);
    const moved = rows.find(menu => menuKey(menu) === key);
    if (!moved) return false;
    moved.category = toCategory;
    moved.overviewGroup = String(toGroup || '').trim() || mappedOverviewGroup(moved, toCategory);
    moved.overviewGroupId = overviewGroupId(toCategory, moved.overviewGroup);
    const groups = {};
    MENU_CATEGORY_ORDER.forEach(category => { groups[category] = []; });
    rows.filter(menu => menuKey(menu) !== key).forEach(menu => {
      const category = MENU_CATEGORY_SET.has(menu.category) ? menu.category : menuCategory(menu);
      groups[category].push(menu);
    });
    MENU_CATEGORY_ORDER.forEach(category => { groups[category] = sortOverviewRows(groups[category]); });
    const targetRows = groups[toCategory];
    const beforeIndex = beforeKey ? targetRows.findIndex(menu => menuKey(menu) === beforeKey) : -1;
    if (beforeIndex >= 0) targetRows.splice(beforeIndex, 0, moved);
    else targetRows.push(moved);
    const items = {};
    MENU_CATEGORY_ORDER.forEach(category => {
      groups[category].forEach((menu, order) => {
        const oldEntry = layout.items[menuKey(menu)] || {};
        items[menuKey(menu)] = Object.assign({}, oldEntry, {
          category,
          order,
          group: menu.overviewGroup || mappedOverviewGroup(menu, category),
          groupId: menu.overviewGroupId || overviewGroupId(category, menu.overviewGroup || mappedOverviewGroup(menu, category))
        });
      });
    });
    layout.items = items;
    return writeOverviewLayout(layout);
  }

  function moveOverviewGroup(menus, groupId, fromCategory, toCategory, beforeGroupId) {
    if (!MENU_CATEGORY_SET.has(toCategory)) return false;
    const layout = cleanOverviewLayout(readOverviewLayout());
    const rows = overviewRowsFor(menus, layout);
    const movedRows = rows.filter(menu => (menu.overviewGroupId || overviewGroupId(menu.category, menu.overviewGroup)) === groupId);
    if (!movedRows.length && !layout.groups[groupId]) return false;
    const groupName = layout.groups[groupId]?.name || movedRows[0]?.overviewGroup || '未分组';
    layout.groups[groupId] = Object.assign({}, layout.groups[groupId] || {}, {
      id: groupId,
      category: toCategory,
      name: groupName
    });
    const groupOrder = {};
    MENU_CATEGORY_ORDER.forEach(category => {
      const names = [];
      overviewGroupedSections(rows, '').filter(section => section.category === category).forEach(section => {
        section.groups.forEach(group => names.push(group.id || overviewGroupId(category, group.name)));
      });
      if (!names.includes(groupId) && category === fromCategory) names.push(groupId);
      groupOrder[category] = names.filter(id => id !== groupId);
    });
    const target = groupOrder[toCategory] || [];
    const beforeIndex = beforeGroupId ? target.indexOf(beforeGroupId) : -1;
    if (beforeIndex >= 0) target.splice(beforeIndex, 0, groupId);
    else target.push(groupId);
    MENU_CATEGORY_ORDER.forEach(category => {
      (groupOrder[category] || []).forEach((id, index) => {
        const entry = layout.groups[id] || {id, name: id.split('::').slice(1).join('::') || '未分组'};
        layout.groups[id] = Object.assign({}, entry, {id, category, order: index});
      });
    });
    rows.forEach(menu => {
      const key = menuKey(menu);
      const rowGroupId = menu.overviewGroupId || overviewGroupId(menu.category, menu.overviewGroup);
      if (rowGroupId === groupId) {
        const itemEntry = layout.items[key] || {};
        layout.items[key] = Object.assign({}, itemEntry, {
          category: toCategory,
          group: groupName,
          groupId
        });
      }
    });
    layout.customMenus.forEach(item => {
      const itemGroupId = overviewGroupId(item.category, item.group);
      if (itemGroupId === groupId || item.group === groupName) {
        item.category = toCategory;
        item.group = groupName;
      }
    });
    return writeOverviewLayout(layout);
  }

  function storageKey() {
    const user = context?.user?.username || context?.user?.account || context?.username || context?.account || 'anonymous';
    return 'simplehpc:navigation:favorites:' + user;
  }

  function readFavoriteKeys() {
    try {
      const raw = root.localStorage?.getItem(storageKey());
      const parsed = raw ? JSON.parse(raw) : null;
      return Array.isArray(parsed) ? parsed.filter(Boolean).map(String) : null;
    } catch (_) {
      return null;
    }
  }

  function writeFavoriteKeys(keys) {
    try {
      root.localStorage?.setItem(storageKey(), JSON.stringify(Array.from(new Set(keys.filter(Boolean)))));
      return true;
    } catch (_) {
      return false;
    }
  }

  function groupFavoriteStorageKey() {
    return storageKey().replace(':navigation:favorites:', ':navigation:group-favorites:');
  }

  function readGroupFavoriteKeys() {
    try {
      const raw = root.localStorage?.getItem(groupFavoriteStorageKey());
      const parsed = raw ? JSON.parse(raw) : null;
      return Array.isArray(parsed) ? parsed.filter(Boolean).map(String) : null;
    } catch (_) {
      return null;
    }
  }

  function writeGroupFavoriteKeys(keys) {
    try {
      root.localStorage?.setItem(groupFavoriteStorageKey(), JSON.stringify(Array.from(new Set(keys.filter(Boolean)))));
      return true;
    } catch (_) {
      return false;
    }
  }

  function defaultFavoriteKeys(menus) {
    const names = context?.flatMenu
      ? ['仪表盘', '队列状态', '数据目录', '作业列表', '终端中心', 'WebSSH']
      : ['仪表盘', '队列状态', '数据目录', '作业列表', '角色管理', '用户管理'];
    const selected = [];
    names.forEach(name => {
      const matched = menus.find(menu => String(menu.name || '').includes(name) || String(menu.code || '').toLowerCase().includes(name.toLowerCase()) || String(menu.path || '').toLowerCase().includes(name.toLowerCase()));
      if (matched) selected.push(menuKey(matched));
    });
    if (!selected.length && menus[0]) selected.push(menuKey(menus[0]));
    return selected;
  }

  function favoriteKeysFor(menus) {
    const allowed = new Set(menus.map(menuKey));
    const saved = readFavoriteKeys();
    const keys = (saved && saved.length ? saved : defaultFavoriteKeys(menus)).filter(key => allowed.has(key));
    return keys.length ? keys : defaultFavoriteKeys(menus).filter(key => allowed.has(key));
  }

  function groupFavoriteKeysFor(sections, menus) {
    const groups = [];
    (sections || []).forEach(section => (section.groups || []).forEach(group => {
      if (group.items?.length) groups.push(group);
    }));
    const allowed = new Set(groups.map(group => group.id));
    const saved = readGroupFavoriteKeys();
    const savedKeys = (saved || []).filter(key => allowed.has(key));
    if (savedKeys.length) return savedKeys;
    const leafFavs = new Set(favoriteKeysFor(menus || []));
    const migrated = [];
    groups.forEach(group => {
      if ((group.items || []).some(menu => leafFavs.has(menuKey(menu)))) migrated.push(group.id);
    });
    if (migrated.length) {
      writeGroupFavoriteKeys(migrated);
      return migrated;
    }
    const defaults = ['运营概览', '资源状态', '数据管理', '作业运行', '监控巡检', '系统配置'];
    const selected = [];
    defaults.forEach(name => {
      const matched = groups.find(group => String(group.name || '').includes(name));
      if (matched && !selected.includes(matched.id)) selected.push(matched.id);
    });
    return selected.length ? selected : groups.slice(0, 6).map(group => group.id);
  }

  function favoriteLimit() {
    const width = root.innerWidth || 1440;
    if (width >= 1920) return 8;
    if (width >= 1600) return 7;
    if (width >= 1366) return 6;
    if (width >= 1180) return 5;
    if (width >= 1024) return 4;
    return 0;
  }

  function ensureTopbar() {
    const doc = root.document;
    const layout = doc?.querySelector('.layout');
    const header = doc?.querySelector('.header');
    if (!doc || !layout || !header) return null;
    doc.documentElement.classList.add('shpc-topnav-enabled');
    layout.classList.add('layout--topnav');
    header.classList.add('app-topbar');

    let brand = header.querySelector('.topbar-brand');
    if (!brand) {
      brand = doc.createElement('div');
      brand.className = 'topbar-brand';
      brand.innerHTML = '<button class="topbar-brand-logo" type="button" aria-label="打开菜单总览"><span>S</span></button><a class="topbar-brand-name" href="index.html">Simple<span>HPC</span></a>';
      header.insertBefore(brand, header.firstChild);
    }
    const logoButton = brand.querySelector('.topbar-brand-logo');
    if (logoButton) logoButton.onclick = event => {
      event.preventDefault();
      openMenuOverview();
    };

    let mobileButton = header.querySelector('.topbar-menu-button');
    if (!mobileButton) {
      mobileButton = doc.createElement('button');
      mobileButton.type = 'button';
      mobileButton.className = 'topbar-menu-button';
      mobileButton.setAttribute('aria-label', '打开常用菜单');
      mobileButton.innerHTML = '<span></span><span></span><span></span>';
      header.insertBefore(mobileButton, brand.nextSibling);
    }

    let navWrap = header.querySelector('.topnav');
    if (!navWrap) {
      navWrap = doc.createElement('nav');
      navWrap.className = 'topnav favorite-nav';
      navWrap.setAttribute('aria-label', '常用菜单');
      navWrap.innerHTML = '<ul class="topnav-list favorite-nav-list"></ul>';
      header.insertBefore(navWrap, mobileButton.nextSibling);
    }

    mobileButton.onclick = () => {
      const open = !navWrap.classList.contains('topnav--open');
      navWrap.classList.toggle('topnav--open', open);
      mobileButton.classList.toggle('topbar-menu-button--open', open);
      mobileButton.setAttribute('aria-expanded', String(open));
    };

    if (!header.dataset.topnavDocumentHandler) {
      header.dataset.topnavDocumentHandler = '1';
      doc.addEventListener('click', event => {
        if (!navWrap.contains(event.target) && !mobileButton.contains(event.target)) {
          navWrap.classList.remove('topnav--open');
          mobileButton.classList.remove('topbar-menu-button--open');
          mobileButton.setAttribute('aria-expanded', 'false');
          doc.querySelectorAll('.topnav-item--open').forEach(item => item.classList.remove('topnav-item--open'));
        }
      }, {capture: true});
    }

    return navWrap.querySelector('.topnav-list');
  }

  function closeSiblingDropdowns(item) {
    item.parentElement?.querySelectorAll('.topnav-item--open').forEach(openItem => {
      if (openItem !== item) openItem.classList.remove('topnav-item--open');
    });
  }

  function topnavFavorite(menu) {
    const item = root.document.createElement('li');
    item.className = 'topnav-item topnav-item--leaf topnav-item--favorite';
    const link = root.document.createElement('a');
    link.href = menu.path || '#';
    link.className = 'favorite-nav-item' + (isActive(menu.path) ? ' active' : '');
    link.dataset.menuCode = menu.code || '';
    link.textContent = menu.name || menu.code;
    item.appendChild(link);
    return item;
  }

  function topLevelMenus(items) {
    return (items || []).filter(item => item && (item.path || (item.children || []).length));
  }

  function menuTreeActive(menu) {
    if (!menu) return false;
    if (isActive(menu.path)) return true;
    return (menu.children || []).some(menuTreeActive);
  }

  function topnavPrimaryLeaf(menu) {
    const item = root.document.createElement('li');
    item.className = 'topnav-item topnav-item--leaf topnav-primary-item';
    const link = root.document.createElement('a');
    link.href = menu.path || '#';
    link.className = 'topnav-link' + (menuTreeActive(menu) ? ' active' : '');
    link.dataset.menuCode = menu.code || '';
    link.textContent = menu.name || menu.code || '';
    item.appendChild(link);
    return item;
  }

  function topnavOverviewGroup(group) {
    const children = (group.items || []).filter(item => item.path);
    const item = root.document.createElement('li');
    item.className = 'topnav-item topnav-item--group topnav-primary-item' + (children.some(child => isActive(child.path)) ? ' active' : '');
    const button = root.document.createElement('button');
    button.type = 'button';
    button.className = 'topnav-group-button' + (children.some(child => isActive(child.path)) ? ' active' : '');
    button.dataset.groupId = group.id || '';
    button.textContent = group.name || '未分组';
    const caret = root.document.createElement('span');
    caret.className = 'topnav-caret';
    caret.setAttribute('aria-hidden', 'true');
    button.appendChild(caret);
    const panel = root.document.createElement('div');
    panel.className = 'topnav-dropdown';
    children.forEach(child => {
      const row = root.document.createElement('a');
      row.href = child.path || '#';
      row.className = 'topnav-dropdown-link' + (isActive(child.path) ? ' active' : '');
      row.title = [group.name, child.name || child.code].filter(Boolean).join(' / ');
      row.textContent = child.name || child.code || '';
      panel.appendChild(row);
    });
    button.addEventListener('click', event => {
      event.stopPropagation();
      closeSiblingDropdowns(item);
      item.classList.toggle('topnav-item--open');
    });
    item.append(button, panel);
    return item;
  }

  function topnavPrimaryMenu(menu) {
    const children = flattenMenus(menu.children || []).filter(item => item.path);
    if (!children.length) return topnavPrimaryLeaf(menu);
    return topnavOverviewGroup({
      id: menu.code || menu.name || menu.path,
      name: groupLabel(menu),
      items: children
    });
  }

  function topnavMore(items) {
    if (!items.length) return null;
    const item = root.document.createElement('li');
    item.className = 'topnav-item topnav-item--group topnav-item--more';
    const button = root.document.createElement('button');
    button.type = 'button';
    button.className = 'favorite-more';
    button.innerHTML = '<span>更多</span><span class="topnav-caret" aria-hidden="true"></span>';
    const panel = root.document.createElement('div');
    panel.className = 'topnav-dropdown topnav-dropdown--more';
    items.forEach(menu => {
      const row = root.document.createElement('a');
      row.href = menu.path || '#';
      row.className = 'topnav-dropdown-link' + (isActive(menu.path) ? ' active' : '');
      row.innerHTML = '<span>' + escapeHTML(menu.name || menu.code) + '</span><span class="topnav-dropdown-star" aria-hidden="true">★</span>';
      panel.appendChild(row);
    });
    button.addEventListener('click', event => {
      event.stopPropagation();
      closeSiblingDropdowns(item);
      item.classList.toggle('topnav-item--open');
    });
    item.append(button, panel);
    return item;
  }

  function overviewAllMenus() {
    const layout = readOverviewLayout();
    return overviewMenusWithCustom(flattenRenderableMenus(context?.menus || []), layout);
  }

  function refreshMenuOverview() {
    const overlay = root.document?.querySelector('.menu-overview-overlay');
    const body = overlay?.querySelector('.menu-overview-body');
    const input = overlay?.querySelector('input');
    if (!body) return;
    const menus = overviewAllMenus();
    renderMenuOverviewBody(body, menus, input?.value || '');
  }

  function menuOverviewItem(menu) {
    const button = root.document.createElement('button');
    button.type = 'button';
    button.className = 'menu-overview-item menu-overview-leaf' + (isActive(menu.path) ? ' active' : '');
    button.dataset.menuKey = menuKey(menu);
    button.dataset.overviewGroup = menu.overviewGroup || '';
    button.draggable = true;
    const description = menuDescription(menu);
    button.title = ['一级：' + (menu.overviewGroup || menu.parentName || '未分组'), '二级：' + (menu.name || menu.code || ''), description].filter(Boolean).join('\n');
    button.innerHTML = '<span class="menu-overview-icon" data-nav-tone="' + escapeHTML(menuTone(menu)) + '">' + navIconSvg(menuTone(menu)) + '</span>' +
      '<span class="menu-overview-copy"><strong>' + escapeHTML(overviewDisplayName(menu)) + '</strong></span>' +
      '<span class="menu-overview-edit" role="button" aria-label="编辑菜单名称">✎</span>';
    button.addEventListener('dragstart', event => {
      button.classList.add('menu-overview-item--dragging');
      event.dataTransfer?.setData('text/plain', menuKey(menu));
      event.dataTransfer?.setData('application/x-simplehpc-menu-key', menuKey(menu));
      if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
    });
    button.addEventListener('dragend', () => {
      button.classList.remove('menu-overview-item--dragging');
      root.document?.querySelectorAll('.menu-overview-category--drop').forEach(node => node.classList.remove('menu-overview-category--drop'));
    });
    button.addEventListener('click', event => {
      if (event.target.closest('.menu-overview-edit')) {
        event.preventDefault();
        event.stopPropagation();
        const next = root.prompt?.('修改二级菜单名称', overviewDisplayName(menu));
        if (next && saveOverviewItemName(menuKey(menu), next)) {
          refreshMenuOverview();
        }
        return;
      }
      if (menu.path) root.location.href = menu.path;
    });
    return button;
  }

  function overviewGroupedSections(rows, query, layout) {
    layout = cleanOverviewLayout(layout || readOverviewLayout());
    const search = String(query || '').trim().toLowerCase();
    return MENU_CATEGORY_ORDER.map(category => {
      const categoryRows = sortOverviewRows((rows || []).filter(menu => menu.category === category));
      const groupMap = new Map();
      const configuredNames = overviewGroupNames(category);
      configuredNames.forEach(name => {
        const id = overviewGroupId(category, name);
        const entry = layout.groups[id] || {};
        const groupCategory = MENU_CATEGORY_SET.has(entry.category) ? entry.category : category;
        if (groupCategory === category) groupMap.set(id, {id, name: entry.name || name, category, order: Number.isFinite(Number(entry.order)) ? Number(entry.order) : configuredNames.indexOf(name), items: []});
      });
      Object.keys(layout.groups || {}).forEach(id => {
        const entry = layout.groups[id] || {};
        const groupCategory = MENU_CATEGORY_SET.has(entry.category) ? entry.category : category;
        if (groupCategory !== category) return;
        if (!groupMap.has(id)) groupMap.set(id, {id, name: entry.name || id.split('::').slice(1).join('::') || '未分组', category, order: Number.isFinite(Number(entry.order)) ? Number(entry.order) : 50000, items: []});
      });
      categoryRows.forEach(menu => {
        const groupName = menu.overviewGroup || mappedOverviewGroup(menu, category);
        const groupId = menu.overviewGroupId || overviewGroupId(category, groupName);
        if (!groupMap.has(groupId)) groupMap.set(groupId, {id: groupId, name: groupName, category, order: 90000, items: []});
        groupMap.get(groupId).items.push(menu);
      });
      const groups = Array.from(groupMap.values()).sort((a, b) =>
        (Number(a.order || 0) - Number(b.order || 0)) ||
        String(a.name || '').localeCompare(String(b.name || ''), 'zh-Hans-CN')
      ).map(group => {
        const groupMatched = search && String(group.name || '').toLowerCase().includes(search);
        const filtered = search && !groupMatched
          ? group.items.filter(menu => menuText(menu).includes(search) || overviewDisplayName(menu).toLowerCase().includes(search) || overviewMatchText(menu).includes(search))
          : group.items;
        return Object.assign({}, group, {items: sortOverviewRows(filtered)});
      }).filter(group => group.items.length || (!search && layout.groups[group.id]));
      return {category, label: MENU_CATEGORY_LABELS[category], groups};
    });
  }

  function renderMenuOverviewBody(container, menus, keyword) {
    const query = String(keyword || '').trim().toLowerCase();
    container.innerHTML = '';
    const layout = cleanOverviewLayout(readOverviewLayout());
    const arranged = overviewRowsFor(menus, layout);
    const sections = overviewGroupedSections(arranged, query, layout);
    const groupFavoriteSet = new Set(groupFavoriteKeysFor(sections, menus));
    if (!sections.some(section => section.groups.length)) {
      container.innerHTML = '<div class="menu-overview-empty">未找到匹配菜单</div>';
      return;
    }
    sections.forEach(section => {
      const category = section.category;
      if (query && !section.groups.length) return;
      const card = root.document.createElement('section');
      card.className = 'menu-overview-category';
      card.innerHTML = '<h3><span>' + escapeHTML(section.label) + '</span><button class="menu-overview-add-group" type="button" title="新增一级菜单">＋ 一级</button></h3><div class="menu-overview-category-list" data-category="' + category + '"></div>';
      const list = card.querySelector('.menu-overview-category-list');
      card.querySelector('.menu-overview-add-group')?.addEventListener('click', event => {
        event.preventDefault();
        event.stopPropagation();
        if (addOverviewGroup(category)) refreshMenuOverview();
      });
      const handleDrop = (event, groupName) => {
        event.preventDefault();
        card.classList.remove('menu-overview-category--drop');
        const draggedKey = event.dataTransfer?.getData('application/x-simplehpc-menu-key') || event.dataTransfer?.getData('text/plain');
        const before = event.target.closest('.menu-overview-item');
        const beforeKey = before && before.dataset.menuKey !== draggedKey ? before.dataset.menuKey : '';
        if (moveOverviewItem(menus, draggedKey, category, beforeKey, groupName)) {
          renderMenuOverviewBody(container, menus, keyword);
        }
      };
      list.addEventListener('dragover', event => {
        event.preventDefault();
        if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
        card.classList.add('menu-overview-category--drop');
      });
      list.addEventListener('dragleave', event => {
        if (!list.contains(event.relatedTarget)) card.classList.remove('menu-overview-category--drop');
      });
      list.addEventListener('drop', event => {
        if (event.target.closest('.menu-overview-group-list')) return;
        const draggedGroupId = event.dataTransfer?.getData('application/x-simplehpc-group-id');
        if (draggedGroupId) {
          event.preventDefault();
          card.classList.remove('menu-overview-category--drop');
          if (moveOverviewGroup(menus, draggedGroupId, '', category, '')) refreshMenuOverview();
          return;
        }
        handleDrop(event, MENU_OVERVIEW_GROUP_DEFAULT[category] || '其他');
      });
      section.groups.forEach(group => {
        const groupEl = root.document.createElement('div');
        groupEl.className = 'menu-overview-group';
        groupEl.dataset.groupId = group.id;
        groupEl.dataset.category = category;
        groupEl.draggable = true;
        groupEl.innerHTML = '<div class="menu-overview-group-title" title="可拖拽一级菜单调整位置"><span class="menu-overview-group-name">' + escapeHTML(group.name) + '</span><span class="menu-overview-group-actions"><button type="button" data-action="favorite-group" class="menu-overview-group-star" title="固定到顶部导航">' + (groupFavoriteSet.has(group.id) ? '★' : '☆') + '</button><button type="button" data-action="rename-group" title="编辑一级菜单名称">✎</button><button type="button" data-action="add-leaf" title="新增二级菜单">＋</button></span></div><div class="menu-overview-group-list" data-category="' + category + '" data-group="' + escapeHTML(group.name) + '"></div>';
        const groupList = groupEl.querySelector('.menu-overview-group-list');
        groupEl.addEventListener('dragstart', event => {
          if (event.target.closest('.menu-overview-item,button')) {
            event.preventDefault();
            return;
          }
          groupEl.classList.add('menu-overview-group--dragging');
          event.dataTransfer?.setData('application/x-simplehpc-group-id', group.id);
          event.dataTransfer?.setData('text/plain', group.id);
          if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
        });
        groupEl.addEventListener('dragend', () => {
          groupEl.classList.remove('menu-overview-group--dragging');
          root.document?.querySelectorAll('.menu-overview-category--drop,.menu-overview-group--drop').forEach(node => node.classList.remove('menu-overview-category--drop', 'menu-overview-group--drop'));
        });
        groupEl.querySelector('[data-action="rename-group"]')?.addEventListener('click', event => {
          event.preventDefault();
          event.stopPropagation();
          const next = root.prompt?.('修改一级菜单名称', group.name);
          if (next && saveOverviewGroupName(category, group.name, next)) refreshMenuOverview();
        });
        groupEl.querySelector('[data-action="favorite-group"]')?.addEventListener('click', event => {
          event.preventDefault();
          event.stopPropagation();
          toggleGroupFavorite(group.id);
        });
        groupEl.querySelector('[data-action="add-leaf"]')?.addEventListener('click', event => {
          event.preventDefault();
          event.stopPropagation();
          if (addOverviewMenu(category, group.name)) refreshMenuOverview();
        });
        groupList.addEventListener('dragover', event => {
          event.preventDefault();
          event.stopPropagation();
          if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
          card.classList.add('menu-overview-category--drop');
          groupEl.classList.add('menu-overview-group--drop');
        });
        groupList.addEventListener('dragleave', event => {
          if (!groupList.contains(event.relatedTarget)) groupEl.classList.remove('menu-overview-group--drop');
        });
        groupList.addEventListener('drop', event => {
          event.stopPropagation();
          const draggedGroupId = event.dataTransfer?.getData('application/x-simplehpc-group-id');
          if (draggedGroupId) {
            event.preventDefault();
            const beforeGroup = event.target.closest('.menu-overview-group');
            const beforeGroupId = beforeGroup && beforeGroup.dataset.groupId !== draggedGroupId ? beforeGroup.dataset.groupId : '';
            groupEl.classList.remove('menu-overview-group--drop');
            if (moveOverviewGroup(menus, draggedGroupId, '', category, beforeGroupId || group.id)) refreshMenuOverview();
            return;
          }
          groupEl.classList.remove('menu-overview-group--drop');
          handleDrop(event, group.name);
        });
        group.items.forEach(menu => groupList.appendChild(menuOverviewItem(menu)));
        list.appendChild(groupEl);
      });
      if (!section.groups.length) {
        const empty = root.document.createElement('div');
        empty.className = 'menu-overview-category-empty';
        empty.textContent = '拖拽菜单到此列';
        list.appendChild(empty);
      }
      container.appendChild(card);
    });
  }

  function openMenuOverview() {
    const doc = root.document;
    if (!doc || !context) return;
    const menus = overviewAllMenus();
    const overlay = doc.createElement('div');
    overlay.className = 'menu-overview-overlay';
    overlay.innerHTML = '<div class="menu-overview-modal" role="dialog" aria-modal="true" aria-label="菜单总览">' +
      '<div class="menu-overview-head"><div><h2>菜单总览</h2><p>选择功能入口，支持拖拽一级/二级菜单、改名和新增自定义入口</p></div><button class="menu-overview-close" type="button" aria-label="关闭">×</button></div>' +
      '<div class="menu-overview-search"><input type="search" placeholder="搜索菜单" aria-label="搜索菜单"></div>' +
      '<div class="menu-overview-body"></div>' +
      '</div>';
    doc.body.appendChild(overlay);
    const close = () => {
      overlay.classList.remove('menu-overview-overlay--in');
      overlay.addEventListener('transitionend', () => overlay.remove(), {once: true});
    };
    const body = overlay.querySelector('.menu-overview-body');
    const input = overlay.querySelector('input');
    renderMenuOverviewBody(body, menus, '');
    overlay.querySelector('.menu-overview-close').onclick = close;
    overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
    const keyHandler = event => {
      if (event.key === 'Escape') {
        close();
        doc.removeEventListener('keydown', keyHandler);
      }
    };
    doc.addEventListener('keydown', keyHandler);
    input.addEventListener('input', () => renderMenuOverviewBody(body, menus, input.value));
    requestAnimationFrame(() => {
      overlay.classList.add('menu-overview-overlay--in');
      input.focus();
    });
  }

  function toggleFavorite(key) {
    const menus = overviewAllMenus();
    const allowed = new Set(menus.map(menuKey));
    if (!allowed.has(key)) return;
    const keys = favoriteKeysFor(menus);
    const set = new Set(keys);
    if (set.has(key)) set.delete(key); else set.add(key);
    const saved = writeFavoriteKeys(Array.from(set));
    renderNavigation();
    const overlay = root.document?.querySelector('.menu-overview-overlay');
    if (overlay) {
      refreshMenuOverview();
    }
    if (!saved) root.App?.toast?.('星标菜单保存失败，请检查浏览器存储权限', 'danger');
  }

  function toggleGroupFavorite(groupId) {
    const menus = overviewAllMenus();
    const layout = cleanOverviewLayout(readOverviewLayout());
    const rows = overviewRowsFor(menus, layout);
    const sections = overviewGroupedSections(rows, '', layout);
    const allowed = new Set();
    sections.forEach(section => (section.groups || []).forEach(group => {
      if (group.items?.length) allowed.add(group.id);
    }));
    if (!allowed.has(groupId)) return;
    const keys = groupFavoriteKeysFor(sections, menus);
    const set = new Set(keys);
    if (set.has(groupId)) set.delete(groupId); else set.add(groupId);
    const saved = writeGroupFavoriteKeys(Array.from(set));
    renderNavigation();
    refreshMenuOverview();
    if (!saved) root.App?.toast?.('导航菜单保存失败，请检查浏览器存储权限', 'danger');
  }

  function renderNavigation() {
    const topnav = ensureTopbar();
    const legacyNav = root.document?.querySelector('.sidebar .nav');
    if (legacyNav) {
      legacyNav.innerHTML = '';
      legacyNav.classList.toggle('nav-dynamic', true);
    }
    if (!topnav || !context) return;
    const menus = overviewAllMenus();
    const layout = cleanOverviewLayout(readOverviewLayout());
    const rows = overviewRowsFor(menus, layout);
    const sections = overviewGroupedSections(rows, '', layout);
    const favoriteGroupIds = groupFavoriteKeysFor(sections, menus);
    const groupMap = new Map();
    sections.forEach(section => (section.groups || []).forEach(group => {
      if (group.items?.length) groupMap.set(group.id, group);
    }));
    const primaryMenus = favoriteGroupIds.map(id => groupMap.get(id)).filter(Boolean);
    topnav.innerHTML = '';
    topnav.className = 'topnav-list topnav-primary-list';
    if (!primaryMenus.length) {
      const empty = root.document.createElement('li');
      empty.className = 'topnav-empty';
      empty.textContent = '暂无可访问菜单';
      topnav.appendChild(empty);
      return;
    }
    primaryMenus.forEach(group => topnav.appendChild(topnavOverviewGroup(group)));
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
