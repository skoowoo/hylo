package view

// settingsTab is one entry of the settings modal's sidebar, in tab order.
type settingsTab struct {
	ID, Label   string
	DesktopOnly bool
	Icon        string // a line icon name (icons/*.svg)
}

// settingsGroup is a headed run of tabs in the settings sidebar.
type settingsGroup struct {
	Label string
	Tabs  []settingsTab
}

var settingsGroups = []settingsGroup{
	{Label: "General", Tabs: []settingsTab{
		{ID: "appearance", Label: "Appearance", Icon: "appearance"},
		{ID: "shortcuts", Label: "Shortcuts", Icon: "keyboard"},
		{ID: "notifications", Label: "Notifications", DesktopOnly: true, Icon: "bell"},
	}},
	{Label: "AI", Tabs: []settingsTab{
		{ID: "agent-bots", Label: "Bots", Icon: "bot"},
		{ID: "skills", Label: "Skills", Icon: "sparkles"},
		{ID: "apps", Label: "Apps", Icon: "link"},
		{ID: "agents", Label: "Agent CLIs", Icon: "cpu"},
	}},
	{Label: "System", Tabs: []settingsTab{
		{ID: "server", Label: "Server", Icon: "server"},
	}},
}

// settingsTabs is every tab in sidebar order.
var settingsTabs = func() []settingsTab {
	var out []settingsTab
	for _, g := range settingsGroups {
		out = append(out, g.Tabs...)
	}
	return out
}()
