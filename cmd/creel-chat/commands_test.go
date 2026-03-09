package main

import (
	"testing"
)

func TestParseCommand_NotACommand(t *testing.T) {
	tests := []string{
		"hello",
		"not a command",
		"",
		"hello /upload file.txt",
	}
	for _, input := range tests {
		cmd := ParseCommand(input)
		if cmd.Type != CmdNone {
			t.Errorf("input %q: expected CmdNone, got %q", input, cmd.Type)
		}
	}
}

func TestParseCommand_Upload(t *testing.T) {
	cmd := ParseCommand("/upload /path/to/file.txt")
	if cmd.Type != CmdUpload {
		t.Errorf("expected CmdUpload, got %q", cmd.Type)
	}
	if cmd.Arg != "/path/to/file.txt" {
		t.Errorf("expected '/path/to/file.txt', got %q", cmd.Arg)
	}
}

func TestParseCommand_UploadNoArg(t *testing.T) {
	cmd := ParseCommand("/upload")
	if cmd.Type != CmdUpload {
		t.Errorf("expected CmdUpload, got %q", cmd.Type)
	}
	if cmd.Arg != "" {
		t.Errorf("expected empty arg, got %q", cmd.Arg)
	}
}

func TestParseCommand_Remember(t *testing.T) {
	cmd := ParseCommand("/remember I like Go programming")
	if cmd.Type != CmdRemember {
		t.Errorf("expected CmdRemember, got %q", cmd.Type)
	}
	if cmd.Arg != "I like Go programming" {
		t.Errorf("expected 'I like Go programming', got %q", cmd.Arg)
	}
}

func TestParseCommand_Forget(t *testing.T) {
	cmd := ParseCommand("/forget favorite color")
	if cmd.Type != CmdForget {
		t.Errorf("expected CmdForget, got %q", cmd.Type)
	}
	if cmd.Arg != "favorite color" {
		t.Errorf("expected 'favorite color', got %q", cmd.Arg)
	}
}

func TestParseCommand_UnknownCommand(t *testing.T) {
	cmd := ParseCommand("/unknown something")
	if cmd.Type != CmdNone {
		t.Errorf("expected CmdNone for unknown command, got %q", cmd.Type)
	}
}

func TestParseCommand_TrimWhitespace(t *testing.T) {
	cmd := ParseCommand("/remember   lots of spaces  ")
	if cmd.Type != CmdRemember {
		t.Errorf("expected CmdRemember, got %q", cmd.Type)
	}
	if cmd.Arg != "lots of spaces" {
		t.Errorf("expected 'lots of spaces', got %q", cmd.Arg)
	}
}

func TestParseCommand_UploadWithSpaces(t *testing.T) {
	cmd := ParseCommand("/upload /path/to/my file.txt")
	if cmd.Type != CmdUpload {
		t.Errorf("expected CmdUpload, got %q", cmd.Type)
	}
	if cmd.Arg != "/path/to/my file.txt" {
		t.Errorf("expected '/path/to/my file.txt', got %q", cmd.Arg)
	}
}
