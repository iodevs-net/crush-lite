package lite

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// InputReader handles interactive input from stdin.
type InputReader struct {
	scanner *bufio.Scanner
}

// NewInputReader creates a new InputReader.
func NewInputReader() *InputReader {
	return &InputReader{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// ReadLine reads a single line from stdin.
func (r *InputReader) ReadLine() (string, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("EOF")
	}
	return r.scanner.Text(), nil
}

// ReadPermission reads a permission response (y/n/a) from stdin.
// Returns 'y', 'n', 'a', or 0 for timeout/default.
func (r *InputReader) ReadPermission() (byte, error) {
	// Check if stdin is a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return r.readPermissionInteractive()
	}
	return r.readPermissionNonInteractive()
}

// readPermissionInteractive reads with terminal echo disabled.
func (r *InputReader) readPermissionInteractive() (byte, error) {
	fmt.Print("> ")

	// Read single character
	char, err := readSingleChar()
	if err != nil {
		return 0, err
	}

	fmt.Println(string(char)) // Echo the character

	switch char {
	case 'y', 'Y':
		return 'y', nil
	case 'n', 'N':
		return 'n', nil
	case 'a', 'A':
		return 'a', nil
	default:
		return 'n', nil // Default to deny
	}
}

// readPermissionNonInteractive reads from piped stdin.
func (r *InputReader) readPermissionNonInteractive() (byte, error) {
	line, err := r.ReadLine()
	if err != nil {
		return 'n', err
	}

	line = strings.TrimSpace(strings.ToLower(line))
	if len(line) == 0 {
		return 'n', nil
	}

	switch line[0] {
	case 'y':
		return 'y', nil
	case 'a':
		return 'a', nil
	default:
		return 'n', nil
	}
}

// readSingleChar reads a single character from stdin without buffering.
func readSingleChar() (byte, error) {
	// Get current terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Not a terminal, fall back to scanner
		return readCharScanner()
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Read single byte
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("no input")
	}

	return buf[0], nil
}

// readCharScanner reads using bufio scanner as fallback.
func readCharScanner() (byte, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 {
			return line[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("no input")
}

// ReadLineWithPrompt displays a prompt and reads input.
func (r *InputReader) ReadLineWithPrompt(prompt string) (string, error) {
	fmt.Print(prompt)
	return r.ReadLine()
}

// IsTerminal returns true if stdin is a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// WaitForInput waits for any input and returns. Used for pausing.
func WaitForInput() {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	fmt.Print("\nPresiona Enter para continuar...")

	// Restore terminal for normal reading
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	buf := make([]byte, 1)
	os.Stdin.Read(buf)
}

// enableRawMode enables raw mode for the terminal.
func enableRawMode() (*term.State, error) {
	return term.MakeRaw(int(os.Stdin.Fd()))
}

// disableRawMode restores the terminal to its previous state.
func disableRawMode(state *term.State) error {
	return term.Restore(int(os.Stdin.Fd()), state)
}

// ReadPassword reads a password without echoing.
// ReadPassword reads a password without echoing.
func ReadPassword() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Use scanner for piped input
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text(), nil
		}
		return "", fmt.Errorf("no input")
	}

	fmt.Print("Password: ")

	// Read password
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}

	fmt.Println() // New line after password
	return string(bytePassword), nil
}

// GetTerminalSize returns the terminal width and height.
func GetTerminalSize() (width, height int, err error) {
	return term.GetSize(int(os.Stdout.Fd()))
}

// ClearScreen clears the terminal screen.
func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}

// MoveCursor moves the cursor to the specified position.
func MoveCursor(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

// HideCursor hides the terminal cursor.
func HideCursor() {
	fmt.Print("\033[?25l")
}

// ShowCursor shows the terminal cursor.
func ShowCursor() {
	fmt.Print("\033[?25h")
}
