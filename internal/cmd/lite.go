package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/lite"
	"github.com/spf13/cobra"
)


var (
	// Flags for lite command
	liteMode      string
	litePlain     bool
	liteAgent     bool
	liteQuiet     bool
	liteVerbose   bool
	liteSkipPerms bool
	liteYolo      bool
)

func init() {
	// Add lite command to root
	rootCmd.AddCommand(liteCmd)

	// Register lite-specific flags
	liteCmd.Flags().StringVar(&liteMode, "mode", "human",
		"Output mode: human (colored), plain (no colors), agent (JSON)")

	liteCmd.Flags().BoolVar(&litePlain, "plain", false,
		"Plain text output without colors or formatting")

	liteCmd.Flags().BoolVar(&liteAgent, "agent", false,
		"Agent mode: JSON structured output for external AI agents")

	liteCmd.Flags().BoolVarP(&liteQuiet, "quiet", "q", false,
		"Hide spinner and non-essential output")

	liteCmd.Flags().BoolVarP(&liteVerbose, "verbose", "v", false,
		"Show detailed logs and debugging information")

	liteCmd.Flags().BoolVar(&liteSkipPerms, "skip-permissions", false,
		"Skip permission prompts (auto-allow)")

	liteCmd.Flags().BoolVarP(&liteYolo, "yolo", "y", false,
		"Allow all actions without prompting (dangerous)")
}

var liteCmd = &cobra.Command{
	Aliases: []string{"l"},
	Use:     "lite [prompt...]",
	Short:   "Lightweight CLI interface for humans and AI agents",
	Long: `A minimalist CLI interface for Crush that works for both human users
and external AI agents. Features include:

  - Clean, readable output for humans
  - Structured JSON output for agents
  - Interactive permission handling
  - Full streaming support
  - Session continuation

This is the recommended mode for:
  - Scripted workflows
  - CI/CD pipelines
  - External agent integration
  - Quick one-off tasks`,
	Example: `
# Interactive mode (default)
crush lite "describe this project"

# Plain text output (no colors)
crush lite --plain "list all files"

# Agent mode (JSON output for AI integration)
crush lite --agent "explain this error" | jq

# Non-interactive with auto-allow permissions
crush lite --skip-permissions "fix the tests"

# Yolo mode (allow everything, use with caution)
crush lite --yolo "delete all .tmp files"

# Quiet mode (minimal output)
crush lite --quiet "generate docs"

# Continue a previous session
crush lite --continue "continue from where we left off"

# Verbose mode for debugging
crush lite --verbose "run the build"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if server is running, if not, start it
		serverStarted, err := ensureServerRunning()
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
		if serverStarted {
			fmt.Fprintln(os.Stderr, "[crush] servidor iniciado")
		}

		// Get flags
		quiet, _ := cmd.Flags().GetBool("quiet")
		verbose, _ := cmd.Flags().GetBool("verbose")
		skipPerms, _ := cmd.Flags().GetBool("skip-permissions")
		yolo, _ := cmd.Flags().GetBool("yolo")
		sessionID, _ := cmd.Flags().GetString("session")
		useLast, _ := cmd.Flags().GetBool("continue")
		cwdFlag, _ := cmd.Flags().GetString("cwd")

		// Determine mode
		mode := lite.ModeHuman
		if litePlain {
			mode = lite.ModePlain
		}
		if liteAgent {
			mode = lite.ModeAgent
		}

		// Build prompt from args or stdin
		prompt := strings.Join(args, " ")
		if prompt == "" {
			// Try to read from stdin if pipe
			prompt = os.Getenv("CRUSH_PROMPT")
			if prompt == "" {
				return fmt.Errorf("no prompt provided: pass as argument or set CRUSH_PROMPT env var")
			}
		}

		// Get cwd
		cwd := cwdFlag
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Create context with cancellation
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		defer cancel()

		// Create and run lite runner
		runner := lite.NewRunner(mode, quiet, verbose, skipPerms, yolo)

		// Handle continue flags
		continueSessionID := sessionID
		if useLast {
			if sessionID != "" {
				return fmt.Errorf("cannot use both --session and --continue")
			}
			// We'll resolve the last session in the runner
		}

		return runner.Run(ctx, cwd, prompt, "", "", continueSessionID)
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// No completion for prompts
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}


// ensureServerRunning checks if a Crush server is running, and starts one if not.
// Returns (true, nil) if we started a new server, (false, nil) if one was already running.
func ensureServerRunning() (bool, error) {
	socketPath := fmt.Sprintf("/tmp/crush-%d.sock", os.Getuid())

	// Check if socket exists and is accessible
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return startServer()
	}

	// Socket exists - try to start server anyway
	// If one is already running, it will fail gracefully
	return startServer()
}

// startServer starts a new Crush server in the background.
func startServer() (bool, error) {
	execPath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start server in background
	cmd := exec.Command(execPath, "server")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for server to be ready (max 10 seconds)
	socketPath := fmt.Sprintf("/tmp/crush-%d.sock", os.Getuid())
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(socketPath); err == nil {
			return true, nil
		}
	}

	// Server didn't start in time
	cmd.Process.Kill()
	return false, fmt.Errorf("server failed to start within 10 seconds")
}