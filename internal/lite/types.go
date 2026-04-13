// Package lite provides a lightweight CLI interface for Crush, designed for
// both human users and external AI agents.
package lite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// OutputMode defines how the CLI outputs information.
type OutputMode int

const (
	// ModeHuman is the default mode with clear, plain text output.
	ModeHuman OutputMode = iota
	// ModePlain is plain text output without any formatting.
	ModePlain
	// ModeAgent is JSON-structured output for external AI agents.
	ModeAgent
)

// String returns the string representation of OutputMode.
func (m OutputMode) String() string {
	switch m {
	case ModePlain:
		return "plain"
	case ModeAgent:
		return "agent"
	default:
		return "human"
	}
}

// PermissionAction represents what action to take on a permission request.
type PermissionAction int

const (
	// PermDeny denies the permission request.
	PermDeny PermissionAction = iota
	// PermAllow allows the permission request once.
	PermAllow
	// PermAllowSession allows the permission request for the entire session.
	PermAllowSession
)

// Runner handles the execution of Crush Lite commands.
type Runner struct {
	client    *client.Client
	workspace *proto.Workspace
	sessionID string

	mode      OutputMode
	quiet     bool
	verbose   bool
	skipPerms bool
	yolo      bool

	renderer    *Renderer
	inputReader *InputReader
	outputMutex sync.Mutex

	// Channel for permission responses from interactive mode
	permResponses chan PermissionResponse

	// Track message streaming for incremental output
	messageStates map[string]int // messageID -> bytesRead
	// Track what we've already output to avoid duplicates
	outputCache map[string]string

	// Agent finished flag
	agentFinished     bool
	agentFinishedMux sync.Mutex

	// Workspace path for local operations
	wsPath string
}

// PermissionResponse represents a response to a permission request.
type PermissionResponse struct {
	ID     string
	Action PermissionAction
}

// NewRunner creates a new Lite runner.
func NewRunner(mode OutputMode, quiet, verbose, skipPerms, yolo bool) *Runner {
	r := &Runner{
		mode:          mode,
		quiet:         quiet,
		verbose:       verbose,
		skipPerms:     skipPerms,
		yolo:          yolo,
		renderer:      NewRenderer(mode),
		inputReader:   NewInputReader(),
		permResponses: make(chan PermissionResponse, 10),
		messageStates: make(map[string]int),
		outputCache:   make(map[string]string),
	}
	return r
}

// Run executes a prompt in lite mode.
func (r *Runner) Run(ctx context.Context, cwd string, prompt string, largeModel, smallModel string, continueSessionID string) error {
	event.SetNonInteractive(r.mode != ModeHuman)
	event.AppInitialized()

	if r.verbose {
		logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slog.SetDefault(slog.New(logHandler))
	}

	r.wsPath = cwd

	// Use client/server mode
	return r.runWithServer(ctx, cwd, prompt, largeModel, smallModel, continueSessionID)
}

// runWithServer connects to an existing Crush server or starts one locally.
func (r *Runner) runWithServer(ctx context.Context, cwd, prompt, largeModel, smallModel, continueSessionID string) error {
	// Check if we should use an existing server or start local mode
	if useClientServer() {
		return r.runWithExistingServer(ctx, cwd, prompt, largeModel, smallModel, continueSessionID)
	}
	return r.runLocal(ctx, cwd, prompt, largeModel, smallModel, continueSessionID)
}

// runWithExistingServer connects to a running Crush server.
func (r *Runner) runWithExistingServer(ctx context.Context, cwd, prompt, largeModel, smallModel, continueSessionID string) error {
	// Try to connect to existing server
	c, err := client.DefaultClient(cwd)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Check if server is running
	if err := c.Health(ctx); err != nil {
		return fmt.Errorf("Crush server is not running. Start it with 'crush server' in another terminal")
	}

	r.client = c

	// Get or create workspace
	ws, err := c.GetWorkspace(ctx, cwd)
	if err != nil {
		// Create new workspace
		ws, err = c.CreateWorkspace(ctx, proto.Workspace{
			Path: cwd,
		})
		if err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}
	}

	r.workspace = ws

	if !ws.Config.IsConfigured() {
		return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
	}

	sess, err := r.resolveSession(ctx, continueSessionID)
	if err != nil {
		return fmt.Errorf("failed to resolve session: %w", err)
	}
	r.sessionID = sess.ID

	if err := r.overrideModels(ctx, largeModel, smallModel); err != nil {
		return err
	}

	return r.runLoop(ctx, prompt)
}

// runLocal starts a local Crush instance.
func (r *Runner) runLocal(ctx context.Context, cwd, prompt, largeModel, smallModel, continueSessionID string) error {
	// Initialize config
	store, err := config.Init(cwd, "", false)
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg := store.Config()
	store.Overrides().SkipPermissionRequests = r.skipPerms || r.yolo

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.Options.DataDirectory, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", cfg.Options.DataDirectory, err)
	}

	// This is a simplified local mode that delegates to the app
	// For full local mode, we'd need the full workspace setup from root.go
	return fmt.Errorf("local mode not fully implemented - please start 'crush server' or use CRUSH_CLIENT_SERVER=0 with a running server")
}

// resolveSession gets or creates a session.
func (r *Runner) resolveSession(ctx context.Context, continueSessionID string) (*proto.Session, error) {
	if r.workspace == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}

	switch {
	case continueSessionID != "":
		sess, err := r.client.GetSession(ctx, r.workspace.ID, continueSessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %s", continueSessionID)
		}
		if sess.ParentSessionID != "" {
			return nil, fmt.Errorf("cannot continue a child session: %s", continueSessionID)
		}
		return sess, nil

	default:
		return r.client.CreateSession(ctx, r.workspace.ID, "lite")
	}
}

// overrideModels sets the model overrides if specified.
func (r *Runner) overrideModels(ctx context.Context, largeModel, smallModel string) error {
	if largeModel == "" && smallModel == "" {
		return nil
	}

	if r.workspace == nil {
		return fmt.Errorf("workspace not initialized")
	}

	if largeModel != "" {
		if err := r.client.UpdatePreferredModel(ctx, r.workspace.ID, config.ScopeWorkspace, config.SelectedModelTypeLarge, config.SelectedModel{
			Model: largeModel,
		}); err != nil {
			return fmt.Errorf("failed to set large model: %w", err)
		}
	}

	if smallModel != "" {
		if err := r.client.UpdatePreferredModel(ctx, r.workspace.ID, config.ScopeWorkspace, config.SelectedModelTypeSmall, config.SelectedModel{
			Model: smallModel,
		}); err != nil {
			return fmt.Errorf("failed to set small model: %w", err)
		}
	}

	return r.client.UpdateAgent(ctx, r.workspace.ID)
}

// runLoop handles the main event loop with SSE and interactive input.
func (r *Runner) runLoop(ctx context.Context, prompt string) error {
	if r.workspace == nil {
		return fmt.Errorf("workspace not initialized")
	}

	wsID := r.workspace.ID

	// Subscribe to events
	events, err := r.client.SubscribeEvents(ctx, wsID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Send initial prompt
	if err := r.client.SendMessage(ctx, wsID, r.sessionID, prompt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Start input handler goroutine for interactive mode
	var inputCancel context.CancelFunc
	if r.mode == ModeHuman {
		ctx, inputCancel = context.WithCancel(ctx)
		go r.handleInput(ctx)
	}

	// Timer for graceful shutdown after agent finishes
	finishTimer := time.NewTimer(0)
	if !finishTimer.Stop() {
		<-finishTimer.C
	}
	waitForFinish := false
	eventsClosed := false

	// Main event loop
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// Events channel closed - mark it but don't exit yet
				// We need to wait for the timer if agent finished
				eventsClosed = true
				break
			}

			if err := r.handleEvent(ev); err != nil {
				if inputCancel != nil {
					inputCancel()
				}
				return err
			}

			// Check if agent finished AFTER processing event
			// (agentFinished is set inside handleEvent when agent_finished event is received)
			if r.agentFinished && !waitForFinish {
				waitForFinish = true
				finishTimer.Reset(30 * time.Second)
			}

		case resp := <-r.permResponses:
			if err := r.handlePermissionResponse(ctx, resp); err != nil {
				slog.Error("Failed to handle permission response", "error", err)
			}

		case <-finishTimer.C:
			// Agent finished and we've waited 30 seconds, safe to exit
			if inputCancel != nil {
				inputCancel()
			}
			return nil

		case <-ctx.Done():
			if inputCancel != nil {
				inputCancel()
			}
			return ctx.Err()
		}

		// If events channel closed, we need to wait for the timer
		if eventsClosed {
			// Try to receive from timer channel without blocking
			select {
			case <-finishTimer.C:
				// Timer fired, we're done
				if inputCancel != nil {
					inputCancel()
				}
				return nil
			default:
				// Timer hasn't fired yet, continue to next iteration
				// but we'll block on select waiting for timer or context
			}
		}
	}
}

// handleEvent processes a single event from the SSE stream.
func (r *Runner) handleEvent(ev any) error {
	switch e := ev.(type) {
	case pubsub.Event[proto.Message]:
		return r.handleMessage(e.Payload)
	case pubsub.Event[proto.AgentEvent]:
		return r.handleAgentEvent(e.Payload)
	case pubsub.Event[proto.PermissionRequest]:
		return r.handlePermissionRequest(e.Payload)
	case pubsub.Event[proto.PermissionNotification]:
		r.handlePermissionNotification(e.Payload)
	default:
		if r.verbose {
			slog.Info("Unknown event", "type", fmt.Sprintf("%T", ev))
		}
	}
	return nil
}

// handleMessage processes a message event.
func (r *Runner) handleMessage(msg proto.Message) error {
	if msg.SessionID != r.sessionID {
		return nil
	}

	switch msg.Role {
	case proto.Assistant:
		return r.handleAssistantMessage(msg)
	case proto.User:
		r.handleUserMessage(msg)
	case proto.Tool:
		r.handleToolMessage(msg)
	}
	return nil
}

// handleAssistantMessage renders assistant output.
func (r *Runner) handleAssistantMessage(msg proto.Message) error {
	if len(msg.Parts) == 0 {
		return nil
	}

	// Handle reasoning if present (only show once per message)
	if reasoning := msg.ReasoningContent(); reasoning.Thinking != "" {
		cacheKey := msg.ID + "_thinking"
		if _, seen := r.outputCache[cacheKey]; !seen {
			r.outputCache[cacheKey] = reasoning.Thinking
			r.output("thinking", r.renderer.FormatThinking(reasoning.Thinking))
		}
	}

	// Handle text content with streaming support
	content := msg.Content().String()
	prevBytes := r.messageStates[msg.ID]
	if len(content) > prevBytes {
		delta := content[prevBytes:]
		if strings.TrimSpace(delta) != "" {
			// First time showing this message - use full format with prefix
			if prevBytes == 0 {
				r.output("text", r.renderer.FormatAssistant(delta))
			} else {
				// Streaming update - show delta without prefix
				r.output("text", r.renderer.FormatAssistantRaw(delta))
			}
		}
		r.messageStates[msg.ID] = len(content)
	}

	// Handle tool calls (only show each tool once)
	for _, tc := range msg.ToolCalls() {
		cacheKey := msg.ID + "_tool_" + tc.Name
		if _, seen := r.outputCache[cacheKey]; !seen {
			r.outputCache[cacheKey] = tc.Input
			r.output("tool_call", r.renderer.FormatToolCall(tc))
		}
	}

	// Handle tool results
	for _, tr := range msg.ToolResults() {
		r.output("tool_result", r.renderer.FormatToolResult(tr))
	}

	// Check if message is finished
	if msg.IsFinished() {
		if reason := msg.FinishReason(); reason == proto.FinishReasonError {
			if finish := msg.FinishPart(); finish != nil {
				return fmt.Errorf("%s", finish.Message)
			}
		}
		// Clean up state for this message
		delete(r.messageStates, msg.ID)
		// Keep cache for deduplication across messages
	}

	return nil
}

// handleUserMessage renders user input echo.
func (r *Runner) handleUserMessage(msg proto.Message) {
	if content := msg.Content().String(); content != "" {
		r.output("user", r.renderer.FormatUser(content))
	}
}

// handleToolMessage renders tool messages.
func (r *Runner) handleToolMessage(msg proto.Message) {
	for _, tr := range msg.ToolResults() {
		r.output("tool", r.renderer.FormatToolResult(tr))
	}
}

// handleAgentEvent processes agent-level events.
func (r *Runner) handleAgentEvent(ev proto.AgentEvent) error {
	if ev.Error != nil {
		return fmt.Errorf("agent error: %v", ev.Error)
	}

	if ev.Type == "agent_finished" {
		r.agentFinishedMux.Lock()
		r.agentFinished = true
		r.agentFinishedMux.Unlock()
	}

	if r.verbose {
		slog.Info("Agent event", "type", ev.Type)
	}
	return nil
}

// handlePermissionRequest presents a permission request to the user/agent.
func (r *Runner) handlePermissionRequest(req proto.PermissionRequest) error {
	if r.sessionID != "" && req.SessionID != r.sessionID {
		return nil
	}

	// Auto-handle based on flags
	if r.skipPerms {
		return r.sendPermissionResponse(req, proto.PermissionAllow)
	}
	if r.yolo {
		return r.sendPermissionResponse(req, proto.PermissionAllowForSession)
	}

	// In agent mode, output JSON and wait for response
	if r.mode == ModeAgent {
		r.output("permission_request", r.renderer.FormatPermission(req))
		// Wait for response via input channel
		resp := <-r.permResponses
		if resp.ID != req.ID {
			// Put back if not matching
			r.permResponses <- resp
			return nil
		}
		return r.handlePermissionResponse(context.Background(), resp)
	}

	// In human mode, prompt for input
	r.output("permission_request", r.renderer.FormatPermission(req))

	// Start input handler if not running
	go func() {
		action, err := r.inputReader.ReadPermission()
		if err != nil {
			slog.Error("Failed to read permission", "error", err)
			r.permResponses <- PermissionResponse{ID: req.ID, Action: PermDeny}
			return
		}

		permAction := PermDeny
		switch action {
		case 'y', 'Y':
			permAction = PermAllow
		case 'a', 'A':
			permAction = PermAllowSession
		case 'n', 'N', 0:
			permAction = PermDeny
		}

		r.permResponses <- PermissionResponse{ID: req.ID, Action: permAction}
	}()

	return nil
}

// handlePermissionNotification acknowledges permission results.
func (r *Runner) handlePermissionNotification(notif proto.PermissionNotification) {
	if notif.Denied {
		r.output("permission", r.renderer.FormatError("Permission denied"))
	}
	// Granted notifications are silent in normal mode
}

// sendPermissionResponse sends a permission response to the server.
func (r *Runner) sendPermissionResponse(req proto.PermissionRequest, action proto.PermissionAction) error {
	if r.workspace == nil {
		return nil
	}

	grant := proto.PermissionGrant{
		Permission: req,
		Action:     action,
	}
	return r.client.GrantPermission(context.Background(), r.workspace.ID, grant)
}

// handlePermissionResponse processes a permission response from the input channel.
func (r *Runner) handlePermissionResponse(_ context.Context, resp PermissionResponse) error {
	if r.verbose {
		slog.Info("Permission response", "id", resp.ID, "action", resp.Action)
	}
	return nil
}

// handleInput processes interactive input in a separate goroutine.
func (r *Runner) handleInput(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := r.inputReader.ReadLine()
			if err != nil {
				if err != io.EOF {
					slog.Error("Input error", "error", err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Handle commands
			switch strings.ToLower(line) {
			case "quit", "exit", "q":
				return
			case "help", "?":
				r.output("help", r.renderer.FormatHelp())
			default:
				// Send as new message if in interactive mode
				if r.client != nil && r.sessionID != "" && r.workspace != nil {
					if err := r.client.SendMessage(ctx, r.workspace.ID, r.sessionID, line); err != nil {
						slog.Error("Failed to send message", "error", err)
					}
				}
			}
		}
	}
}

// output prints formatted output in a thread-safe manner.
func (r *Runner) output(eventType string, content string) {
	if r.quiet && (eventType == "thinking" || eventType == "tool_call") {
		return
	}

	r.outputMutex.Lock()
	defer r.outputMutex.Unlock()

	switch r.mode {
	case ModeAgent:
		// JSON structured output
		evt := AgentEvent{
			Type:    eventType,
			Content: content,
			Time:    time.Now().Unix(),
		}
		data, _ := json.Marshal(evt)
		fmt.Fprintln(os.Stdout, string(data))

	case ModePlain, ModeHuman:
		fmt.Fprint(os.Stdout, content)
	}
}

// AgentEvent represents a structured event for agent mode.
type AgentEvent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

// useClientServer returns true if client/server mode should be used.
func useClientServer() bool {
	return os.Getenv("CRUSH_CLIENT_SERVER") != "0"
}

// WorkspaceID returns the workspace ID.
func (r *Runner) WorkspaceID() string {
	if r.workspace == nil {
		return ""
	}
	return r.workspace.ID
}

// GrantPermission grants a permission for local mode.
func (r *Runner) GrantPermission(req proto.PermissionRequest, action proto.PermissionAction) error {
	if r.client != nil && r.workspace != nil {
		grant := proto.PermissionGrant{
			Permission: req,
			Action:     action,
		}
		return r.client.GrantPermission(context.Background(), r.workspace.ID, grant)
	}
	return nil
}

// FormatHelp returns the help text.
func (r *Renderer) FormatHelp() string {
	return `
Commands:
  help, ?   - Show this help
  quit, q   - Exit

In interactive mode, type any prompt to continue the conversation.
`
}
