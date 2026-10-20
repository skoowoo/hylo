// Package skills ships Hylo's built-in skills inside the binary; every top-level
// directory embedded here is one.
package skills

import "embed"

//go:embed hylo-bots hylo-compile-note hylo-editor hylo-index-knowledge hylo-memory hylo-notes hylo-recall hylo-setup hylo-writing
var FS embed.FS
