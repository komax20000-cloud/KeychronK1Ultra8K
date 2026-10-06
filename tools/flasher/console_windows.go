//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	stdInputHandle  = uintptr(uint32(0xfffffff6))
	keyEvent        = 0x0001
	virtualKeyUp    = 0x26
	virtualKeyDown  = 0x28
	virtualKeyEsc   = 0x1b
	virtualKeyEnter = 0x0d
)

type consoleKeyEvent struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

type consoleInputRecord struct {
	EventType uint16
	_         [2]byte
	KeyEvent  consoleKeyEvent
}

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getStdHandle     = kernel32.NewProc("GetStdHandle")
	readConsoleInput = kernel32.NewProc("ReadConsoleInputW")
)

func consoleAPIError(operation string, callErr error) error {
	if callErr == nil || callErr == syscall.Errno(0) {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s failed: %w", operation, callErr)
}

func readMenuKey() (rune, error) {
	handle, _, callErr := getStdHandle.Call(stdInputHandle)
	if handle == 0 || handle == ^uintptr(0) {
		return 0, consoleAPIError("GetStdHandle", callErr)
	}
	for {
		var record consoleInputRecord
		var read uint32
		ok, _, callErr := readConsoleInput.Call(
			handle,
			uintptr(unsafe.Pointer(&record)),
			1,
			uintptr(unsafe.Pointer(&read)),
		)
		if ok == 0 {
			return 0, consoleAPIError("ReadConsoleInputW", callErr)
		}
		if read == 0 || record.EventType != keyEvent || record.KeyEvent.KeyDown == 0 {
			continue
		}
		switch record.KeyEvent.VirtualKeyCode {
		case virtualKeyUp:
			return keyUp, nil
		case virtualKeyDown:
			return keyDown, nil
		case virtualKeyEnter:
			return keyEnter, nil
		case virtualKeyEsc:
			return keyEscape, nil
		default:
			if record.KeyEvent.UnicodeChar != 0 {
				return rune(record.KeyEvent.UnicodeChar), nil
			}
		}
	}
}
