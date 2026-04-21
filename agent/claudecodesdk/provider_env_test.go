package claudecodesdk

import (
	"strings"
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestProviderEnv_BaseURLWithAPIKey(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name:    "custom",
				BaseURL: "https://example.com/v1",
				APIKey:  "secret-key",
				Model:   "claude-sonnet-4",
			},
		},
		activeIdx: 0,
	}

	env := envSliceToMap(a.providerEnvLocked())

	if got := env["ANTHROPIC_BASE_URL"]; got != "https://example.com/v1" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want %q", got, "https://example.com/v1")
	}
	if got := env["ANTHROPIC_AUTH_TOKEN"]; got != "secret-key" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN = %q, want %q", got, "secret-key")
	}
	if got := env["ANTHROPIC_API_KEY"]; got != "" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want empty (cleared to skip x-api-key validation)", got)
	}
	if got := env["ANTHROPIC_MODEL"]; got != "claude-sonnet-4" {
		t.Errorf("ANTHROPIC_MODEL = %q, want %q", got, "claude-sonnet-4")
	}
}

func TestProviderEnv_APIKeyOnly(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name:   "anthropic",
				APIKey: "sk-ant-xxx",
			},
		},
		activeIdx: 0,
	}

	env := envSliceToMap(a.providerEnvLocked())

	if got := env["ANTHROPIC_API_KEY"]; got != "sk-ant-xxx" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want %q", got, "sk-ant-xxx")
	}
	if _, ok := env["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Error("ANTHROPIC_AUTH_TOKEN should not be set without BaseURL")
	}
	if _, ok := env["ANTHROPIC_BASE_URL"]; ok {
		t.Error("ANTHROPIC_BASE_URL should not be set when empty")
	}
}

func TestProviderEnv_NoModelWhenEmpty(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name:    "no-model",
				BaseURL: "https://example.com/v1",
				APIKey:  "key",
			},
		},
		activeIdx: 0,
	}
	env := envSliceToMap(a.providerEnvLocked())
	if _, ok := env["ANTHROPIC_MODEL"]; ok {
		t.Error("ANTHROPIC_MODEL should not be set when provider has no model")
	}
}

func TestProviderEnv_ClearReturnsNil(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{Name: "p", BaseURL: "https://x.com", APIKey: "k", Model: "m"},
		},
		activeIdx: 0,
	}
	a.SetActiveProvider("")
	env := a.providerEnvLocked()
	if env != nil {
		t.Errorf("expected nil env after clearing provider, got %v", env)
	}
}

func TestProviderEnv_ExtraEnvVars(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name: "bedrock",
				Env: map[string]string{
					"CLAUDE_CODE_USE_BEDROCK": "1",
				},
			},
		},
		activeIdx: 0,
	}

	env := envSliceToMap(a.providerEnvLocked())
	if got := env["CLAUDE_CODE_USE_BEDROCK"]; got != "1" {
		t.Errorf("CLAUDE_CODE_USE_BEDROCK = %q, want %q", got, "1")
	}
}

func TestProviderEnv_SwitchProvider(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name:    "provider-a",
				BaseURL: "https://a.example.com/v1",
				APIKey:  "key-a",
				Model:   "model-a",
			},
			{
				Name:    "provider-b",
				BaseURL: "https://b.example.com/v1",
				APIKey:  "key-b",
				Model:   "model-b",
			},
		},
		activeIdx: 0,
	}

	env := envSliceToMap(a.providerEnvLocked())
	if got := env["ANTHROPIC_BASE_URL"]; got != "https://a.example.com/v1" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want provider-a URL", got)
	}
	if got := env["ANTHROPIC_MODEL"]; got != "model-a" {
		t.Errorf("ANTHROPIC_MODEL = %q, want model-a", got)
	}

	a.SetActiveProvider("provider-b")
	env = envSliceToMap(a.providerEnvLocked())
	if got := env["ANTHROPIC_BASE_URL"]; got != "https://b.example.com/v1" {
		t.Errorf("after switch: ANTHROPIC_BASE_URL = %q, want provider-b URL", got)
	}
	if got := env["ANTHROPIC_MODEL"]; got != "model-b" {
		t.Errorf("after switch: ANTHROPIC_MODEL = %q, want model-b", got)
	}
}

func TestStartSession_ProviderEnvInjected(t *testing.T) {
	a := &Agent{
		providers: []core.ProviderConfig{
			{
				Name:    "custom",
				BaseURL: "https://custom.example.com/v1",
				APIKey:  "test-key",
				Model:   "custom-model",
			},
		},
		activeIdx: 0,
		sessionEnv: []string{
			"CC_PROJECT=test",
		},
		agentEnv: []string{
			"AGENT_EXTRA=1",
		},
	}

	a.mu.RLock()
	providerEnv := a.providerEnvLocked()
	sessionEnv := a.sessionEnv
	agentEnv := a.agentEnv
	a.mu.RUnlock()

	// Simulate the env assembly that happens in StartSession
	var extraEnv []string
	extraEnv = append(extraEnv, providerEnv...)
	extraEnv = append(extraEnv, sessionEnv...)
	extraEnv = append(extraEnv, agentEnv...)

	envMap := envSliceToMap(extraEnv)

	if got := envMap["ANTHROPIC_BASE_URL"]; got != "https://custom.example.com/v1" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want custom base URL", got)
	}
	if got := envMap["ANTHROPIC_AUTH_TOKEN"]; got != "test-key" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN = %q, want test-key", got)
	}
	if got := envMap["ANTHROPIC_API_KEY"]; got != "" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want empty", got)
	}
	if got := envMap["ANTHROPIC_MODEL"]; got != "custom-model" {
		t.Errorf("ANTHROPIC_MODEL = %q, want custom-model", got)
	}
	if got := envMap["CC_PROJECT"]; got != "test" {
		t.Errorf("CC_PROJECT = %q, want test", got)
	}
	if got := envMap["AGENT_EXTRA"]; got != "1" {
		t.Errorf("AGENT_EXTRA = %q, want 1", got)
	}
}

func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}
