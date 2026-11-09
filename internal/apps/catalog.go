package apps

// Catalog is every app Hylo can connect, in the order Settings lists them.
var Catalog = []App{
	{
		ID: "podwise", Name: "Podwise", Kind: KindMCP,
		About:  "Podcasts: what you listened to, episode summaries, transcripts and your clips.",
		URL:    "https://mcp.podwise.ai/mcp",
		Check:  "get_me",
		KeyURL: "https://app.podwise.ai/dashboard/settings?section=developer",
		// Bots read the library and process what's in it; they don't add to it
		// (imports, uploads, clips), delete from it, or send it elsewhere.
		Hidden: []string{"import_episode", "start_audio_upload", "complete_audio_upload", "create_clip", "delete_clip",
			"send_episode", "send_clips", "enterprise_*"},
	},
	{
		ID: "weread", Name: "WeChat Reading", Kind: KindSkill,
		About:  "Reading: your shelf, progress, highlights, notes and reading time.",
		Repo:   "Tencent/WeChatReading",
		Skill:  "weread-skills",
		Env:    "WEREAD_API_KEY",
		KeyURL: "https://weread.qq.com/r/weread-skills", KeyPrefix: "wrk-",
		verifyKey: verifyWeRead,
	},
}
