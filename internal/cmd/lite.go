package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

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
