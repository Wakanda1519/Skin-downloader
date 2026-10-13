//go:build !windows

package ui

func enableVirtualTerminal() {}

func ownsConsole() bool { return false }
