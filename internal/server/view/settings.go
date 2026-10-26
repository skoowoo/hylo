package view

import "html/template"

// settingsTab is one entry of the settings modal's sidebar, in tab order.
type settingsTab struct {
	ID, Label   string
	DesktopOnly bool
	Icon        template.HTML
}

var settingsTabs = []settingsTab{
	{ID: "appearance", Label: "Appearance", Icon: `<circle cx="12" cy="12" r="10"/><path d="M12 18a6 6 0 0 0 0-12z" fill="currentColor" stroke="none"/>`},
	{ID: "server", Label: "Server", Icon: `<rect width="20" height="8" x="2" y="2" rx="2"/><rect width="20" height="8" x="2" y="14" rx="2"/><path stroke-linecap="round" d="M6 6h.01"/><path stroke-linecap="round" d="M6 18h.01"/>`},
	{ID: "shortcuts", Label: "Shortcuts", Icon: `<path stroke-linecap="round" d="M10 8h.01"/><path stroke-linecap="round" d="M12 12h.01"/><path stroke-linecap="round" d="M14 8h.01"/><path stroke-linecap="round" d="M16 12h.01"/><path stroke-linecap="round" d="M18 8h.01"/><path stroke-linecap="round" d="M6 8h.01"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 16h10"/><path stroke-linecap="round" d="M8 12h.01"/><rect width="20" height="16" x="2" y="4" rx="2"/>`},
	{ID: "notifications", Label: "Notifications", DesktopOnly: true, Icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path stroke-linecap="round" stroke-linejoin="round" d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>`},
	{ID: "agent-bots", Label: "Bots", Icon: `<path d="M12 6V2H8"/><path d="M15 11v2"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="M20 16a2 2 0 0 1-2 2H8.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 4 20.286V8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2z"/><path d="M9 11v2"/>`},
	{ID: "skills", Label: "Skills", Icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 2v4"/><path stroke-linecap="round" stroke-linejoin="round" d="M22 4h-4"/><circle cx="4" cy="20" r="2"/>`},
	{ID: "agents", Label: "Agents", Icon: `<path stroke-linecap="round" stroke-linejoin="round" d="M12 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 2v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 2v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 12h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 17h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M2 7h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 12h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 17h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M20 7h2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 20v2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 2v2"/><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="8" y="8" width="8" height="8" rx="1"/>`},
}
