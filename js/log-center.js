(function(){
  'use strict';
  const page=document.body.dataset.logPage;
  const token=localStorage.getItem('simplehpc_token')||'';
  const headers=token?{Authorization:'Bearer '+token}:{};
  const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  async function request(url){const response=await fetch(url,{cache:'no-store',headers});const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);return data;}
  function eventText(value){return {login:'登录',logout:'退出'}[value]||value;}
  async function load(){
    const rows=document.getElementById('logRows');
    rows.innerHTML=`<tr><td colspan="${page==='auth'?7:4}" class="api-data-missing">正在读取真实日志...</td></tr>`;
    try{
      if(page==='auth'){
        const query=new URLSearchParams({pageSize:'100',keyword:document.getElementById('logKeyword').value,event:document.getElementById('authEvent').value,result:document.getElementById('authResult').value});
        const data=await request('/api/v1/logs/auth-events?'+query);
        rows.innerHTML=(data.items||[]).length?data.items.map(item=>`<tr><td>${esc(item.createdAt)}</td><td><strong>${esc(item.username)}</strong><br><small>${esc(item.displayName)}</small></td><td>${esc(item.accountType)}</td><td>${esc(eventText(item.event))}</td><td><span class="pill ${item.result==='success'?'pill-success':'pill-danger'}">${item.result==='success'?'成功':'失败'}</span></td><td>${esc(item.ipAddress)}</td><td title="${esc(item.userAgent)}" style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(item.userAgent)}</td></tr>`).join(''):'<tr><td colspan="7" class="api-data-missing">当前没有登录日志</td></tr>';
        document.getElementById('logTotal').textContent=`共 ${data.total||0} 条`;
      }else{
        const query=new URLSearchParams({source:document.getElementById('systemSource').value,since:document.getElementById('systemSince').value,level:document.getElementById('systemLevel').value,limit:'500',keyword:document.getElementById('logKeyword').value});
        const data=await request('/api/v1/logs/system?'+query);
        rows.innerHTML=(data.items||[]).length?data.items.map(item=>`<tr><td>${esc(item.timestamp||'—')}</td><td>${esc(item.source)}</td><td><span class="level-${esc(item.level)}">${esc(item.level)}</span></td><td class="log-message">${esc(item.message)}</td></tr>`).join(''):'<tr><td colspan="4" class="api-data-missing">当前时间范围没有日志</td></tr>';
        document.getElementById('logTotal').textContent=`显示 ${data.count||0} 条`;
      }
    }catch(error){rows.innerHTML=`<tr><td colspan="${page==='auth'?7:4}" class="api-data-missing">数据未获取：${esc(error.message)}</td></tr>`;}
  }
  document.addEventListener('DOMContentLoaded',()=>{document.getElementById('applyLogFilters').addEventListener('click',load);document.getElementById('refreshLogs').addEventListener('click',load);load();});
}());
