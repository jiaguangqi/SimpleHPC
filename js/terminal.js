(function () {
  'use strict';

  const screen = document.getElementById('terminalScreen');
  const output = document.getElementById('terminalOutput');
  const emptyState = document.getElementById('terminalEmptyState');
  const status = document.getElementById('terminalStatus');
  const connectBtn = document.getElementById('terminalConnectBtn');
  const disconnectBtn = document.getElementById('terminalDisconnectBtn');
  const searchBtn = document.getElementById('terminalSearchBtn');
  const clearBtn = document.getElementById('terminalClearBtn');
  const copyBtn = document.getElementById('terminalCopyBtn');
  const pasteBtn = document.getElementById('terminalPasteBtn');
  const fullscreenBtn = document.getElementById('terminalFullscreenBtn');
  const settingsBtn = document.querySelector('.shpc-webssh-settings');
  const wrapToggle = document.getElementById('terminalWrapToggle');
  const sizeLabel = document.getElementById('terminalSizeLabel');
  const connState = document.getElementById('terminalConnState');
  const sessionId = document.getElementById('terminalSessionId');
  const sessionUser = document.getElementById('terminalSessionUser');
  const sessionPath = document.getElementById('terminalSessionPath');
  const sessionTitle = document.getElementById('terminalSessionTitle');
  const currentPathLabel = document.getElementById('terminalCurrentPath');
  const pathCopyBtn = document.getElementById('terminalPathCopyBtn');
  const parentBtn = document.getElementById('terminalParentBtn');
  const fileTree = document.getElementById('terminalFileTree');
  const fileSearch = document.getElementById('terminalFileSearch');
  const fileRefreshBtn = document.getElementById('terminalFileRefreshBtn');
  const fileUploadBtn = document.getElementById('terminalFileUploadBtn');
  const fileUploadInput = document.getElementById('terminalFileUploadInput');
  const loginNodeLabel = document.getElementById('terminalLoginNode');
  const latencyLabel = document.getElementById('terminalLatency');
  const latencyBadge = document.getElementById('terminalLatencyBadge');
  const tabs = document.getElementById('terminalTabs');
  const decoder = new TextDecoder('utf-8');
  const token = localStorage.getItem('simplehpc_token') || '';
  const jsonHeaders = Object.assign({'Content-Type': 'application/json'}, token ? {Authorization: 'Bearer ' + token} : {});
  const authHeaders = token ? {Authorization: 'Bearer ' + token} : {};
  const xtermAvailable = typeof window.Terminal === 'function';
  const FitAddonCtor = window.FitAddon?.FitAddon;
  const SearchAddonCtor = window.SearchAddon?.SearchAddon;

  const sessions = new Map();
  let activeSessionId = '';
  let fontSize = 14;
  let currentUser = 'user';
  let currentPath = '';
  let parentPath = '';
  let canGoParent = false;
  let storageRoots = [];
  let currentEntries = [];
  let selectedPaths = new Set();
  let terminalConfig = {strategy: 'round_robin', nodes: []};
  let selectedNode = '';
  let fileSearchTimer = null;
  let lastTerminalSearchTerm = '';
  let lastTerminalSearchIndex = -1;

  function escapeHTML(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch]));
  }

  async function fetchJSON(url, options) {
    const response = await fetch(url, Object.assign({cache: 'no-store', credentials: 'same-origin', headers: jsonHeaders}, options || {}));
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || data.message || `HTTP ${response.status}`);
    return data;
  }

  function toast(message, type) {
    if (window.App?.toast) App.toast(message, type || 'info');
    else console.log(message);
  }

  function sanitizeTerminalText(text) {
    return String(text || '')
      .replace(/\x1b\][^\x07]*(\x07|\x1b\\)/g, '')
      .replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '')
      .replace(/\r\n/g, '\n')
      .replace(/\r/g, '\n');
  }

  function absoluteWSURL(path) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + path;
  }

  function activeSession() {
    return activeSessionId ? sessions.get(activeSessionId) : null;
  }

  function setTerminalEmptyVisible(visible) {
    if (emptyState) emptyState.hidden = !visible;
    if (output) output.hidden = !!visible;
    if (screen) screen.classList.toggle('has-session', !visible);
  }

  function focusActiveTerminal() {
    const item = activeSession();
    if (item?.term) {
      try { item.term.focus(); } catch (_) {}
      const helper = item.term.element?.querySelector?.('.xterm-helper-textarea');
      try {
        helper?.focus?.({preventScroll: true});
      } catch (_) {
        try { helper?.focus?.(); } catch (_) {}
      }
      return true;
    }
    try { screen?.focus?.({preventScroll: true}); } catch (_) { try { screen?.focus?.(); } catch (_) {} }
    return false;
  }

  function terminalSize() {
    const session = activeSession();
    if (session?.term) {
      const cols = session.term.cols || 120;
      const rows = session.term.rows || 36;
      if (sizeLabel) sizeLabel.textContent = cols + 'x' + rows;
      return {cols, rows};
    }
    const style = getComputedStyle(output || screen);
    const px = parseFloat(style.fontSize) || fontSize;
    const lineHeight = parseFloat(style.lineHeight) || px * 1.5;
    const cols = Math.max(40, Math.min(220, Math.floor(screen.clientWidth / (px * 0.62))));
    const rows = Math.max(12, Math.min(80, Math.floor(screen.clientHeight / lineHeight)));
    if (sizeLabel) sizeLabel.textContent = cols + 'x' + rows;
    return {cols, rows};
  }

  function setStatus(text, tone) {
    if (status) {
      status.textContent = text;
      status.dataset.tone = tone || 'idle';
    }
    if (connState) connState.textContent = text;
  }

  function renderSessionTabs() {
    if (!tabs) return;
    const items = Array.from(sessions.values());
    if (!items.length) {
      tabs.innerHTML = '<div class="shpc-webssh-tabs-empty">暂无会话</div>';
      return;
    }
    tabs.innerHTML = items.map(item => {
      const tone = item.status === 'connected' ? 'ok' : (item.status === 'connecting' ? 'pending' : 'idle');
      return '<button type="button" class="' + (item.id === activeSessionId ? 'is-active' : '') + '" data-session-tab="' + escapeHTML(item.id) + '">' +
        '<span data-tone="' + tone + '"></span><strong>' + escapeHTML(item.node || 'login') + ' / bash</strong><em data-session-close="' + escapeHTML(item.id) + '">×</em></button>';
    }).join('');
    tabs.querySelectorAll('[data-session-tab]').forEach(button => {
      button.addEventListener('click', event => {
        if (event.target?.dataset?.sessionClose) return;
        activateSession(button.dataset.sessionTab);
      });
    });
    tabs.querySelectorAll('[data-session-close]').forEach(button => {
      button.addEventListener('click', event => {
        event.stopPropagation();
        closeSession(button.dataset.sessionClose);
      });
    });
  }

  function activateSession(id) {
    if (!sessions.has(id)) return;
    activeSessionId = id;
    const item = sessions.get(id);
    setTerminalEmptyVisible(false);
    mountTerminal(item);
    if (sessionId) sessionId.textContent = item.id || '—';
    if (sessionTitle) sessionTitle.textContent = item.node || 'login';
    if (sessionUser) sessionUser.textContent = item.username || currentUser;
    if (sessionPath) sessionPath.textContent = item.path || currentPath || '/data/home/' + currentUser;
    if (disconnectBtn) disconnectBtn.disabled = item.status !== 'connected' && item.status !== 'connecting';
    setStatus(sessionStatusText(item.status), sessionStatusTone(item.status));
    renderSessionTabs();
    setTimeout(() => {
      focusActiveTerminal();
      screen.scrollTop = screen.scrollHeight;
      fitActiveTerminal();
      sendResize();
    }, 30);
    if (!isSocketOpen(item) && (item.status === 'disconnected' || item.status === 'closed' || item.status === 'failed')) {
      reconnectSession(id, true);
    }
  }

  function showEmptySessionState() {
    activeSessionId = '';
    setTerminalEmptyVisible(true);
    if (output) {
      output.innerHTML = '';
    }
    if (sessionId) sessionId.textContent = '—';
    if (sessionTitle) sessionTitle.textContent = activeNodeName();
    if (sessionUser) sessionUser.textContent = currentUser;
    if (sessionPath) sessionPath.textContent = currentPath || '/data/home/' + currentUser;
    if (disconnectBtn) disconnectBtn.disabled = true;
    setStatus('未连接', 'idle');
    renderSessionTabs();
  }

  function sessionStatusText(value) {
    if (value === 'connected') return '已连接';
    if (value === 'connecting') return '连接中';
    if (value === 'failed') return '连接异常';
    return '已断开';
  }

  function sessionStatusTone(value) {
    if (value === 'connected') return 'ok';
    if (value === 'connecting') return 'pending';
    if (value === 'failed') return 'danger';
    return 'idle';
  }

  function appendToSession(item, data) {
    const text = typeof data === 'string' ? data : decoder.decode(data, {stream: true});
    item.output += text;
    if (item.id === activeSessionId) {
      if (item.term) {
        item.term.write(text);
      } else if (output) {
        output.textContent = sanitizeTerminalText(item.output);
        screen.scrollTop = screen.scrollHeight;
      }
    }
  }

  function activeNodeName() {
    const enabled = (terminalConfig.nodes || []).filter(node => node.enabled !== false);
    const found = enabled.find(node => node.name === selectedNode || node.host === selectedNode || node.hostname === selectedNode || node.address === selectedNode);
    const node = found || enabled[0] || {};
    return node.name || node.hostname || node.host || node.address || '未配置登录节点';
  }

  function hasLoginNodes() {
    return (terminalConfig.nodes || []).some(node => node.enabled !== false && (node.name || node.host || node.hostname || node.address));
  }

  function loginNodeMissingMessage() {
    return '未配置可用登录节点，请到系统设置页面中设置登录节点信息才可以使用该功能。';
  }

  async function createWebSSHSession() {
    if (!hasLoginNodes()) {
      throw new Error(loginNodeMissingMessage());
    }
    const size = terminalSize();
    return fetchJSON('/api/v1/webssh/sessions', {
      method: 'POST',
      body: JSON.stringify({
        node: selectedNode || activeNodeName(),
        shell: 'bash',
        initialPath: currentPath || '',
        cols: size.cols,
        rows: size.rows
      })
    });
  }

  function isSocketOpen(item) {
    return item?.socket && item.socket.readyState === WebSocket.OPEN;
  }

  function normalizeSession(raw) {
    const id = raw.sessionId || raw.id || '';
    return {
      id,
      wsUrl: raw.wsUrl || (id ? '/api/v1/webssh/sessions/' + encodeURIComponent(id) + '/ws' : ''),
      node: raw.node || raw.nodeName || activeNodeName(),
      username: raw.username || currentUser,
      ownerUsername: raw.ownerUsername || currentUser,
      status: raw.status || 'disconnected',
      output: '',
      socket: null,
      term: null,
      fitAddon: null,
      searchAddon: null,
      dataDisposable: null,
      resizeDisposable: null,
      path: raw.currentPath || raw.initialPath || currentPath || '/data/home/' + currentUser
    };
  }

  function createTerminal(item) {
    if (!item || item.term || !xtermAvailable) return item?.term || null;
    const term = new window.Terminal({
      allowProposedApi: true,
      cursorBlink: true,
      cursorStyle: 'block',
      convertEol: false,
      scrollback: 8000,
      fontSize,
      fontFamily: '"SFMono-Regular", "Cascadia Mono", "JetBrains Mono", Menlo, Consolas, monospace',
      lineHeight: 1.25,
      tabStopWidth: 8,
      theme: {
        background: '#0b1220',
        foreground: '#dbe7ff',
        cursor: '#ffffff',
        cursorAccent: '#0b1220',
        selectionBackground: '#2563eb66',
        black: '#111827',
        red: '#ef4444',
        green: '#22c55e',
        yellow: '#f59e0b',
        blue: '#3b82f6',
        magenta: '#a855f7',
        cyan: '#06b6d4',
        white: '#e5e7eb',
        brightBlack: '#64748b',
        brightRed: '#f87171',
        brightGreen: '#4ade80',
        brightYellow: '#fbbf24',
        brightBlue: '#60a5fa',
        brightMagenta: '#c084fc',
        brightCyan: '#22d3ee',
        brightWhite: '#ffffff'
      }
    });
    if (FitAddonCtor) {
      item.fitAddon = new FitAddonCtor();
      term.loadAddon(item.fitAddon);
    }
    if (SearchAddonCtor) {
      item.searchAddon = new SearchAddonCtor();
      term.loadAddon(item.searchAddon);
    }
    item.dataDisposable = term.onData(data => {
      send(data);
    });
    item.resizeDisposable = term.onResize(size => {
      if (sizeLabel) sizeLabel.textContent = size.cols + 'x' + size.rows;
      sendResize(size);
    });
    item.term = term;
    return term;
  }

  function mountTerminal(item) {
    if (!output || !item) return;
    setTerminalEmptyVisible(false);
    if (!xtermAvailable) {
      output.textContent = item.output || '';
      output.style.fontSize = fontSize + 'px';
      return;
    }
    output.innerHTML = '';
    const term = createTerminal(item);
    if (!term) return;
    if (!term.element) {
      term.open(output);
      if (item.output) term.write(item.output);
    } else {
      output.appendChild(term.element);
    }
    output.classList.toggle('no-wrap', !wrapToggle?.checked);
    term.options.fontSize = fontSize;
    fitActiveTerminal();
    setTimeout(() => {
      focusActiveTerminal();
      fitActiveTerminal();
    }, 30);
  }

  function fitActiveTerminal() {
    const item = activeSession();
    if (!item?.term) {
      terminalSize();
      return;
    }
    try {
      item.fitAddon?.fit();
    } catch (_) {}
    const size = {cols: item.term.cols || 120, rows: item.term.rows || 36};
    if (sizeLabel) sizeLabel.textContent = size.cols + 'x' + size.rows;
  }

  function disposeTerminal(item) {
    try { item?.dataDisposable?.dispose?.(); } catch (_) {}
    try { item?.resizeDisposable?.dispose?.(); } catch (_) {}
    try { item?.term?.dispose?.(); } catch (_) {}
    if (item) {
      item.term = null;
      item.fitAddon = null;
      item.searchAddon = null;
      item.dataDisposable = null;
      item.resizeDisposable = null;
    }
  }

  function attachWebSocket(item, reconnecting) {
    if (!item?.id || !item.wsUrl) return;
    try {
      if (item.socket && item.socket.readyState !== WebSocket.CLOSED) item.socket.close();
    } catch (_) {}
    item.status = 'connecting';
    if (!item.output) item.output = reconnecting ? '正在恢复终端会话...\n' : '正在连接终端...\n';
    if (item.id === activeSessionId) {
      if (!item.term && output) output.textContent = sanitizeTerminalText(item.output);
      setStatus('连接中', 'pending');
    }
    renderSessionTabs();
    try {
      const ws = new WebSocket(absoluteWSURL(item.wsUrl));
      item.socket = ws;
      ws.binaryType = 'arraybuffer';
      ws.onopen = function () {
        item.status = 'connected';
        if (item.id === activeSessionId) setStatus('已连接', 'ok');
        renderSessionTabs();
        focusActiveTerminal();
        sendResize();
      };
      ws.onmessage = function (event) {
        appendToSession(item, event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : event.data);
      };
      ws.onerror = function () {
        item.status = 'failed';
        appendToSession(item, '\n终端连接异常：请确认当前平台账号已映射 Linux/LDAP 用户，登录节点可达，且用户 SSH 互信已初始化。\n');
        if (item.id === activeSessionId) setStatus('连接异常', 'danger');
        renderSessionTabs();
      };
      ws.onclose = function () {
        item.status = item.status === 'failed' ? 'failed' : 'disconnected';
        appendToSession(item, '\n[会话已断开]\n');
        if (item.id === activeSessionId) setStatus(sessionStatusText(item.status), sessionStatusTone(item.status));
        renderSessionTabs();
      };
    } catch (error) {
      item.status = 'failed';
      appendToSession(item, 'WebSocket 初始化失败：' + error.message + '\n');
      if (item.id === activeSessionId) setStatus('连接异常', 'danger');
      renderSessionTabs();
    }
  }

  async function reconnectSession(id, silent) {
    const item = sessions.get(id);
    if (!item) return;
    try {
      const data = await fetchJSON('/api/v1/webssh/sessions/' + encodeURIComponent(id) + '/reconnect', {method: 'POST'});
      if (data.wsUrl) item.wsUrl = data.wsUrl;
      attachWebSocket(item, true);
    } catch (error) {
      item.status = 'failed';
      if (!silent) toast('终端会话恢复失败：' + error.message, 'danger');
      if (id === activeSessionId) setStatus('连接异常', 'danger');
      renderSessionTabs();
    }
  }

  async function loadExistingSessions() {
    try {
      const data = await fetchJSON('/api/v1/webssh/sessions');
      const existing = (data.sessions || data.items || []).map(normalizeSession).filter(item => item.id);
      if (!existing.length) {
        showEmptySessionState();
        return;
      }
      existing.forEach(item => sessions.set(item.id, item));
      const preferred = existing.find(item => item.status !== 'closed') || existing[0];
      activateSession(preferred.id);
    } catch (error) {
      showEmptySessionState();
      toast('终端会话恢复失败：' + error.message, 'warn');
    }
  }

  async function connect() {
    if (connectBtn) connectBtn.disabled = true;
    let created;
    try {
      created = await createWebSSHSession();
    } catch (error) {
      toast('终端会话创建失败：' + error.message, 'danger');
      if (connectBtn) connectBtn.disabled = !hasLoginNodes();
      return;
    }
    const id = created.sessionId || '';
    if (!id) {
      toast('终端会话创建失败：后端未返回 sessionId', 'danger');
      if (connectBtn) connectBtn.disabled = !hasLoginNodes();
      return;
    }
    const item = {
      id,
      wsUrl: created.wsUrl || '',
      node: created.node || activeNodeName(),
      username: created.username || currentUser,
      ownerUsername: created.ownerUsername || currentUser,
      status: 'connecting',
      output: '正在连接终端...\n',
      socket: null,
      path: currentPath || '/data/home/' + currentUser
    };
    sessions.set(id, item);
    activateSession(id);

    attachWebSocket(item, false);
    if (connectBtn) connectBtn.disabled = !hasLoginNodes();
  }

  async function closeSession(id) {
    const item = sessions.get(id);
    if (!item) return;
    if ((item.status === 'connected' || item.status === 'connecting') && !confirm('确认断开并关闭该终端会话？')) return;
    try {
      item.socket?.close();
    } catch (_) {}
    disposeTerminal(item);
    await fetch('/api/v1/webssh/sessions/' + encodeURIComponent(id), {
      method: 'DELETE',
      headers: authHeaders,
      credentials: 'same-origin'
    }).catch(() => {});
    sessions.delete(id);
    if (activeSessionId === id) {
      const next = sessions.keys().next().value;
      if (next) activateSession(next);
      else showEmptySessionState();
    } else {
      renderSessionTabs();
    }
  }

  function disconnect() {
    if (activeSessionId) closeSession(activeSessionId);
  }

  function send(value) {
    const item = activeSession();
    if (!item?.socket || item.socket.readyState !== WebSocket.OPEN) return false;
    item.socket.send(value);
    return true;
  }

  async function sendResize(size) {
    const item = activeSession();
    if (!item?.socket || item.socket.readyState !== WebSocket.OPEN) return;
    const nextSize = size || terminalSize();
    item.socket.send(JSON.stringify(Object.assign({type: 'resize'}, nextSize)));
    fetch('/api/v1/webssh/sessions/' + encodeURIComponent(item.id) + '/resize', {
      method: 'POST',
      headers: jsonHeaders,
      credentials: 'same-origin',
      body: JSON.stringify(nextSize)
    }).catch(() => {});
  }

  function onKeyDown(event) {
    if (xtermAvailable) return;
    const item = activeSession();
    if (!item || item.status !== 'connected') return;
    let value = '';
    if (event.ctrlKey) {
      const key = event.key.toLowerCase();
      if (key === 'c') value = '\x03';
      else if (key === 'd') value = '\x04';
      else if (key === 'l') value = '\x0c';
      else return;
    } else if (event.key === 'Enter') value = '\r';
    else if (event.key === 'Backspace') value = '\x7f';
    else if (event.key === 'Tab') value = '\t';
    else if (event.key === 'ArrowUp') value = '\x1b[A';
    else if (event.key === 'ArrowDown') value = '\x1b[B';
    else if (event.key === 'ArrowRight') value = '\x1b[C';
    else if (event.key === 'ArrowLeft') value = '\x1b[D';
    else if (event.key === 'Home') value = '\x1b[H';
    else if (event.key === 'End') value = '\x1b[F';
    else if (event.key.length === 1 && !event.metaKey && !event.altKey) value = event.key;
    else return;
    event.preventDefault();
    send(value);
  }

  function updatePathMeta(data) {
    currentPath = data.effectivePath || currentPath;
    parentPath = data.parentPath || '';
    canGoParent = Boolean(data.canGoParent);
    if (currentPathLabel) currentPathLabel.textContent = currentPath || '数据未获取';
    if (sessionPath) sessionPath.textContent = activeSession()?.path || currentPath || ('/data/home/' + currentUser);
    if (parentBtn) parentBtn.disabled = !canGoParent;
  }

  function renderRoots() {
    if (!fileTree) return;
    if (!storageRoots.length) {
      fileTree.innerHTML = '<div class="shpc-webssh-tree-empty">当前账号没有可访问的存储目录</div>';
      return;
    }
    const rootsHTML = storageRoots.map((root, index) => {
      const path = root.effectivePath || root.path || '';
      const active = path === currentPath || currentPath.startsWith(path + '/') ? ' selected' : '';
      const label = root.name || root.path || '存储目录';
      return '<option value="' + index + '"' + active + ' title="' + escapeHTML(path) + '">' +
        escapeHTML(label + (path ? ' / ' + path : '')) + '</option>';
    }).join('');
    const activeRoot = storageRoots.find(root => {
      const path = root.effectivePath || root.path || '';
      return path && (path === currentPath || currentPath.startsWith(path + '/'));
    });
    const activePath = activeRoot?.effectivePath || activeRoot?.path || currentPath || '';
    fileTree.innerHTML = '<div class="shpc-webssh-root-select">' +
      '<label for="terminalRootSelect">授权目录</label><select id="terminalRootSelect" title="' + escapeHTML(activePath) + '">' + rootsHTML + '</select></div>' +
      '<div class="shpc-webssh-current-list"><div class="shpc-webssh-current-head"><strong>当前目录</strong><button type="button" id="terminalPathTerminalBtn">进入终端</button></div>' +
      '<div class="shpc-webssh-selection-bar" id="terminalSelectionBar" hidden><span id="terminalSelectionText">已选择 0 项</span><button type="button" id="terminalArchiveSelectedBtn">打包下载</button><button type="button" id="terminalClearSelectionBtn">清空</button></div>' +
      '<div id="terminalEntryList" class="shpc-webssh-entry-scroll"></div></div>';
    document.getElementById('terminalRootSelect')?.addEventListener('change', event => {
      const root = storageRoots[Number(event.target.value)];
      if (root) {
        if (fileSearch) fileSearch.value = '';
        loadDirectory(root.effectivePath || root.path);
      }
    });
    document.getElementById('terminalPathTerminalBtn')?.addEventListener('click', () => enterPathInTerminal(currentPath));
    document.getElementById('terminalArchiveSelectedBtn')?.addEventListener('click', archiveSelected);
    document.getElementById('terminalClearSelectionBtn')?.addEventListener('click', clearSelection);
    renderEntries();
  }

  function renderEntries() {
    const container = document.getElementById('terminalEntryList');
    if (!container) return;
    const keyword = (fileSearch?.value || '').trim().toLowerCase();
    const entries = currentEntries.filter(item => !keyword || String(item.name || '').toLowerCase().includes(keyword));
    renderSelectionBar();
    if (!entries.length) {
      container.innerHTML = '<div class="shpc-webssh-tree-empty">' + (keyword ? '未找到匹配的文件或目录' : '当前目录为空') + '</div>';
      return;
    }
    container.innerHTML = entries.map(item => {
      const actualIndex = currentEntries.indexOf(item);
      const isDir = item.type === 'directory';
      const icon = isDir ? '<i></i>' : '<em>' + fileExtLabel(item.name) + '</em>';
      const checked = item.path && selectedPaths.has(item.path) ? ' checked' : '';
      const title = item.path || item.name || '';
      return '<div class="shpc-webssh-entry ' + (isDir ? 'is-dir' : 'is-file') + '" data-entry-index="' + actualIndex + '">' +
        '<input class="terminal-entry-check" type="checkbox" data-entry-select="' + actualIndex + '"' + checked + ' aria-label="选择 ' + escapeHTML(item.name) + '">' +
        icon + '<button type="button" class="terminal-entry-name" title="' + escapeHTML(title) + '" data-entry-action="' + actualIndex + '">' + escapeHTML(item.name) + '</button>' +
        '<small class="terminal-entry-size">' + escapeHTML(formatFileSize(item.size, isDir)) + '</small>' +
        '<div class="terminal-entry-actions">' +
          (isDir ? '<button type="button" data-entry-terminal="' + actualIndex + '">进终端</button><button type="button" data-entry-download="' + actualIndex + '">下载</button>' : '<button type="button" data-entry-download="' + actualIndex + '">下载</button>') +
        '</div></div>';
    }).join('');
    container.querySelectorAll('[data-entry-select]').forEach(input => {
      input.addEventListener('change', () => toggleSelection(Number(input.dataset.entrySelect), input.checked));
    });
    container.querySelectorAll('[data-entry-action]').forEach(button => {
      button.addEventListener('click', () => openEntry(Number(button.dataset.entryAction)));
    });
    container.querySelectorAll('[data-entry-download]').forEach(button => {
      button.addEventListener('click', () => downloadEntry(Number(button.dataset.entryDownload)));
    });
    container.querySelectorAll('[data-entry-terminal]').forEach(button => {
      button.addEventListener('click', () => enterEntryInTerminal(Number(button.dataset.entryTerminal)));
    });
  }

  function renderSelectionBar() {
    const bar = document.getElementById('terminalSelectionBar');
    const text = document.getElementById('terminalSelectionText');
    if (!bar || !text) return;
    const count = selectedPaths.size;
    bar.hidden = count === 0;
    text.textContent = '已选择 ' + count + ' 项';
  }

  function toggleSelection(index, checked) {
    const item = currentEntries[index];
    if (!item?.path) return;
    if (checked) selectedPaths.add(item.path);
    else selectedPaths.delete(item.path);
    renderSelectionBar();
  }

  function clearSelection() {
    selectedPaths.clear();
    renderEntries();
  }

  function fileExtLabel(name) {
    const ext = String(name || '').split('.').pop();
    if (!ext || ext === name) return 'F';
    return ext.slice(0, 2).toUpperCase();
  }

  function formatFileSize(size, isDir) {
    if (isDir) return '目录';
    const value = Number(size || 0);
    if (!Number.isFinite(value) || value <= 0) return '0 B';
    if (value < 1024) return value + ' B';
    if (value < 1024 * 1024) return (value / 1024).toFixed(value < 10 * 1024 ? 1 : 0) + ' KB';
    if (value < 1024 * 1024 * 1024) return (value / 1024 / 1024).toFixed(value < 10 * 1024 * 1024 ? 1 : 0) + ' MB';
    return (value / 1024 / 1024 / 1024).toFixed(1) + ' GB';
  }

  async function loadRoots() {
    try {
      const data = await fetchJSON('/api/v1/webssh/files/tree');
      storageRoots = data.roots || data.items || [];
      renderRoots();
      if (storageRoots.length) {
        await loadDirectory(storageRoots[0].effectivePath || storageRoots[0].path);
      }
    } catch (error) {
      if (fileTree) fileTree.innerHTML = '<div class="shpc-webssh-tree-empty">目录数据未获取：' + escapeHTML(error.message) + '</div>';
    }
  }

  async function loadDirectory(path) {
    if (!path) return;
    try {
      const data = await fetchJSON('/api/v1/webssh/files/list?path=' + encodeURIComponent(path) + '&showHidden=false');
      currentEntries = data.entries || data.items || [];
      selectedPaths = new Set();
      updatePathMeta(data);
      renderRoots();
    } catch (error) {
      toast('目录读取失败：' + error.message, 'danger');
    }
  }

  function openEntry(index) {
    const item = currentEntries[index];
    if (!item) return;
    if (item.type === 'directory') loadDirectory(item.path);
    else downloadEntry(index);
  }

  async function downloadEntry(index) {
    const item = currentEntries[index];
    if (!item) return;
    if (item.type === 'directory') {
      await archivePaths([item.path], (item.name || 'directory') + '.zip');
      return;
    }
    try {
      toast('正在下载：' + item.name, 'info');
      const response = await fetch('/api/v1/webssh/files/download?path=' + encodeURIComponent(item.path), {
        headers: authHeaders,
        credentials: 'same-origin'
      });
      if (!response.ok) {
        const text = await response.text().catch(() => '');
        throw new Error(text || `HTTP ${response.status}`);
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = item.name || 'download';
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      toast('下载已开始：' + item.name, 'success');
    } catch (error) {
      toast('下载失败：' + error.message, 'danger');
    }
  }

  function downloadBlob(blob, filename) {
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename || 'download';
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  async function archivePaths(paths, filename) {
    const safePaths = (paths || []).filter(Boolean);
    if (!safePaths.length) {
      toast('请先选择文件或目录', 'warn');
      return;
    }
    try {
      toast('正在打包下载：' + safePaths.length + ' 项', 'info');
      const response = await fetch('/api/v1/webssh/files/archive', {
        method: 'POST',
        headers: jsonHeaders,
        credentials: 'same-origin',
        body: JSON.stringify({paths: safePaths})
      });
      if (!response.ok) {
        const data = await response.json().catch(() => null);
        const text = data?.error || data?.message || await response.text().catch(() => '');
        throw new Error(text || `HTTP ${response.status}`);
      }
      downloadBlob(await response.blob(), filename || 'simplehpc-selected.zip');
      toast('打包下载已开始', 'success');
    } catch (error) {
      toast('打包下载失败：' + error.message, 'danger');
    }
  }

  async function archiveSelected() {
    await archivePaths(Array.from(selectedPaths), 'simplehpc-selected.zip');
  }

  function uploadConfirmModal(files, targetPath) {
    const fileList = Array.from(files || []);
    if (!fileList.length) return Promise.resolve(false);
    const totalSize = fileList.reduce((sum, file) => sum + Number(file.size || 0), 0);
    const preview = fileList.slice(0, 6).map(file =>
      '<li><span class="webssh-upload-file-name">' + escapeHTML(file.name) + '</span><small>' + escapeHTML(formatFileSize(file.size, false)) + '</small></li>'
    ).join('');
    const more = fileList.length > 6 ? '<li class="webssh-upload-more">还有 ' + (fileList.length - 6) + ' 个文件未展示</li>' : '';
    const content =
      '<div class="webssh-upload-confirm">' +
        '<div class="webssh-upload-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12 16V4"/><path d="m7 9 5-5 5 5"/><path d="M5 16v3a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-3"/></svg></div>' +
        '<div class="webssh-upload-copy"><h3>确认上传数据</h3><p>文件将上传到当前授权目录，上传前后端仍会校验目录权限。</p></div>' +
        '<div class="webssh-upload-target"><span>上传到</span><code>' + escapeHTML(targetPath) + '</code></div>' +
        '<div class="webssh-upload-summary"><strong>' + fileList.length + ' 个文件</strong><span>总大小 ' + escapeHTML(formatFileSize(totalSize, false)) + '</span></div>' +
        '<ul class="webssh-upload-files">' + preview + more + '</ul>' +
      '</div>';
    return new Promise(resolve => {
      if (!window.App?.modal) {
        resolve(true);
        return;
      }
      let submitted = false;
      window.App.modal({
        title: '上传数据',
        width: '560px',
        content,
        confirmText: '开始上传',
        cancelText: '取消',
        onSubmit: async () => {
          submitted = true;
          resolve(true);
        },
        onClose: () => {
          if (!submitted) resolve(false);
        }
      });
    });
  }

  async function uploadFiles(files) {
    if (!currentPath) {
      toast('请先选择上传目录', 'warn');
      return;
    }
    if (!(await uploadConfirmModal(files, currentPath))) return;
    for (const file of files) {
      const form = new FormData();
      form.append('path', currentPath);
      form.append('file', file);
      const response = await fetch('/api/v1/webssh/files/upload', {method: 'POST', headers: authHeaders, credentials: 'same-origin', body: form});
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    }
    toast('上传完成：' + files.length + ' 个文件', 'success');
    await loadDirectory(currentPath);
  }

  async function copyText(text, message) {
    const value = String(text || '');
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard API unavailable');
      await navigator.clipboard.writeText(value);
      toast(message || '已复制', 'success');
      return true;
    } catch (error) {
      const textarea = document.createElement('textarea');
      textarea.value = value;
      textarea.setAttribute('readonly', 'readonly');
      textarea.style.position = 'fixed';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      let ok = false;
      try {
        ok = document.execCommand('copy');
      } catch (_) {
        ok = false;
      }
      textarea.remove();
      if (ok) {
        toast(message || '已复制', 'success');
        return true;
      }
      toast('复制受限，请检查浏览器权限', 'warn');
      return false;
    }
  }

  function modalOrPrompt(opts) {
    if (window.App?.modal) return window.App.modal(opts);
    return null;
  }

  function searchTerminalOutput() {
    const session = activeSession();
    const text = session?.term ? sanitizeTerminalText(session.output || '') : (output?.textContent || '');
    if (!text.trim()) {
      toast(activeSessionId ? '当前终端暂无可搜索内容' : '请先创建并连接终端会话', 'warn');
      return;
    }
    const defaultValue = lastTerminalSearchTerm || '';
    let modal;
    const content =
      '<div class="_shpc-form-grid--single" style="display:flex;flex-direction:column;gap:14px">' +
        '<div class="_shpc-field"><label>搜索终端输出</label><input id="terminalSearchKeyword" autocomplete="off" placeholder="输入关键字，回车或点击定位下一处" value="' + escapeHTML(defaultValue) + '"></div>' +
        '<p style="margin:0;color:#6b7280;font-size:13px;line-height:1.6">搜索范围为当前终端缓冲区输出；再次搜索同一关键字会定位下一处。</p>' +
      '</div>';
    const run = () => {
      const input = modal?.el?.querySelector('#terminalSearchKeyword');
      const term = (input?.value || defaultValue || '').trim();
      if (!term) throw new Error('请输入搜索关键字');
      locateTerminalText(term);
    };
    modal = modalOrPrompt({
      title: '搜索终端输出',
      width: '520px',
      content,
      confirmText: '定位下一处',
      cancelText: '关闭',
      errorPrefix: '搜索失败',
      onSubmit: async () => run()
    });
    if (!modal) {
      const term = prompt('搜索终端输出：', defaultValue);
      if (term) locateTerminalText(term);
      return;
    }
    const input = modal.el.querySelector('#terminalSearchKeyword');
    input?.focus();
    input?.select();
    input?.addEventListener('keydown', event => {
      if (event.key === 'Enter') {
        event.preventDefault();
        try {
          run();
        } catch (error) {
          toast(error.message, 'warn');
        }
      }
    });
  }

  function locateTerminalText(term) {
    const session = activeSession();
    if (session?.searchAddon) {
      const ok = session.searchAddon.findNext(term, {
        caseSensitive: false,
        wholeWord: false,
        regex: false,
        decorations: {
          matchBackground: '#f59e0b55',
          activeMatchBackground: '#2563eb88',
          matchBorder: '#f59e0b',
          activeMatchBorder: '#ffffff',
          matchOverviewRuler: '#f59e0b',
          activeMatchColorOverviewRuler: '#2563eb'
        }
      });
      focusActiveTerminal();
      toast(ok ? '已定位匹配内容' : ('未找到匹配内容：' + term), ok ? 'success' : 'warn');
      return;
    }
    const text = output?.textContent || '';
    const source = text.toLowerCase();
    const needle = String(term || '').toLowerCase();
    if (!needle) return;
    let start = 0;
    if (needle === lastTerminalSearchTerm.toLowerCase()) {
      start = Math.max(0, lastTerminalSearchIndex + needle.length);
    }
    let index = source.indexOf(needle, start);
    if (index < 0 && start > 0) index = source.indexOf(needle, 0);
    if (index < 0) {
      lastTerminalSearchTerm = term;
      lastTerminalSearchIndex = -1;
      toast('未找到匹配内容：' + term, 'warn');
      screen?.focus();
      return;
    }
    lastTerminalSearchTerm = term;
    lastTerminalSearchIndex = index;
    const before = text.slice(0, index);
    const line = before.split('\n').length;
    const style = getComputedStyle(output);
    const px = parseFloat(style.fontSize) || fontSize;
    const lineHeight = parseFloat(style.lineHeight) || px * 1.5;
    screen.scrollTop = Math.max(0, (line - 5) * lineHeight);
    const node = output.firstChild;
    if (node && node.nodeType === Node.TEXT_NODE) {
      const range = document.createRange();
      range.setStart(node, index);
      range.setEnd(node, Math.min(text.length, index + term.length));
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    }
    screen?.focus();
    toast('已定位匹配内容', 'success');
  }

  function openManualPasteModal() {
    let modal;
    const content =
      '<div class="_shpc-form-grid--single" style="display:flex;flex-direction:column;gap:14px">' +
        '<div class="_shpc-field"><label>手动粘贴到终端</label><textarea id="terminalManualPasteText" rows="7" placeholder="浏览器限制自动读取剪贴板时，请在这里粘贴内容"></textarea></div>' +
        '<p style="margin:0;color:#6b7280;font-size:13px;line-height:1.6">确认后会把文本发送到当前已连接终端。请确认内容安全，避免误执行敏感命令。</p>' +
      '</div>';
    modal = modalOrPrompt({
      title: '粘贴到终端',
      width: '560px',
      content,
      confirmText: '发送到终端',
      cancelText: '取消',
      errorPrefix: '粘贴失败',
      onSubmit: async () => {
        const text = modal.el.querySelector('#terminalManualPasteText')?.value || '';
        if (!text) throw new Error('请输入需要粘贴的内容');
        if (!send(text)) throw new Error('当前终端未连接');
        screen?.focus();
      }
    });
    if (!modal) {
      const text = prompt('浏览器限制读取剪贴板，请在此粘贴内容后确认发送：', '');
      if (text) send(text);
      screen?.focus();
      return;
    }
    modal.el.querySelector('#terminalManualPasteText')?.focus();
  }

  function toggleWrap() {
    if (!wrapToggle || !output) return;
    wrapToggle.checked = !wrapToggle.checked;
    output.classList.toggle('no-wrap', !wrapToggle.checked);
    fitActiveTerminal();
  }

  function openTerminalSettings() {
    const session = activeSession();
    const content =
      '<div class="_shpc-form-grid--single" style="display:flex;flex-direction:column;gap:16px">' +
        '<div style="display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px">' +
          '<div class="soft-card" style="padding:14px;border:1px solid #e5e7eb;border-radius:14px;background:#f8fafc"><small style="color:#64748b">会话状态</small><strong style="display:block;margin-top:6px">' + escapeHTML(sessionStatusText(session?.status || 'disconnected')) + '</strong></div>' +
          '<div class="soft-card" style="padding:14px;border:1px solid #e5e7eb;border-radius:14px;background:#f8fafc"><small style="color:#64748b">字号</small><strong style="display:block;margin-top:6px">' + escapeHTML(fontSize + 'px') + '</strong></div>' +
        '</div>' +
        '<div style="display:flex;gap:10px;flex-wrap:wrap">' +
          '<button type="button" class="btn btn-ghost" onclick="window.SimpleHPCTerminal?.fontMinus?.()">字体缩小 A-</button>' +
          '<button type="button" class="btn btn-ghost" onclick="window.SimpleHPCTerminal?.fontPlus?.()">字体放大 A+</button>' +
          '<button type="button" class="btn btn-ghost" onclick="window.SimpleHPCTerminal?.toggleWrap?.()">切换自动换行</button>' +
        '</div>' +
        '<div style="display:grid;grid-template-columns:120px 1fr;gap:8px 12px;font-size:13px;color:#475569">' +
          '<span>会话 ID</span><code>' + escapeHTML(session?.id || '—') + '</code>' +
          '<span>登录节点</span><code>' + escapeHTML(session?.node || activeNodeName()) + '</code>' +
          '<span>登录用户</span><code>' + escapeHTML(session?.username || currentUser) + '</code>' +
          '<span>当前路径</span><code>' + escapeHTML(session?.path || currentPath || '—') + '</code>' +
          '<span>字符编码</span><code>UTF-8</code>' +
        '</div>' +
      '</div>';
    if (window.App?.modal) {
      window.App.modal({
        title: '终端设置',
        width: '620px',
        content,
        confirmText: '关闭',
        cancelText: '关闭',
        onSubmit: async () => {}
      });
    } else {
      alert('终端设置\n字号：' + fontSize + 'px\n会话：' + (session?.id || '—'));
    }
  }

  function copyEntryPath(index) {
    const item = currentEntries[index];
    if (item?.path) copyText(item.path, '文件路径已复制');
  }

  function enterEntryInTerminal(index) {
    const item = currentEntries[index];
    if (item?.path) enterPathInTerminal(item.path);
  }

  function enterPathInTerminal(path) {
    if (!path) return;
    const target = String(path).replace(/'/g, "'\\''");
    if (!send("cd '" + target + "'\n")) {
      toast('请先创建并连接终端会话', 'warn');
      return;
    }
    const item = activeSession();
    if (item) item.path = path;
    if (sessionPath) sessionPath.textContent = path;
  }

  async function loadTerminalConfig() {
    try {
      const data = await fetchJSON('/api/v1/webssh/nodes');
      terminalConfig = {strategy: data.strategy || 'round_robin', nodes: data.nodes || []};
      const enabled = (terminalConfig.nodes || []).filter(node => node.enabled !== false);
      if (enabled.length) {
        selectedNode = enabled[0].name || enabled[0].hostname || enabled[0].host || enabled[0].address || '';
      } else {
        selectedNode = '';
      }
      updateNodeLabel();
    } catch (error) {
      terminalConfig = {strategy: 'round_robin', nodes: []};
      selectedNode = '';
      updateNodeLabel();
      toast('登录节点配置读取失败：' + error.message, 'danger');
    }
  }

  function updateNodeLabel() {
    if (!hasLoginNodes()) {
      if (loginNodeLabel) loginNodeLabel.innerHTML = '未配置登录节点 <em>需设置</em>';
      if (latencyLabel) latencyLabel.textContent = '不可用';
      if (latencyBadge) latencyBadge.textContent = '待配置';
      if (connectBtn) connectBtn.disabled = true;
      if (!activeSessionId && sessionTitle) sessionTitle.textContent = '未配置登录节点';
      if (!activeSessionId && emptyState) {
        emptyState.hidden = false;
        emptyState.innerHTML = '<strong>WebSSH 暂不可用</strong><p>' + escapeHTML(loginNodeMissingMessage()) + '</p><p><a href="settings.html">前往系统设置配置登录节点 →</a></p>';
      }
      setStatus('需配置登录节点', 'danger');
      return;
    }
    const name = activeNodeName();
    if (loginNodeLabel) loginNodeLabel.innerHTML = escapeHTML(name) + ' <em>在线</em>';
    if (latencyLabel) latencyLabel.textContent = terminalConfig.strategy === 'least_sessions' ? '负载' : '轮询';
    if (latencyBadge) latencyBadge.textContent = terminalConfig.strategy === 'least_sessions' ? '负载均衡' : '轮询分配';
    if (connectBtn) connectBtn.disabled = false;
    if (!activeSessionId && sessionTitle) sessionTitle.textContent = name;
  }

  function chooseNode() {
    const enabled = (terminalConfig.nodes || []).filter(node => node.enabled !== false);
    if (!enabled.length) {
      toast(loginNodeMissingMessage(), 'warn');
      return;
    }
    const options = enabled.map((node, index) => `${index + 1}. ${node.name || node.hostname || node.host || node.address} ${node.host || node.address ? '(' + (node.host || node.address) + ')' : ''}`).join('\n');
    const raw = prompt('选择登录节点编号：\n' + options, '1');
    const index = Number(raw) - 1;
    if (!Number.isInteger(index) || index < 0 || index >= enabled.length) return;
    selectedNode = enabled[index].name || enabled[index].hostname || enabled[index].host || enabled[index].address;
    updateNodeLabel();
  }

  function applyUser(user) {
    currentUser = user?.username || user?.name || currentUser;
    if (sessionUser) sessionUser.textContent = currentUser;
    if (!currentPath && currentPathLabel) currentPathLabel.textContent = '/data/home/' + currentUser;
    if (!currentPath && sessionPath) sessionPath.textContent = '/data/home/' + currentUser;
  }

  function fontMinus() {
    fontSize = Math.max(12, fontSize - 1);
    const item = activeSession();
    if (item?.term) item.term.options.fontSize = fontSize;
    else output.style.fontSize = fontSize + 'px';
    fitActiveTerminal();
    sendResize();
  }

  function fontPlus() {
    fontSize = Math.min(22, fontSize + 1);
    const item = activeSession();
    if (item?.term) item.term.options.fontSize = fontSize;
    else output.style.fontSize = fontSize + 'px';
    fitActiveTerminal();
    sendResize();
  }

  connectBtn?.addEventListener('click', connect);
  document.querySelector('.shpc-webssh-node')?.addEventListener('click', chooseNode);
  disconnectBtn?.addEventListener('click', disconnect);
  searchBtn?.addEventListener('click', searchTerminalOutput);
  clearBtn?.addEventListener('click', function () {
    const item = activeSession();
    if (item) item.output = '';
    if (item?.term) {
      item.term.clear();
      focusActiveTerminal();
    } else {
      output.textContent = '';
      focusActiveTerminal();
    }
  });
  copyBtn?.addEventListener('click', function () {
    const item = activeSession();
    const selected = item?.term?.getSelection?.() || '';
    const text = selected || sanitizeTerminalText(item?.output || output.textContent || '');
    copyText(text, selected ? '已复制选中内容' : '终端输出已复制');
  });
  pasteBtn?.addEventListener('click', async function () {
    try {
      if (!navigator.clipboard?.readText) throw new Error('Clipboard API unavailable');
      const text = await navigator.clipboard.readText();
      if (text) send(text);
      focusActiveTerminal();
    } catch (_) {
      setStatus('粘贴受限', 'danger');
      openManualPasteModal();
    }
  });
  fullscreenBtn?.addEventListener('click', function () {
    const card = document.querySelector('.shpc-webssh-terminal-card');
    if (!document.fullscreenElement) card?.requestFullscreen?.();
    else document.exitFullscreen?.();
    setTimeout(() => {
      fitActiveTerminal();
      sendResize();
      focusActiveTerminal();
    }, 180);
  });
  wrapToggle?.addEventListener('change', function () {
    output.classList.toggle('no-wrap', !wrapToggle.checked);
    fitActiveTerminal();
  });
  settingsBtn?.addEventListener('click', openTerminalSettings);
  pathCopyBtn?.addEventListener('click', function () {
    copyText(currentPath || currentPathLabel?.textContent || '', '当前路径已复制');
  });
  parentBtn?.addEventListener('click', function () {
    if (canGoParent && parentPath) loadDirectory(parentPath);
  });
  fileRefreshBtn?.addEventListener('click', function () {
    if (currentPath) loadDirectory(currentPath);
    else loadRoots();
  });
  fileUploadBtn?.addEventListener('click', function () {
    fileUploadInput?.click();
  });
  fileUploadInput?.addEventListener('change', function () {
    if (this.files.length) uploadFiles(this.files).catch(error => toast('上传失败：' + error.message, 'danger'));
    this.value = '';
  });
  fileSearch?.addEventListener('input', function () {
    clearTimeout(fileSearchTimer);
    fileSearchTimer = setTimeout(renderEntries, 200);
  });
  screen?.addEventListener('keydown', onKeyDown);
  screen?.addEventListener('mousedown', function () {
    focusActiveTerminal();
  });
  screen?.addEventListener('click', function () {
    focusActiveTerminal();
  });
  window.addEventListener('resize', function () {
    setTimeout(() => {
      fitActiveTerminal();
      sendResize();
    }, 120);
  });

  document.addEventListener('simplehpc:authz-ready', function (event) {
    applyUser(event.detail?.user || window.App?.currentUser || window.App?.authz?.user || {});
    if (window.App?.authz && !window.App.authz.can('action.webssh.terminal.create') && !window.App.authz.can('action.terminal.connect')) {
      connectBtn.disabled = true;
      setStatus('无终端权限', 'danger');
    }
  });

  window.SimpleHPCTerminal = {fontMinus, fontPlus, toggleWrap, searchTerminalOutput, openTerminalSettings};
  terminalSize();
  if (output) output.style.fontSize = fontSize + 'px';
  async function bootstrap() {
    showEmptySessionState();
    await loadTerminalConfig();
    await loadRoots();
    await loadExistingSessions();
  }

  bootstrap();
})();
