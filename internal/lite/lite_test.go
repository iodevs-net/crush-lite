package lite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputModeString(t *testing.T) {
	tests := []struct {
		mode     OutputMode
		expected string
	}{
		{ModeHuman, "human"},
		{ModePlain, "plain"},
		{ModeAgent, "agent"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name      string
		mode      OutputMode
		quiet     bool
		verbose   bool
		skipPerms bool
		yolo      bool
	}{
		{"human mode", ModeHuman, false, false, false, false},
		{"plain mode", ModePlain, false, false, false, false},
		{"agent mode", ModeAgent, false, false, false, false},
		{"quiet mode", ModeHuman, true, false, false, false},
		{"verbose mode", ModeHuman, false, true, false, false},
		{"skip perms", ModeHuman, false, false, true, false},
		{"yolo mode", ModeHuman, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner(tt.mode, tt.quiet, tt.verbose, tt.skipPerms, tt.yolo)
			require.NotNil(t, r)
			require.Equal(t, tt.mode, r.mode)
			require.Equal(t, tt.quiet, r.quiet)
			require.Equal(t, tt.verbose, r.verbose)
			require.Equal(t, tt.skipPerms, r.skipPerms)
			require.Equal(t, tt.yolo, r.yolo)
			require.NotNil(t, r.renderer)
			require.NotNil(t, r.inputReader)
			require.NotNil(t, r.permResponses)
			require.NotNil(t, r.messageStates)
		})
	}
}

func TestPermissionResponse(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		action PermissionAction
	}{
		{"deny", "test-1", PermDeny},
		{"allow", "test-2", PermAllow},
		{"allow session", "test-3", PermAllowSession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := PermissionResponse{ID: tt.id, Action: tt.action}
			require.Equal(t, tt.id, resp.ID)
			require.Equal(t, tt.action, resp.Action)
		})
	}
}

func TestAgentEvent(t *testing.T) {
	evt := AgentEvent{
		Type:    "test",
		Content: "test content",
		Time:    1234567890,
	}

	require.Equal(t, "test", evt.Type)
	require.Equal(t, "test content", evt.Content)
	require.Equal(t, int64(1234567890), evt.Time)
}

func TestUseClientServer(t *testing.T) {
	// Test default behavior (CRUSH_CLIENT_SERVER is not set, so it should return true)
	result := useClientServer()
	require.True(t, result)
}
