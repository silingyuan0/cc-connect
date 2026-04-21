package claudecodesdk

// ── Go → Sidecar commands (written to stdin) ────────────────

// sidecarCommand is a command sent from Go to the sidecar via stdin.
type sidecarCommand struct {
	Type    string `json:"type"`              // "send" | "close" | "abort"
	Content string `json:"content,omitempty"` // for "send"
}

// sidecarSetMode is a hot mode switch command.
type sidecarSetMode struct {
	Type string `json:"type"` // always "set_mode"
	Mode string `json:"mode"`
}

// sidecarAbort is an abort command.
type sidecarAbort struct {
	Type string `json:"type"` // always "abort"
}

// sidecarPermission is a permission response sent to the sidecar.
type sidecarPermission struct {
	Type      string            `json:"type"`      // always "permission"
	RequestID string            `json:"requestId"`
	Result    sidecarPermResult `json:"result"`
}

type sidecarPermResult struct {
	Behavior     string         `json:"behavior"`               // "allow" | "deny"
	UpdatedInput map[string]any `json:"updatedInput,omitempty"` // for allow
	Message      string         `json:"message,omitempty"`      // for deny
}

// ── Sidecar → Go events (read from stdout) ──────────────────

// sidecarEvent is an event received from the sidecar via stdout.
type sidecarEvent struct {
	Event     string   `json:"event"`
	Content   string   `json:"content,omitempty"`
	Subtype   string   `json:"subtype,omitempty"`
	ToolName  string   `json:"toolName,omitempty"`
	ToolID    string   `json:"toolId,omitempty"`
	Input     any      `json:"input,omitempty"`
	SessionID string   `json:"sessionId,omitempty"`
	Model     string   `json:"model,omitempty"`
	CWD       string   `json:"cwd,omitempty"`
	Tools         []string `json:"tools,omitempty"`
	SlashCommands []string `json:"slashCommands,omitempty"`
	Message       string   `json:"message,omitempty"`
	RequestID     string   `json:"requestId,omitempty"`

	// Result fields
	Usage *sidecarUsage `json:"usage,omitempty"`
	Cost  *float64      `json:"cost,omitempty"`
}

type sidecarUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ── Sidecar config (passed via SIDECAR_CONFIG env var) ───────

// sidecarConfig holds SDK query options passed to the sidecar via the
// SIDECAR_CONFIG environment variable as a JSON string.
// Fields map directly to SDK Options as documented in sdk.d.ts.
type sidecarConfig struct {
	// ── System prompt ──────────────────────────────────────
	// CustomSystemPrompt (DISABLED: 会完全覆盖 cc-connect 系统提示，导致 cron/relay 指令丢失)
	// CustomSystemPrompt string `json:"customSystemPrompt,omitempty"`
	AppendSystemPrompt string `json:"appendSystemPrompt,omitempty"`

	// ── Reasoning / thinking ───────────────────────────────
	// Effort level: "low", "medium", "high", "xhigh", "max"
	Effort string `json:"effort,omitempty"`
	// Thinking config: {"type":"adaptive"} or {"type":"enabled","budgetTokens":N} or {"type":"disabled"}
	Thinking map[string]any `json:"thinking,omitempty"`

	// ── Tool restrictions ──────────────────────────────────
	AllowedTools    []string `json:"allowedTools,omitempty"`
	DisallowedTools []string `json:"disallowedTools,omitempty"`
	// Explicit tool set (DISABLED: cc-connect 需要完整工具能力)
	// Tools any `json:"tools,omitempty"`

	// ── MCP servers ────────────────────────────────────────
	MCPServers map[string]any `json:"mcpServers,omitempty"`

	// ── Settings ───────────────────────────────────────────
	// Path to settings JSON file or inline settings object
	SettingsPath string `json:"settingsPath,omitempty"`
	// Which filesystem setting sources to load: "user", "project", "local"
	SettingSources []string `json:"settingSources,omitempty"`

	// ── Limits ─────────────────────────────────────────────
	MaxTurns     int     `json:"maxTurns,omitempty"`
	MaxBudgetUsd float64 `json:"maxBudgetUsd,omitempty"`

	// ── Model ──────────────────────────────────────────────
	FallbackModel string `json:"fallbackModel,omitempty"`

	// ── Agent system (DISABLED: cc-connect 不需要自定义子代理) ──
	// Named agent to use (defined in agents or settings)
	// Agent string `json:"agent,omitempty"`
	// Programmatically defined subagents
	// Agents map[string]any `json:"agents,omitempty"`

	// ── Plugins (DISABLED: 无插件生态使用) ─────────────────────
	// Plugins to load (e.g. [{type:"local",path:"./my-plugin"}])
	// Plugins []any `json:"plugins,omitempty"`

	// ── Sandbox (DISABLED: 容器/VM 环境下冗余) ────────────────
	// Sandbox settings for command execution isolation
	// Sandbox map[string]any `json:"sandbox,omitempty"`

	// ── Other (DISABLED: 小众/桌面端特性) ────────────────────
	// StrictMCPConfig      bool   `json:"strictMcpConfig,omitempty"`
	// IncludeHookEvents    bool   `json:"includeHookEvents,omitempty"`
	// EnableFileCheckpoint bool   `json:"enableFileCheckpointing,omitempty"`
	// Debug                bool   `json:"debug,omitempty"`
	// DebugFile            string `json:"debugFile,omitempty"`

	// ── Session (DISABLED: cc-connect 通过 session ID 自行管理恢复) ──
	// Continue the most recent conversation instead of starting new one.
	// Continue bool `json:"continue,omitempty"`
}
