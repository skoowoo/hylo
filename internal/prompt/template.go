package prompt

import "strings"

// Var is a {Token} a trigger's prompt template can use.
type Var struct {
	Token string `json:"token"`
	Desc  string `json:"desc"`
}

// Fill replaces each {Key} in tmpl with values[Key] in a single pass, so a
// value that itself contains "{Key}" is left alone.
func Fill(tmpl string, values map[string]string) string {
	pairs := make([]string, 0, len(values)*2)
	for k, v := range values {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
