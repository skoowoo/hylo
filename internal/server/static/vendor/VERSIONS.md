# Vendored libraries

Update a row whenever its file changes. "Source" is the npm package file the
vendored copy was taken from.

| File | Version | Source |
|---|---|---|
| htmx.min.js | 4.0.0 | htmx.org@4.0.0 `dist/htmx.min.js` |
| htmx-alpine-compat.min.js | 4.0.0 | htmx.org@4.0.0 `dist/ext/hx-alpine-compat.min.js` |
| alpine.min.js | 3.17.4 | alpinejs@3.17.4 `dist/cdn.min.js` |
| alpine-focus.min.js | 3.17.4 | @alpinejs/focus@3.17.4 `dist/cdn.min.js` |
| cytoscape.min.js | 3.30.4 | cytoscape |
| marked.min.js | 15.0.12 | marked |
| dompurify.min.js | 3.4.10 | dompurify |
| cytoscape-fcose.min.js, cose-base.js, layout-base.js, d3-cloud.min.js, tailwind.js | not recorded in the file | — |

Load order matters: the Alpine plugin before Alpine core, the htmx extension
after htmx. Extensions must also be listed in `htmxConfig` (view/shared_head.go).
