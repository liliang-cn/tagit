package run

import (
	"testing"

	"github.com/liliang-cn/tagit/internal/domain"
)

func TestInterpretTriageOutput(t *testing.T) {
	tests := []struct {
		name       string
		out        string
		wantAnswer string
		wantTask   bool
	}{
		{"empty means task", "", "", true},
		{"whitespace means task", "  \n\t ", "", true},
		{"sentinel alone is a task", triageTaskSentinel, "", true},
		{"sentinel amid noise is a task", "thinking...\n" + triageTaskSentinel + "\n", "", true},
		{"plain greeting is an answer", "Yes, I'm here! 👋", "Yes, I'm here! 👋", false},
		{"answer is trimmed", "\n  Anytime!  \n", "Anytime!", false},
		{"chinese answer", "在的，有什么可以帮你？", "在的，有什么可以帮你？", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAnswer, gotTask := interpretTriageOutput(tt.out)
			if gotTask != tt.wantTask {
				t.Errorf("isTask = %v, want %v", gotTask, tt.wantTask)
			}
			if gotAnswer != tt.wantAnswer {
				t.Errorf("answer = %q, want %q", gotAnswer, tt.wantAnswer)
			}
		})
	}
}

func TestTriageArgs(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"/usr/local/bin/claude", []string{"-p", "PROMPT", "--allowedTools", "mcp__plugin_cortexdb_cortexdb"}},
		{"gemini", []string{"-p", "PROMPT"}},
		{"copilot", []string{"-p", "PROMPT"}},
		{"/opt/codex", []string{"exec", "--skip-git-repo-check", "-s", "read-only", "-c", "approval_policy=never", "PROMPT"}},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := triageArgs(domain.AgentProfile{Command: tt.command}, "PROMPT")
			if len(got) != len(tt.want) {
				t.Fatalf("args = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("args = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
