The user connected these apps, so you can see their own data there:
{{- range .}}
- {{.Name}}: {{.About}}{{if .Skill}} Use its skill `{{.Skill}}`; its key is already set.{{else}} Its tools run through `apps_tools` and `apps_call` with app `{{.ID}}`: list them first, then call one.{{end}}
{{- end}}

Reach for them when the user asks about what they listened to or read, or when their notes would gain from it.
