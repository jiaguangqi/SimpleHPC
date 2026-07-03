(function () {
  'use strict';

  const page = location.pathname.split('/').pop() || 'index.html';
  const apiByPage = {
    'users.html': '/api/v1/account/users',
    'teams.html': '/api/v1/account/teams',
    'units.html': '/api/v1/account/units',
    'admins.html': '/api/v1/account/admins',
    'roles.html': '/api/v1/account/roles'
  };

  if (!apiByPage[page]) return;

  const esc = (value) => String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');

  const statusPill = (status) => {
    const normalized = String(status || 'active').toLowerCase();
    if (normalized === 'active' || normalized === 'normal' || normalized === 'enabled') {
      return '<span class="pill pill-success">正常</span>';
    }
    if (normalized === 'frozen' || normalized === 'disabled' || normalized === 'locked') {
      return '<span class="pill pill-warn">冻结</span>';
    }
    if (normalized === 'deleted' || normalized === 'cancelled') {
      return '<span class="pill pill-danger">已注销</span>';
    }
    return '<span class="pill pill-info">' + esc(status || '未知') + '</span>';
  };

  const emptyRow = (colspan, text) => '<tr><td colspan="' + colspan + '" style="text-align:center;color:var(--muted);padding:28px;">' + esc(text) + '</td></tr>';

  async function fetchJSON(url, options) {
    const res = await fetch(url, options || {});
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
    return data;
  }

  function tableBody() {
    return document.querySelector('.main table tbody');
  }

  function renderUsers(data) {
    const items = Array.isArray(data.items) ? data.items : [];
    const cards = document.querySelectorAll('.stat-card');
    const frozen = items.filter((item) => String(item.status || '').toLowerCase() !== 'active').length;
    if (cards[0]) cards[0].innerHTML = '<div class="stat-label">LDAP 用户</div><div class="stat-value">' + items.length + '</div><div class="stat-note success">已同步到项目库 ' + (data.sync?.users ?? items.length) + '</div>';
    if (cards[1]) cards[1].innerHTML = '<div class="stat-label">冻结用户</div><div class="stat-value">' + frozen + '</div><div class="stat-note warning">来自项目库账号状态</div>';
    if (cards[2]) cards[2].innerHTML = '<div class="stat-label">邮件通知</div><div class="stat-value">独立配置</div><div class="stat-note">状态以通知配置测试结果为准</div>';
    const tbody = tableBody();
    if (!tbody) return;
    if (!items.length) {
      tbody.innerHTML = emptyRow(11, data.syncError ? 'LDAP 数据未获取：' + data.syncError : 'LDAP/项目库暂无用户数据');
      return;
    }
    tbody.innerHTML = items.map((item) => {
      const username = String(item.username || '');
      const frozen = ['frozen', 'disabled', 'locked'].includes(String(item.status || '').toLowerCase());
      const stateAction = frozen
        ? `<button class="small-action" data-account-action="unfreeze" data-account="${esc(username)}">解冻</button>`
        : `<button class="small-action" data-account-action="freeze" data-account="${esc(username)}">冻结</button>`;
      return `
      <tr>
        <td><input type="checkbox" class="user-table-row" onchange="window.syncUserTableMaster && window.syncUserTableMaster()"></td>
        <td>${esc(item.username)}</td>
        <td>${esc(item.uidNumber || item.studentId || '数据未获取')}</td>
        <td>${esc(item.displayName || item.username)}</td>
        <td>${esc(item.unit || '数据未获取')}</td>
        <td>${esc(item.team || '数据未获取')}</td>
        <td>${esc(item.leaderName || '数据未获取')}</td>
        <td><span class="pill pill-info">${esc(item.role || '普通用户')}</span></td>
        <td>${statusPill(item.status)}</td>
        <td>${esc(item.email || '数据未获取')}</td>
        <td><button class="small-action" data-account-action="edit" data-account="${esc(username)}" data-name="${esc(item.displayName)}" data-email="${esc(item.email)}">编辑</button> ${stateAction} <button class="small-action" data-account-action="reset-password" data-account="${esc(username)}">改密</button></td>
      </tr>`;
    }).join('');
  }

  function renderTeams(data) {
    const items = Array.isArray(data.items) ? data.items : [];
    const tbody = tableBody();
    if (!tbody) return;
    if (!items.length) {
      tbody.innerHTML = emptyRow(9, data.syncError ? 'LDAP 组数据未获取：' + data.syncError : 'LDAP/项目库暂无团队数据');
      return;
    }
    tbody.innerHTML = items.map((item) => `
      <tr>
        <td><input type="checkbox" class="batch-row"></td>
        <td>${esc(item.name)}</td>
        <td>${esc(item.groupName || item.name)}</td>
        <td>${esc(item.unit || '数据未获取')}</td>
        <td>${esc(item.leaderName || item.leaderUsername || '数据未获取')}</td>
        <td>${esc(item.members || 0)}/${esc(item.memberLimit || 50)}</td>
        <td>${esc(item.resourcePolicy || '数据未获取')}</td>
        <td>${statusPill(item.status)}</td>
        <td class="team-actions"><button class="small-action" data-team-action="edit" data-team="${esc(item.name)}">编辑</button> <button class="small-action" data-team-action="add-member" data-team="${esc(item.name)}">新增成员</button> <button class="small-action" data-team-action="members" data-team="${esc(item.name)}">查看成员</button> <button class="small-action" data-team-action="freeze" data-team="${esc(item.name)}">冻结</button> <button class="small-action" data-team-action="delete" data-team="${esc(item.name)}">删除</button></td>
      </tr>`).join('');
  }

  function renderUnits(data) {
    const items = Array.isArray(data.items) ? data.items : [];
    const tbody = tableBody();
    if (!tbody) return;
    if (!items.length) {
      tbody.innerHTML = emptyRow(7, data.syncError ? 'LDAP 单位数据未获取：' + data.syncError : 'LDAP/项目库暂无单位数据');
      return;
    }
    tbody.innerHTML = items.map((item) => `
      <tr>
        <td><input type="checkbox" class="batch-row"></td>
        <td>${esc(item.name)}</td>
        <td>${esc(item.code || '数据未获取')}</td>
        <td>${esc(item.admin || '数据未获取')}</td>
        <td>${esc(item.teams || 0)}</td>
        <td>${esc(item.members || 0)}</td>
        <td>${statusPill(item.status)}</td>
        <td><button class="small-action" data-unit-action="edit" data-code="${esc(item.code)}" data-name="${esc(item.name)}" data-admin="${esc(item.admin)}" data-status="${esc(item.status)}">编辑</button> <button class="small-action" data-unit-action="delete" data-code="${esc(item.code)}">删除</button></td>
      </tr>`).join('');
  }

  function renderAdmins(data) {
    const items = Array.isArray(data.items) ? data.items : [];
    const tbody = tableBody();
    if (!tbody) return;
    if (!items.length) {
      tbody.innerHTML = emptyRow(7, '项目库暂无管理员账号数据');
      return;
    }
    tbody.innerHTML = items.map((item) => `
      <tr>
        <td>${esc(item.username)}</td>
        <td>${esc(item.roleName || '数据未获取')}</td>
        <td>${statusPill(item.status)}</td>
        <td>${esc(item.email || '数据未获取')}</td>
        <td>${esc(item.createdBy || '数据未获取')}</td>
        <td>${esc(item.lastLogin || '数据未获取')}</td>
        <td><button class="small-action" data-admin-action="edit" data-admin="${esc(item.username)}" data-email="${esc(item.email)}" data-role="${esc(item.roleName)}" data-status="${esc(item.status)}">编辑</button> <button class="small-action" data-admin-action="reset-password" data-admin="${esc(item.username)}">重置密码</button> <button class="small-action" style="color:var(--danger)" data-admin-action="delete" data-admin="${esc(item.username)}">删除</button></td>
      </tr>`).join('');
  }

  function adminForm(button) {
    const username = button.dataset.admin || '';
    const email = button.dataset.email || '';
    const role = button.dataset.role || '';
    const status = String(button.dataset.status || 'active').toLowerCase();
    const roles = [
      ['super_admin', '超级管理员'],
      ['cluster_admin', '集群管理员'],
      ['config_admin', '配置管理员'],
      ['unit_admin', '学院单位管理员'],
      ['team_admin', '团队管理员'],
      ['audit_assistant', '审计助理']
    ];
    return `
      <form id="adminEditForm" class="_shpc-form-grid">
        <div class="_shpc-field"><label>管理员账号</label><input value="${esc(username)}" disabled></div>
        <div class="_shpc-field"><label>绑定邮箱</label><input name="email" type="email" value="${esc(email)}" required></div>
        <div class="_shpc-field"><label>角色</label><select name="roleName" required>
          ${roles.map(([value, label]) => `<option value="${value}"${value === role ? ' selected' : ''}>${label}</option>`).join('')}
        </select></div>
        <div class="_shpc-field"><label>状态</label><select name="status">
          <option value="active"${status === 'active' ? ' selected' : ''}>正常</option>
          <option value="frozen"${status === 'frozen' ? ' selected' : ''}>冻结</option>
        </select></div>
      </form>`;
  }

  function wireAdminActions() {
    if (page !== 'admins.html') return;
    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-admin-action]');
      if (!button) return;
      const username = button.dataset.admin || '';
      if (button.dataset.adminAction === 'edit') {
        window.App?.modal({
          title: '编辑管理员：' + username,
          width: '620px',
          content: adminForm(button),
          onSubmit: async () => {
            const form = document.getElementById('adminEditForm');
            const values = Object.fromEntries(new FormData(form).entries());
            await fetchJSON('/api/v1/account/admins/' + encodeURIComponent(username), {
              method: 'PUT',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(values)
            });
            window.App?.toast('管理员信息已保存', 'success');
            renderAdmins(await fetchJSON(apiByPage[page]));
          }
        });
        return;
      }
      if (button.dataset.adminAction === 'delete') {
        window.App?.confirm('确认永久删除管理员 ' + username + '？此操作不可恢复。', {
          danger: true,
          onConfirm: async () => {
            try {
              const data = await fetchJSON('/api/v1/account/admins/' + encodeURIComponent(username), { method: 'DELETE' });
              window.App?.toast(data.message || '管理员账号已删除', 'success');
              renderAdmins(await fetchJSON(apiByPage[page]));
            } catch (err) {
              window.App?.toast('删除管理员失败：' + err.message, 'danger', 6000);
            }
          }
        });
        return;
      }
      window.App?.confirm('确认重置管理员 ' + username + ' 的密码？新密码将发送到其绑定邮箱。', {
        danger: true,
        onConfirm: async () => {
          try {
            const data = await fetchJSON('/api/v1/account/admins/' + encodeURIComponent(username) + '/reset-password', { method: 'POST' });
            window.App?.toast(data.message || '新密码已发送到管理员邮箱', 'success', 5000);
          } catch (err) {
            window.App?.toast('重置密码失败：' + err.message, 'danger', 6000);
          }
        }
      });
    });
  }

  function renderRoles(data) {
    const items = Array.isArray(data.items) ? data.items : [];
    const tbody = tableBody();
    if (!tbody) return;
    if (!items.length) {
      tbody.innerHTML = emptyRow(6, '项目库暂无角色数据');
      return;
    }
    tbody.innerHTML = items.map((item) => `
      <tr>
        <td>${esc(item.name)}</td>
        <td>${esc(item.code)}</td>
        <td><span class="pill pill-info">${esc(item.scopeType || 'global')}</span></td>
        <td>${esc(item.permissionSummary || '数据未获取')}</td>
        <td>${esc(item.userCount || 0)}</td>
        <td><button class="small-action" data-role-action="edit" data-code="${esc(item.code)}" data-name="${esc(item.name)}" data-scope="${esc(item.scopeType)}" data-summary="${esc(item.permissionSummary)}">编辑</button></td>
      </tr>`).join('');
  }

  const renderers = {
    'users.html': renderUsers,
    'teams.html': renderTeams,
    'units.html': renderUnits,
    'admins.html': renderAdmins,
    'roles.html': renderRoles
  };

  function wireTeamActions() {
    if (page !== 'teams.html') return;
    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-team-action]');
      if (!button) return;
      const team = button.dataset.team || '';
      const action = button.dataset.teamAction;
      const handlers = {
        edit: window.openTeamDrawer,
        'add-member': window.openAddMemberDrawer,
        members: window.openTeamMembers,
        freeze: window.confirmFreezeTeam,
        delete: window.confirmDeleteTeam
      };
      const handler = handlers[action];
      if (typeof handler === 'function') {
        handler(team);
      } else if (window.App) {
        window.App.toast('页面操作函数未加载，请刷新后重试', 'warn');
      }
    });
  }

  async function accountOperation(account, action, options) {
    const method = options?.method || 'POST';
    const url = action === 'delete'
      ? '/api/v1/account/users/' + encodeURIComponent(account)
      : '/api/v1/account/users/' + encodeURIComponent(account) + '/' + action;
    return fetchJSON(url, { method });
  }

  function wireAccountActions() {
    if (page !== 'users.html') return;
    document.addEventListener('click', async (event) => {
      const button = event.target.closest('[data-account-action]');
      if (!button) return;
      const account = button.dataset.account || '';
      const action = button.dataset.accountAction;
      if (!account) return;
      try {
        if (action === 'edit') {
          window.App?.modal({title:'编辑用户：'+account,content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>姓名</label><input id="editUserName" value="'+esc(button.dataset.name||'')+'"></div><div class="_shpc-field"><label>邮箱</label><input id="editUserEmail" value="'+esc(button.dataset.email||'')+'"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/users/'+encodeURIComponent(account),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({displayName:document.getElementById('editUserName').value,email:document.getElementById('editUserEmail').value})});renderUsers(await fetchJSON(apiByPage[page]));window.App.toast('用户信息已同步到 LDAP','success');}});
          return;
        } else if (action === 'freeze') {
          await accountOperation(account, 'freeze');
          window.App?.toast('账号已冻结，LDAP 登录已禁用', 'warn');
        } else if (action === 'unfreeze') {
          await accountOperation(account, 'unfreeze');
          window.App?.toast('账号已解冻', 'success');
        } else if (action === 'reset-password') {
          const data = await accountOperation(account, 'reset-password');
          const password = data.result?.password || '数据未获取';
          window.App?.modal({
            title: '密码已重置：' + esc(account),
            width: '520px',
            content: '<div class="one-time-password"><span>新密码仅显示一次</span><code>' + esc(password) + '</code></div><p class="muted-small">LDAP 密码已更新；关闭后页面不再显示。</p>',
            showFooter: false
          });
        }
        const data = await fetchJSON(apiByPage[page]);
        renderers[page](data);
      } catch (err) {
        window.App?.toast('账号操作失败：' + err.message, 'danger', 5000);
      }
    });
  }

  function wireUnitRoleActions() {
    document.addEventListener('click', event => {
      const unit = event.target.closest('[data-unit-action]');
      if (unit) {
        const code = unit.dataset.code;
        if (unit.dataset.unitAction === 'delete') {
          return window.App.confirm('确认删除单位 '+code+'？仅无团队和用户引用时允许删除。',{danger:true,onConfirm:async()=>{await fetchJSON('/api/v1/account/units/'+encodeURIComponent(code),{method:'DELETE'});renderUnits(await fetchJSON('/api/v1/account/units'));window.App.toast('单位已删除','success');}});
        }
        return window.App.modal({title:'编辑单位：'+code,content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>名称</label><input id="editUnitName" value="'+esc(unit.dataset.name)+'"></div><div class="_shpc-field"><label>编码</label><input id="editUnitCode" value="'+esc(code)+'"></div><div class="_shpc-field"><label>管理员</label><input id="editUnitAdmin" value="'+esc(unit.dataset.admin)+'"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/units/'+encodeURIComponent(code),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:document.getElementById('editUnitName').value,code:document.getElementById('editUnitCode').value,admin:document.getElementById('editUnitAdmin').value,status:unit.dataset.status||'active'})});renderUnits(await fetchJSON('/api/v1/account/units'));window.App.toast('单位信息已保存','success');}});
      }
      const role = event.target.closest('[data-role-action]');
      if (role) {
        const code=role.dataset.code;
        window.App.modal({title:'编辑角色：'+code,content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>名称</label><input id="editRoleName" value="'+esc(role.dataset.name)+'"></div><div class="_shpc-field"><label>作用域</label><input id="editRoleScope" value="'+esc(role.dataset.scope)+'"></div><div class="_shpc-field"><label>权限摘要</label><input id="editRoleSummary" value="'+esc(role.dataset.summary)+'"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/roles/'+encodeURIComponent(code),{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({code,name:document.getElementById('editRoleName').value,scopeType:document.getElementById('editRoleScope').value,permissionSummary:document.getElementById('editRoleSummary').value})});renderRoles(await fetchJSON('/api/v1/account/roles'));window.App.toast('角色权限已保存','success');}});
      }
    });
  }

  document.addEventListener('DOMContentLoaded', async () => {
    wireTeamActions();
    wireAccountActions();
    wireAdminActions();
    wireUnitRoleActions();
    const tbody = tableBody();
    if (tbody) tbody.innerHTML = emptyRow(tbody.closest('table')?.querySelectorAll('thead th').length || 1, '正在获取后端数据...');
    try {
      const data = await fetchJSON(apiByPage[page]);
      renderers[page](data);
    } catch (err) {
      if (tbody) tbody.innerHTML = emptyRow(tbody.closest('table')?.querySelectorAll('thead th').length || 1, '数据未获取：' + err.message);
    }
  });

  window.simpleHPCSyncLDAP = async function () {
    await fetchJSON('/api/v1/account/sync-ldap', { method: 'POST' });
    const data = await fetchJSON(apiByPage[page]);
    renderers[page](data);
  };
  window.openUserCreator = function () {
    Promise.all([fetchJSON('/api/v1/account/teams'),fetchJSON('/api/v1/account/units')]).then(([teams,units])=>{
      const teamOptions=(teams.items||[]).map(item=>'<option value="'+esc(item.groupName||item.name)+'">'+esc(item.name)+'</option>').join('');
      const unitOptions=(units.items||[]).map(item=>'<option value="'+esc(item.code||item.name)+'">'+esc(item.name)+'</option>').join('');
      window.App.modal({title:'新建 LDAP 用户',content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>账号</label><input id="createUserName"></div><div class="_shpc-field"><label>姓名</label><input id="createDisplayName"></div><div class="_shpc-field"><label>邮箱</label><input id="createUserEmail"></div><div class="_shpc-field"><label>单位</label><select id="createUserUnit">'+unitOptions+'</select></div><div class="_shpc-field"><label>团队</label><select id="createUserTeam">'+teamOptions+'</select></div><div class="_shpc-field"><label>主目录</label><input id="createUserHome" placeholder="留空使用 /data/home/账号"></div></div>',onSubmit:async()=>{const result=await fetchJSON('/api/v1/account/users',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:document.getElementById('createUserName').value,displayName:document.getElementById('createDisplayName').value,email:document.getElementById('createUserEmail').value,unit:document.getElementById('createUserUnit').value,team:document.getElementById('createUserTeam').value,homeDirectory:document.getElementById('createUserHome').value})});renderUsers(await fetchJSON(apiByPage[page]));window.App.modal({title:'用户创建成功',content:'<div class="one-time-password"><span>初始密码仅显示一次</span><code>'+esc(result.password||'数据未获取')+'</code></div>',showFooter:false});}});
    }).catch(error=>window.App.toast(error.message,'danger'));
  };
  window.openUnitEditor = function () {
    window.App.modal({title:'新建单位',content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>单位名称</label><input id="newUnitName"></div><div class="_shpc-field"><label>单位编码</label><input id="newUnitCode"></div><div class="_shpc-field"><label>管理员</label><input id="newUnitAdmin"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/units',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:document.getElementById('newUnitName').value,code:document.getElementById('newUnitCode').value,admin:document.getElementById('newUnitAdmin').value,status:'active'})});renderUnits(await fetchJSON(apiByPage[page]));window.App.toast('单位已创建','success');}});
  };
  window.openRoleEditor = function () {
    window.App.modal({title:'新建角色',content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>角色名称</label><input id="newRoleName"></div><div class="_shpc-field"><label>角色编码</label><input id="newRoleCode"></div><div class="_shpc-field"><label>作用域</label><select id="newRoleScope"><option value="global">全局</option><option value="unit">单位</option><option value="team">团队</option><option value="self">个人</option></select></div><div class="_shpc-field"><label>权限摘要</label><input id="newRoleSummary"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/roles',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:document.getElementById('newRoleName').value,code:document.getElementById('newRoleCode').value,scopeType:document.getElementById('newRoleScope').value,permissionSummary:document.getElementById('newRoleSummary').value})});renderRoles(await fetchJSON(apiByPage[page]));window.App.toast('角色已创建','success');}});
  };
  window.openAdminCreator = function () {
    window.App.modal({title:'新建管理员',content:'<div class="_shpc-form-grid"><div class="_shpc-field"><label>账号</label><input id="newAdminName"></div><div class="_shpc-field"><label>邮箱</label><input id="newAdminEmail" type="email"></div><div class="_shpc-field"><label>角色</label><input id="newAdminRole" value="cluster_admin"></div><div class="_shpc-field"><label>初始密码</label><input id="newAdminPassword" type="password"></div></div>',onSubmit:async()=>{await fetchJSON('/api/v1/account/admins',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:document.getElementById('newAdminName').value,email:document.getElementById('newAdminEmail').value,roleName:document.getElementById('newAdminRole').value,password:document.getElementById('newAdminPassword').value})});renderAdmins(await fetchJSON(apiByPage[page]));window.App.toast('管理员已创建','success');}});
  };
})();
