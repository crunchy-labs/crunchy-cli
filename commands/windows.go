//go:build windows

package commands

import (
	"bufio"
	"os"
	"syscall"
)

// https://github.com/bgentry/speakeasy/blob/master/speakeasy_windows.go
func readLineSilent() ([]byte, error) {
	var oldMode uint32

	if err := syscall.GetConsoleMode(syscall.Stdin, &oldMode); err != nil {
		return nil, err
	}

	newMode := oldMode &^ 0x0004

	err := setConsoleMode(syscall.Stdin, newMode)
	defer setConsoleMode(syscall.Stdin, oldMode)

	if err != nil {
		return nil, err
	}

	l, _, err := bufio.NewReader(os.Stdin).ReadLine()
	if err != nil {
		return nil, err
	}
	return l, err
}

func setConsoleMode(console syscall.Handle, mode uint32) error {
	dll := syscall.MustLoadDLL("kernel32")
	proc := dll.MustFindProc("SetConsoleMode")
	_, _, err := proc.Call(uintptr(console), uintptr(mode))

	return err
}
