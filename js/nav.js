(function () {
  'use strict';

  const NAV_TREE = [
    {
      label: '概览',
      icon: '▦',
      items: [
        { label: '仪表盘', href: 'index.html' }
      ]
    },
    {
      label: '用户与权限',
      icon: '☷',
      items: [
        { label: '用户管理', href: 'users.html#users' },
        { label: '单位与团队', href: 'users.html#orgs' },
        { label: '角色权限', href: 'users.html#roles' }
      ]
    },
    {
      label: '资源与数据',
      icon: '◇',
      items: [
        { label: '队列分区', href: 'resources.html#partitions' },
        { label: '节点资源', href: 'resources.html#nodes' },
        { label: 'QOS 策略', href: 'resources.html#qos' },
        { label: '数据目录', href: 'data.html#directories' },
        { label: '访问授权', href: 'data.html#acl' },
        { label: '操作审计', href: 'data.html#audit' }
      ]
    },
    {
      label: '作业中心',
      icon: '▶',
      items: [
        { label: '作业模板', href: 'jobs.html#templates' },
        { label: '作业列表', href: 'jobs.html#jobs' },
        { label: 'VNC 桌面', href: 'jobs.html#vnc' }
      ]
    },
    {
      label: '监控运维',
      icon: '◎',
      items: [
        { label: '运行监控', href: 'monitoring.html#overview' },
        { label: '告警列表', href: 'monitoring.html#alerts' },
        { label: '自动巡检', href: 'monitoring.html#inspection' },
        { label: '节点状态', href: 'monitoring.html#nodes' }
      ]
    },
    {
      label: '系统',
      icon: '⚙',
      items: [
        { label: 'LDAP 配置', href: 'settings.html#ldap' },
        { label: 'Slurm 配置', href: 'settings.html#slurm' },
        { label: '存储配置', href: 'settings.html#storage' },
        { label: '通知渠道', href: 'settings.html#notify' },
        { label: '平台外观', href: 'settings.html#appearance' },
        { label: '审计日志', href: 'settings.html#audit' }
      ]
    }
  ];

  const DEFAULT_HASH = {
    'users.html': '#users',
    'settings.html': '#ldap',
    'resources.html': '#partitions',
    'data.html': '#directories',
    'jobs.html': '#templates',
    'monitoring.html': '#overview'
  };

  function currentFile() {
    const file = window.location.pathname.split('/').pop();
    return file || 'index.html';
  }

  function splitHref(href) {
    const parts = href.split('#');
    return {
      file: parts[0] || 'index.html',
      hash: parts[1] ? '#' + parts[1] : ''
    };
  }

  function currentHash() {
    return window.location.hash || DEFAULT_HASH[currentFile()] || '';
  }

  function isItemActive(item) {
    const target = splitHref(item.href);
    if (target.file !== currentFile()) return false;
    if (!target.hash) return !window.location.hash;
    return target.hash === currentHash();
  }

  function sectionActive(section) {
    return section.items.some(isItemActive);
  }

  function createSection(section, index) {
    const active = sectionActive(section);
    const sectionEl = document.createElement('li');
    sectionEl.className = 'nav-section' + (active ? ' active' : '');

    const parent = document.createElement('button');
    parent.className = 'nav-parent';
    parent.type = 'button';
    parent.setAttribute('aria-expanded', active ? 'true' : 'false');
    parent.innerHTML = '<span class="nav-parent-main"><span class="nav-icon" aria-hidden="true">' +
      section.icon + '</span><span class="nav-parent-label">' + section.label +
      '</span></span><span class="nav-chevron" aria-hidden="true">⌄</span>';

    const sub = document.createElement('div');
    sub.className = 'nav-sub';
    sub.id = 'nav-sub-' + index;
    parent.setAttribute('aria-controls', sub.id);

    section.items.forEach(function (item) {
      const link = document.createElement('a');
      link.className = 'nav-sub-link' + (isItemActive(item) ? ' active' : '');
      link.href = item.href;
      link.textContent = item.label;
      sub.appendChild(link);
    });

    if (!active) {
      sectionEl.classList.add('collapsed');
    }

    parent.addEventListener('click', function () {
      const willCollapse = !sectionEl.classList.contains('collapsed');
      sectionEl.classList.toggle('collapsed', willCollapse);
      parent.setAttribute('aria-expanded', willCollapse ? 'false' : 'true');
    });

    sectionEl.appendChild(parent);
    sectionEl.appendChild(sub);
    return sectionEl;
  }

  function renderNav() {
    const nav = document.querySelector('.sidebar .nav');
    if (!nav) return;
    nav.innerHTML = '';
    nav.classList.add('nav-tree');
    NAV_TREE.forEach(function (section, index) {
      nav.appendChild(createSection(section, index));
    });
  }

  function activateUsersTab() {
    if (currentFile() !== 'users.html') return;
    const key = (currentHash() || '#users').replace('#', '');
    if (!['users', 'orgs', 'roles'].includes(key)) return;
    if (typeof window.showTab === 'function') window.showTab(key, { silent: true });
  }

  function activateSettingsTab() {
    if (currentFile() !== 'settings.html') return;
    const key = (currentHash() || '#ldap').replace('#', '');
    const item = document.querySelector('.setting-nav-item[data-setting-key="' + key + '"]');
    if (item && typeof window.selectSetting === 'function') {
      window.selectSetting(item, key, { silent: true });
    }
  }

  function syncPageFromHash() {
    activateUsersTab();
    activateSettingsTab();
  }

  document.addEventListener('DOMContentLoaded', function () {
    renderNav();
    syncPageFromHash();
  });

  window.addEventListener('hashchange', function () {
    renderNav();
    syncPageFromHash();
  });
})();
