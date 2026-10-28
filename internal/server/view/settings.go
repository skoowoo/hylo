package view

// settingsTab is one entry of the settings modal's sidebar, in tab order.
type settingsTab struct {
	ID, Label   string
	DesktopOnly bool
	Icon        string // a line icon name (icons/*.svg)
}

var settingsTabs = []settingsTab{
	{ID: "appearance", Label: "Appearance", Icon: "appearance"},
	{ID: "server", Label: "Server", Icon: "server"},
	{ID: "shortcuts", Label: "Shortcuts", Icon: "keyboard"},
	{ID: "notifications", Label: "Notifications", DesktopOnly: true, Icon: "bell"},
	{ID: "agent-bots", Label: "Bots", Icon: "bot"},
	{ID: "skills", Label: "Skills", Icon: "sparkles"},
	{ID: "agents", Label: "Agents", Icon: "cpu"},
}
