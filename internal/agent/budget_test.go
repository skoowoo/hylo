package agent

import (
	"strings"
	"testing"
)

func TestWindowsBudgetIncludesNativeInstructions(t *testing.T) {
	def := GetAgentDef("codex")
	args := []string{"exec", "-c", "developer_instructions=" + strings.Repeat("x", windowsCreateProcessLimit)}
	if err := CheckPromptArgvBudget(def, strings.Repeat("x", windowsCreateProcessLimit)); err != nil {
		t.Fatal("stdin prompt should not be limited by argv budget")
	}
	if err := CheckWindowsDirectExeCommandLineBudget(def, `C:\bin\codex.exe`, args); err == nil {
		t.Fatal("large native instructions bypassed Windows executable budget")
	}
	if err := CheckWindowsCmdShimCommandLineBudget(def, `C:\bin\codex.cmd`, args); err == nil {
		t.Fatal("large native instructions bypassed Windows shim budget")
	}
	if err := CheckWindowsDirectExeCommandLineBudget(def, "/usr/bin/codex", args); err != nil {
		t.Fatal("Windows budget applied to a Unix executable")
	}
	if err := CheckWindowsDirectExeCommandLineBudget(def, `C:\bin\codex.exe`, []string{"exec", "-c", `developer_instructions="short"`}); err != nil {
		t.Fatal(err)
	}
}
