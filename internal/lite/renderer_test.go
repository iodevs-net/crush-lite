package lite

import (
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestRendererFormatThinking(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		content  string
		contains string
	}{
		{"human mode", ModeHuman, "This is a test", "Thinking"},
		{"plain mode", ModePlain, "This is a test", "THINKING"},
		{"short content", ModeHuman, "Short", "Short"},
		{"long content", ModeHuman, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5", "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer(tt.mode)
			result := r.FormatThinking(tt.content)
			require.NotEmpty(t, result)
		})
	}
}

func TestRendererFormatToolCall(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		toolCall proto.ToolCall
		contains string
	}{
		{
			"bash command",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-1",
				Name:  "Bash",
				Input: `{"command": "ls -la"}`,
			},
			"Bash",
		},
		{
			"view file",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-2",
				Name:  "View",
				Input: `{"path": "/tmp/test.txt"}`,
			},
			"View",
		},
		{
			"write file",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-3",
				Name:  "Write",
				Input: `{"path": "/tmp/new.txt"}`,
			},
			"Write",
		},
		{
			"edit file",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-4",
				Name:  "Edit",
				Input: `{"path": "/tmp/edit.txt"}`,
			},
			"Edit",
		},
		{
			"grep",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-5",
				Name:  "Grep",
				Input: `{"pattern": "test"}`,
			},
			"Grep",
		},
		{
			"glob",
			ModeHuman,
			proto.ToolCall{
				ID:    "tc-6",
				Name:  "Glob",
				Input: `{"pattern": "*.go"}`,
			},
			"Glob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer(tt.mode)
			result := r.FormatToolCall(tt.toolCall)
			require.NotEmpty(t, result)
			require.Contains(t, result, tt.contains)
		})
	}
}

func TestRendererFormatToolResult(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		result   proto.ToolResult
		contains string
	}{
		{
			"success result",
			ModeHuman,
			proto.ToolResult{
				ToolCallID: "tc-1",
				Name:       "Bash",
				Content:    "file1.txt\nfile2.txt",
				IsError:    false,
			},
			"✓",
		},
		{
			"error result",
			ModeHuman,
			proto.ToolResult{
				ToolCallID: "tc-2",
				Name:       "Bash",
				Content:    "Permission denied",
				IsError:    true,
			},
			"✗",
		},
		{
			"long content truncated",
			ModeHuman,
			proto.ToolResult{
				ToolCallID: "tc-3",
				Name:       "View",
				Content:    string(make([]byte, 600)),
				IsError:    false,
			},
			"...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer(tt.mode)
			result := r.FormatToolResult(tt.result)
			require.NotEmpty(t, result)
			require.Contains(t, result, tt.contains)
		})
	}
}

func TestRendererFormatUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		content  string
		contains string
	}{
		{"simple", ModeHuman, "Hello", ">"},
		{"multiline", ModeHuman, "Line 1\nLine 2", ">"},
		{"truncated", ModeHuman, "Line 1\nLine 2\nLine 3\nLine 4", "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRenderer(tt.mode)
			result := r.FormatUserMessage(tt.content)
			require.NotEmpty(t, result)
			require.Contains(t, result, tt.contains)
		})
	}
}

func TestRendererFormatPermissionRequestHuman(t *testing.T) {
	r := NewRenderer(ModeHuman)

	req := proto.PermissionRequest{
		ID:          "perm-1",
		SessionID:   "sess-1",
		ToolName:    "Bash",
		Description: "Execute command",
		Action:      "execute",
		Path:        "/bin/ls",
	}

	result := r.FormatPermissionRequestHuman(req)
	require.NotEmpty(t, result)
	require.Contains(t, result, "PERMISO")
	require.Contains(t, result, "Bash")
	require.Contains(t, result, "Execute command")
	require.Contains(t, result, "/bin/ls")
}

func TestRendererFormatPermissionRequestJSON(t *testing.T) {
	r := NewRenderer(ModeAgent)

	req := proto.PermissionRequest{
		ID:          "perm-1",
		SessionID:   "sess-1",
		ToolName:    "Bash",
		Description: "Execute command",
		Action:      "execute",
		Path:        "/bin/ls",
	}

	result := r.FormatPermissionRequestJSON(req)
	require.NotEmpty(t, result)
	require.Contains(t, result, "perm-1")
	require.Contains(t, result, "Bash")
	require.Contains(t, result, "Execute command")
}

func TestRendererFormatPermissionDenied(t *testing.T) {
	r := NewRenderer(ModeHuman)
	result := r.FormatPermissionDenied()
	require.NotEmpty(t, result)
	require.Contains(t, result, "denegado")
	require.Contains(t, result, "✗")
}

func TestRendererFormatHelp(t *testing.T) {
	r := NewRenderer(ModeHuman)
	result := r.FormatHelp()
	require.NotEmpty(t, result)
	require.Contains(t, result, "help")
	require.Contains(t, result, "quit")
	require.Contains(t, result, "exit")
}
