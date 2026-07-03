(function(){
  'use strict';
  const token=localStorage.getItem('simplehpc_token')||'';
  const headers=token?{Authorization:'Bearer '+token}:{};
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  async function load(){const rows=document.getElementById('auditRows');try{const response=await fetch('/api/v1/audit/logs?pageSize=100',{cache:'no-store',headers});const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);const items=data.items||[];rows.innerHTML=items.length?items.map(item=>`<tr><td>${esc(item.createdAt)}</td><td>${esc(item.actor)}</td><td>${esc(item.action)}</td><td>${esc(item.target)}</td><td><span class="pill ${item.result==='success'?'pill-success':'pill-danger'}">${esc(item.result)}</span></td></tr>`).join(''):'<tr><td colspan="5" class="api-data-missing">当前没有审计记录</td></tr>';}catch(error){rows.innerHTML=`<tr><td colspan="5" class="api-data-missing">数据未获取：${esc(error.message)}</td></tr>`;}}
  document.addEventListener('DOMContentLoaded',load);
}());
