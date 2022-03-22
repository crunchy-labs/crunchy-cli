package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var prefix, progressDown, progressDownFinish string

func initPrefixBecauseWindowsSucksBallsHard() {
	// dear windows user, please change to a good OS, linux in the best case.
	// MICROSHIT DOES NOT GET IT DONE TO SHOW THE SYMBOLS IN THE ELSE CLAUSE
	// CORRECTLY. NOT IN THE CMD NOR POWERSHELL. WHY TF, IT IS ONE OF THE MOST
	// PROFITABLE COMPANIES ON THIS PLANET AND CANNOT SHOW A PROPER UTF-8 SYMBOL
	// IN THEIR OWN PRODUCT WHICH GETS USED MILLION TIMES A DAY
	if runtime.GOOS == "windows" {
		prefix = "=>"
		progressDown = "|"
		progressDownFinish = "->"
	} else {
		prefix = "➞"
		progressDown = "↓"
		progressDownFinish = "↳"
	}
}

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
	lock     sync.Mutex
}

func newLogger(debug, info, err bool) *logger {
	initPrefixBecauseWindowsSucksBallsHard()

	debugLog, infoLog, errLog := log.New(io.Discard, prefix+" ", 0), log.New(io.Discard, prefix+" ", 0), log.New(io.Discard, prefix+" ", 0)

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
	} else if l.devView {
		l.Debug(format, v...)
		return
	}

	initialMessage := fmt.Sprintf(format, v...)

	p := progress{
		message: initialMessage,
	}

	l.lock.Lock()
	if l.done != nil {
		l.progress <- p
		return
	} else {
		l.progress = make(chan progress, 1)
		l.progress <- p
		l.done = make(chan interface{})
	}

	go func() {
		states := []string{"-", "\\", "|", "/"}

		var count int

		for i := 0; ; i++ {
			select {
			case p := <-l.progress:
				if p.stop {
					fmt.Printf("\r" + strings.Repeat(" ", len(prefix)+len(initialMessage)))
					if count > 1 {
						fmt.Printf("\r%s %s\n", progressDownFinish, p.message)
					} else {
						fmt.Printf("\r%s %s\n", prefix, p.message)
					}

					if l.done != nil {
						l.done <- nil
					}
					l.progress = nil

					l.lock.Unlock()
					return
				} else {
					if count > 0 {
						fmt.Printf("\r%s %s\n", progressDown, p.message)
					}
					l.progress = make(chan progress, 1)

					count++

					fmt.Printf("\r%s %s", states[i/10%4], initialMessage)
					l.lock.Unlock()
				}
			default:
				if i%10 == 0 {
					fmt.Printf("\r%s %s", states[i/10%4], initialMessage)
				}
				time.Sleep(35 * time.Millisecond)
			}
		}
	}()
}

func (l *logger) StopProgress(format string, v ...interface{}) {
	if out.InfoLog.Writer() == io.Discard {
		return
	} else if l.devView {
		l.Debug(format, v...)
		return
	}

	l.lock.Lock()
	l.progress <- progress{
		message: fmt.Sprintf(format, v...),
		stop:    true,
	}
	<-l.done
	l.done = nil
}
