package view

// themeBootstrapScript runs in <head> before first paint, so there's no
// flash of the wrong theme. Dark lives on bare :root (shared_tokens.go);
// light applies via :root[data-theme="light"] or an unforced OS preference.
//
// The Settings → Editor "Theme" toggle stores light/dark/auto under the
// 'hylo-theme' localStorage key and calls window.__hyloApplyTheme (see
// shared_settings_modal.go's setTheme()); "auto" also gets a live
// prefers-color-scheme listener so it follows an OS change without a reload.
const themeBootstrapScript = `  <script>(function(){
  window.__hyloApplyTheme=function(pref){
    try{
      var sysLight=window.matchMedia&&window.matchMedia('(prefers-color-scheme: light)').matches;
      var effectiveLight=pref==='light'?true:(pref==='dark'?false:sysLight);
      if(pref==='light'){document.documentElement.setAttribute('data-theme','light');}
      else if(pref==='dark'){document.documentElement.setAttribute('data-theme','dark');}
      else{document.documentElement.removeAttribute('data-theme');}
      if(window.hyloDesktop&&window.hyloDesktop.setViewBgColor){
        window.hyloDesktop.setViewBgColor(effectiveLight?'#f9f9fb':'#18191e',pref==='light'?'light':(pref==='dark'?'dark':''));
      }
      if(window.__hyloApplyAccent)window.__hyloApplyAccent();
    }catch(_){}
  };
  var stored=localStorage.getItem('hylo-theme');
  window.__hyloApplyTheme(stored==='light'||stored==='dark'?stored:'auto');
  if(window.matchMedia){
    window.matchMedia('(prefers-color-scheme: light)').addEventListener('change',function(){
      var cur=localStorage.getItem('hylo-theme');
      if(cur!=='light'&&cur!=='dark')window.__hyloApplyTheme('auto');
    });
  }
})()</script>`

// desktopBootstrapScript adds the 'macos' class to <html> when running inside
// the Hylo desktop app on macOS.
const desktopBootstrapScript = `  <script>(function(){if(window.hyloDesktop&&window.hyloDesktop.platform==='darwin')document.documentElement.classList.add('macos');})()</script>`

// alpineStoresScript initializes all Alpine.js global stores.
// Wrap it inside a document.addEventListener('alpine:init', () => { … }) call.
const alpineStoresScript = `    Alpine.store('settingsModal', { open: false });`
