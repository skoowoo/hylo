These connected apps are available within this bot's role and the current task:
{{- range .}}
- {{.Name}}: {{.About}}{{if .Skill}} Use its skill `{{.Skill}}`; its key is already set.{{else}} Its tools run through `apps_tools` and `apps_call` with app `{{.ID}}`: list them first, then call one.{{end}}
{{- end}}

Their tools or skills define what you can access, including personal data and platform content. Access doesn't expand the bot's role or authorize additional tasks.
