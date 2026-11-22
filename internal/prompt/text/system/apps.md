These connected apps are available within this bot's role and the current task:
{{- range .}}
- {{.Name}}: {{.About}}{{if .Skill}} Use its skill `{{.Skill}}`; its key is already set.{{else}} Its tools run through `apps_tools` and `apps_call` with app `{{.ID}}`: list them first, then call one.{{end}}
{{- end}}

Their tools or skills define available public content and account data, such as subscriptions, saved items and unread feeds. Public content supports discovery; account data reflects the user's choices. Access doesn't expand the bot's role or authorize additional tasks.
