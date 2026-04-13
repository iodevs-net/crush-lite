// Package lite provides a lightweight CLI interface for Crush, designed for
// both human users and external AI agents.
package lite

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/proto"
)

// Renderer handles formatting of events for display.
// Output is plain text with clear prefixes for universal CLI readability.
type Renderer struct {
	mode OutputMode
}

// NewRenderer creates a new Renderer with the specified output mode.
func NewRenderer(mode OutputMode) *Renderer {
	return &Renderer{mode: mode}
}

// Prefix styles for different event types
const (
	PrefixUser     = "[USER]"
	PrefixAssistant = "[CRUSH]"
	PrefixThinking  = "[THINKING]"
	PrefixTool      = "[TOOL]"
	PrefixWrite     = "[WRITE]"
	PrefixRead      = "[READ]"
	PrefixEdit      = "[EDIT]"
	PrefixAsk       = "[ASK]"
	PrefixPerm      = "[PERM]"
	PrefixError     = "[ERROR]"
	PrefixOK        = "[OK]"
	PrefixMCP       = "[MCP]"
)

// FormatUser formats a user message.
func (r *Renderer) FormatUser(content string) string {
	return fmt.Sprintf("%s %s\n", PrefixUser, content)
}

// FormatAssistant formats an assistant response.
func (r *Renderer) FormatAssistant(content string) string {
	// Remove thinking markers if present
	content = strings.ReplaceAll(content, "<reasoning>", "")
	content = strings.ReplaceAll(content, "</reasoning>", "")
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return fmt.Sprintf("%s %s\n", PrefixAssistant, content)
}

// FormatThinking formats thinking/reasoning content.
func (r *Renderer) FormatThinking(content string) string {
	if content == "" {
		return ""
	}
	// Clean up thinking content
	content = strings.ReplaceAll(content, "<reasoning>", "")
	content = strings.ReplaceAll(content, "</reasoning>", "")
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return fmt.Sprintf("%s %s\n", PrefixThinking, content)
}

// FormatToolCall formats a tool call.
func (r *Renderer) FormatToolCall(tc proto.ToolCall) string {
	toolName := tc.Name
	if toolName == "" {
		toolName = "unknown"
	}

	var details string
	var input map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &input); err == nil {
		switch toolName {
		case "Bash", "bash":
			if cmd, ok := input["command"].(string); ok {
				details = cmd
			}
		case "View", "view", "ReadFile", "read_file":
			if path, ok := input["path"].(string); ok {
				details = path
				return fmt.Sprintf("%s %s %s\n", PrefixRead, toolName, details)
			}
		case "Write", "write", "WriteFile", "write_file":
			if path, ok := input["path"].(string); ok {
				details = path
				return fmt.Sprintf("%s %s %s\n", PrefixWrite, toolName, details)
			}
		case "Edit", "edit":
			if path, ok := input["path"].(string); ok {
				details = path
				return fmt.Sprintf("%s %s %s\n", PrefixEdit, toolName, details)
			}
		case "Glob", "glob", "Grep", "grep", "Search", "search":
			if pattern, ok := input["pattern"].(string); ok {
				details = pattern
			}
		case "WebFetch", "web_fetch", "Fetch", "fetch":
			if url, ok := input["url"].(string); ok {
				details = url
			}
		default:
			details = toolName
		}
	}

	if details != "" {
		return fmt.Sprintf("%s %s %s\n", PrefixTool, toolName, details)
	}
	return fmt.Sprintf("%s %s\n", PrefixTool, toolName)
}

// FormatToolResult formats a tool result.
func (r *Renderer) FormatToolResult(tr proto.ToolResult) string {
	if tr.IsError {
		content := tr.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		return fmt.Sprintf("%s %s\n", PrefixError, content)
	}

	content := tr.Content
	if len(content) > 500 {
		content = content[:500] + "..."
	}
	// Make it a single line for clarity
	content = strings.ReplaceAll(content, "\n", "\\n")
	return fmt.Sprintf("%s %s\n", PrefixOK, content)
}

// FormatPermission formats a permission request.
func (r *Renderer) FormatPermission(req proto.PermissionRequest) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%s %s", PrefixPerm, req.ToolName))

	if req.Description != "" {
		lines = append(lines, fmt.Sprintf("   %s", req.Description))
	}
	if req.Path != "" {
		lines = append(lines, fmt.Sprintf("   %s", req.Path))
	}

	// Add params if relevant
	if req.Params != nil {
		if params, ok := req.Params.(map[string]any); ok {
			for k, v := range params {
				if s, ok := v.(string); ok && len(s) > 80 {
					v = s[:80] + "..."
				}
				lines = append(lines, fmt.Sprintf("   %s=%v", k, v))
			}
		}
	}

	lines = append(lines, fmt.Sprintf("   [y]es / [n]o / [a]lways"))
	return strings.Join(lines, "\n") + "\n"
}

// FormatError formats an error message.
func (r *Renderer) FormatError(err string) string {
	return fmt.Sprintf("%s %s\n", PrefixError, err)
}

// FormatAsk formats a question to the user.
func (r *Renderer) FormatAsk(question string) string {
	return fmt.Sprintf("%s %s [y/n]: ", PrefixAsk, question)
}

// FormatMCP formats an MCP server event.
func (r *Renderer) FormatMCP(server, action, details string) string {
	if details != "" {
		return fmt.Sprintf("%s %s: %s - %s\n", PrefixMCP, server, action, details)
	}
	return fmt.Sprintf("%s %s: %s\n", PrefixMCP, server, action)
}
