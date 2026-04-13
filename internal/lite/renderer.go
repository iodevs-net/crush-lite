package lite

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/mattn/go-runewidth"
)

// Styles for human-readable output.
var (
	// Colors for terminal output.
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	resultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	permStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	thinkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
)

// Renderer handles formatting of events for display.
type Renderer struct {
	mode OutputMode
}

// NewRenderer creates a new Renderer with the specified output mode.
func NewRenderer(mode OutputMode) *Renderer {
	return &Renderer{mode: mode}
}

// FormatThinking formats reasoning/thinking content.
func (r *Renderer) FormatThinking(content string) string {
	if r.mode == ModePlain {
		return formatBox("THINKING", content, "...")
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		content = strings.Join(lines[:3], "\n") + "\n  ..."
	}
	return fmt.Sprintf("%s\n%s\n\n", thinkStyle.Render("💭 Thinking..."), indent(content, 2))
}

// FormatToolCall formats a tool call for display.
func (r *Renderer) FormatToolCall(tc proto.ToolCall) string {
	toolName := tc.Name
	if toolName == "" {
		toolName = "Unknown"
	}

	var details strings.Builder
	details.WriteString(toolStyle.Render("[" + toolName + "]"))

	// Parse input for relevant details
	var input map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &input); err == nil {
		switch toolName {
		case "Bash", "bash":
			if cmd, ok := input["command"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(truncate(cmd, 100)))
			}
		case "View", "view", "ReadFile", "read_file":
			if path, ok := input["path"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(path))
			}
		case "Write", "write", "WriteFile", "write_file":
			if path, ok := input["path"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(path))
			}
		case "Edit", "edit":
			if path, ok := input["path"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(path))
			}
		case "Grep", "grep", "Search", "search":
			if pattern, ok := input["pattern"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render("\"" + pattern + "\""))
			}
		case "Glob", "glob":
			if pattern, ok := input["pattern"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(pattern))
			}
		case "WebFetch", "web_fetch", "Fetch", "fetch":
			if url, ok := input["url"].(string); ok {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(truncate(url, 60)))
			}
		default:
			// Generic format for unknown tools
			if len(input) > 0 {
				details.WriteString(" ")
				details.WriteString(dimStyle.Render(fmt.Sprintf("%v", input)))
			}
		}
	}

	return details.String() + "\n"
}

// FormatToolResult formats a tool result for display.
func (r *Renderer) FormatToolResult(tr proto.ToolResult) string {
	if tr.IsError {
		return fmt.Sprintf("%s %s\n\n", errorStyle.Render("✗"), truncate(tr.Content, 200))
	}

	// Truncate long results
	content := tr.Content
	if len(content) > 500 {
		content = content[:500] + "\n  ..."
	}

	return fmt.Sprintf("%s %s\n\n", resultStyle.Render("✓"), indent(content, 2))
}

// FormatUserMessage formats a user message for display.
func (r *Renderer) FormatUserMessage(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		content = strings.Join(lines[:3], "\n") + "\n  ..."
	}
	return fmt.Sprintf("%s %s\n\n", toolStyle.Render(">"), content)
}

// FormatPermissionRequestHuman formats a permission request for human interaction.
func (r *Renderer) FormatPermissionRequestHuman(req proto.PermissionRequest) string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, permStyle.Render("┌─────────────────────────────────────────────────────────────┐"))
	lines = append(lines, permStyle.Render("│ PERMISO REQUERIDO                                              │"))
	lines = append(lines, permStyle.Render("└─────────────────────────────────────────────────────────────┘"))
	lines = append(lines, "")

	// Tool info
	lines = append(lines, fmt.Sprintf("  Herramienta: %s", toolStyle.Render(req.ToolName)))

	// Action/description
	if req.Description != "" {
		lines = append(lines, fmt.Sprintf("  Acción: %s", req.Description))
	}

	// Path if relevant
	if req.Path != "" {
		lines = append(lines, fmt.Sprintf("  Ruta: %s", dimStyle.Render(req.Path)))
	}

	// Params
	if req.Params != nil {
		if params, ok := req.Params.(map[string]any); ok {
			var paramLines []string
			for k, v := range params {
				if s, ok := v.(string); ok && len(s) > 100 {
					v = s[:100] + "..."
				}
				paramLines = append(paramLines, fmt.Sprintf("    %s: %v", k, v))
			}
			if len(paramLines) > 0 {
				lines = append(lines, "  Parámetros:")
				lines = append(lines, paramLines...)
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, warnStyle.Render("  ¿Permitir? [y]es / [n]o / [a]llow always: "))

	return strings.Join(lines, "\n")
}

// FormatPermissionRequestJSON formats a permission request as JSON for agent mode.
func (r *Renderer) FormatPermissionRequestJSON(req proto.PermissionRequest) string {
	data := map[string]any{
		"id":          req.ID,
		"tool_name":   req.ToolName,
		"description": req.Description,
		"action":      req.Action,
		"path":        req.Path,
		"params":      req.Params,
	}
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// FormatPermissionDenied formats a permission denied notification.
func (r *Renderer) FormatPermissionDenied() string {
	return fmt.Sprintf("%s Permiso denegado\n\n", errorStyle.Render("✗"))
}

// FormatHelp formats the help text.
func (r *Renderer) FormatHelp() string {
	return `
Comandos disponibles:
  help, ?     - Mostrar esta ayuda
  quit, exit, q - Salir

  En modo interactivo, puedes escribir cualquier prompt.
`
}

// formatBox creates a formatted box with title and content.
func formatBox(title, content, prefix string) string {
	width := 60
	lines := []string{
		fmt.Sprintf("┌─ %s %s", title, strings.Repeat("─", width-len(title)-len(prefix)-4)) + "┐",
	}
	for _, line := range strings.Split(content, "\n") {
		padding := width - runewidth.StringWidth(line) - 2
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, fmt.Sprintf("│ %s%s │", line, strings.Repeat(" ", padding)))
	}
	lines = append(lines, fmt.Sprintf("└%s┘", strings.Repeat("─", width-1)))
	return strings.Join(lines, "\n")
}

// indent indents text by the specified number of spaces.
func indent(text string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	return prefix + strings.ReplaceAll(text, "\n", "\n"+prefix)
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
