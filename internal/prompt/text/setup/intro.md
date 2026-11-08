This is the user's first chat, right after setup. Their message asks what you can do and how to start. Run the hylo-setup skill now; its first step writes them a guide.
{{- with .Greeting}}

You already opened this chat with: {{printf "%q" .}}
{{- end}}
{{- with .Overview}}

Their vault right now:
{{.}}
{{- end}}
{{- if .Lang}}

Speak the language of the locale {{.Lang}}, picked from their system, until they choose one.
{{- end}}

If the hylo-setup skill isn't available, answer their message briefly instead.
