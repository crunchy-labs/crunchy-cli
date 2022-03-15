package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type progress struct {
	message string
	stop    bool
}

type logger struct {
	DebugLog *log.Logger
	InfoLog  *log.Logger
	ErrLog   *log.Logger

	devView bool

	progress chan progress
	done     chan interface{}
}

func newLogger(debug, info, err bool) *logger {
	debugLog, infoLog, errLog := log.New(io.Discard, "➞ ", 0), log.New(io.Discard, "➞ ", 0), log.New(io.Discard, "➞ ", 0)

	if debug {
		debugLog.SetOutput(os.Stdout)
	}
	if info {
		infoLog.SetOutput(os.Stdout)
	}
	if err {
		errLog.SetOutput(os.Stderr)
	}

	if debug {
		debugLog = log.New(debugLog.Writer(), "[debug] ", 0)
		infoLog = log.New(infoLog.Writer(), "[info] ", 0)
		errLog = log.New(errLog.Writer(), "[err] ", 0)
	}

	return &logger{
		DebugLog: debugLog,
		InfoLog:  infoLog,
		ErrLog:   errLog,

		devView: debug,
	}
}

func (l *logger) IsDev() bool {
	return l.devView
}

func (l *logger) IsQuiet() bool {
	return l.DebugLog.Writer() == io.Discard && l.InfoLog.Writer() == io.Discard && l.ErrLog.Writer() == io.Discard
}

func (l *logger) Debug(format string, v ...interface{}) {
	l.DebugLog.Printf(format, v...)
}

func (l *logger) Info(format string, v ...interface{}) {
	l.InfoLog.Printf(format, v...)
}

func (l *logger) Err(format string, v ...interface{}) {
	l.ErrLog.Printf(format, v...)
}

func (l *logger) Exit(format string, v ...interface{}) {
	fmt.Fprintln(l.ErrLog.Writer(), fmt.Sprintf(format, v...))
}

func (l *logger) Empty() {
	if !l.devView && l.InfoLog.Writer() != io.Discard {
		fmt.Println("")
	}
}

func (l *logger) SetProgress(format string, v ...interface{}) {
	if out.InfoLog.Writer() == io.Discard {
		return
	}

	message := fmt.Sprintf(format, v...)

	if l.progress != nil {
		l.progress <- progress{
			message: message,
			stop:    false,
		}
		return
	}

	l.progress = make(chan progress)
	l.done = make(chan interface{})

	go func() {
		states := []string{"-", "\\", "|", "/"}
		var count int

		for i := 0; ; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
			select {
			case p := <-l.progress:
				cancel()

				if p.stop {
					if !l.devView {
						fmt.Printf("\r" + strings.Repeat(" ", 2+len(message)))
						fmt.Printf("\r➞ %s\n", p.message)
					} else {
						l.Debug(p.message)
					}

					l.progress = nil

					if count > 0 {
						fmt.Printf("↳ %s\n", p.message)
					}

					l.done <- nil
					return
				} else {
					if !l.devView {
						fmt.Printf("\r↓ %s\n", message)
					} else {
						l.Debug(message)
					}

					l.progress = make(chan progress)
					count++

					if !l.devView {
						fmt.Printf("\r" + strings.Repeat(" ", 2+len(message)))
						fmt.Printf("\r➞ %s\n", p.message)
					} else {
						l.Debug(p.message)
					}
					message = p.message
				}
			case <-ctx.Done():
				if !l.devView && i%10 == 0 {
					fmt.Printf("\r%s %s", states[i/10%4], message)
				}
			}
		}
	}()
}

func (l *logger) StopProgress(format string, v ...interface{}) {
	if out.InfoLog.Writer() == io.Discard {
		return
	}

	l.progress <- progress{
		message: fmt.Sprintf(format, v...),
		stop:    true,
	}
	<-l.done
}
