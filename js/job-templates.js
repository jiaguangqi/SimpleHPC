(function () {
  'use strict';
  const token = localStorage.getItem('simplehpc_token') || '';
  const user = JSON.parse(localStorage.getItem('simplehpc_user') || '{}');
  const state = { items: [], canManage: false, view: 'library', layout: 'grid', selected: new Set(), fields: [], selectedField: -1, designerTab: 'ui', projects: [] };
  const kinds = { batch: '非交互式作业', novnc: 'noVNC 桌面', webapp: 'Web 应用转发' };
  const fieldTypes = {
    section:'分组标题', divider:'分隔线', hint:'提示文字',
    text:'单行输入', textarea:'多行输入', number:'数字输入', select:'下拉选项',
    radio:'单选项', multiselect:'多选项', checkbox:'开关', date:'日期', time:'时间', slider:'滑块',
    partition:'队列选择', cpu:'CPU 数量', gpu:'GPU 数量', file:'输入文件', directory:'目录选择'
  };
  const fieldGroups = [
    {label:'容器与展示', types:['section','divider','hint']},
    {label:'基础字段', types:['text','textarea','number','select','radio','multiselect','checkbox','date','time','slider']},
    {label:'HPC 扩展字段', types:['partition','cpu','gpu','file','directory']}
  ];
  const displayTypes = new Set(['section','divider','hint']);
  const $ = (s, r) => (r || document).querySelector(s);
  const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const actionIcons = {
    detail: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2.1 12s3.6-7 9.9-7 9.9 7 9.9 7-3.6 7-9.9 7-9.9-7-9.9-7Z"/><circle cx="12" cy="12" r="3"/></svg>',
    edit: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>',
    grant: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="m16 11 2 2 4-4"/></svg>',
    publish: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V4"/><path d="m7 9 5-5 5 5"/><path d="M20 15v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-4"/></svg>',
    unpublish: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M10 9v6M14 9v6"/></svg>',
    export: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4v12"/><path d="m7 11 5 5 5-5"/><path d="M5 20h14"/></svg>',
    delete: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18"/><path d="M8 6V4h8v2M19 6l-1 15H6L5 6"/><path d="M10 11v5M14 11v5"/></svg>',
    use: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="m10 8 6 4-6 4Z"/></svg>',
    request: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="8" r="3"/><path d="M3 20v-2a5 5 0 0 1 10 0v2"/><path d="M17 8v6M14 11h6"/></svg>'
  };
  const iconActions = new Set(Object.keys(actionIcons));
  async function api(path, options) {
    const response = await fetch('/api/v1' + path, Object.assign({ headers: {'Content-Type':'application/json','Authorization':'Bearer '+token} }, options || {}));
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || '请求失败 (' + response.status + ')');
    return body;
  }
  function button(label, action, id, cls) {
    const iconOnly = iconActions.has(action);
    return `<button type="button" class="btn ${cls||'btn-ghost'}${iconOnly?' tpl-action-icon':''}" data-action="${esc(action)}" data-id="${esc(id)}"${iconOnly?` aria-label="${esc(label)}" title="${esc(label)}"`:''}>${iconOnly?actionIcons[action]+`<span class="sr-only">${esc(label)}</span>`:esc(label)}</button>`;
  }
  function downloadFile(name, content, type) {
    const url = URL.createObjectURL(new Blob([content], {type:type}));
    const link = document.createElement('a');
    link.href = url; link.download = name; link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function safeFileName(value) {
    return String(value || 'template').trim().replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^-+|-+$/g, '') || 'template';
  }
  function safeJobName(name, id) {
    const value = String(name || '').trim()
      .replace(/\s+/g, '-')
      .replace(/[^A-Za-z0-9._-]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 128);
    return value || `job-template-${id}`;
  }
  function selectedProjectFromURL() {
    const params = new URLSearchParams(location.search);
    return { projectId: params.get('projectId') || '', account: params.get('account') || '' };
  }
  async function exportTemplate(id) {
    const item = await api('/job-templates/' + id);
    const base = 'simplehpc-' + safeFileName(item.name);
    downloadFile(base + '.json', JSON.stringify(item, null, 2) + '\n', 'application/json');
    downloadFile(base + '-run.sh', item.scriptTemplate.replace(/\r\n/g, '\n').replace(/\s*$/, '') + '\n', 'text/x-shellscript');
    App.toast('已导出前端 JSON 和后端执行脚本', 'success');
  }
  async function exportSelectedTemplates() {
    const ids = [...state.selected];
    if (!ids.length) return;
    const items = await Promise.all(ids.map(id => api('/job-templates/' + id)));
    const date = new Date().toISOString().slice(0, 10);
    downloadFile('simplehpc-template-bundle-' + date + '.json', JSON.stringify(items, null, 2) + '\n', 'application/json');
    App.toast('已导出 ' + items.length + ' 个模板', 'success');
  }
  async function load() {
    try {
      const [data, projectData] = await Promise.all([
        api('/job-templates'),
        api('/projects').catch(() => ({items: []}))
      ]);
      state.items = data.items || []; state.canManage = !!data.canManage;
      state.projects = projectData.items || [];
      $('#managerActions').hidden = !state.canManage || state.view !== 'manage';
      document.querySelectorAll('.manager-only').forEach(el => el.hidden = !state.canManage);
      const categories = [...new Set(state.items.map(item => item.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'zh-CN'));
      $('#templateCategory').innerHTML = '<option value="">全部分类</option>' + categories.map(value => `<option value="${esc(value)}">${esc(value)}</option>`).join('');
      render();
    } catch (error) { $('#templateContent').innerHTML = `<div class="tpl-empty tpl-error">数据未获取：${esc(error.message)}</div>`; }
  }
  function render() {
    if (state.view === 'requests') return renderRequests();
    const query = ($('#templateSearch').value || '').toLowerCase();
    const category = $('#templateCategory').value;
    const kind = $('#templateKind').value;
    const status = $('#templateStatus').value;
    const items = state.items.filter(x =>
      (!query || (x.name+' '+x.description+' '+x.category).toLowerCase().includes(query)) &&
      (!category || x.category === category) &&
      (!kind || x.kind === kind) &&
      (!status || x.status === status) &&
      (state.view === 'manage' || x.status === 'published')
    );
    if (!items.length) { $('#templateContent').innerHTML='<div class="tpl-empty">当前没有符合条件的真实模板数据</div>'; return; }
    if (state.view === 'manage' && state.layout === 'list') {
      $('#templateContent').innerHTML = templateTable(items);
      syncBatchExport();
      return;
    }
    $('#templateContent').innerHTML = `<div class="tpl-grid">${items.map(card).join('')}</div>`;
  }
  function card(item) {
    const manage = state.canManage && state.view === 'manage';
    const status = item.status === 'published' ? '<span class="pill pill-success">已发布</span>' : '<span class="pill pill-warn">草稿</span>';
    let actions = button('详情','detail',item.id);
    if (manage && item.status === 'published') actions += button('取消发布','unpublish',item.id)+button('导出','export',item.id);
    else if (manage) actions += button('设计','edit',item.id)+button('授权','grant',item.id)+button('发布','publish',item.id)+button('导出','export',item.id)+button('删除','delete',item.id,'btn-danger');
    else if (item.authorized) actions += button(item.kind==='novnc'?'创建桌面':item.kind==='webapp'?'启动应用':'使用模板','use',item.id,'btn-primary');
    else actions += button(item.requestStatus==='pending'?'申请审批中':'申请使用','request',item.id,item.requestStatus==='pending'?'btn-ghost':'btn-primary');
    return `<article class="tpl-card"><div class="tpl-card-head"><span class="tpl-kind">${esc(kinds[item.kind]||item.kind)}</span>${status}</div><h3>${esc(item.name)}</h3><p>${esc(item.description||'暂无说明')}</p><div class="tpl-meta"><span>v${item.version}</span><span>${esc(item.category||'未分类')}</span><span>${(item.formSchema||[]).length} 个参数</span></div><div class="tpl-card-actions">${actions}</div></article>`;
  }
  function managementActions(item) {
    let actions = button('详情','detail',item.id);
    if (item.status === 'published') return actions + button('取消发布','unpublish',item.id) + button('导出','export',item.id);
    return actions + button('设计','edit',item.id) + button('授权','grant',item.id) + button('发布','publish',item.id) + button('导出','export',item.id) + button('删除','delete',item.id,'btn-danger');
  }
  function templateTable(items) {
    return `<div class="table-wrap tpl-list-wrap"><table class="tpl-list-table"><thead><tr><th><input type="checkbox" id="templateSelectAll" aria-label="全选模板"></th><th>模板名称</th><th>类型</th><th>分类</th><th>版本</th><th>参数</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead><tbody>${items.map(item=>`<tr><td><input type="checkbox" data-template-select="${item.id}" ${state.selected.has(String(item.id))?'checked':''}></td><td><b>${esc(item.name)}</b><small>${esc(item.description||'暂无说明')}</small></td><td>${esc(kinds[item.kind]||item.kind)}</td><td>${esc(item.category||'未分类')}</td><td>v${item.version}</td><td>${(item.formSchema||[]).length}</td><td>${item.status==='published'?'<span class="pill pill-success">已发布</span>':'<span class="pill pill-warn">草稿</span>'}</td><td>${esc(item.updatedAt ? new Date(item.updatedAt).toLocaleString('zh-CN',{hour12:false}) : '数据未获取')}</td><td><div class="tpl-card-actions">${managementActions(item)}</div></td></tr>`).join('')}</tbody></table></div>`;
  }
  function syncBatchExport() {
    const boxes = [...document.querySelectorAll('[data-template-select]')];
    const all = $('#templateSelectAll');
    if (all) {
      all.checked = boxes.length > 0 && boxes.every(box => box.checked);
      all.indeterminate = boxes.some(box => box.checked) && !all.checked;
    }
    $('#batchExportTemplates').disabled = state.selected.size === 0;
  }
  function projectAccountSelect(projects) {
    const usable = (projects || []).filter(project => project.slurmAccount);
    const sorted = usable.slice().sort((a, b) => (b.currentUserDefaultProject ? 1 : 0) - (a.currentUserDefaultProject ? 1 : 0) || String(a.name).localeCompare(String(b.name), 'zh-CN'));
    const preferred = selectedProjectFromURL();
    if (!sorted.length) {
      return `<label class="wide">所属项目<select data-core="account" data-project-select disabled><option value="">暂无可用项目</option></select><small>请先在项目中心加入项目，或联系项目负责人授权。</small></label>`;
    }
    return `<label class="wide">所属项目<select data-core="account" data-project-select>
      ${sorted.map(project => {
        const matched = preferred.projectId ? String(project.id) === String(preferred.projectId) : (preferred.account ? String(project.slurmAccount) === preferred.account : project.currentUserDefaultProject);
        return `<option value="${esc(project.slurmAccount)}" data-project-id="${esc(project.id)}" ${matched ? 'selected' : ''}>${esc(project.name)}${project.currentUserDefaultProject ? '（默认）' : ''} · ${esc(project.slurmAccount)}</option>`;
      }).join('')}
    </select><small>系统会把所选项目写入 Slurm 脚本的 --account。</small></label>`;
  }
  function templateParameterForm(item, username, projects) {
    const fields = (item.formSchema || []).map(submitField).join('');
    const walltime = item.kind === 'novnc'
      ? '<label>预计运行时间<input data-core="walltime" value="24:00:00" pattern="[0-9]+:[0-5][0-9]:[0-5][0-9]"><small>默认 24 小时，格式：小时:分钟:秒</small></label>'
      : '';
    return `<div class="tpl-use-heading"><div><h3>作业参数</h3><p>按本次任务需要调整资源、输入文件和工作目录。</p></div></div>
      <div class="tpl-submit-grid">
        <label>作业名称<input data-core="jobName" value="${esc(safeJobName(item.name, item.id))}" pattern="[A-Za-z0-9._-]+"></label>
        ${projectAccountSelect(projects || [])}
        <label>分区<input data-core="partition" value="debug"></label>
        <label>节点数<input data-core="nodes" type="number" min="1" value="1"></label>
        <label>CPU 核数<input data-core="cpus" type="number" min="1" value="1"></label>
        <label>GPU 数量<input data-core="gpus" type="number" min="0" value="0"></label>
        <label>工作目录<input data-core="workdir" value="/data/home/${esc(username || 'user')}"></label>
        ${walltime}
        ${fields}
      </div>`;
  }
  async function detail(id) {
    const item = await api('/job-templates/' + id);
    const modal = App.modal({
      title:'模板预览：' + item.name,
      width:'min(94vw, 1120px)',
      showFooter:false,
      content:`<div class="tpl-template-preview">
        <header class="tpl-template-preview-head">
          <div><span>${esc(kinds[item.kind] || item.kind)}</span><b>${esc(item.name)}</b><p>${esc(item.description || '暂无模板说明')}</p></div>
          <div><span class="pill ${item.status === 'published' ? 'pill-success' : 'pill-warn'}">${item.status === 'published' ? '已发布' : '草稿'}</span><small>v${esc(item.version)}</small></div>
        </header>
        <section class="tpl-template-preview-form">${templateParameterForm(item, user.username || 'user', state.projects)}</section>
      </div>`
    });
    modal.el.classList.add('tpl-template-preview-modal');
    modal.el.querySelectorAll('.tpl-template-preview input, .tpl-template-preview textarea').forEach(control => {
      if (control.type === 'checkbox' || control.type === 'range') control.disabled = true;
      else control.readOnly = true;
      control.tabIndex = -1;
    });
    modal.el.querySelectorAll('.tpl-template-preview select').forEach(control => {
      control.disabled = true;
      control.tabIndex = -1;
    });
  }
  function makeField(type) {
    const n = state.fields.filter(field => !displayTypes.has(field.type)).length + 1;
    const field = {
      id:'field_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6),
      type:type,
      label:fieldTypes[type] || '新字段',
      variable:displayTypes.has(type) ? '' : 'PARAM_' + n,
      required:false,
      default:'',
      placeholder:'',
      help:''
    };
    if (['select','radio','multiselect'].includes(type)) {
      field.options = [{label:'选项一',value:'option_1'},{label:'选项二',value:'option_2'}];
    }
    if (type === 'number' || type === 'cpu' || type === 'gpu' || type === 'slider') {
      field.min = 0;
      field.max = type === 'gpu' ? 8 : 128;
    }
    return field;
  }
  function addField(type, root) {
    state.fields.push(makeField(type));
    state.selectedField = state.fields.length - 1;
    renderBuilder(root);
  }
  function fieldControlPreview(field) {
    if (field.type === 'section') return `<h3>${esc(field.label || '分组标题')}</h3>`;
    if (field.type === 'divider') return '<hr>';
    if (field.type === 'hint') return `<div class="tpl-canvas-hint">${esc(field.label || '提示文字')}</div>`;
    if (field.type === 'textarea') return `<textarea disabled placeholder="${esc(field.placeholder || '多行文本')}"></textarea>`;
    if (['select','partition','radio','multiselect'].includes(field.type)) return `<select disabled><option>${esc(field.placeholder || '请选择')}</option></select>`;
    if (field.type === 'checkbox') return '<span class="tpl-switch-preview"><i></i></span>';
    if (field.type === 'slider') return '<input type="range" disabled>';
    const type = ['number','cpu','gpu'].includes(field.type) ? 'number' : field.type === 'date' ? 'date' : field.type === 'time' ? 'time' : 'text';
    return `<input type="${type}" disabled placeholder="${esc(field.placeholder || fieldTypes[field.type] || '')}">`;
  }
  function fieldCanvas(field, index) {
    const selected = index === state.selectedField ? ' selected' : '';
    const variable = displayTypes.has(field.type) ? '' : `<code>\${${esc(field.variable || '未设置变量')}}</code>`;
    return `<div class="tpl-canvas-field${selected}" draggable="true" data-field-index="${index}">
      <button type="button" class="tpl-canvas-drag" title="拖拽排序">⋮⋮</button>
      <div class="tpl-canvas-content">
        ${displayTypes.has(field.type) ? fieldControlPreview(field) : `<div class="tpl-canvas-label"><b>${esc(field.label || '未命名字段')}${field.required ? ' *' : ''}</b>${variable}</div>${fieldControlPreview(field)}${field.help ? `<small>${esc(field.help)}</small>` : ''}`}
      </div>
      <button type="button" class="btn-icon tpl-canvas-remove" data-remove="${index}" title="删除组件">×</button>
    </div>`;
  }
  function optionsToText(field) {
    return (field.options || []).map(option => `${option.label}=${option.value}`).join('\n');
  }
  function textToOptions(value) {
    return String(value || '').split(/\n/).map(line => line.trim()).filter(Boolean).map(line => {
      const parts = line.split('=');
      return {label:(parts.shift() || '').trim(), value:(parts.join('=') || line).trim()};
    });
  }
  function renderProperties(root) {
    const panel = $('.tpl-property-panel', root);
    const field = state.fields[state.selectedField];
    if (!field) {
      panel.innerHTML = '<div class="tpl-property-empty">在画布中选择一个组件后，可在这里编辑名称、变量和内容。</div>';
      return;
    }
    const isDisplay = displayTypes.has(field.type);
    const hasOptions = ['select','radio','multiselect'].includes(field.type);
    const hasRange = ['number','cpu','gpu','slider'].includes(field.type);
    panel.innerHTML = `<div class="tpl-property-heading"><span>${esc(fieldTypes[field.type] || field.type)}</span><code>#${esc(field.id)}</code></div>
      <label>组件类型<select data-field-prop="type">${Object.entries(fieldTypes).map(([value,label])=>`<option value="${value}" ${field.type===value?'selected':''}>${label}</option>`).join('')}</select></label>
      <label>${isDisplay ? '显示内容' : '字段名称'}<input data-field-prop="label" value="${esc(field.label)}"></label>
      ${isDisplay ? '' : `<label>变量名称<input data-field-prop="variable" value="${esc(field.variable)}" placeholder="INPUT_FILE"><small>保存为 Slurm 脚本可调用的环境变量</small></label>
      <label>默认值<input data-field-prop="default" value="${esc(field.default == null ? '' : field.default)}"></label>
      <label>占位提示<input data-field-prop="placeholder" value="${esc(field.placeholder || '')}"></label>
      <label>帮助说明<input data-field-prop="help" value="${esc(field.help || '')}"></label>
      <label class="tpl-property-check"><input type="checkbox" data-field-prop="required" ${field.required?'checked':''}> 必填字段</label>`}
      ${hasOptions ? `<label>选项配置<textarea data-field-prop="optionsText" rows="5" placeholder="显示名称=实际值">${esc(optionsToText(field))}</textarea><small>每行一个选项，格式：名称=值</small></label>` : ''}
      ${hasRange ? `<div class="tpl-property-range"><label>最小值<input type="number" data-field-prop="min" value="${esc(field.min == null ? '' : field.min)}"></label><label>最大值<input type="number" data-field-prop="max" value="${esc(field.max == null ? '' : field.max)}"></label></div>` : ''}`;
    panel.querySelectorAll('[data-field-prop]').forEach(control => {
      control.addEventListener(control.type === 'checkbox' || control.tagName === 'SELECT' ? 'change' : 'input', () => {
        const current = state.fields[state.selectedField];
        const prop = control.dataset.fieldProp;
        if (prop === 'optionsText') current.options = textToOptions(control.value);
        else if (control.type === 'checkbox') current[prop] = control.checked;
        else if (['min','max'].includes(prop)) current[prop] = control.value === '' ? null : Number(control.value);
        else current[prop] = control.value;
        if (prop === 'type') {
          if (displayTypes.has(current.type)) current.variable = '';
          else if (!current.variable) current.variable = 'PARAM_' + (state.selectedField + 1);
          renderBuilder(root);
        } else {
          renderCanvas(root);
        }
      });
    });
  }
  function renderCanvas(root) {
    const list = $('.tpl-builder-list', root);
    if (!list) return;
    list.innerHTML = state.fields.map(fieldCanvas).join('') || '<div class="tpl-builder-empty"><b>将左侧组件拖到这里</b><span>也可以点击组件快速添加</span></div>';
    const count = $('.tpl-canvas-toolbar small', root);
    if (count) count.textContent = `${state.fields.length} 个组件`;
    list.querySelectorAll('[data-field-index]').forEach(card => {
      card.onclick = event => {
        if (event.target.closest('[data-remove]')) return;
        state.selectedField = Number(card.dataset.fieldIndex);
        renderCanvas(root);
        renderProperties(root);
      };
      card.ondragstart = event => {
        event.dataTransfer.setData('text/x-template-index', card.dataset.fieldIndex);
        event.dataTransfer.effectAllowed = 'move';
      };
      card.ondragover = event => event.preventDefault();
      card.ondrop = event => {
        event.preventDefault();
        const from = Number(event.dataTransfer.getData('text/x-template-index'));
        if (!Number.isNaN(from)) {
          const to = Number(card.dataset.fieldIndex);
          const moved = state.fields.splice(from, 1)[0];
          state.fields.splice(to, 0, moved);
          state.selectedField = to;
          renderBuilder(root);
        }
      };
    });
    list.querySelectorAll('[data-remove]').forEach(button => {
      button.onclick = () => {
        const index = Number(button.dataset.remove);
        state.fields.splice(index, 1);
        state.selectedField = Math.min(state.selectedField, state.fields.length - 1);
        renderBuilder(root);
      };
    });
  }
  function variableButtons() {
    const componentVariables = state.fields.filter(field => !displayTypes.has(field.type) && field.variable);
    const runtime = [
      ['SLURM_JOB_ID','Slurm 作业 ID'],['SLURM_JOB_NODELIST','运行节点'],
      ['SIMPLEHPC_CALLBACK_URL','服务回调地址'],['SIMPLEHPC_ACCESS_TOKEN','访问令牌']
    ];
    return `<section><h4>组件变量</h4>${componentVariables.map(field=>`<button type="button" data-insert-var="${esc(field.variable)}"><code>\${${esc(field.variable)}}</code><span>${esc(field.label)}</span></button>`).join('') || '<p>请先在 UI 设计中添加输入组件</p>'}</section>
      <section><h4>运行时变量</h4>${runtime.map(([variable,label])=>`<button type="button" data-insert-var="${variable}"><code>\${${variable}}</code><span>${label}</span></button>`).join('')}</section>`;
  }
  function syncVim(root) {
    const editor = $('#teScript', root);
    const lines = $('.tpl-vim-lines', root);
    const position = $('.tpl-vim-position', root);
    if (!editor || !lines) return;
    const update = () => {
      const count = Math.max(1, editor.value.split('\n').length);
      lines.textContent = Array.from({length:count}, (_, index) => index + 1).join('\n');
      const before = editor.value.slice(0, editor.selectionStart).split('\n');
      position.textContent = `Ln ${before.length}, Col ${before[before.length - 1].length + 1}`;
    };
    editor.oninput = update;
    editor.onkeyup = update;
    editor.onclick = update;
    editor.onscroll = () => { lines.scrollTop = editor.scrollTop; };
    editor.onkeydown = event => {
      if (event.key !== 'Tab') return;
      event.preventDefault();
      const start = editor.selectionStart;
      editor.setRangeText('  ', start, editor.selectionEnd, 'end');
      update();
    };
    update();
  }
  function insertVariable(root, variable) {
    const editor = $('#teScript', root);
    const value = '${' + variable + '}';
    const start = editor.selectionStart;
    editor.setRangeText(value, start, editor.selectionEnd, 'end');
    editor.focus();
    editor.dispatchEvent(new Event('input'));
  }
  function renderDesignerTab(root) {
    root.querySelectorAll('[data-designer-tab]').forEach(button => {
      const active = button.dataset.designerTab === state.designerTab;
      button.classList.toggle('active', active);
      button.setAttribute('aria-selected', active ? 'true' : 'false');
    });
    $('.tpl-ui-workspace', root).hidden = state.designerTab !== 'ui';
    $('.tpl-script-workspace', root).hidden = state.designerTab !== 'script';
    if (state.designerTab === 'script') {
      $('.tpl-variable-library', root).innerHTML = variableButtons();
      root.querySelectorAll('[data-insert-var]').forEach(button => button.onclick = () => insertVariable(root, button.dataset.insertVar));
      syncVim(root);
    }
  }
  function renderBuilder(root) {
    renderCanvas(root);
    renderProperties(root);
    if (state.designerTab === 'script') renderDesignerTab(root);
  }
  async function edit(id) {
    const item=id?await api('/job-templates/'+id):{name:'',description:'',category:'',kind:'batch',status:'draft',formSchema:[],scriptTemplate:'echo "请编辑执行命令"',runtime:{}};
    state.fields=JSON.parse(JSON.stringify(item.formSchema||[]));
    state.selectedField=state.fields.length ? 0 : -1;
    state.designerTab='ui';
    const modal=App.modal({title:id?'编辑作业模板':'新建作业模板',width:'min(96vw, 1520px)',content:`<div class="tpl-editor tpl-editor-v2">
      <div class="tpl-editor-basics"><label>模板名称<input id="teName" value="${esc(item.name)}"></label><label>作业类型<select id="teKind">${Object.entries(kinds).map(([v,l])=>`<option value="${v}" ${item.kind===v?'selected':''}>${l}</option>`).join('')}</select></label><label>场景分类<input id="teCategory" value="${esc(item.category)}" placeholder="如 AI训练、科学计算"></label><label>模板说明<textarea id="teDescription">${esc(item.description)}</textarea></label></div>
      <div class="tpl-designer-tabs" role="tablist" aria-label="作业模板设计步骤"><button type="button" class="active" role="tab" aria-selected="true" data-designer-tab="ui"><b>1</b> 前端 UI 设计</button><button type="button" role="tab" aria-selected="false" data-designer-tab="script"><b>2</b> Slurm 脚本设计</button></div>
      <div class="tpl-ui-workspace">
        <aside class="tpl-component-library"><div class="tpl-panel-title">组件库 <small>拖拽或点击添加</small></div>${fieldGroups.map(group=>`<section><h4>${group.label}</h4><div>${group.types.map(type=>`<button type="button" draggable="true" data-add="${type}"><span>＋</span>${fieldTypes[type]}</button>`).join('')}</div></section>`).join('')}</aside>
        <main class="tpl-design-canvas"><div class="tpl-canvas-toolbar"><span>表单设计画布</span><small>${state.fields.length} 个组件</small></div><div class="tpl-builder-list"></div></main>
        <aside class="tpl-property-sidebar"><div class="tpl-panel-title">组件设置</div><div class="tpl-property-panel"></div></aside>
      </div>
      <div class="tpl-script-workspace" hidden>
        <aside><div class="tpl-panel-title">可用变量 <small>点击插入脚本</small></div><div class="tpl-variable-library"></div></aside>
        <main><div class="tpl-vim"><div class="tpl-vim-title"><span>job-template.sh</span><small>VIM · UTF-8 · Shell</small></div><div class="tpl-vim-body"><pre class="tpl-vim-lines"></pre><textarea id="teScript" spellcheck="false" wrap="off">${esc(item.scriptTemplate)}</textarea></div><div class="tpl-vim-status"><b>-- INSERT --</b><span>资源相关 #SBATCH 由系统根据提交表单自动生成</span><span class="tpl-vim-position">Ln 1, Col 1</span></div></div></main>
      </div>
    </div>`,onSubmit:async()=>{const payload={name:$('#teName',modal.el).value,description:$('#teDescription',modal.el).value,category:$('#teCategory',modal.el).value,kind:$('#teKind',modal.el).value,status:item.status||'draft',formSchema:state.fields,scriptTemplate:$('#teScript',modal.el).value,runtime:item.runtime||{}};await api('/job-templates'+(id?'/'+id:''),{method:id?'PUT':'POST',body:JSON.stringify(payload)});App.toast('前端表单与 Slurm 脚本已保存','success');await load();}});
    const root=modal.el;
    root.classList.add('tpl-editor-modal');
    root.querySelectorAll('[data-designer-tab]').forEach(button=>button.onclick=()=>{state.designerTab=button.dataset.designerTab;renderDesignerTab(root);});
    root.querySelectorAll('[data-add]').forEach(button=>{
      button.onclick=()=>addField(button.dataset.add,root);
      button.ondragstart=event=>{event.dataTransfer.setData('text/x-template-field',button.dataset.add);event.dataTransfer.effectAllowed='copy';};
    });
    const canvas=$('.tpl-builder-list',root);
    canvas.ondragover=event=>event.preventDefault();
    canvas.ondrop=event=>{const type=event.dataTransfer.getData('text/x-template-field');if(type){event.preventDefault();addField(type,root);}};
    renderBuilder(root);
    renderDesignerTab(root);
  }
  function submitField(field) {
    if (field.type === 'section') return `<h3 class="tpl-submit-section">${esc(field.label)}</h3>`;
    if (field.type === 'divider') return '<hr class="tpl-submit-divider">';
    if (field.type === 'hint') return `<div class="tpl-submit-hint">${esc(field.label)}</div>`;
    let input = '';
    if (field.type === 'textarea') input = `<textarea data-field="${field.id}" placeholder="${esc(field.placeholder || '')}">${esc(field.default || '')}</textarea>`;
    else if (['select','partition','radio','multiselect'].includes(field.type)) input = `<select data-field="${field.id}" ${field.type==='multiselect'?'multiple':''}>${(field.options||[]).map(option=>`<option value="${esc(option.value)}">${esc(option.label)}</option>`).join('')}</select>`;
    else if (field.type === 'checkbox') input = `<input data-field="${field.id}" type="checkbox" ${field.default?'checked':''}>`;
    else if (field.type === 'slider') input = `<input data-field="${field.id}" type="range" min="${esc(field.min == null ? 0 : field.min)}" max="${esc(field.max == null ? 100 : field.max)}" value="${esc(field.default == null ? 0 : field.default)}">`;
    else {
      const type = ['number','cpu','gpu'].includes(field.type) ? 'number' : field.type === 'date' ? 'date' : field.type === 'time' ? 'time' : field.type === 'file' ? 'text' : 'text';
      input = `<input data-field="${field.id}" type="${type}" value="${esc(field.default || '')}" placeholder="${esc(field.placeholder || '')}" ${field.min==null?'':`min="${esc(field.min)}"`} ${field.max==null?'':`max="${esc(field.max)}"`}>`;
    }
    return `<label>${esc(field.label)}${field.required?' *':''}${input}${field.help?`<small>${esc(field.help)}</small>`:''}</label>`;
  }
  async function useTemplate(id) {
    const [item, projectData]=await Promise.all([
      api('/job-templates/'+id),
      api('/projects').catch(() => ({items: state.projects || []}))
    ]);
    state.projects = projectData.items || state.projects || [];
    const username=user.username||'user';
    const modal=App.modal({
      title:'使用模板：'+item.name,
      width:'min(96vw, 1480px)',
      confirmText:'提交作业',
      errorPrefix:'提交失败',
      content:`<div class="tpl-use-layout">
        <section class="tpl-use-form">${templateParameterForm(item, username, state.projects)}</section>
        <section class="tpl-use-preview"><div class="tpl-use-preview-head"><div><h3>预览最终脚本</h3><p id="previewState">根据左侧当前参数生成</p></div><button type="button" class="btn btn-ghost" id="previewScript">刷新脚本内容</button></div><pre id="scriptPreview">正在生成脚本...</pre></section>
      </div>`,
      onSubmit:async()=>{
        const run=await api('/job-templates/'+id+'/submit',{method:'POST',body:JSON.stringify(collect(modal.el))});
        App.toast((item.kind==='novnc'?'VNC 桌面作业 ':'作业 ')+run.slurmJobId+' 已提交','success',5000);
      }
    });
    modal.el.classList.add('tpl-use-modal');
    const refresh=async()=>{
      const stateLabel=$('#previewState',modal.el);
      stateLabel.textContent='正在生成...';
      const data=await api('/job-templates/'+id+'/preview',{method:'POST',body:JSON.stringify(collect(modal.el))});
      $('#scriptPreview',modal.el).textContent=data.script;
      stateLabel.textContent='已按当前参数更新';
    };
    $('#previewScript',modal.el).onclick=()=>refresh().catch(error=>App.toast(error.message,'danger',5000));
    modal.el.querySelectorAll('[data-core],[data-field]').forEach(control=>control.addEventListener('input',()=>{$('#previewState',modal.el).textContent='参数已变化，请刷新脚本';}));
    await refresh();
  }
  function collect(root){const values={};root.querySelectorAll('[data-core]').forEach(x=>{values[x.dataset.core]=x.type==='number'?Number(x.value):x.value;if(x.matches('[data-project-select]')){const option=x.selectedOptions&&x.selectedOptions[0];values.projectId=option&&option.dataset.projectId?Number(option.dataset.projectId):0;}});root.querySelectorAll('[data-field]').forEach(x=>{if(x.type==='checkbox')values[x.dataset.field]=x.checked;else if(x.multiple)values[x.dataset.field]=Array.from(x.selectedOptions).map(o=>o.value).join(',');else values[x.dataset.field]=x.type==='number'?Number(x.value):x.value;});return values;}
  async function request(id){const modal=App.modal({title:'申请使用模板',content:'<label class="tpl-block-label">申请理由<textarea id="requestReason" placeholder="请说明研究场景和使用目的"></textarea></label>',onSubmit:async()=>{await api('/job-templates/'+id+'/access-requests',{method:'POST',body:JSON.stringify({reason:$('#requestReason',modal.el).value})});App.toast('授权申请已提交','success');load();}});}
  async function grantTemplate(id) {
    const [item, usersData, teamsData] = await Promise.all([
      api('/job-templates/' + id),
      api('/account/users'),
      api('/account/teams')
    ]);
    const grants = item.grants || [];
    const allGranted = grants.some(grant => grant.targetType === 'all');
    const selectedUsers = new Set(grants.filter(grant => grant.targetType === 'user').map(grant => String(grant.targetId)));
    const selectedTeams = new Set(grants.filter(grant => grant.targetType === 'team').map(grant => String(grant.targetId)));
    const users = usersData.items || usersData.users || [];
    const teams = teamsData.items || teamsData.teams || [];
    const modal = App.modal({
      title:'模板授权：' + item.name,
      width:'900px',
      content:`<div class="tpl-grant-editor">
        <label class="tpl-grant-all"><input type="checkbox" id="grantAll" ${allGranted?'checked':''}> <span><b>全部用户</b><small>所有有效用户无需单独申请即可使用</small></span></label>
        <div class="tpl-grant-columns">
          <section><h3>指定用户 <span>${users.length}</span></h3><input class="tpl-grant-search" data-search-list="grantUsers" placeholder="搜索账号或姓名"><div id="grantUsers" class="tpl-grant-list">${users.map(entry=>{const value=String(entry.username||entry.account||entry.id||'');return `<label data-search-text="${esc((value+' '+(entry.name||entry.displayName||'')).toLowerCase())}"><input type="checkbox" data-grant-user="${esc(value)}" ${selectedUsers.has(value)?'checked':''}><span><b>${esc(entry.name||entry.displayName||value)}</b><small>${esc(value)}</small></span></label>`;}).join('')||'<p>暂无用户数据</p>'}</div></section>
          <section><h3>指定团队 <span>${teams.length}</span></h3><input class="tpl-grant-search" data-search-list="grantTeams" placeholder="搜索团队或组名"><div id="grantTeams" class="tpl-grant-list">${teams.map(entry=>{const value=String(entry.groupName||entry.name||entry.id||'');return `<label data-search-text="${esc((value+' '+(entry.teamName||entry.displayName||'')).toLowerCase())}"><input type="checkbox" data-grant-team="${esc(value)}" ${selectedTeams.has(value)?'checked':''}><span><b>${esc(entry.teamName||entry.displayName||value)}</b><small>${esc(value)}</small></span></label>`;}).join('')||'<p>暂无团队数据</p>'}</div></section>
        </div>
      </div>`,
      onSubmit:async()=>{
        const all=$('#grantAll',modal.el).checked;
        const next=all ? [{targetType:'all',targetId:'*'}] : [
          ...[...modal.el.querySelectorAll('[data-grant-user]:checked')].map(input=>({targetType:'user',targetId:input.dataset.grantUser})),
          ...[...modal.el.querySelectorAll('[data-grant-team]:checked')].map(input=>({targetType:'team',targetId:input.dataset.grantTeam}))
        ];
        await api('/job-templates/'+id+'/grants',{method:'PUT',body:JSON.stringify({grants:next})});
        App.toast('模板授权已更新','success');
        await load();
      }
    });
    modal.el.querySelectorAll('[data-search-list]').forEach(input=>input.oninput=()=>{
      const query=input.value.trim().toLowerCase();
      modal.el.querySelectorAll('#'+input.dataset.searchList+' [data-search-text]').forEach(row=>row.hidden=!!query&&!row.dataset.searchText.includes(query));
    });
  }
  async function renderRequests(){try{const data=await api('/job-template-access-requests');$('#templateContent').innerHTML=`<div class="tpl-request-list">${(data.items||[]).map(r=>`<div><span><b>${esc(r.templateName)}</b><small>${esc(r.username)}：${esc(r.reason||'未填写理由')}</small></span><span class="pill">${esc(r.status)}</span>${r.status==='pending'?button('批准','approve',r.id,'btn-primary')+button('拒绝','reject',r.id):''}</div>`).join('')||'<div class="tpl-empty">暂无授权申请</div>'}</div>`;}catch(e){$('#templateContent').innerHTML='<div class="tpl-empty tpl-error">'+esc(e.message)+'</div>';}}
  document.addEventListener('click',async e=>{
    const layout=e.target.closest('[data-layout]');
    if(layout){
      state.layout=layout.dataset.layout;
      document.querySelectorAll('[data-layout]').forEach(button=>{button.classList.toggle('btn-primary',button===layout);button.classList.toggle('btn-ghost',button!==layout);});
      $('#batchExportTemplates').hidden=state.view!=='manage'||state.layout!=='list';
      render();
      return;
    }
    const btn=e.target.closest('[data-action]');
    if(!btn)return;
    try{
      const id=btn.dataset.id,a=btn.dataset.action;
      if(a==='detail')await detail(id);
      if(a==='edit')await edit(id);
      if(a==='grant')await grantTemplate(id);
      if(a==='use')await useTemplate(id);
      if(a==='request')await request(id);
      if(a==='publish'){await api('/job-templates/'+id+'/publish',{method:'POST'});App.toast('模板已发布到应用广场','success');await load();}
      if(a==='unpublish'){App.confirm('取消发布后，用户端将立即提示“模板维护中，请稍后！”。确认继续？',{onConfirm:async()=>{await api('/job-templates/'+id+'/unpublish',{method:'POST'});App.toast('模板已进入维护状态','success');await load();}});}
      if(a==='delete'){App.confirm('确认删除该模板及其授权记录？',{danger:true,onConfirm:async()=>{await api('/job-templates/'+id,{method:'DELETE'});load();}});}
      if(a==='export')await exportTemplate(id);
      if(a==='approve'||a==='reject'){await api('/job-template-access-requests/'+id+'/'+a,{method:'POST'});renderRequests();}
    }catch(error){App.toast(error.message,'danger',5000);}
  });
  document.addEventListener('change',event=>{
    if(event.target.matches('[data-template-select]')){
      const id=event.target.dataset.templateSelect;
      if(event.target.checked)state.selected.add(id);else state.selected.delete(id);
      syncBatchExport();
    }
    if(event.target.id==='templateSelectAll'){
      document.querySelectorAll('[data-template-select]').forEach(box=>{box.checked=event.target.checked;if(box.checked)state.selected.add(box.dataset.templateSelect);else state.selected.delete(box.dataset.templateSelect);});
      syncBatchExport();
    }
  });
  document.querySelectorAll('.tpl-tabs button').forEach(btn=>btn.onclick=()=>{
    document.querySelectorAll('.tpl-tabs button').forEach(x=>x.classList.remove('active'));
    btn.classList.add('active');
    state.view=btn.dataset.view;
    state.selected.clear();
    const manage=state.view==='manage';
    const requests=state.view==='requests';
    $('#templatePageTitle').textContent=manage?'模板管理':requests?'授权审批':'应用广场';
    $('#templatePageDescription').textContent=manage?'维护草稿、授权范围和发布状态；已发布模板需先取消发布后才能设计。':requests?'审批用户对应用模板的使用申请。':'选择已发布并获得授权的应用模板，填写参数后直接提交作业。';
    $('#managerActions').hidden=!state.canManage||!manage;
    $('#templateFilters').hidden=requests;
    $('#templateStatus').hidden=!manage;
    $('#templateViewSwitch').hidden=!manage;
    $('#batchExportTemplates').hidden=!manage||state.layout!=='list';
    render();
  });
  $('#templateSearch').oninput=render;
  ['templateCategory','templateKind','templateStatus'].forEach(id=>$('#'+id).onchange=render);
  $('#newTemplate').onclick=()=>edit(0);
  $('#batchExportTemplates').onclick=()=>exportSelectedTemplates().catch(error=>App.toast(error.message,'danger',5000));
  $('#importTemplate').onchange=async e=>{
    try{
      const parsed=JSON.parse(await e.target.files[0].text());
      const items=Array.isArray(parsed)?parsed:[parsed];
      for(const item of items)await api('/job-templates/import',{method:'POST',body:JSON.stringify(item)});
      App.toast('已导入 '+items.length+' 个模板为草稿','success');
      await load();
    }catch(error){App.toast(error.message,'danger',5000);}
    finally{e.target.value='';}
  };
  load();
})();
