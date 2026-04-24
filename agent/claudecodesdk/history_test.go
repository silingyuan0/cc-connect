package claudecodesdk

import (
	"encoding/json"
	"testing"
)

func TestEncodeClaudeProjectKey(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/project", "-home-user-project"},
		{"/home/user/my_project", "-home-user-my-project"},
		{"/home/user/中文项目", "-home-user-----"},
		{"C:\\Users\\test\\project", "C--Users-test-project"},
	}

	for _, tt := range tests {
		got := encodeClaudeProjectKey(tt.path)
		if got != tt.want {
			t.Errorf("encodeClaudeProjectKey(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractTextContent_PlainString(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	got := extractTextContent(raw)
	if got != "hello world" {
		t.Errorf("extractTextContent(string) = %q, want %q", got, "hello world")
	}
}

func TestExtractTextContent_TextBlock(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello from block"}]`)
	got := extractTextContent(raw)
	if got != "hello from block" {
		t.Errorf("extractTextContent(blocks) = %q, want %q", got, "hello from block")
	}
}

func TestExtractTextContent_ThinkingBlock(t *testing.T) {
	raw := json.RawMessage(`[{"type":"thinking","thinking":"deep thoughts"},{"type":"text","text":"actual text"}]`)
	got := extractTextContent(raw)
	if got != "actual text" {
		t.Errorf("extractTextContent(thinking+text) = %q, want %q", got, "actual text")
	}
}

func TestExtractTextContent_Empty(t *testing.T) {
	got := extractTextContent(nil)
	if got != "" {
		t.Errorf("extractTextContent(nil) = %q, want empty", got)
	}

	got = extractTextContent(json.RawMessage(``))
	if got != "" {
		t.Errorf("extractTextContent(empty) = %q, want empty", got)
	}
}

func TestExtractTextContent_NoTextBlock(t *testing.T) {
	raw := json.RawMessage(`[{"type":"tool_use","name":"Read"}]`)
	got := extractTextContent(raw)
	if got != "" {
		t.Errorf("extractTextContent(no text block) = %q, want empty", got)
	}
}

func TestStripXMLTags(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"<tag>content</tag>", "content"},
		{"no tags", "no tags"},
		{"<b>bold</b> text", "bold text"},
		{"", ""},
	}

	for _, tt := range tests {
		got := stripXMLTags(tt.in)
		if got != tt.want {
			t.Errorf("stripXMLTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestUniqueSkillDirs(t *testing.T) {
	dirs := []string{
		"/home/user/.claude/skills",
		"/home/user/project/.claude/skills",
		"/home/user/.claude/skills", // duplicate
	}
	got := uniqueSkillDirs(dirs)
	if len(got) != 2 {
		t.Errorf("uniqueSkillDirs count = %d, want 2: %v", len(got), got)
	}
}

func TestUniqueSkillDirs_EmptyStrings(t *testing.T) {
	dirs := []string{"", "/a", ""}
	got := uniqueSkillDirs(dirs)
	if len(got) != 1 {
		t.Errorf("uniqueSkillDirs with empties count = %d, want 1", len(got))
	}
}
