(function () {
  'use strict';
  const token = localStorage.getItem('simplehpc_token') || '';
  const user = JSON.parse(localStorage.getItem('simplehpc_user') || '{}');
  const templateViews = new Set(['library', 'manage', 'requests']);
  const state = {
    items: [],
    canManage: false,
    view: initialTemplateView(),
    layout: 'grid',
    selected: new Set(),
    fields: [],
    selectedField: -1,
    selectedFieldId: '',
    designerTab: 'ui',
    projects: [],
    dynamicOptions: { loaded: false, partitions: [], qos: [], associations: [], errors: {} }
  };
  const kinds = { batch: '非交互式作业', novnc: 'noVNC 桌面', webapp: 'Web 应用转发' };
  const fieldTypes = {
    section:'分组标题', divider:'分隔线', hint:'提示文字',
    text:'单行输入', textarea:'多行输入', number:'数字输入', select:'下拉选项',
    radio:'单选项', multiselect:'多选项', checkbox:'开关', date:'日期', time:'时间', slider:'滑块',
    'slurm-job-name':'作业名称', 'slurm-partition':'队列名称', 'slurm-account':'项目名称（Account）',
    'slurm-nodes':'节点数', 'slurm-ntasks':'任务总数', 'slurm-ntasks-per-node':'每节点任务数',
    'slurm-cpus-per-task':'每任务 CPU 核数', 'slurm-gpus':'GPU 总卡数', 'slurm-gpus-per-node':'每节点 GPU 卡数',
    'slurm-mem':'内存大小', 'slurm-time':'运行时长', 'slurm-mail-user':'通知邮箱', 'slurm-mail-type':'邮件事件',
    'slurm-output':'标准输出', 'slurm-error':'错误输出', 'slurm-workdir':'工作目录',
    'slurm-constraint':'节点约束', 'slurm-qos':'QOS', 'slurm-array':'作业数组', 'slurm-exclusive':'独占节点',
    'slurm-custom':'高级 Slurm 参数', 'app-file':'应用输入文件', 'app-directory':'服务器目录', 'custom':'自定义变量',
    partition:'队列选择（旧）', cpu:'CPU 数量（旧）', gpu:'GPU 数量（旧）', file:'输入文件（旧）', directory:'目录选择（旧）'
  };
  const componentCatalog = {
    section:{group:'容器与展示', control:'display', label:'分组标题'},
    divider:{group:'容器与展示', control:'display', label:'分隔线'},
    hint:{group:'容器与展示', control:'display', label:'提示文字'},
    'slurm-job-name':{group:'Slurm 资源申请', control:'text', label:'作业名称', variable:'SLURM_JOB_NAME_UI', slurmOption:'--job-name', required:true, placeholder:'如 fluent-cfd-001', help:'对应 sbatch --job-name，用于队列和作业列表展示。'},
    'slurm-partition':{group:'Slurm 资源申请', control:'select', label:'队列名称', variable:'SLURM_PARTITION_UI', slurmOption:'--partition', required:true, dataSource:'slurm.partitions', help:'对应 sbatch --partition，提交时按当前用户和 Account 可用队列动态加载。'},
    'slurm-account':{group:'Slurm 资源申请', control:'project', label:'项目名称（Account）', variable:'SLURM_ACCOUNT_UI', slurmOption:'--account', required:true, dataSource:'projects.accounts', help:'对应 sbatch --account，用户只能选择自己参与或被授权的项目。'},
    'slurm-nodes':{group:'Slurm 资源申请', control:'number', label:'节点数', variable:'SLURM_NODES_UI', slurmOption:'--nodes', required:true, default:'1', min:1, max:1024, help:'对应 sbatch --nodes。'},
    'slurm-ntasks':{group:'Slurm 资源申请', control:'number', label:'任务总数', variable:'SLURM_NTASKS_UI', slurmOption:'--ntasks', default:'1', min:1, max:100000},
    'slurm-ntasks-per-node':{group:'Slurm 资源申请', control:'number', label:'每节点任务数', variable:'SLURM_TASKS_PER_NODE_UI', slurmOption:'--ntasks-per-node', min:1, max:4096},
    'slurm-cpus-per-task':{group:'Slurm 资源申请', control:'number', label:'每任务 CPU 核数', variable:'SLURM_CPUS_PER_TASK_UI', slurmOption:'--cpus-per-task', required:true, default:'1', min:1, max:4096, help:'常用于表达每个计算进程需要的 CPU 核数。'},
    'slurm-gpus':{group:'Slurm 资源申请', control:'number', label:'GPU 总卡数', variable:'SLURM_GPUS_UI', slurmOption:'--gpus', default:'0', min:0, max:128},
    'slurm-gpus-per-node':{group:'Slurm 资源申请', control:'number', label:'每节点 GPU 卡数', variable:'SLURM_GPUS_PER_NODE_UI', slurmOption:'--gpus-per-node', default:'0', min:0, max:16},
    'slurm-mem':{group:'Slurm 资源申请', control:'text', label:'内存大小', variable:'SLURM_MEM_UI', slurmOption:'--mem', placeholder:'如 64G 或 128000M'},
    'slurm-time':{group:'Slurm 资源申请', control:'text', label:'运行时长', variable:'SLURM_TIME_UI', slurmOption:'--time', placeholder:'如 02:00:00 或 2-00:00:00'},
    'slurm-mail-user':{group:'Slurm 资源申请', control:'text', label:'通知邮箱', variable:'SLURM_MAIL_USER_UI', slurmOption:'--mail-user', placeholder:'user@example.com'},
    'slurm-mail-type':{group:'Slurm 资源申请', control:'multiselect', label:'邮件事件', variable:'SLURM_MAIL_TYPE_UI', slurmOption:'--mail-type', options:[{label:'开始',value:'BEGIN'},{label:'结束',value:'END'},{label:'失败',value:'FAIL'},{label:'全部',value:'ALL'}]},
    'slurm-output':{group:'Slurm 资源申请', control:'text', label:'标准输出', variable:'SLURM_OUTPUT_UI', slurmOption:'--output', default:'slurm-%j.out'},
    'slurm-error':{group:'Slurm 资源申请', control:'text', label:'错误输出', variable:'SLURM_ERROR_UI', slurmOption:'--error', default:'slurm-%j.err'},
    'slurm-workdir':{group:'Slurm 资源申请', control:'directory', label:'工作目录', variable:'SLURM_WORKDIR_UI', slurmOption:'--chdir', placeholder:'/data/home/user/work'},
    'slurm-constraint':{group:'Slurm 资源申请', control:'text', label:'节点约束', variable:'SLURM_CONSTRAINT_UI', slurmOption:'--constraint', placeholder:'如 gpu 或 intel'},
    'slurm-qos':{group:'Slurm 资源申请', control:'select', label:'QOS', variable:'SLURM_QOS_UI', slurmOption:'--qos', dataSource:'slurm.qos', help:'对应 sbatch --qos，按 Slurm QOS 配置动态加载；为空则不生成该参数。'},
    'slurm-array':{group:'Slurm 资源申请', control:'text', label:'作业数组', variable:'SLURM_ARRAY_UI', slurmOption:'--array', placeholder:'如 0-99%10'},
    'slurm-exclusive':{group:'Slurm 资源申请', control:'checkbox', label:'独占节点', variable:'SLURM_EXCLUSIVE_UI', slurmOption:'--exclusive', help:'勾选后生成 #SBATCH --exclusive。'},
    'slurm-custom':{group:'Slurm 资源申请', control:'text', label:'高级 Slurm 参数', variable:'SLURM_CUSTOM_UI', slurmOption:'--comment', placeholder:'参数值', help:'可在右侧修改 Slurm 参数名，例如 --reservation、--licenses。'},
    'app-file':{group:'应用输入', control:'file', label:'输入文件', variable:'INPUT_FILE', required:false, placeholder:'/data/home/user/input.dat', help:'可填写服务器文件路径，也可在提交时上传到指定目录。'},
    'app-directory':{group:'应用输入', control:'directory', label:'服务器目录', variable:'INPUT_DIR', placeholder:'/data/home/user/work'},
    custom:{group:'应用输入', control:'text', label:'自定义变量', variable:'CUSTOM_PARAM', placeholder:'自定义参数值'},
    partition:{group:'兼容旧字段', control:'select', label:'队列选择', variable:'PARTITION'},
    cpu:{group:'兼容旧字段', control:'number', label:'CPU 数量', variable:'CPU_COUNT', min:1, max:4096},
    gpu:{group:'兼容旧字段', control:'number', label:'GPU 数量', variable:'GPU_COUNT', min:0, max:128},
    file:{group:'兼容旧字段', control:'file', label:'输入文件', variable:'INPUT_FILE'},
    directory:{group:'兼容旧字段', control:'directory', label:'目录选择', variable:'INPUT_DIR'},
    text:{group:'基础字段', control:'text', label:'单行输入'},
    textarea:{group:'基础字段', control:'textarea', label:'多行输入'},
    number:{group:'基础字段', control:'number', label:'数字输入', min:0, max:999999},
    select:{group:'基础字段', control:'select', label:'下拉选项', options:[{label:'选项一',value:'option_1'},{label:'选项二',value:'option_2'}]},
    radio:{group:'基础字段', control:'radio', label:'单选项', options:[{label:'选项一',value:'option_1'},{label:'选项二',value:'option_2'}]},
    multiselect:{group:'基础字段', control:'multiselect', label:'多选项', options:[{label:'选项一',value:'option_1'},{label:'选项二',value:'option_2'}]},
    checkbox:{group:'基础字段', control:'checkbox', label:'开关'},
    date:{group:'基础字段', control:'date', label:'日期'},
    time:{group:'基础字段', control:'time', label:'时间'},
    slider:{group:'基础字段', control:'slider', label:'滑块', min:0, max:100}
  };
  const fieldGroups = [
    {label:'容器与展示', types:['section','divider','hint']},
    {label:'Slurm 资源申请', types:['slurm-job-name','slurm-partition','slurm-account','slurm-nodes','slurm-cpus-per-task','slurm-gpus-per-node','slurm-time','slurm-workdir','slurm-output','slurm-error','slurm-mail-user','slurm-mail-type','slurm-ntasks','slurm-ntasks-per-node','slurm-gpus','slurm-mem','slurm-constraint','slurm-qos','slurm-array','slurm-exclusive','slurm-custom']},
    {label:'应用输入', types:['app-file','app-directory','custom','textarea','number','select','radio','multiselect','checkbox','date','time','slider']}
  ];
  const displayTypes = new Set(['section','divider','hint']);
  const $ = (s, r) => (r || document).querySelector(s);
  const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const cssSafe = value => (window.CSS && CSS.escape ? CSS.escape(String(value)) : String(value).replace(/["\\\]]/g, '\\$&'));
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
  function initialTemplateView() {
    const view = new URLSearchParams(location.search).get('view') || 'library';
    return templateViews.has(view) ? view : 'library';
  }
  function allowedTemplateView(view) {
    const next = templateViews.has(view) ? view : 'library';
    if (next !== 'library' && !state.canManage) return 'library';
    return next;
  }
  function syncTemplateViewURL(replace) {
    const url = new URL(location.href);
    if (state.view === 'library') url.searchParams.delete('view');
    else url.searchParams.set('view', state.view);
    const next = url.pathname + url.search + url.hash;
    const current = location.pathname + location.search + location.hash;
    if (next === current) return;
    history[replace ? 'replaceState' : 'pushState']({view: state.view}, '', next);
  }
  function syncTemplateViewChrome() {
    document.querySelectorAll('.tpl-tabs button').forEach(button => {
      button.classList.toggle('active', button.dataset.view === state.view);
    });
    const manage = state.view === 'manage';
    const requests = state.view === 'requests';
    $('#templatePageTitle').textContent = manage ? '模板管理' : requests ? '授权审批' : '应用广场';
    $('#templatePageDescription').textContent = manage
      ? '维护草稿、授权范围和发布状态；已发布模板需先取消发布后才能设计。'
      : requests
        ? '审批用户对应用模板的使用申请。'
        : '选择已发布并获得授权的应用模板，填写参数后直接提交作业。';
    $('#managerActions').hidden = !state.canManage || !manage;
    $('#templateFilters').hidden = requests;
    $('#templateStatus').hidden = !manage;
    $('#templateViewSwitch').hidden = !manage;
    $('#batchExportTemplates').hidden = !manage || state.layout !== 'list';
  }
  function setTemplateView(view, options) {
    state.view = allowedTemplateView(view);
    state.selected.clear();
    syncTemplateViewChrome();
    if (options?.writeURL) syncTemplateViewURL(!!options.replace);
    render();
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
      document.querySelectorAll('.manager-only').forEach(el => el.hidden = !state.canManage);
      state.view = allowedTemplateView(state.view);
      syncTemplateViewChrome();
      syncTemplateViewURL(true);
      const categories = [...new Set(state.items.map(item => item.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'zh-CN'));
      $('#templateCategory').innerHTML = '<option value="">全部分类</option>' + categories.map(value => `<option value="${esc(value)}">${esc(value)}</option>`).join('');
      render();
    } catch (error) { $('#templateContent').innerHTML = `<div class="tpl-empty tpl-error">数据未获取：${esc(error.message)}</div>`; }
  }
  async function loadDynamicTemplateOptions(force) {
    if (state.dynamicOptions.loaded && !force) return state.dynamicOptions;
    const [partitionData, qosData] = await Promise.all([
      api('/slurm/partitions').catch(error => ({items: [], errors: {partitions: error.message}})),
      api('/slurm/qos').catch(error => ({qos: [], associations: [], errors: {qos: error.message, associations: error.message}}))
    ]);
    state.dynamicOptions = {
      loaded: true,
      partitions: partitionData.items || [],
      qos: qosData.qos || [],
      associations: qosData.associations || [],
      errors: Object.assign({}, partitionData.errors || {}, qosData.errors || {})
    };
    return state.dynamicOptions;
  }
  function partitionOptionSummary(partition) {
    if (!partition) return '';
    const parts = [];
    if (partition.cpusPerNode) parts.push(`每节点 CPU ${partition.cpusPerNode}`);
    if (partition.gres && partition.gres !== '(null)' && partition.gres !== 'N/A') parts.push(`GRES ${partition.gres}`);
    if (partition.maxTime) parts.push(`最长 ${partition.maxTime}`);
    if (partition.nodeList) parts.push(`节点 ${partition.nodeList}`);
    return parts.join(' · ');
  }
  function parseGresGpuCount(gres) {
    const text = String(gres || '');
    let max = 0;
    text.split(',').forEach(part => {
      const match = part.match(/gpu(?::[^:,\s]+)?:([0-9]+)/i);
      if (match) max = Math.max(max, Number(match[1]));
    });
    return max;
  }
  function currentPartition(root) {
    const select = root?.querySelector?.('[data-dynamic-source="slurm.partitions"]');
    return select ? (state.dynamicOptions.partitions || []).find(item => String(item.name) === String(select.value)) : null;
  }
  function dynamicOptionsForField(field, projects, account) {
    const source = fieldDataSource(field);
    if (source === 'projects.accounts') return projectAccountOptions(projects);
    if (source === 'slurm.partitions') {
      const permission = authorizedPartitionNames(account);
      const items = (state.dynamicOptions.partitions || []).filter(item => permission.allowsAll || permission.names.has(item.name));
      return items.map(item => ({
        label: item.name,
        value: item.name,
        hint: partitionOptionSummary(item)
      }));
    }
    if (source === 'slurm.qos') {
      return (state.dynamicOptions.qos || []).filter(item => item.name).map(item => ({
        label: item.description ? `${item.name} · ${item.description}` : item.name,
        value: item.name
      }));
    }
    return field.options || [];
  }
  function selectableOptions(field, projects, account) {
    const options = fieldDataSource(field) ? dynamicOptionsForField(field, projects, account) : (field.options || []);
    return options.length ? options : [{label:'暂无可用选项', value:'', disabled:true}];
  }
  function updateSubmitResourceHints(root) {
    const partition = currentPartition(root);
    const hint = root.querySelector('[data-partition-hint]');
    if (hint) hint.textContent = partition ? partitionOptionSummary(partition) || '该队列未返回资源上限信息' : '请选择队列后查看资源约束';
    const cpus = root.querySelector('[data-slurm-option="--cpus-per-task"]');
    if (cpus && partition?.cpusPerNode) cpus.max = partition.cpusPerNode;
    const gpusPerNode = root.querySelector('[data-slurm-option="--gpus-per-node"]');
    const gpuMax = parseGresGpuCount(partition?.gres);
    if (gpusPerNode && gpuMax > 0) gpusPerNode.max = String(gpuMax);
  }
  function renderOptionTags(options, selectedValue) {
    return options.map(option => `<option value="${esc(option.value)}" ${option.disabled?'disabled':''} ${String(selectedValue)===String(option.value)?'selected':''}>${esc(option.label)}</option>`).join('');
  }
  function hydrateDynamicSubmitOptions(root, projects) {
    const account = currentSelectedAccount(root);
    root.querySelectorAll('[data-dynamic-source]').forEach(select => {
      const field = normalizeTemplateFields({formSchema: [{id: select.dataset.field, dataSource: select.dataset.dynamicSource, options: []}]}, false)[0];
      const selected = select.value;
      const source = select.dataset.dynamicSource;
      field.dataSource = source;
      const options = selectableOptions(field, projects, account);
      select.innerHTML = renderOptionTags(options, options.some(option => String(option.value) === String(selected)) ? selected : options[0]?.value || '');
    });
    updateSubmitResourceHints(root);
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
  function controlLabel(value) {
    return {
      text:'输入框', textarea:'多行输入', number:'数字输入', select:'下拉框',
      radio:'单选框', multiselect:'多选框', checkbox:'开关', date:'日期',
      time:'时间', slider:'滑动条', file:'输入文件', directory:'目录路径', project:'项目下拉框'
    }[value] || value;
  }
  const dataSourceLabels = {
    'projects.accounts': '项目 Account（按成员授权）',
    'slurm.partitions': 'Slurm 队列（按用户/Account 授权）',
    'slurm.qos': 'Slurm QOS（按集群配置）'
  };
  function fieldDataSource(field) {
    return field?.dataSource || componentCatalog[field?.type]?.dataSource || '';
  }
  function defaultDataSourceFor(field) {
    if (field?.slurmOption === '--account' || field?.type === 'slurm-account' || controlType(field) === 'project') return 'projects.accounts';
    if (field?.slurmOption === '--partition' || field?.type === 'slurm-partition') return 'slurm.partitions';
    if (field?.slurmOption === '--qos' || field?.type === 'slurm-qos') return 'slurm.qos';
    return 'slurm.partitions';
  }
  function currentUsername() {
    return String(user.username || user.account || '').trim();
  }
  function canSeeAllSlurmOptions() {
    return user.type === 'admin' || ['admin','cluster_admin','hpc_admin'].includes(String(user.role || '').toLowerCase()) || state.canManage;
  }
  function optionLabel(value, label) {
    const text = String(label || value || '').trim();
    return text || '未命名';
  }
  function splitSlurmList(value) {
    return String(value || '').split(/[,\s]+/).map(item => item.trim()).filter(Boolean);
  }
  function userAssociations() {
    const username = currentUsername();
    const associations = state.dynamicOptions.associations || [];
    if (canSeeAllSlurmOptions()) return associations;
    return associations.filter(item => String(item.user || '').trim() === username);
  }
  function authorizedAccountNames() {
    const names = new Set();
    userAssociations().forEach(item => {
      if (item.account) names.add(String(item.account).trim());
    });
    return names;
  }
  function authorizedPartitionNames(account) {
    const associations = userAssociations().filter(item => !account || !item.account || String(item.account) === String(account));
    const names = new Set();
    let allowsAll = canSeeAllSlurmOptions() || !associations.length;
    associations.forEach(item => {
      const parts = splitSlurmList(item.partition);
      if (!parts.length) allowsAll = true;
      parts.forEach(name => names.add(name));
    });
    return { names, allowsAll };
  }
  function currentSelectedAccount(root) {
    const select = root?.querySelector?.('[data-core="account"]');
    return select ? select.value : '';
  }
  function accountFallbackOptions() {
    const authorized = authorizedAccountNames();
    return [...authorized].sort((a, b) => a.localeCompare(b, 'zh-CN')).map(account => ({
      label: `Slurm Account：${account}`,
      value: account,
      projectId: ''
    }));
  }
  function projectAccountOptions(projects) {
    const usable = (projects || []).filter(project => project.slurmAccount).map(project => ({
      label: `${project.name}${project.currentUserDefaultProject ? '（默认）' : ''} · ${project.slurmAccount}`,
      value: project.slurmAccount,
      projectId: project.id,
      default: !!project.currentUserDefaultProject,
      sortName: project.name
    }));
    const seen = new Set(usable.map(option => String(option.value)));
    accountFallbackOptions().forEach(option => {
      if (!seen.has(String(option.value))) usable.push(option);
    });
    return usable.sort((a, b) => (b.default ? 1 : 0) - (a.default ? 1 : 0) || String(a.sortName || a.label).localeCompare(String(b.sortName || b.label), 'zh-CN'));
  }
  function projectAccountSelect(projects, field) {
    const sorted = projectAccountOptions(projects);
    const preferred = selectedProjectFromURL();
    if (!sorted.length) {
      return `<label class="wide">${esc(field?.label || '所属项目')}<select data-core="account" data-field="${esc(field?.id || '')}" data-project-select disabled><option value="">暂无可用项目</option></select><small>请先在项目中心加入项目，或联系项目负责人授权。</small></label>`;
    }
    return `<label class="wide">${esc(field?.label || '所属项目')}<select data-core="account" data-field="${esc(field?.id || '')}" data-project-select>
      ${sorted.map(option => {
        const matched = preferred.projectId ? String(option.projectId) === String(preferred.projectId) : (preferred.account ? String(option.value) === preferred.account : option.default);
        return `<option value="${esc(option.value)}" data-project-id="${esc(option.projectId || '')}" ${matched ? 'selected' : ''}>${esc(option.label)}</option>`;
      }).join('')}
    </select><small>系统会把所选项目写入 Slurm 脚本的 --account；未授权的 Account 不会出现在列表中。</small></label>`;
  }
  function controlType(field) {
    return field.control || componentCatalog[field.type]?.control || field.type || 'text';
  }
  function ensureFieldId(field, index) {
    if (!field.id) field.id = 'field_' + Date.now() + '_' + index + '_' + Math.random().toString(36).slice(2, 6);
    return field.id;
  }
  function selectedFieldIndex() {
    const byId = state.selectedFieldId ? state.fields.findIndex(field => field.id === state.selectedFieldId) : -1;
    if (byId >= 0) {
      state.selectedField = byId;
      return byId;
    }
    const fallback = Number.isInteger(state.selectedField) ? state.selectedField : -1;
    if (fallback >= 0 && fallback < state.fields.length) {
      state.selectedFieldId = state.fields[fallback]?.id || '';
      return fallback;
    }
    state.selectedField = state.fields.length ? 0 : -1;
    state.selectedFieldId = state.fields[0]?.id || '';
    return state.selectedField;
  }
  function selectedField() {
    const index = selectedFieldIndex();
    return index >= 0 ? state.fields[index] : null;
  }
  function selectFieldById(fieldId) {
    const index = state.fields.findIndex(field => field.id === fieldId);
    state.selectedField = index;
    state.selectedFieldId = index >= 0 ? fieldId : '';
    return index;
  }
  function selectFieldAt(index) {
    const field = state.fields[index];
    state.selectedField = field ? index : -1;
    state.selectedFieldId = field?.id || '';
  }
  function hasSlurmFields(fields) {
    return (fields || []).some(field => field && field.slurmOption);
  }
  function defaultSlurmFields(template) {
    const base = ['slurm-job-name','slurm-account','slurm-partition','slurm-nodes','slurm-cpus-per-task','slurm-gpus-per-node','slurm-time','slurm-workdir','slurm-output','slurm-error'];
    return base.map(type => {
      const field = makeField(type);
      if (type === 'slurm-job-name') field.default = safeJobName(template?.name || 'simpleHPC', template?.id || 0);
      return field;
    });
  }
  function normalizeTemplateFields(item, forEdit) {
    const fields = JSON.parse(JSON.stringify(item.formSchema || []));
    fields.forEach((field, index) => {
      ensureFieldId(field, index);
      const spec = componentCatalog[field.type] || {};
      if (!field.control && spec.control && spec.control !== 'display') field.control = spec.control;
      if (!field.slurmOption && spec.slurmOption) field.slurmOption = spec.slurmOption;
      if (!field.dataSource && spec.dataSource) field.dataSource = spec.dataSource;
      if (!field.optionMode) field.optionMode = field.dataSource ? 'dynamic' : 'fixed';
      if (!field.label) field.label = spec.label || fieldTypes[field.type] || '未命名字段';
    });
    if (forEdit && fields.length && !hasSlurmFields(fields)) return [...defaultSlurmFields(item), ...fields];
    if (forEdit && !fields.length) return [...defaultSlurmFields(item), makeField('app-file'), makeField('custom')];
    return fields;
  }
  function templateParameterForm(item, username, projects) {
    const fields = normalizeTemplateFields(item, false).map(field => submitField(field, username, projects)).join('');
    return `<div class="tpl-use-heading"><div><h3>作业参数</h3><p>按本次任务需要调整资源、输入文件和工作目录。</p></div></div>
      <div class="tpl-submit-grid">
        ${fields || '<div class="tpl-submit-hint wide">该模板尚未配置参数组件，请联系管理员在模板管理中添加 Slurm 资源申请和应用输入组件。</div>'}
      </div>`;
  }
  async function detail(id) {
    const [item] = await Promise.all([
      api('/job-templates/' + id),
      loadDynamicTemplateOptions()
    ]);
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
    const spec = componentCatalog[type] || {};
    const field = {
      id:'field_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6),
      type:type,
      label:spec.label || fieldTypes[type] || '新字段',
      control:displayTypes.has(type) ? '' : (spec.control || type),
      variable:displayTypes.has(type) ? '' : (spec.variable || 'PARAM_' + n),
      slurmOption:spec.slurmOption || '',
      dataSource:spec.dataSource || '',
      optionMode:spec.dataSource ? 'dynamic' : 'fixed',
      required:!!spec.required,
      default:spec.default || '',
      placeholder:spec.placeholder || '',
      help:spec.help || ''
    };
    if (spec.options || ['select','radio','multiselect'].includes(field.control)) {
      field.options = JSON.parse(JSON.stringify(spec.options || [{label:'选项一',value:'option_1'},{label:'选项二',value:'option_2'}]));
    }
    if (['number','slider'].includes(field.control)) {
      field.min = spec.min == null ? 0 : spec.min;
      field.max = spec.max == null ? 128 : spec.max;
    }
    return field;
  }
  function addField(type, root) {
    const field = makeField(type);
    state.fields.push(field);
    selectFieldById(field.id);
    renderBuilder(root);
  }
  function fieldControlPreview(field) {
    const control = controlType(field);
    if (field.type === 'section') return `<h3>${esc(field.label || '分组标题')}</h3>`;
    if (field.type === 'divider') return '<hr>';
    if (field.type === 'hint') return `<div class="tpl-canvas-hint">${esc(field.label || '提示文字')}</div>`;
    if (control === 'textarea') return `<textarea disabled placeholder="${esc(field.placeholder || '多行文本')}"></textarea>`;
    if (['select','project','radio','multiselect'].includes(control)) return `<select disabled><option>${esc(field.placeholder || '请选择')}</option></select>`;
    if (control === 'checkbox') return '<span class="tpl-switch-preview"><i></i></span>';
    if (control === 'slider') return '<input type="range" disabled>';
    if (control === 'file') return `<div class="tpl-file-preview"><span>服务器路径 / 上传文件</span></div>`;
    const type = control === 'number' ? 'number' : control === 'date' ? 'date' : control === 'time' ? 'time' : 'text';
    return `<input type="${type}" disabled placeholder="${esc(field.placeholder || fieldTypes[field.type] || '')}">`;
  }
  function fieldCanvas(field, index) {
    const selected = field.id === state.selectedFieldId ? ' selected' : '';
    const slurm = field.slurmOption ? `<code>${esc(field.slurmOption)}</code>` : '';
    const variable = displayTypes.has(field.type) ? '' : `<span>${slurm}<code>\${${esc(field.variable || '未设置变量')}}</code></span>`;
    return `<div class="tpl-canvas-field${selected}" data-field-id="${esc(field.id)}" data-field-index="${index}">
      <button type="button" class="tpl-canvas-drag" draggable="true" data-field-action="drag" title="拖拽排序">⋮⋮</button>
      <div class="tpl-canvas-content">
        ${displayTypes.has(field.type) ? fieldControlPreview(field) : `<div class="tpl-canvas-label"><b>${esc(field.label || '未命名字段')}${field.required ? ' *' : ''}</b>${variable}</div>${fieldControlPreview(field)}${field.help ? `<small>${esc(field.help)}</small>` : ''}`}
      </div>
      <button type="button" class="btn-icon tpl-canvas-remove" data-remove="${esc(field.id)}" data-field-action="delete" title="删除组件">×</button>
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
    const field = selectedField();
    if (!field) {
      panel.innerHTML = '<div class="tpl-property-empty">在画布中选择一个组件后，可在这里编辑名称、变量和内容。</div>';
      return;
    }
    const isDisplay = displayTypes.has(field.type);
    const control = controlType(field);
    const hasOptions = ['select','radio','multiselect','project'].includes(control);
    const optionMode = field.optionMode || (fieldDataSource(field) ? 'dynamic' : 'fixed');
    const dataSource = fieldDataSource(field);
    const showFixedOptions = hasOptions && optionMode !== 'dynamic';
    const showDynamicSource = hasOptions && optionMode === 'dynamic';
    const hasRange = ['number','slider'].includes(control);
    panel.innerHTML = `<div class="tpl-property-heading"><span>${esc(fieldTypes[field.type] || field.type)}</span><code>#${esc(field.id)}</code></div>
      <label>组件类型<select data-field-prop="type">${Object.entries(fieldTypes).map(([value,label])=>`<option value="${value}" ${field.type===value?'selected':''}>${label}</option>`).join('')}</select></label>
      <label>${isDisplay ? '显示内容' : '字段名称'}<input data-field-prop="label" value="${esc(field.label)}"></label>
      ${isDisplay ? '' : `<label>控件类型<select data-field-prop="control">${['text','textarea','number','select','radio','multiselect','checkbox','date','time','slider','file','directory','project'].map(value=>`<option value="${value}" ${control===value?'selected':''}>${esc(controlLabel(value))}</option>`).join('')}</select></label>
      <label>变量名称<input data-field-prop="variable" value="${esc(field.variable)}" placeholder="INPUT_FILE"><small>保存为 Slurm 脚本可调用的环境变量</small></label>
      <label>Slurm 参数名<input data-field-prop="slurmOption" value="${esc(field.slurmOption || '')}" placeholder="例如 --nodes"><small>填写后会自动生成 #SBATCH；留空则只导出变量</small></label>
      ${hasOptions ? `<label>选项来源<select data-field-prop="optionMode"><option value="fixed" ${optionMode==='fixed'?'selected':''}>固定值：手工维护选项</option><option value="dynamic" ${optionMode==='dynamic'?'selected':''}>动态值：按用户权限自动获取</option></select><small>队列、项目 Account、QOS 推荐使用动态来源。</small></label>` : ''}
      ${showDynamicSource ? `<label>动态数据源<select data-field-prop="dataSource">${Object.entries(dataSourceLabels).map(([value,label])=>`<option value="${value}" ${dataSource===value?'selected':''}>${esc(label)}</option>`).join('')}</select><small>提交作业时会按当前用户权限加载；Slurm 查询失败时显示友好降级提示。</small></label>` : ''}
      <label>默认值<input data-field-prop="default" value="${esc(field.default == null ? '' : field.default)}"></label>
      <label>占位提示<input data-field-prop="placeholder" value="${esc(field.placeholder || '')}"></label>
      <label>帮助说明<input data-field-prop="help" value="${esc(field.help || '')}"></label>
      <label class="tpl-property-check"><input type="checkbox" data-field-prop="required" ${field.required?'checked':''}> 必填字段</label>`}
      ${showFixedOptions ? `<label>选项配置<textarea data-field-prop="optionsText" rows="5" placeholder="显示名称=实际值">${esc(optionsToText(field))}</textarea><small>每行一个选项，格式：名称=值。保存后提交页按这些固定值展示。</small></label>` : ''}
      ${hasRange ? `<div class="tpl-property-range"><label>最小值<input type="number" data-field-prop="min" value="${esc(field.min == null ? '' : field.min)}"></label><label>最大值<input type="number" data-field-prop="max" value="${esc(field.max == null ? '' : field.max)}"></label></div>` : ''}`;
    panel.querySelectorAll('[data-field-prop]').forEach(control => {
      control.addEventListener(control.type === 'checkbox' || control.tagName === 'SELECT' ? 'change' : 'input', () => {
        const current = selectedField();
        if (!current) return;
        const prop = control.dataset.fieldProp;
        if (prop === 'optionsText') current.options = textToOptions(control.value);
        else if (control.type === 'checkbox') current[prop] = control.checked;
        else if (['min','max'].includes(prop)) current[prop] = control.value === '' ? null : Number(control.value);
        else if (prop === 'optionMode') {
          current.optionMode = control.value;
          current.dataSource = control.value === 'dynamic' ? (current.dataSource || defaultDataSourceFor(current)) : '';
          if (control.value === 'fixed' && (!current.options || !current.options.length) && ['select','radio','multiselect'].includes(controlType(current))) current.options = textToOptions('选项一=option_1\n选项二=option_2');
        } else if (prop === 'dataSource') {
          current.dataSource = control.value;
          current.optionMode = 'dynamic';
        } else current[prop] = control.value;
        if (prop === 'type') {
          const spec = componentCatalog[current.type] || {};
          if (displayTypes.has(current.type)) {
            current.variable = '';
            current.slurmOption = '';
            current.control = '';
            current.dataSource = '';
            current.optionMode = 'fixed';
          } else {
            current.control = spec.control || current.control || 'text';
            current.slurmOption = spec.slurmOption || current.slurmOption || '';
            current.dataSource = spec.dataSource || current.dataSource || '';
            current.optionMode = current.dataSource ? 'dynamic' : (current.optionMode || 'fixed');
            if (!current.variable) current.variable = spec.variable || 'PARAM_' + (selectedFieldIndex() + 1);
          }
          renderBuilder(root);
        } else if (prop === 'control') {
          if (['select','radio','multiselect'].includes(controlType(current)) && current.optionMode !== 'dynamic' && (!current.options || !current.options.length)) {
            current.options = textToOptions('选项一=option_1\n选项二=option_2');
          }
          renderBuilder(root);
        } else if (prop === 'optionMode' || prop === 'dataSource') {
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
    selectedFieldIndex();
    list.innerHTML = state.fields.map(fieldCanvas).join('') || '<div class="tpl-builder-empty"><b>将左侧组件拖到这里</b><span>也可以点击组件快速添加</span></div>';
    const count = $('.tpl-canvas-toolbar small', root);
    if (count) count.textContent = `${state.fields.length} 个组件`;
    list.querySelectorAll('[data-field-id]').forEach(card => {
      card.onpointerdown = event => {
        if (event.target.closest('[data-field-action]')) return;
        if (selectFieldById(card.dataset.fieldId) < 0) return;
        renderCanvas(root);
        renderProperties(root);
      };
      card.ondragover = event => event.preventDefault();
      card.ondrop = event => {
        event.preventDefault();
        const type = event.dataTransfer.getData('text/x-template-field');
        if (type) {
          const to = state.fields.findIndex(field => field.id === card.dataset.fieldId);
          const field = makeField(type);
          state.fields.splice(to >= 0 ? to + 1 : state.fields.length, 0, field);
          selectFieldById(field.id);
          renderBuilder(root);
          return;
        }
        const fromId = event.dataTransfer.getData('text/x-template-field-id');
        const from = state.fields.findIndex(field => field.id === fromId);
        const to = state.fields.findIndex(field => field.id === card.dataset.fieldId);
        if (from >= 0 && to >= 0 && from !== to) {
          const moved = state.fields.splice(from, 1)[0];
          state.fields.splice(to, 0, moved);
          selectFieldById(moved.id);
          renderBuilder(root);
        }
      };
    });
    list.querySelectorAll('[data-field-action="drag"]').forEach(handle => {
      handle.ondragstart = event => {
        const card = handle.closest('[data-field-id]');
        if (!card) return;
        event.dataTransfer.setData('text/x-template-field-id', card.dataset.fieldId);
        event.dataTransfer.effectAllowed = 'move';
      };
    });
    list.querySelectorAll('[data-remove]').forEach(button => {
      button.onclick = event => {
        event.stopPropagation();
        const id = button.dataset.remove;
        const index = state.fields.findIndex(field => field.id === id);
        if (index < 0) return;
        state.fields.splice(index, 1);
        const nextIndex = Math.min(index, state.fields.length - 1);
        selectFieldAt(nextIndex);
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
    state.fields=normalizeTemplateFields(item, true);
    selectFieldAt(state.fields.length ? 0 : -1);
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
  function fileField(field) {
    const id = esc(field.id);
    const destination = esc(field.uploadPath || field.workdir || field.defaultUploadPath || '');
    return `<label class="wide tpl-file-control" data-file-control="${id}">${esc(field.label)}${field.required?' *':''}
      <div class="tpl-file-mode">
        <select data-file-mode="${id}">
          <option value="server">选择服务器文件路径</option>
          <option value="upload">上传本地文件到服务器目录</option>
        </select>
        <input data-file-path="${id}" data-field="${id}" value="${esc(field.default || '')}" placeholder="${esc(field.placeholder || '/data/home/user/input.dat')}">
      </div>
      <div class="tpl-file-upload-row" data-file-upload-row="${id}" hidden>
        <input data-upload-dir="${id}" value="${destination}" placeholder="/data/home/user/work">
        <input type="file" data-upload-file="${id}">
      </div>
      ${field.help?`<small>${esc(field.help)}</small>`:'<small>服务器路径用于脚本变量；上传文件会在提交前写入指定目录。</small>'}
    </label>`;
  }
  function submitField(field, username, projects) {
    if (field.type === 'section') return `<h3 class="tpl-submit-section">${esc(field.label)}</h3>`;
    if (field.type === 'divider') return '<hr class="tpl-submit-divider">';
    if (field.type === 'hint') return `<div class="tpl-submit-hint">${esc(field.label)}</div>`;
    const control = controlType(field);
    const source = fieldDataSource(field);
    const common = `data-field="${esc(field.id)}" data-slurm-option="${esc(field.slurmOption || '')}"`;
    if (control === 'project' || field.slurmOption === '--account') return projectAccountSelect(projects || [], field);
    if (control === 'file') return fileField(field);
    const options = selectableOptions(field, projects, '');
    let input = '';
    if (control === 'textarea') input = `<textarea ${common} placeholder="${esc(field.placeholder || '')}">${esc(field.default || '')}</textarea>`;
    else if (control === 'select') input = `<select ${common} ${source?`data-dynamic-source="${esc(source)}"`:''}>${options.map(option=>`<option value="${esc(option.value)}" ${option.disabled?'disabled':''} ${String(field.default)===String(option.value)?'selected':''}>${esc(option.label)}</option>`).join('')}</select>${source==='slurm.partitions'?'<small data-partition-hint>请选择队列后查看资源约束</small>':''}`;
    else if (control === 'radio') input = `<div class="tpl-choice-group">${options.map(option=>`<label><input ${common} name="tpl_${esc(field.id)}" type="radio" value="${esc(option.value)}" ${option.disabled?'disabled':''} ${String(field.default)===String(option.value)?'checked':''}> ${esc(option.label)}</label>`).join('')}</div>`;
    else if (control === 'multiselect') input = `<select ${common} multiple ${source?`data-dynamic-source="${esc(source)}"`:''}>${options.map(option=>`<option value="${esc(option.value)}" ${option.disabled?'disabled':''} ${String(field.default).split(',').includes(String(option.value))?'selected':''}>${esc(option.label)}</option>`).join('')}</select>`;
    else if (control === 'checkbox') input = `<input ${common} type="checkbox" ${field.default?'checked':''}>`;
    else if (control === 'slider') input = `<input ${common} type="range" min="${esc(field.min == null ? 0 : field.min)}" max="${esc(field.max == null ? 100 : field.max)}" value="${esc(field.default == null ? 0 : field.default)}">`;
    else {
      const type = control === 'number' ? 'number' : control === 'date' ? 'date' : control === 'time' ? 'time' : 'text';
      const fallback = control === 'directory' && !field.default ? `/data/home/${username || 'user'}` : (field.default || '');
      input = `<input ${common} type="${type}" value="${esc(fallback)}" placeholder="${esc(field.placeholder || '')}" ${field.min==null?'':`min="${esc(field.min)}"`} ${field.max==null?'':`max="${esc(field.max)}"`}>`;
    }
    const sourceHelp = source && source !== 'slurm.partitions' ? `<small>选项来源：${esc(dataSourceLabels[source] || source)}</small>` : '';
    return `<label>${esc(field.label)}${field.required?' *':''}${input}${field.help?`<small>${esc(field.help)}</small>`:sourceHelp}</label>`;
  }
  async function useTemplate(id) {
    const [item, projectData]=await Promise.all([
      api('/job-templates/'+id),
      api('/projects').catch(() => ({items: state.projects || []})),
      loadDynamicTemplateOptions()
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
        const run=await api('/job-templates/'+id+'/submit',{method:'POST',body:JSON.stringify(await collect(modal.el, {uploadFiles:true}))});
        App.toast((item.kind==='novnc'?'VNC 桌面作业 ':'作业 ')+run.slurmJobId+' 已提交','success',5000);
      }
    });
    modal.el.classList.add('tpl-use-modal');
    const refresh=async()=>{
      const stateLabel=$('#previewState',modal.el);
      stateLabel.textContent='正在生成...';
      const data=await api('/job-templates/'+id+'/preview',{method:'POST',body:JSON.stringify(await collect(modal.el, {uploadFiles:false}))});
      $('#scriptPreview',modal.el).textContent=data.script;
      stateLabel.textContent='已按当前参数更新';
    };
    $('#previewScript',modal.el).onclick=()=>refresh().catch(error=>App.toast(error.message,'danger',5000));
    hydrateDynamicSubmitOptions(modal.el, state.projects);
    modal.el.querySelectorAll('[data-core],[data-field],[data-file-mode],[data-upload-dir],[data-upload-file]').forEach(control=>{
      const markDirty=()=>{$('#previewState',modal.el).textContent='参数已变化，请刷新脚本';};
      control.addEventListener('input', markDirty);
      control.addEventListener('change', () => {
        if (control.matches('[data-core="account"]')) hydrateDynamicSubmitOptions(modal.el, state.projects);
        if (control.matches('[data-dynamic-source="slurm.partitions"]')) updateSubmitResourceHints(modal.el);
        if (control.matches('[data-file-mode]')) {
          const id = control.dataset.fileMode;
          const row = modal.el.querySelector(`[data-file-upload-row="${cssSafe(id)}"]`);
          const path = modal.el.querySelector(`[data-file-path="${cssSafe(id)}"]`);
          if (row) row.hidden = control.value !== 'upload';
          if (path) path.hidden = control.value === 'upload';
        }
        markDirty();
      });
    });
    await refresh();
  }
  async function uploadTemplateFile(directory, file) {
    const form = new FormData();
    form.append('path', directory);
    form.append('file', file);
    const response = await fetch('/api/v1/storage/upload', {method:'POST', headers:{'Authorization':'Bearer '+token}, body:form});
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || '文件上传失败 (' + response.status + ')');
  }
  function joinPath(directory, name) {
    return String(directory || '').replace(/\/+$/, '') + '/' + String(name || '').replace(/^\/+/, '');
  }
  async function collect(root, options){const values={};const upload=options&&options.uploadFiles;root.querySelectorAll('[data-core]').forEach(x=>{values[x.dataset.core]=x.type==='number'?Number(x.value):x.value;if(x.matches('[data-project-select]')){const option=x.selectedOptions&&x.selectedOptions[0];values.projectId=option&&option.dataset.projectId?Number(option.dataset.projectId):0;}});for(const wrap of root.querySelectorAll('[data-file-control]')){const id=wrap.dataset.fileControl;const safe=cssSafe(id);const mode=root.querySelector(`[data-file-mode="${safe}"]`)?.value||'server';if(mode==='upload'){const directory=root.querySelector(`[data-upload-dir="${safe}"]`)?.value.trim();const file=root.querySelector(`[data-upload-file="${safe}"]`)?.files?.[0];if(!directory)throw new Error('请填写上传目录');if(file&&upload)await uploadTemplateFile(directory,file);values[id]=file?joinPath(directory,file.name):'';}else{values[id]=root.querySelector(`[data-file-path="${safe}"]`)?.value||'';}}root.querySelectorAll('[data-field]').forEach(x=>{if(values[x.dataset.field]!==undefined)return;if(x.type==='radio'&&!x.checked)return;if(x.type==='checkbox')values[x.dataset.field]=x.checked;else if(x.multiple)values[x.dataset.field]=Array.from(x.selectedOptions).map(o=>o.value).join(',');else values[x.dataset.field]=x.type==='number'?Number(x.value):x.value;});return values;}
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
  document.querySelectorAll('.tpl-tabs button').forEach(btn=>btn.onclick=()=>setTemplateView(btn.dataset.view,{writeURL:true}));
  window.addEventListener('popstate',()=>setTemplateView(initialTemplateView(),{writeURL:false}));
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
