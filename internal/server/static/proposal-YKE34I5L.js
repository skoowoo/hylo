import{a as y,c as x,d as n}from"./chunk-JVA7RCZ3.js";import{c as h}from"./chunk-BK7Y6OKV.js";var v=`
.hy-proposal{display:flex;flex-direction:column;gap:var(--space-xs,8px);padding:var(--space-sm,12px) 14px;border-radius:var(--r-lg,12px);background:var(--surface-soft,#f5f5f7);font:var(--text-sm,.8125rem)/1.5 var(--font-ui,system-ui,sans-serif);color:var(--fg,#18181b)}
.hy-proposal-title{font-weight:var(--fw-semibold,600)}
.hy-proposal-summary{color:var(--prose-body,#27272a)}
.hy-proposal-more{all:unset;cursor:pointer;align-self:flex-start;font-size:var(--text-xs,.75rem);color:var(--muted,#6d7080)}
.hy-proposal-more:hover{color:var(--fg,#18181b)}
.hy-proposal-more:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px;border-radius:2px}
.hy-proposal-details{font-size:var(--text-xs,.75rem);color:var(--muted,#6d7080)}
.hy-proposal-details[hidden]{display:none}
.hy-proposal .hy-choices{padding-top:4px}
`;function C(l,s){x("hy-proposal-style",v);let i=document.createElement("div");i.className="hy-proposal",l.replaceChildren(i);let t=!1,a=null,m=e=>e.items.map(r=>"- "+r).join(`
`);function u(){let e=y(s),r=[];e.field("title")&&r.push(n(p("hy-proposal-title"),e.field("title"))),e.field("summary")&&r.push(n(p("hy-proposal-summary"),e.field("summary")));let f=e.field("details");if(f){let c=n(p("hy-proposal-details"),f);c.hidden=!t;let o=document.createElement("button");o.type="button",o.className="hy-proposal-more",o.textContent=t?"Details \u25BE":"Details \u25B8",o.setAttribute("aria-expanded",String(t)),o.addEventListener("click",()=>{t=!t,c.hidden=!t,o.textContent=t?"Details \u25BE":"Details \u25B8",o.setAttribute("aria-expanded",String(t))}),r.push(o,c)}if(a){i.replaceChildren(...r,a.slot),a.handle.update(m(e));return}let d=p("");i.replaceChildren(...r,d),a={slot:d,handle:h(d,m(e))}}return u(),{update(e){s=e,u()},destroy(){a&&a.handle.destroy()}}}function p(l){let s=document.createElement("div");return l&&(s.className=l),s}export{C as mount};
