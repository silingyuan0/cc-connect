package claudecodesdk

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestSDKSessionMultiTurn(t *testing.T) {
	nodePath := findNode(t)
	claudePath := findClaude(t)
	sidecarPath := findSidecar(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sess, err := newSDKSession(ctx, nodePath, sidecarPath, "/tmp", "", "bypassPermissions", "", claudePath, nil)
	if err != nil {
		t.Fatalf("newSDKSession: %v", err)
	}
	defer sess.Close()

	// Turn 1: send first message
	if err := sess.Send("回复两个字：你好", nil, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var turn1SessionID string
	for evt := range sess.Events() {
		t.Logf("Turn 1 event: %s content=%q sessionID=%s", evt.Type, truncateStr(evt.Content, 50), evt.SessionID)
		if evt.Type == core.EventResult {
			turn1SessionID = evt.SessionID
			break
		}
		if evt.Type == core.EventError {
			t.Fatalf("Turn 1 error: %v", evt.Error)
		}
	}

	if turn1SessionID == "" {
		t.Fatal("no session ID received in turn 1")
	}
	t.Logf("Turn 1 session ID: %s", turn1SessionID)

	// Turn 2: multi-turn in same query — should keep same session ID
	if err := sess.Send("我上一条消息说了什么？只回答内容", nil, nil); err != nil {
		t.Fatalf("Send 2: %v", err)
	}

	var turn2SessionID string
	for evt := range sess.Events() {
		t.Logf("Turn 2 event: %s content=%q sessionID=%s", evt.Type, truncateStr(evt.Content, 50), evt.SessionID)
		if evt.Type == core.EventResult {
			turn2SessionID = evt.SessionID
			break
		}
		if evt.Type == core.EventError {
			t.Fatalf("Turn 2 error: %v", evt.Error)
		}
	}

	// Verify session ID stays the same across turns
	if turn2SessionID != turn1SessionID {
		t.Errorf("session ID changed between turns: turn1=%s turn2=%s", turn1SessionID, turn2SessionID)
	} else {
		t.Logf("SUCCESS: session ID consistent across turns: %s", turn1SessionID)
	}
}

func findNode(t *testing.T) string {
	t.Helper()
	paths := []string{
		"/home/bot/.nvm/versions/node/v24.14.1/bin/node",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("node not found — skipping integration test")
	return ""
}

func findClaude(t *testing.T) string {
	t.Helper()
	paths := []string{
		"/home/bot/.local/bin/claude",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("claude CLI not found — skipping integration test")
	return ""
}

func findSidecar(t *testing.T) string {
	t.Helper()
	p := "../../sidecar/sidecar.mjs"
	if _, err := os.Stat(p); err == nil {
		return p
	}
	t.Skip("sidecar not found — run 'cd sidecar && npm install' first")
	return ""
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
