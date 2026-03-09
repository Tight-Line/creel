package main

import (
	"strings"
)

// Command types for REPL commands.
const (
	CmdNone     = ""
	CmdUpload   = "upload"
	CmdRemember = "remember"
	CmdForget   = "forget"
)

// ParsedCommand represents a parsed REPL command.
type ParsedCommand struct {
	Type string
	Arg  string
}

// ParseCommand checks if the input is a slash command and parses it.
// Returns a ParsedCommand with Type == CmdNone if the input is not a command.
func ParseCommand(input string) ParsedCommand {
	if !strings.HasPrefix(input, "/") {
		return ParsedCommand{Type: CmdNone}
	}

	// Split into command and argument.
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.TrimPrefix(parts[0], "/")
	var arg string
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "upload":
		return ParsedCommand{Type: CmdUpload, Arg: arg}
	case "remember":
		return ParsedCommand{Type: CmdRemember, Arg: arg}
	case "forget":
		return ParsedCommand{Type: CmdForget, Arg: arg}
	default:
		return ParsedCommand{Type: CmdNone}
	}
}
