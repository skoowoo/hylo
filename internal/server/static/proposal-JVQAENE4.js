import{d as h}from"./chunk-GSQK2KCC.js";import{a as y,c as x,d as n}from"./chunk-JVA7RCZ3.js";import"./chunk-B2S6MO3C.js";var v=`
.hy-proposal{display:flex;flex-direction:column;gap:var(--space-xs,8px);padding:var(--space-sm,12px) 14px;border-radius:var(--r-lg,12px);background:var(--surface-soft,#f2f2f4);font:var(--text-sm,.8125rem)/1.5 var(--font-ui,system-ui,sans-serif);color:var(--fg,#17181b)}
.hy-proposal-title{font-weight:var(--fw-semibold,600)}
.hy-proposal-summary{color:var(--prose-body,#3e3f46)}
.hy-proposal-more{all:unset;cursor:pointer;align-self:flex-start;font-size:var(--text-xs,.75rem);color:var(--muted,#696b75)}
.hy-proposal-more:hover{color:var(--fg,#17181b)}
.hy-proposal-more:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px;border-radius:var(--r-xs,4px)}
.hy-proposal-details{font-size:var(--text-xs,.75rem);color:var(--muted,#696b75)}
.hy-proposal-details[hidden]{display:none}
.hy-proposal .hy-choices{padding-top:4px}
`;function C(l,a){x("hy-proposal-style",v);let i=document.createElement("div");i.className="hy-proposal",l.replaceChildren(i);let t=!1,s=null,m=e=>e.items.map(r=>"- "+r).join(`
`);function f(){let e=y(a),r=[];e.field("title")&&r.push(n(p("hy-proposal-title"),e.field("title"))),e.field("summary")&&r.push(n(p("hy-proposal-summary"),e.field("summary")));let u=e.field("details");if(u){let c=n(p("hy-proposal-details"),u);c.hidden=!t;let o=document.createElement("button");o.type="button",o.className="hy-proposal-more",o.textContent=t?"Details \u25BE":"Details \u25B8",o.setAttribute("aria-expanded",String(t)),o.addEventListener("click",()=>{t=!t,c.hidden=!t,o.textContent=t?"Details \u25BE":"Details \u25B8",o.setAttribute("aria-expanded",String(t))}),r.push(o,c)}if(s){i.replaceChildren(...r,s.slot),s.handle.update(m(e));return}let d=p("");i.replaceChildren(...r,d),s={slot:d,handle:h(d,m(e))}}return f(),{update(e){a=e,f()},destroy(){s&&s.handle.destroy()}}}function p(l){let a=document.createElement("div");return l&&(a.className=l),a}export{C as mount};
