package prompt

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed text/*.md
var textFS embed.FS

// load reads text/<name>.md; a missing file panics at init so tests catch it.
func load(name string) string {
	b, err := textFS.ReadFile("text/" + name + ".md")
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(string(b), "\n")
}

func parse(name string) *template.Template {
	return template.Must(template.New(name).Option("missingkey=error").Parse(load(name)))
}

func execute(t *template.Template, data any) string {
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		panic(err)
	}
	return sb.String()
}
