(function () {
  'use strict';
  const state = {roles: [], menus: [], permissions: [], view: 'list'};
  const scopeNames = {global:'全局', unit:'本单位', team:'本团队', self:'个人', granted:'被授权数据', none:'无权限',team_shared:'团队共享目录',unit_shared:'单位共享目录',team_members:'团队成员目录',unit_members:'单位成员目录'};
  const accessNames = {none:'无权限', view:'查看', manage:'管理'};
  const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

  async function request(url, options) {
    const token = localStorage.getItem('simplehpc_token') || '';
    const response = await fetch(url, Object.assign({
      credentials:'same-origin',
      headers:Object.assign({'Content-Type':'application/json'}, token ? {Authorization:'Bearer '+token} : {})
    }, options || {}));
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    return data;
  }

  const flattenMenus = items => (items || []).flatMap(item => [item].concat(flattenMenus(item.children)));
  const pages = () => state.menus.filter(item => item.type === 'page');

  async function load() {
    const [roles, menus, permissions] = await Promise.all([
      request('/api/v1/rbac/roles'), request('/api/v1/rbac/menus'), request('/api/v1/rbac/permissions')
    ]);
    state.roles = roles.items || [];
    state.menus = menus.items || [];
    state.permissions = permissions.items || [];
    render();
  }

  function statusChip(role) {
    return `<span class="rbac-chip ${role.status === 'active' ? 'is-active' : 'is-disabled'}">${role.status === 'active' ? '启用' : '禁用'}</span>`;
  }

  function renderList() {
    const body = document.getElementById('roleListBody');
    body.innerHTML = state.roles.map(role => `<tr>
      <td><div class="rbac-role-name"><strong>${esc(role.name)}</strong>${role.isBuiltin ? '<span class="rbac-chip">内置</span>' : '<span class="rbac-chip custom">自定义</span>'}</div><small>${esc(role.description || '暂无说明')}</small></td>
      <td><code>${esc(role.code)}</code></td><td>${esc(scopeNames[role.scopeType] || role.scopeType)}</td>
      <td>${esc(role.permissionSummary || '尚未配置摘要')}</td><td><b>${role.userCount || 0}</b> 人</td><td>${statusChip(role)}</td>
      <td><div class="rbac-actions">
        <button class="btn btn-sm" data-role-action="edit" data-code="${esc(role.code)}" data-permission="action.roles.edit">编辑</button>
        <button class="btn btn-sm" data-role-action="copy" data-code="${esc(role.code)}" data-permission="action.roles.copy">复制</button>
        <button class="btn btn-sm" data-role-action="toggle" data-code="${esc(role.code)}" data-status="${esc(role.status)}" data-permission="action.roles.edit" ${role.code === 'cluster_admin' ? 'disabled title="最高权限角色不可禁用"' : ''}>${role.status === 'active' ? '禁用' : '启用'}</button>
        ${role.allowDelete ? `<button class="btn btn-sm btn-danger-ghost" data-role-action="delete" data-code="${esc(role.code)}" data-permission="action.roles.delete">删除</button>` : ''}
      </div></td></tr>`).join('') || '<tr><td colspan="7" class="empty-state">暂无角色</td></tr>';
    window.App?.authz?.applyButtonPermissions(document);
  }

  function render() {
    document.getElementById('roleListView').hidden = state.view !== 'list';
    document.getElementById('roleMatrixView').hidden = state.view !== 'matrix';
    document.querySelectorAll('[data-role-view]').forEach(btn => btn.classList.toggle('active', btn.dataset.roleView === state.view));
    if (state.view === 'list') renderList(); else renderMatrix();
  }

  function input(id, label, value, attrs) {
    return `<label class="rbac-field"><span>${label}</span><input id="${id}" value="${esc(value)}" ${attrs || ''}></label>`;
  }

  function tabButton(key, label, active) {
    return `<button type="button" class="rbac-tab ${active ? 'active' : ''}" data-editor-tab="${key}">${label}</button>`;
  }

  function editorShell(config) {
    const role = config.role || {};
    const checked = new Set(config.permissions || []);
    const menuTree = state.menus.filter(item => item.type === 'group').map(group => {
      const children = state.menus.filter(item => item.parentCode === group.code);
      return `<section class="rbac-check-group"><h4>${esc(group.name)}</h4>${children.map(item =>
        `<label><input type="checkbox" data-permission-key="${esc(item.permission)}" ${checked.has(item.permission) ? 'checked' : ''}> <span>${esc(item.name)}</span><small>${esc(item.permission)}</small></label>`
      ).join('')}</section>`;
    }).join('') + state.menus.filter(item => item.type === 'page' && !item.parentCode).map(item =>
      `<section class="rbac-check-group"><label><input type="checkbox" data-permission-key="${esc(item.permission)}" ${checked.has(item.permission) ? 'checked' : ''}> <span>${esc(item.name)}</span><small>${esc(item.permission)}</small></label></section>`
    ).join('');
    const operations = state.permissions.filter(item => item.type === 'action').reduce((groups, item) => {
      (groups[item.module] ||= []).push(item); return groups;
    }, {});
    const operationHTML = Object.entries(operations).map(([module, items]) =>
      `<section class="rbac-check-group"><h4>${esc(module)}</h4>${items.map(item =>
        `<label><input type="checkbox" data-permission-key="${esc(item.key)}" ${checked.has(item.key) ? 'checked' : ''}> <span>${esc(item.name)}</span><small>${esc(item.key)}</small></label>`
      ).join('')}</section>`).join('');
    return `<div class="rbac-editor">
      <nav class="rbac-tabs">
        ${tabButton('basic','基础信息',true)}${tabButton('menus','菜单权限')}${tabButton('actions','操作权限')}
        ${tabButton('scopes','数据范围')}${tabButton('files','文件目录权限')}${tabButton('bindings','绑定用户')}
      </nav>
      <section class="rbac-pane active" data-editor-pane="basic"><div class="rbac-form-grid">
        ${input('roleName','角色名称',role.name || '')}${input('roleCode','角色编码',role.code || '', role.code ? 'readonly' : 'placeholder="例如 project_admin"')}
        <label class="rbac-field"><span>角色作用域</span><select id="roleScope">${['global','unit','team','self'].map(v=>`<option value="${v}" ${role.scopeType===v?'selected':''}>${scopeNames[v]}</option>`).join('')}</select></label>
        <label class="rbac-field"><span>权限编辑</span><select id="rolePermissionEdit"><option value="true" ${role.allowPermissionEdit!==false?'selected':''}>允许</option><option value="false" ${role.allowPermissionEdit===false?'selected':''}>禁止</option></select></label>
        ${input('roleSummary','权限摘要',role.permissionSummary || '')}
        <label class="rbac-field wide"><span>角色说明</span><textarea id="roleDescription" rows="4">${esc(role.description || '')}</textarea></label>
      </div>${role.isBuiltin ? '<div class="rbac-note">内置角色可作为模板复制；删除保护由后端强制执行。</div>' : ''}</section>
      <section class="rbac-pane" data-editor-pane="menus"><div class="rbac-permission-grid">${menuTree}</div></section>
      <section class="rbac-pane" data-editor-pane="actions"><div class="rbac-permission-grid">${operationHTML}</div><details class="rbac-advanced"><summary>高级：路由与接口权限</summary><div class="rbac-permission-grid">${state.permissions.filter(p=>p.type==='route'||p.type==='api').map(item=>`<label><input type="checkbox" data-permission-key="${esc(item.key)}" ${checked.has(item.key)?'checked':''}> ${esc(item.name)} <small>${esc(item.key)}</small></label>`).join('')}</div></details></section>
      <section class="rbac-pane" data-editor-pane="scopes"><div id="scopeRows">${(config.dataScopes||[]).map(scopeRow).join('')}</div><button type="button" class="btn btn-sm" data-add-row="scope">+ 添加数据范围</button></section>
      <section class="rbac-pane" data-editor-pane="files"><div class="rbac-note">文件策略独立于普通数据范围合并；未显式授权即不可访问。</div><div id="fileRows">${(config.filePolicies||[]).map(fileRow).join('')}</div><button type="button" class="btn btn-sm" data-add-row="file">+ 添加文件策略</button></section>
      <section class="rbac-pane" data-editor-pane="bindings"><div id="bindingRows">${(config.bindings||[]).map(bindingRow).join('')}</div><button type="button" class="btn btn-sm" data-add-row="binding">+ 绑定用户</button></section>
    </div>`;
  }

  function scopeRow(item={}) {
    return `<div class="rbac-row" data-row="scope">${input('','资源代码',item.resource||'','data-field="resource"')}${select('范围','scope',['global','unit','team','self','granted','none'],item.scope)}${select('操作级别','access',['none','view','manage'],item.access)}<button type="button" class="btn btn-sm" data-remove-row>移除</button></div>`;
  }
  function fileRow(item={}) {
    return `<div class="rbac-row" data-row="file">${input('','存储根路径',item.storageRoot||'','data-field="storageRoot" placeholder="/data/home"')}${select('主体范围','subjectScope',['global','self','team_shared','unit_shared','team_members','unit_members'],item.subjectScope)}${select('权限','access',['none','view','manage'],item.access)}<label class="rbac-inline"><input type="checkbox" data-field="allowHidden" ${item.allowHidden?'checked':''}> 隐藏文件</label><button type="button" class="btn btn-sm" data-remove-row>移除</button></div>`;
  }
  function bindingRow(item={}) {
    return `<div class="rbac-row" data-row="binding">${select('账号类型','accountType',['admin','ldap'],item.accountType)}${input('','用户账号',item.username||'','data-field="username"')}${select('绑定范围','scopeType',['global','unit','team','self'],item.scopeType)}${input('','范围 ID',item.scopeId||'','data-field="scopeId" placeholder="全局可留空"')}<button type="button" class="btn btn-sm" data-remove-row>移除</button></div>`;
  }
  function select(label, field, values, selected) {
    return `<label class="rbac-field"><span>${label}</span><select data-field="${field}">${values.map(v=>`<option value="${v}" ${selected===v?'selected':''}>${scopeNames[v]||accessNames[v]||v}</option>`).join('')}</select></label>`;
  }

  function collectRows(root, type, fields) {
    return Array.from(root.querySelectorAll(`[data-row="${type}"]`)).map(row => {
      const item = {};
      fields.forEach(field => {
        const el = row.querySelector(`[data-field="${field}"]`);
        item[field] = el?.type === 'checkbox' ? !!el.checked : (el?.value || '');
      });
      return item;
    }).filter(item => Object.values(item).some(Boolean));
  }

  async function openEditor(code) {
    const config = code ? await request('/api/v1/rbac/roles/'+encodeURIComponent(code)) :
      {role:{scopeType:'self',allowPermissionEdit:true},permissions:[],dataScopes:[],filePolicies:[],bindings:[]};
    let modal;
    modal = window.App.modal({
      title: code ? `编辑角色 · ${code}` : '新建自定义角色', width:'1180px', confirmText:'保存全部配置',
      content:editorShell(config), errorPrefix:'角色保存失败',
      onSubmit:async () => {
        const root = modal.el;
        const payload = {
          code:root.querySelector('#roleCode').value.trim(), name:root.querySelector('#roleName').value.trim(),
          description:root.querySelector('#roleDescription').value.trim(), scopeType:root.querySelector('#roleScope').value,
          permissionSummary:root.querySelector('#roleSummary').value.trim(),
          allowPermissionEdit:root.querySelector('#rolePermissionEdit').value === 'true'
        };
        const saved = await request('/api/v1/rbac/roles'+(code?'/'+encodeURIComponent(code):''), {method:code?'PUT':'POST', body:JSON.stringify(payload)});
        const roleCode = saved.code || payload.code;
        const keys = Array.from(root.querySelectorAll('[data-permission-key]:checked')).map(el=>el.dataset.permissionKey);
        const unmanaged = (config.permissions||[]).filter(key => !state.permissions.some(p => p.key===key));
        await request(`/api/v1/rbac/roles/${encodeURIComponent(roleCode)}/permissions`, {method:'PUT',body:JSON.stringify({permissions:[...new Set(keys.concat(unmanaged))]})});
        await request(`/api/v1/rbac/roles/${encodeURIComponent(roleCode)}/data-scopes`, {method:'PUT',body:JSON.stringify({items:collectRows(root,'scope',['resource','scope','access'])})});
        await request(`/api/v1/rbac/roles/${encodeURIComponent(roleCode)}/file-policies`, {method:'PUT',body:JSON.stringify({items:collectRows(root,'file',['storageRoot','subjectScope','access','allowHidden'])})});
        await request(`/api/v1/rbac/roles/${encodeURIComponent(roleCode)}/users`, {method:'PUT',body:JSON.stringify({items:collectRows(root,'binding',['accountType','username','scopeType','scopeId'])})});
        await load(); await window.App.authz?.refresh(); window.App.toast('角色与全部权限配置已保存','success');
      }
    });
    wireEditor(modal.el);
    if (config.role.allowPermissionEdit === false) {
      modal.el.querySelectorAll('[data-permission-key], [data-add-row], [data-remove-row]').forEach(el=>el.disabled=true);
    }
  }

  function wireEditor(root) {
    root.querySelectorAll('[data-editor-tab]').forEach(button => button.addEventListener('click', () => {
      root.querySelectorAll('[data-editor-tab]').forEach(el=>el.classList.toggle('active',el===button));
      root.querySelectorAll('[data-editor-pane]').forEach(el=>el.classList.toggle('active',el.dataset.editorPane===button.dataset.editorTab));
    }));
    root.addEventListener('click', event => {
      const remove = event.target.closest('[data-remove-row]');
      if (remove) remove.closest('[data-row]').remove();
      const add = event.target.closest('[data-add-row]');
      if (!add) return;
      const type=add.dataset.addRow, target=root.querySelector('#'+({scope:'scopeRows',file:'fileRows',binding:'bindingRows'}[type]));
      target.insertAdjacentHTML('beforeend', type==='scope'?scopeRow():type==='file'?fileRow():bindingRow());
    });
  }

  async function copyRole(code) {
    const source = state.roles.find(role => role.code === code) || {scopeType:'self'};
    window.App.modal({title:`复制角色 · ${code}`,content:`<div class="rbac-form-grid">${input('copyName','新角色名称','')}${input('copyCode','新角色编码','','placeholder="例如 senior_user"')}</div>`,
      onSubmit:async()=>{await request(`/api/v1/rbac/roles/${encodeURIComponent(code)}/copy`,{method:'POST',body:JSON.stringify({code:document.getElementById('copyCode').value.trim(),name:document.getElementById('copyName').value.trim(),scopeType:source.scopeType,description:`复制自 ${code}`,permissionSummary:source.permissionSummary||'基于现有角色复制',allowPermissionEdit:true})});await load();window.App.toast('角色已复制','success');}});
  }

  async function renderMatrix() {
    const host=document.getElementById('matrixTable');
    host.innerHTML='<div class="empty-state">正在生成权限矩阵...</div>';
    const data=await request('/api/v1/rbac/matrix');
    const roles=data.roles||[], menus=(data.menus||[]).filter(item=>item.type==='page');
    host.innerHTML=`<table class="rbac-matrix"><thead><tr><th>功能菜单</th>${roles.map(x=>`<th>${esc(x.role.name)}<small>${esc(x.role.code)}</small></th>`).join('')}</tr></thead><tbody>${menus.map(menu=>`<tr><th>${esc(menu.name)}<small>${esc(menu.resource)}</small></th>${roles.map(config=>`<td>${matrixCell(config,menu)}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
  }

  function matrixCell(config, menu) {
    if (!(config.permissions||[]).includes('*') && !(config.permissions||[]).includes(menu.permission)) return '<span class="rbac-level none">无权限</span>';
    const relevant=(config.dataScopes||[]).filter(item=>item.resource===menu.resource);
    const rank={none:0,granted:1,self:1,team:2,unit:3,global:4};
    const best=relevant.sort((a,b)=>(rank[b.scope]||0)-(rank[a.scope]||0))[0];
    const action=(config.permissions||[]).some(key=>key.startsWith('action.') && !key.endsWith('.view'))?'管理':'查看';
    return `<span class="rbac-level">${esc(best ? (scopeNames[best.scope]||best.scope)+' · '+(accessNames[best.access]||best.access) : action)}</span>`;
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('[data-role-view]').forEach(btn=>btn.addEventListener('click',()=>{state.view=btn.dataset.roleView;render();}));
    document.getElementById('newRoleButton')?.addEventListener('click',()=>openEditor());
    document.getElementById('roleListBody')?.addEventListener('click', async event => {
      const button=event.target.closest('[data-role-action]'); if(!button)return;
      const code=button.dataset.code, action=button.dataset.roleAction;
      if(action==='edit') return openEditor(code);
      if(action==='copy') return copyRole(code);
      if(action==='toggle'){await request(`/api/v1/rbac/roles/${encodeURIComponent(code)}/status`,{method:'PUT',body:JSON.stringify({status:button.dataset.status==='active'?'disabled':'active'})});await load();return;}
      if(action==='delete') window.App.confirm(`确认删除自定义角色 ${code}？`,{danger:true,onConfirm:async()=>{await request(`/api/v1/rbac/roles/${encodeURIComponent(code)}`,{method:'DELETE'});await load();}});
    });
    load().catch(error=>{document.getElementById('roleListBody').innerHTML=`<tr><td colspan="7" class="empty-state">数据未获取：${esc(error.message)}</td></tr>`;});
  });
}());
