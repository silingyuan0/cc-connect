package claudecodesdk

import (
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestMapEvent_AskUserQuestionToolUse(t *testing.T) {
	s := &sdkSession{}

	// AskUserQuestion tool_use should be filtered (return nil)
	evt := &sidecarEvent{
		Event:    "tool_use",
		ToolName: "AskUserQuestion",
		Input: map[string]any{
			"questions": []any{
				map[string]any{"question": "Which option?", "options": []any{}},
			},
		},
	}
	got := s.mapEvent(evt)
	if got != nil {
		t.Errorf("mapEvent(AskUserQuestion tool_use) = %+v, want nil", got)
	}
}

func TestMapEvent_AskUserQuestionPermission(t *testing.T) {
	s := &sdkSession{}

	// AskUserQuestion permission_request should parse questions
	evt := &sidecarEvent{
		Event:     "permission_request",
		ToolName:  "AskUserQuestion",
		RequestID: "req-1",
		Input: map[string]any{
			"questions": []any{
				map[string]any{
					"question":    "Which model?",
					"header":      "Model",
					"multiSelect": false,
					"options": []any{
						map[string]any{"label": "Sonnet", "description": "Balanced"},
						map[string]any{"label": "Opus", "description": "Most capable"},
					},
				},
			},
		},
	}
	got := s.mapEvent(evt)
	if got == nil {
		t.Fatal("mapEvent(AskUserQuestion permission) = nil, want event")
	}
	if got.Type != core.EventPermissionRequest {
		t.Errorf("Type = %v, want EventPermissionRequest", got.Type)
	}
	if len(got.Questions) != 1 {
		t.Fatalf("Questions count = %d, want 1", len(got.Questions))
	}
	q := got.Questions[0]
	if q.Question != "Which model?" {
		t.Errorf("Question = %q, want %q", q.Question, "Which model?")
	}
	if q.Header != "Model" {
		t.Errorf("Header = %q, want %q", q.Header, "Model")
	}
	if q.MultiSelect {
		t.Error("MultiSelect = true, want false")
	}
	if len(q.Options) != 2 {
		t.Fatalf("Options count = %d, want 2", len(q.Options))
	}
	if q.Options[0].Label != "Sonnet" {
		t.Errorf("Options[0].Label = %q, want %q", q.Options[0].Label, "Sonnet")
	}
	if q.Options[1].Description != "Most capable" {
		t.Errorf("Options[1].Description = %q, want %q", q.Options[1].Description, "Most capable")
	}
}

func TestMapEvent_NormalToolUse(t *testing.T) {
	s := &sdkSession{}

	// Non-AskUserQuestion tool_use should produce EventToolUse
	evt := &sidecarEvent{
		Event:    "tool_use",
		ToolName: "Read",
		Input:    map[string]any{"file_path": "/tmp/test.go"},
	}
	got := s.mapEvent(evt)
	if got == nil {
		t.Fatal("mapEvent(Read tool_use) = nil, want event")
	}
	if got.Type != core.EventToolUse {
		t.Errorf("Type = %v, want EventToolUse", got.Type)
	}
	if got.ToolName != "Read" {
		t.Errorf("ToolName = %q, want %q", got.ToolName, "Read")
	}
}

func TestMapEvent_SystemInit(t *testing.T) {
	s := &sdkSession{}

	evt := &sidecarEvent{
		Event:     "system",
		Subtype:   "init",
		SessionID: "sess-abc123",
	}
	got := s.mapEvent(evt)
	if got == nil {
		t.Fatal("mapEvent(system init) = nil, want EventText with SessionID")
	}
	if got.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-abc123")
	}

	// Verify internal session ID was stored
	if sid := s.CurrentSessionID(); sid != "sess-abc123" {
		t.Errorf("CurrentSessionID() = %q, want %q", sid, "sess-abc123")
	}
}

func TestMapEvent_SystemInitNoSessionID(t *testing.T) {
	s := &sdkSession{}

	evt := &sidecarEvent{
		Event:   "system",
		Subtype: "init",
	}
	got := s.mapEvent(evt)
	if got != nil {
		t.Errorf("mapEvent(system init without sessionId) = %+v, want nil", got)
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"CLAUDECODE=1",
		"CLAUDECODE_FOO=bar",
		"HOME=/root",
	}

	filtered := filterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Errorf("filtered count = %d, want 2: %v", len(filtered), filtered)
	}
	for _, e := range filtered {
		if e == "CLAUDECODE=1" || e == "CLAUDECODE_FOO=bar" {
			t.Errorf("CLAUDECODE var not filtered: %s", e)
		}
	}
}

func TestFilterEnv_NoMatch(t *testing.T) {
	env := []string{"A=1", "B=2"}
	filtered := filterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Errorf("filtered count = %d, want 2", len(filtered))
	}
}

func TestSummarizeInput_Grep(t *testing.T) {
	got := summarizeInput("Grep", map[string]any{"pattern": "TODO", "path": "."})
	if got != "TODO" {
		t.Errorf("summarizeInput(Grep) = %q, want %q", got, "TODO")
	}
}

func TestSummarizeInput_Glob(t *testing.T) {
	got := summarizeInput("Glob", map[string]any{"pattern": "**/*.go"})
	if got != "**/*.go" {
		t.Errorf("summarizeInput(Glob) = %q, want %q", got, "**/*.go")
	}
}

func TestSummarizeInput_Read(t *testing.T) {
	got := summarizeInput("Read", map[string]any{"file_path": "/tmp/test.go"})
	if got != "/tmp/test.go" {
		t.Errorf("summarizeInput(Read) = %q, want %q", got, "/tmp/test.go")
	}
}

func TestSummarizeInput_Bash(t *testing.T) {
	got := summarizeInput("Bash", map[string]any{"command": "go test ./..."})
	if got != "go test ./..." {
		t.Errorf("summarizeInput(Bash) = %q, want %q", got, "go test ./...")
	}
}

func TestSummarizeInput_Unknown(t *testing.T) {
	got := summarizeInput("UnknownTool", map[string]any{"key": "value"})
	if got != `{"key":"value"}` {
		t.Errorf("summarizeInput(UnknownTool) = %q, want JSON fallback", got)
	}
}

func TestParseUserQuestions_Nil(t *testing.T) {
	if got := parseUserQuestions(nil); got != nil {
		t.Errorf("parseUserQuestions(nil) = %v, want nil", got)
	}
}

func TestParseUserQuestions_EmptyQuestions(t *testing.T) {
	input := map[string]any{"questions": []any{}}
	if got := parseUserQuestions(input); got != nil {
		t.Errorf("parseUserQuestions(empty) = %v, want nil", got)
	}
}

func TestParseUserQuestions_NoQuestionsKey(t *testing.T) {
	input := map[string]any{"other": "data"}
	if got := parseUserQuestions(input); got != nil {
		t.Errorf("parseUserQuestions(no questions key) = %v, want nil", got)
	}
}
