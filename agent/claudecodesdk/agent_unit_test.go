package claudecodesdk

import (
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestNormalizeEffort(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"low", "low"},
		{"medium", "medium"},
		{"med", "medium"},
		{"high", "high"},
		{"max", "max"},
		{"HIGH", "high"},
		{"  medium  ", "medium"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := normalizeEffort(tt.in)
		if got != tt.want {
			t.Errorf("normalizeEffort(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetModel_ProviderOverride(t *testing.T) {
	a := &Agent{
		model: "sonnet",
		providers: []core.ProviderConfig{
			{Name: "custom", Model: "claude-sonnet-4-20250514"},
		},
		activeIdx: 0,
	}

	got := a.GetModel()
	if got != "claude-sonnet-4-20250514" {
		t.Errorf("GetModel() = %q, want provider model %q", got, "claude-sonnet-4-20250514")
	}
}

func TestGetModel_NoProvider(t *testing.T) {
	a := &Agent{
		model:     "sonnet",
		activeIdx: -1,
	}

	got := a.GetModel()
	if got != "sonnet" {
		t.Errorf("GetModel() = %q, want %q", got, "sonnet")
	}
}

func TestGetModel_ProviderNoModel(t *testing.T) {
	a := &Agent{
		model: "sonnet",
		providers: []core.ProviderConfig{
			{Name: "custom"}, // no Model field
		},
		activeIdx: 0,
	}

	got := a.GetModel()
	if got != "sonnet" {
		t.Errorf("GetModel() = %q, want fallback %q", got, "sonnet")
	}
}

func TestSetPlatformPrompt(t *testing.T) {
	a := &Agent{}
	a.SetPlatformPrompt("Use Slack mrkdwn format")
	if a.platformPrompt != "Use Slack mrkdwn format" {
		t.Errorf("platformPrompt = %q, want %q", a.platformPrompt, "Use Slack mrkdwn format")
	}
}

func TestSetReasoningEffort(t *testing.T) {
	a := &Agent{}
	a.SetReasoningEffort("high")
	if a.reasoningEffort != "high" {
		t.Errorf("reasoningEffort = %q, want %q", a.reasoningEffort, "high")
	}
}

func TestGetReasoningEffort(t *testing.T) {
	a := &Agent{reasoningEffort: "medium"}
	if got := a.GetReasoningEffort(); got != "medium" {
		t.Errorf("GetReasoningEffort() = %q, want %q", got, "medium")
	}
}

func TestAvailableReasoningEfforts(t *testing.T) {
	a := &Agent{}
	efforts := a.AvailableReasoningEfforts()
	want := []string{"low", "medium", "high", "max"}
	if len(efforts) != len(want) {
		t.Fatalf("AvailableReasoningEfforts() count = %d, want %d", len(efforts), len(want))
	}
	for i, e := range efforts {
		if e != want[i] {
			t.Errorf("efforts[%d] = %q, want %q", i, e, want[i])
		}
	}
}

func TestAddAllowedTools(t *testing.T) {
	a := &Agent{}

	if err := a.AddAllowedTools("Read", "Write"); err != nil {
		t.Fatalf("AddAllowedTools: %v", err)
	}

	tools := a.GetAllowedTools()
	if len(tools) != 2 {
		t.Fatalf("GetAllowedTools() count = %d, want 2", len(tools))
	}
	if tools[0] != "Read" || tools[1] != "Write" {
		t.Errorf("GetAllowedTools() = %v, want [Read Write]", tools)
	}
}

func TestAddAllowedTools_Dedup(t *testing.T) {
	a := &Agent{allowedTools: []string{"Read"}}

	a.AddAllowedTools("Read", "Write")

	tools := a.GetAllowedTools()
	if len(tools) != 2 {
		t.Errorf("GetAllowedTools() count = %d, want 2 (dedup)", len(tools))
	}
}

func TestAddAllowedTools_PropagatesToConfig(t *testing.T) {
	a := &Agent{
		config: sidecarConfig{AllowedTools: []string{"Bash"}},
	}

	a.AddAllowedTools("Read", "Write")

	// Verify allowedTools field is updated
	if len(a.allowedTools) != 2 {
		t.Errorf("allowedTools count = %d, want 2", len(a.allowedTools))
	}

	// StartSession snapshot should use a.allowedTools, not a.config.AllowedTools
	a.mu.RLock()
	snapshot := make([]string, len(a.allowedTools))
	copy(snapshot, a.allowedTools)
	a.mu.RUnlock()

	if len(snapshot) != 2 || snapshot[0] != "Read" || snapshot[1] != "Write" {
		t.Errorf("snapshot = %v, want [Read Write]", snapshot)
	}
}

func TestGetDisallowedTools(t *testing.T) {
	a := &Agent{
		config: sidecarConfig{
			DisallowedTools: []string{"Bash", "Edit"},
		},
	}

	got := a.GetDisallowedTools()
	if len(got) != 2 {
		t.Fatalf("GetDisallowedTools() count = %d, want 2", len(got))
	}
	if got[0] != "Bash" || got[1] != "Edit" {
		t.Errorf("GetDisallowedTools() = %v, want [Bash Edit]", got)
	}

	// Verify it returns a copy
	got[0] = "Modified"
	if a.config.DisallowedTools[0] != "Bash" {
		t.Error("GetDisallowedTools() should return a copy")
	}
}

func TestNormalizeMode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "default"},
		{"auto", "auto"},
		{"plan", "plan"},
		{"acceptedits", "acceptEdits"},
		{"accept-edits", "acceptEdits"},
		{"edit", "acceptEdits"},
		{"bypasspermissions", "bypassPermissions"},
		{"yolo", "bypassPermissions"},
		{"dontask", "dontAsk"},
	}

	for _, tt := range tests {
		got := normalizeMode(tt.in)
		if got != tt.want {
			t.Errorf("normalizeMode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
