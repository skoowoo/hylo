import{d as y}from"./chunk-JEW57VBF.js";var w="hy-choices-style",S=`
.hy-choices{display:flex;gap:var(--space-xs,8px);padding:2px 0}
.hy-choices-row{align-items:center}
.hy-choices.hy-choices-row>*{flex:none;white-space:nowrap}
.hy-choices-measuring{visibility:hidden}
.hy-choices-probe{position:absolute;top:0;left:0;right:0;visibility:hidden;pointer-events:none}
.hy-choice{all:unset;box-sizing:border-box;position:relative;display:inline-flex;align-items:center;gap:8px;max-width:100%;padding:6px 12px;border-radius:var(--r-md,8px);font:var(--fw-regular,400) var(--text-sm,.8125rem)/1.4 var(--font-ui,system-ui,sans-serif);color:var(--fg,#17181b);background:var(--btn-bg,rgba(23,24,27,.06));white-space:normal;overflow-wrap:anywhere;transition:background-color var(--motion-fast,100ms),color var(--motion-fast,100ms)}
.hy-choices-active .hy-choice{cursor:pointer}
.hy-choices-active .hy-choice:hover{background:var(--btn-bg-hov,rgba(23,24,27,.1))}
.hy-choice:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
.hy-choice-box,.hy-choice-mark{flex:none;display:flex;color:var(--muted,#696b75);transition:color var(--motion-fast,100ms)}
.hy-choice-box .line-icon,.hy-choice-mark .line-icon{width:16px;height:16px}
.hy-choice-box .line-icon+.line-icon,.hy-choice-on .hy-choice-box .line-icon:first-child,
.hy-choice-mark .line-icon+.line-icon,.hy-choice-on .hy-choice-mark .line-icon:first-child{display:none}
.hy-choice-on .hy-choice-box .line-icon+.line-icon,.hy-choice-on .hy-choice-mark .line-icon+.line-icon{display:block}
.hy-choice-on .hy-choice-box,.hy-choice-on .hy-choice-mark{color:var(--accent-text,#5e6ad2)}
.hy-choices-row .hy-choice-mark{display:none}
.hy-choices-row .hy-choice-on{background:var(--btn-bg-on,rgba(23,24,27,.15))}
.hy-choices-row.hy-choices-answered .hy-choice:not(.hy-choice-on){opacity:.5}
.hy-choices-col{flex-direction:column;align-items:stretch;gap:0;padding:var(--space-xxs,4px);border-radius:var(--r-lg,12px);background:var(--surface-soft,#f2f2f4)}
.hy-choices-col .hy-choice{display:flex;align-items:flex-start;gap:10px;padding:9px 12px;font-size:var(--text-base,.875rem);line-height:1.5;color:var(--prose-body,#3e3f46);background:transparent}
.hy-choices-col .hy-choice-box,.hy-choices-col .hy-choice-mark{height:1.5em;align-items:center}
.hy-choices-col .hy-choice+.hy-choice::before{content:"";position:absolute;top:0;left:38px;right:12px;height:1px;border-radius:0;background:var(--line-hair,rgba(23,24,27,.09))}
.hy-choices-col.hy-choices-active .hy-choice:hover{background:var(--card-hov,rgba(23,24,27,.05));color:var(--fg,#17181b)}
.hy-choices-col.hy-choices-active .hy-choice:hover .hy-choice-mark{color:var(--fg,#17181b)}
.hy-choices-col .hy-choice-on{background:var(--btn-bg,rgba(23,24,27,.06));color:var(--fg,#17181b)}
.hy-choices-col.hy-choices-active .hy-choice:hover::before,.hy-choices-col.hy-choices-active .hy-choice:hover+.hy-choice::before,
.hy-choices-col .hy-choice+.hy-choice-on::before,.hy-choices-col .hy-choice-on+.hy-choice::before{background:transparent}
.hy-choices-col.hy-choices-answered .hy-choice:not(.hy-choice-on),
.hy-choices-col.hy-choices-answered .hy-choice:not(.hy-choice-on) .hy-choice-mark,
.hy-choices-col.hy-choices-answered .hy-choice:not(.hy-choice-on) .hy-choice-box{color:var(--muted-soft,#9c9ea7)}
.hy-choices-send{all:unset;box-sizing:border-box;cursor:pointer;padding:6px 12px;border-radius:var(--r-md,8px);font:var(--fw-medium,500) var(--text-sm,.8125rem)/1.4 var(--font-ui,system-ui,sans-serif);color:var(--accent-fg,#fff);background:var(--accent,#5e6ad2)}
.hy-choices-col .hy-choices-send{align-self:flex-end;margin:var(--space-xxs,4px)}
.hy-choices-send[disabled]{cursor:default;opacity:.4}
.hy-choices-send:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
.hy-proposal .hy-choices-col{background:transparent;padding:0;margin-inline:-12px}
`;function E(){if(document.getElementById(w))return;let e=document.createElement("style");e.id=w,e.textContent=S,document.head.appendChild(e)}var L=/^\s*(?:[-*+]|\d+[.)])\s+(\[[ xX]\]\s+)?(.*\S)\s*$/,C=", ";function k(e){let l=[],r=!1;for(let i of(e||"").split(`
`)){let t=L.exec(i);t&&(t[1]&&(r=!0),l.push(t[2]))}return{multi:r,options:l}}function N(e){return e.replace(/!?\[([^\]]*)\]\([^)]*\)/g,"$1").replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g,(l,r,i)=>i||r).replace(/(\*\*|~~|`)(.+?)\1/g,"$2").replace(/(?<![\w*])\*(?!\s)(.+?)(?<!\s)\*(?![\w*])/g,"$1").replace(/\\([\\`*_[\]()#+\-.!~|])/g,"$1").trim()||e}function z(e,l){let r=(l||"").trim();if(!r)return new Set;if(e.includes(r))return new Set([r]);let i=new Set(r.split(C).map(t=>t.trim()));return new Set(e.filter(t=>i.has(t)))}function _(e,l,r={}){E(),e.classList.add("hy-choices-host");let i=k(l),t={mode:"static",picked:""},a=new Set,u=-1,g=c=>e.dispatchEvent(new CustomEvent("hylo:choice",{bubbles:!0,detail:{text:c}}));function f(){let c=t.mode==="active",n=t.mode==="answered"?z(i.options,t.picked):a,s=document.createElement("div");s.className=`hy-choices hy-choices-${t.mode} hy-choices-measuring`,s.setAttribute("role",i.multi?"group":"list");for(let o of i.options){let h=document.createElement(c?"button":"span");h.className="hy-choice",n.has(o)&&h.classList.add("hy-choice-on");let d=document.createElement("span");i.multi?(d.className="hy-choice-box",d.append(y("square"),y("select")),c&&h.setAttribute("aria-pressed",String(n.has(o)))):(d.className="hy-choice-mark",d.append(y("reply"),y("check"))),h.appendChild(d),h.appendChild(document.createTextNode(N(o))),c&&(h.type="button",h.addEventListener("click",()=>{if(!i.multi)return g(o);let m=!a.has(o);m?a.add(o):a.delete(o),h.classList.toggle("hy-choice-on",m),h.setAttribute("aria-pressed",String(m));let x=s.querySelector(".hy-choices-send");x&&(x.disabled=a.size===0)})),s.appendChild(h)}if(c&&i.multi){let o=document.createElement("button");o.type="button",o.className="hy-choices-send",o.textContent="Send",o.disabled=a.size===0,o.addEventListener("click",()=>g(i.options.filter(h=>a.has(h)).join(C))),s.appendChild(o)}e.replaceChildren(s),u=-1,b()}function b(){let c=e.firstChild;if(!c||!e.isConnected||e.clientWidth===u)return;u=e.clientWidth;let n=c.cloneNode(!0);n.className="hy-choices hy-choices-row hy-choices-probe",e.appendChild(n);let s=n.scrollWidth<=n.clientWidth+1;n.remove(),c.classList.toggle("hy-choices-row",s),c.classList.toggle("hy-choices-col",!s),c.classList.remove("hy-choices-measuring")}let p=typeof ResizeObserver>"u"?null:new ResizeObserver(()=>b());p&&p.observe(e),e._hyChoices={setState(c){let n=t.mode,s=c.mode||"static",o=c.picked||"";s===n&&o===t.picked||(t={mode:s,picked:o},n!=="active"&&(a=new Set),f())}},f();let v=()=>e.dispatchEvent(new CustomEvent("hylo:choices-mount",{bubbles:!0}));return e.isConnected?v():requestAnimationFrame(()=>{e.isConnected&&(b(),v())}),{update(c){i=k(c),a=new Set([...a].filter(n=>i.options.includes(n))),f()},destroy(){p&&p.disconnect(),e.classList.remove("hy-choices-host"),delete e._hyChoices}}}export{k as a,N as b,z as c,_ as d};
