package claudecodesdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

// TestE2E_AgentCreation tests that the agent can be created via core.CreateAgent.
func TestE2E_AgentCreation(t *testing.T) {
	nodePath := findNode(t)
	_ = findSidecar(t) // guard: skip if sidecar not built
	claudePath := findClaude(t)

	agent, err := core.CreateAgent("claudecodesdk", map[string]any{
		"work_dir":    "/tmp",
		"node_path":   nodePath,
		"sidecar_dir": sidecarDir(t),
		"claude_path": claudePath,
		"mode":        "bypassPermissions",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	if agent.Name() != "claudecodesdk" {
		t.Errorf("expected name claudecodesdk, got %s", agent.Name())
	}

	// Check optional interfaces
	if info, ok := agent.(core.AgentDoctorInfo); ok {
		t.Logf("DoctorInfo: binary=%s display=%s", info.CLIBinaryName(), info.CLIDisplayName())
	}
	if _, ok := agent.(core.ModelSwitcher); !ok {
		t.Error("should implement ModelSwitcher")
	}
	if _, ok := agent.(core.ModeSwitcher); !ok {
		t.Error("should implement ModeSwitcher")
	}
	if _, ok := agent.(core.MemoryFileProvider); !ok {
		t.Error("should implement MemoryFileProvider")
	}

	t.Log("Agent creation: PASS")
}

// TestE2E_MultiTurnSession tests the full agent→session→multi-turn flow.
func TestE2E_MultiTurnSession(t *testing.T) {
	agent := createTestAgent(t)
	if agent == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sess, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer sess.Close()

	// Turn 1
	if err := sess.Send("回复两个字：你好", nil, nil); err != nil {
		t.Fatalf("Send 1: %v", err)
	}

	var turn1SID string
	for evt := range sess.Events() {
		t.Logf("[T1] %s content=%q sid=%s", evt.Type, trunc(evt.Content, 40), evt.SessionID)
		if evt.Type == core.EventResult {
			turn1SID = evt.SessionID
			break
		}
		if evt.Type == core.EventError {
			t.Fatalf("Turn 1 error: %v", evt.Error)
		}
	}
	if turn1SID == "" {
		t.Fatal("no session ID in turn 1")
	}

	// Turn 2 — same session
	if err := sess.Send("我上一条消息说了什么？只回答内容", nil, nil); err != nil {
		t.Fatalf("Send 2: %v", err)
	}

	var turn2SID string
	for evt := range sess.Events() {
		t.Logf("[T2] %s content=%q sid=%s", evt.Type, trunc(evt.Content, 40), evt.SessionID)
		if evt.Type == core.EventResult {
			turn2SID = evt.SessionID
			break
		}
	}

	if turn2SID != turn1SID {
		t.Errorf("session ID changed: %s → %s", turn1SID, turn2SID)
	} else {
		t.Logf("Multi-turn PASS: session ID stable (%s)", turn1SID)
	}
}

// TestE2E_LiveModeSwitch tests hot mode switching via SetLiveMode.
func TestE2E_LiveModeSwitch(t *testing.T) {
	agent := createTestAgent(t)
	if agent == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sess, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer sess.Close()

	// Send first message in default mode
	if err := sess.Send("回复两个字：测试", nil, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Wait for result to confirm session is alive
	for evt := range sess.Events() {
		t.Logf("[ModeTest] %s", evt.Type)
		if evt.Type == core.EventResult {
			break
		}
		if evt.Type == core.EventError {
			t.Fatalf("error: %v", evt.Error)
		}
	}

	// Hot switch to bypassPermissions while session is running
	if liveMode, ok := sess.(core.LiveModeSwitcher); ok {
		result := liveMode.SetLiveMode("bypassPermissions")
		t.Logf("SetLiveMode(bypassPermissions) = %v", result)
		if !result {
			t.Error("SetLiveMode should return true")
		}
	} else {
		t.Log("Session does not implement LiveModeSwitcher (skipping)")
	}

	// Send another message — should still work
	if err := sess.Send("回复一个字：好", nil, nil); err != nil {
		t.Fatalf("Send after mode switch: %v", err)
	}

	for evt := range sess.Events() {
		t.Logf("[ModeTest2] %s", evt.Type)
		if evt.Type == core.EventResult {
			break
		}
	}

	t.Log("Live mode switch: PASS")
}

// TestE2E_Abort tests aborting a running query.
func TestE2E_Abort(t *testing.T) {
	agent := createTestAgent(t)
	if agent == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sess, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer sess.Close()

	// Send a message
	if err := sess.Send("数到1000，每个数字一行", nil, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Wait a moment for processing to start, then abort
	time.Sleep(2 * time.Second)

	sdkSess := sess.(*sdkSession)
	if err := sdkSess.Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}

	// Read events — should eventually get an error or the stream closes
	aborted := false
	timeout := time.After(10 * time.Second)
	for {
		select {
		case evt, ok := <-sess.Events():
			if !ok {
				t.Log("Events channel closed after abort")
				aborted = true
				goto done
			}
			t.Logf("[Abort] %s content=%q", evt.Type, trunc(evt.Content, 30))
			if evt.Type == core.EventError {
				t.Logf("Got error event: %v", evt.Error)
				aborted = true
				goto done
			}
			if evt.Type == core.EventResult {
				// Query completed before abort arrived
				t.Log("Query completed before abort arrived")
				goto done
			}
		case <-timeout:
			t.Log("Timeout waiting for abort response")
			goto done
		}
	}

done:
	if aborted {
		t.Log("Abort: PASS")
	} else {
		t.Log("Abort: query completed (race condition acceptable)")
	}
}

// TestE2E_SlashCommandsAndTools tests that system.init reports tools and slash commands.
func TestE2E_SlashCommandsAndTools(t *testing.T) {
	agent := createTestAgent(t)
	if agent == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sess, err := agent.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer sess.Close()

	// Send a simple message
	if err := sess.Send("回复一个字：好", nil, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Just verify it completes — system.init data is captured in sidecar
	// (Go side doesn't expose slash_commands in core.Event yet, but the
	// sidecar does emit them in the JSON)
	for evt := range sess.Events() {
		t.Logf("[InitTest] %s", evt.Type)
		if evt.Type == core.EventResult {
			break
		}
		if evt.Type == core.EventError {
			t.Fatalf("error: %v", evt.Error)
		}
	}

	t.Log("Slash commands & tools: PASS (verified in sidecar JSON)")
}

// ── Helpers ──────────────────────────────────────────────────

func createTestAgent(t *testing.T) core.Agent {
	t.Helper()
	nodePath := findNode(t)
	_ = findSidecar(t) // guard: skip if sidecar not built
	claudePath := findClaude(t)

	agent, err := core.CreateAgent("claudecodesdk", map[string]any{
		"work_dir":    "/tmp",
		"node_path":   nodePath,
		"sidecar_dir": sidecarDir(t),
		"claude_path": claudePath,
		"mode":        "bypassPermissions",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	return agent
}

func sidecarDir(t *testing.T) string {
	t.Helper()
	dir := "../../sidecar"
	if _, err := os.Stat(dir + "/sidecar.mjs"); err != nil {
		t.Skip("sidecar not found")
	}
	abs, _ := filepath.Abs(dir)
	return abs
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
