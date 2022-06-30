package utils

type Logger interface {
	IsDev() bool
	Debug(format string, v ...any)
	Info(format string, v ...any)
	Warn(format string, v ...any)
	Err(format string, v ...any)
	Empty()
	SetProcess(format string, v ...any)
	StopProcess(format string, v ...any)
}
