package terminal

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

const (
	terminalDevicePath      = "/dev/tty"
	termProgramEnvironment  = "TERM_PROGRAM"
	ghosttyTermProgram      = "ghostty"
	terminalReadBufferSize  = 128
	minimumPollMilliseconds = 1
	colorSchemeQuery        = "\x1b[?996n"
	backgroundQuery         = "\x1b]11;?\a"
)

func terminalThemeQuery() string {
	// Ghostty's color-scheme DSR reports its effective light/dark appearance
	// directly and still works when OSC color reporting is disabled. Retain
	// OSC 11 for terminals that do not identify themselves as Ghostty.
	if strings.EqualFold(os.Getenv(termProgramEnvironment), ghosttyTermProgram) {
		return colorSchemeQuery
	}
	return backgroundQuery
}

func detectTerminalTheme(timeout time.Duration) (Mode, bool) {
	tty, err := os.OpenFile(terminalDevicePath, os.O_RDWR, 0)
	if err != nil {
		return Dark, false
	}
	defer func() { _ = tty.Close() }()

	state, err := term.MakeRaw(tty.Fd())
	if err != nil {
		return Dark, false
	}
	defer func() { _ = term.Restore(tty.Fd(), state) }()

	if _, err := tty.Write([]byte(terminalThemeQuery())); err != nil {
		return Dark, false
	}

	deadline := time.Now().Add(timeout)
	var buffer []byte
	for time.Now().Before(deadline) {
		remaining := max(
			int(time.Until(deadline)/time.Millisecond),
			minimumPollMilliseconds,
		)
		events := []unix.PollFd{{
			Fd:     int32(tty.Fd()),
			Events: unix.POLLIN,
		}}
		count, err := unix.Poll(events, remaining)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			break
		}
		if count == 0 {
			break
		}
		chunk := make([]byte, terminalReadBufferSize)
		read, err := tty.Read(chunk)
		if read > 0 {
			buffer = append(buffer, chunk[:read]...)
			if mode, ok := ParseReport(buffer); ok {
				return mode, true
			}
		}
		if err != nil {
			break
		}
	}
	return ParseReport(buffer)
}
