(function(){
  'use strict';
  function titleSuffix(title){
    const parts=String(title||'').split('—');
    return parts.length>1?' — '+parts.slice(1).join('—').trim():'';
  }
  function apply(config){
    config=config||{};
    const name=config.name||'simpleHPC';
    document.querySelectorAll('.brand-name,.login-brand-name').forEach(node=>node.textContent=name);
    document.querySelectorAll('.login-footer').forEach(node=>node.textContent=name+' © '+new Date().getFullYear());
    document.title=name+titleSuffix(document.title);
    if(config.logo){
      document.querySelectorAll('.brand-icon,.login-brand-icon').forEach(node=>{
        node.textContent='';
        const image=document.createElement('img');
        image.src=config.logo;image.alt=name+' Logo';
        Object.assign(image.style,{width:'100%',height:'100%',objectFit:'contain',borderRadius:'inherit'});
        node.appendChild(image);
      });
      document.querySelectorAll('.login-logo-img,.login-logo-mark').forEach(image=>{
        image.src=config.logo;
        image.alt=name+' Logo';
      });
    }
    const loginPage=document.querySelector('.login-page');
    if(loginPage&&config.loginImage){
      loginPage.style.setProperty('--login-server-image','url("'+String(config.loginImage).replace(/"/g,'%22')+'")');
    }
    window.PlatformUI.config=config;
    document.dispatchEvent(new CustomEvent('platform-config-applied',{detail:config}));
  }
  async function loadAndApply(){
    const response=await fetch('/api/v1/config/platform/public',{cache:'no-store'});
    const data=await response.json().catch(()=>({}));
    if(!response.ok)throw new Error(data.error||('HTTP '+response.status));
    apply(data.config||{});
    return data.config||{};
  }
  window.PlatformUI={apply,loadAndApply,config:{}};
}());
