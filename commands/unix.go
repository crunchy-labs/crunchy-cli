//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package commands

import (
	"bufio"
	"os"
	"os/exec"
	"syscall"
)

// https://github.com/bgentry/speakeasy/blob/master/speakeasy_unix.go
var stty string

func init() {
	var err error
	if stty, err = exec.LookPath("stty"); err != nil {
		panic(err)
	}
}

func readLineSilent() ([]byte, error) {
	pid, err := setEcho(false)
	if err != nil {
		return nil, err
	}
	defer setEcho(true)

	syscall.Wait4(pid, nil, 0, nil)

	l, _, err := bufio.NewReader(os.Stdin).ReadLine()
	return l, err
}

func setEcho(on bool) (pid int, err error) {
	fds := []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()}

	if on {
		pid, err = syscall.ForkExec(stty, []string{"stty", "echo"}, &syscall.ProcAttr{Files: fds})
	} else {
		pid, err = syscall.ForkExec(stty, []string{"stty", "-echo"}, &syscall.ProcAttr{Files: fds})
	}

	if err != nil {
		return 0, err
	}
	return
}
