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
    load();
  });
}());
