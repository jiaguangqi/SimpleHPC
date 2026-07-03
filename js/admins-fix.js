// Admin page functions
function editAdmin(username, role, email, status) {
  var html = '<div class="_shpc-form-grid">' +
    '<div class="_shpc-field"><label>管理员账号</label><input id="editAdminUsername" value="' + username + '" readonly style="background:var(--bg);color:var(--muted);"></div>' +
    '<div class="_shpc-field"><label>绑定邮箱</label><input id="editAdminEmail" type="email" value="' + email + '" placeholder="admin@seu.edu.cn"></div>' +
    '<div class="_shpc-field"><label>角色</label><select id="editAdminRole"><option' + (role==='超级管理员'?' selected':'') + '>超级管理员</option><option' + (role==='集群管理员'?' selected':'') + '>集群管理员</option><option' + (role==='配置管理员'?' selected':'') + '>配置管理员</option><option' + (role==='单位管理员'?' selected':'') + '>单位管理员</option><option' + (role==='审计助理'?' selected':'') + '>审计助理</option></select></div>' +
    '<div class="_shpc-field"><label>状态</label><select id="editAdminStatus"><option value="正常"' + (status==='正常'?' selected':'') + '>正常</option><option value="冻结"' + (status==='冻结'?' selected':'') + '>冻结</option></select></div>' +
    '</div>';
  
  App.drawer({
    title: '编辑管理员 — ' + username,
    width: '680px',
    content: html,
    onSubmit: function() {
      var newEmail = document.getElementById('editAdminEmail').value.trim();
      var newRole = document.getElementById('editAdminRole').value;
      var newStatus = document.getElementById('editAdminStatus').value;
      if (!newEmail) { App.toast('邮箱不能为空', 'warn'); return false; }
      App.toast('管理员 ' + username + ' 已更新：' + newRole + ' / ' + newStatus, 'success');
      return true;
    }
  });
}

function resetAdminPassword(username, email) {
  var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%';
  var pwd = '';
  for (var i = 0; i < 12; i++) {
    pwd += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  
  var html = '<div style="padding:20px 0;text-align:center;">' +
    '<div style="font-size:14px;color:var(--muted);margin-bottom:16px;">密码已重置并发送至 <strong>' + email + '</strong></div>' +
    '<div style="background:var(--bg);border:1px solid var(--border);border-radius:var(--radius-sm);padding:16px 20px;margin-bottom:16px;display:inline-block;">' +
    '<div style="font-size:12px;color:var(--muted);margin-bottom:8px;">临时密码（仅显示一次，请复制保存）</div>' +
    '<code id="resetPwdText" style="font-family:var(--font-mono);font-size:18px;letter-spacing:1px;color:var(--fg);display:block;">' + pwd + '</code>' +
    '</div>' +
    '<div style="display:flex;gap:12px;justify-content:center;">' +
    '<button class="btn btn-primary" onclick="copyResetPassword()">复制密码</button>' +
    '<button class="btn btn-ghost" onclick="App.modal({title:\'发送记录\',content:\'<div style=\\'padding:12px 0\\'>密码已发送至 ' + email + '<br><br>发送时间：' + new Date().toLocaleString() + '</div>\',showFooter:false})">查看发送记录</button>' +
    '</div>' +
    '</div>';
  
  App.modal({
    title: '重置密码 — ' + username,
    content: html,
    showFooter: false
  });
  
  // Simulate API call to send email
  setTimeout(function() {
    App.toast('密码重置邮件已发送至 ' + email, 'success');
  }, 500);
}

function copyResetPassword() {
  var pwd = document.getElementById('resetPwdText').textContent;
  if (navigator.clipboard) {
    navigator.clipboard.writeText(pwd).then(function() {
      App.toast('密码已复制到剪贴板', 'success');
    });
  } else {
    var ta = document.createElement('textarea');
    ta.value = pwd;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    App.toast('密码已复制到剪贴板', 'success');
  }
}
