(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.SessionUI = api;
}(typeof window !== 'undefined' ? window : globalThis, function () {
  'use strict';

  function viewModel(user) {
    user = user || {};
    const name = user.displayName || user.username || '未登录';
    return {
      name,
      account: user.username || '',
      role: user.role || user.type || '',
      avatar: name.slice(0, 1)
    };
  }

  function healthMessage(services) {
    return Object.entries(services || {})
      .map(([name, item]) => `${name}: ${(item || {}).status || 'unknown'}`)
      .join(' · ');
  }

  function renderIdentity(documentRef, storage) {
    let user = {};
    try {
      user = JSON.parse(storage.getItem('simplehpc_user') || '{}') || {};
    } catch (error) {
      user = {};
    }
    const view = viewModel(user);
    documentRef.querySelectorAll('.sidebar-user').forEach(element => {
      const avatar = element.querySelector('.avatar');
      const detail = Array.from(element.children).find(child => !child.classList.contains('avatar'));
      const labels = detail ? Array.from(detail.children) : [];
      if (avatar) avatar.textContent = view.avatar;
      if (labels[0]) labels[0].textContent = view.name;
      if (labels[1]) labels[1].textContent = view.role || view.account;
    });
    documentRef.querySelectorAll('.header-actions > .avatar').forEach(avatar => {
      avatar.textContent = view.avatar;
    });
    return view;
  }

  async function renderHealth(documentRef, fetchRef) {
    const buttons = documentRef.querySelectorAll('.btn-icon[aria-label="通知"]');
    try {
      const response = await fetchRef('/api/health', {cache: 'no-store'});
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
      const message = healthMessage(data.services);
      buttons.forEach(button => {
        button.dataset.dropdown = JSON.stringify({
          html: `<div class="_shpc-dropdown-item">实时服务状态</div><div class="_shpc-dropdown-item">${message}</div>`,
          width: '360px',
          placement: 'bottom-end'
        });
        button.title = message;
      });
      return data;
    } catch (error) {
      buttons.forEach(button => {
        button.dataset.dropdown = JSON.stringify({
          html: `<div class="_shpc-dropdown-item">服务状态获取失败：${String(error.message || error)}</div>`,
          width: '360px',
          placement: 'bottom-end'
        });
      });
      return null;
    }
  }

  function init(documentRef, storage, fetchRef) {
    renderIdentity(documentRef, storage);
    return renderHealth(documentRef, fetchRef);
  }

  return {viewModel, healthMessage, renderIdentity, renderHealth, init};
}));
