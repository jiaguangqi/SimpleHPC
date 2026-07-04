(function(){
  'use strict';
  const token=localStorage.getItem('simplehpc_token')||'';
  const auth=token?{Authorization:'Bearer '+token}:{};
  async function jsonRequest(url,options){
    const response=await fetch(url,Object.assign({cache:'no-store',headers:Object.assign({'Content-Type':'application/json'},auth)},options||{}));
    const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);return data;
  }
  async function upload(kind,file){
    const form=new FormData();form.append('file',file);
    const response=await fetch('/api/v1/config/platform/assets/'+kind,{method:'POST',headers:auth,body:form});
    const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||`HTTP ${response.status}`);return data.url;
  }
  let terminalNodes=[];
  function renderTerminalNodes(){
    const rows=document.getElementById('terminalNodeRows');
    if(!rows)return;
    if(!terminalNodes.length){
      rows.innerHTML='<tr><td colspan="4" style="color:var(--muted);">尚未配置登录节点。请点击“添加登录节点”，例如主机名 cae、地址 10.10.38.152。</td></tr>';
      return;
    }
    rows.innerHTML=terminalNodes.map((node,index)=>`
      <tr>
        <td><input type="checkbox" data-terminal-enabled="${index}" ${node.enabled!==false?'checked':''}></td>
        <td><input style="width:100%" data-terminal-hostname="${index}" value="${h(node.hostname||'')}" placeholder="例如 login01"></td>
        <td><input style="width:100%" data-terminal-address="${index}" value="${h(node.address||'')}" placeholder="例如 10.10.38.152"></td>
        <td><button class="small-action" type="button" data-terminal-delete="${index}">删除</button></td>
      </tr>`).join('');
    rows.querySelectorAll('[data-terminal-delete]').forEach(btn=>btn.addEventListener('click',()=>{terminalNodes.splice(Number(btn.dataset.terminalDelete),1);renderTerminalNodes();}));
  }
  function collectTerminalNodes(){
    terminalNodes=terminalNodes.map((node,index)=>({
      enabled:document.querySelector(`[data-terminal-enabled="${index}"]`)?.checked!==false,
      hostname:(document.querySelector(`[data-terminal-hostname="${index}"]`)?.value||'').trim(),
      address:(document.querySelector(`[data-terminal-address="${index}"]`)?.value||'').trim()
    })).filter(node=>node.hostname||node.address);
    return terminalNodes;
  }
  function h(value){return String(value==null?'':value).replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));}
  async function loadTerminal(){
    try{
      const data=await jsonRequest('/api/v1/config/terminal');
      const config=data.config||{};
      document.getElementById('terminalStrategy').value=config.strategy||'round_robin';
      terminalNodes=(config.nodes||[]).map(node=>({hostname:node.hostname||'',address:node.address||'',enabled:node.enabled!==false}));
      renderTerminalNodes();
    }catch(error){
      renderTerminalNodes();
      App.toast('登录节点配置未获取：'+error.message,'danger');
    }
  }
  function preview(inputId,imageId,hiddenId){
    const input=document.getElementById(inputId),image=document.getElementById(imageId);
    input.addEventListener('change',()=>{const file=input.files[0];if(!file)return;image.src=URL.createObjectURL(file);image.style.display='block';});
    const value=document.getElementById(hiddenId).value;if(value){image.src=value;image.style.display='block';}
  }
  function showSaved(id,url){const image=document.getElementById(id);if(url){image.src=url;image.style.display='block';}else{image.removeAttribute('src');image.style.display='none';}}
  async function load(){
    try{const data=await jsonRequest('/api/v1/config/platform');const value=data.config||{};
      document.getElementById('platformName').value=value.name||'simpleHPC';
      document.getElementById('platformLogo').value=value.logo||'';
      document.getElementById('platformLoginImage').value=value.loginImage||'';
      document.getElementById('platformLanguage').value='zh-CN';
      showSaved('platformLogoPreview',value.logo);showSaved('platformLoginImagePreview',value.loginImage);
    }catch(error){App.toast('平台设置数据未获取：'+error.message,'danger');}
  }
  document.addEventListener('DOMContentLoaded',function(){
    preview('platformLogoFile','platformLogoPreview','platformLogo');
    preview('platformLoginImageFile','platformLoginImagePreview','platformLoginImage');
    document.getElementById('savePlatformSettings').addEventListener('click',async function(){
      const button=this;button.disabled=true;
      try{
        const logoFile=document.getElementById('platformLogoFile').files[0],loginFile=document.getElementById('platformLoginImageFile').files[0];
        if(logoFile)document.getElementById('platformLogo').value=await upload('logo',logoFile);
        if(loginFile)document.getElementById('platformLoginImage').value=await upload('login-image',loginFile);
        const result=await jsonRequest('/api/v1/config/platform',{method:'PUT',body:JSON.stringify({name:document.getElementById('platformName').value,logo:document.getElementById('platformLogo').value,loginImage:document.getElementById('platformLoginImage').value,language:'zh-CN'})});
        if(window.PlatformUI)await PlatformUI.loadAndApply();
        App.toast('平台设置已保存并应用','success');showSaved('platformLogoPreview',result.config.logo);showSaved('platformLoginImagePreview',result.config.loginImage);
      }catch(error){App.toast('保存失败：'+error.message,'danger');}finally{button.disabled=false;}
    });
    document.getElementById('addTerminalNode')?.addEventListener('click',function(){
      collectTerminalNodes();
      terminalNodes.push({hostname:'',address:'',enabled:true});
      renderTerminalNodes();
    });
    document.getElementById('saveTerminalSettings')?.addEventListener('click',async function(){
      const button=this;button.disabled=true;
      try{
        const nodes=collectTerminalNodes();
        const result=await jsonRequest('/api/v1/config/terminal',{method:'PUT',body:JSON.stringify({strategy:document.getElementById('terminalStrategy').value,nodes})});
        terminalNodes=(result.config.nodes||[]).map(node=>({hostname:node.hostname||'',address:node.address||'',enabled:node.enabled!==false}));
        renderTerminalNodes();
        App.toast('登录节点设置已保存','success');
      }catch(error){App.toast('保存登录节点失败：'+error.message,'danger');}finally{button.disabled=false;}
    });
    load();
    loadTerminal();
  });
}());
