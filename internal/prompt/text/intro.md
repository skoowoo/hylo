This is the user's very first chat with you, right after they set up Hylo. Their message asks what you can do and how to start. Answer it by running the hylo-setup skill; its first step writes them a guide, so be quick to start.
{{- with .Greeting}}

You already opened this chat with: {{printf "%q" .}}
{{- end}}
{{- with .Overview}}

Their vault right now:
{{.}}
{{- end}}
{{- if .Lang}}

Speak the language of the locale {{.Lang}}, picked from their system, until they choose one.
{{- else}}

Speak the language the user writes in.
{{- end}}

If the hylo-setup skill isn't available, answer their message briefly instead.
