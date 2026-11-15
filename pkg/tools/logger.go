package tools

// noopLogger 给 go-resty 使用的空日志
type noopLogger struct{}

func (noopLogger) Errorf(format string, v ...any) {}

func (noopLogger) Warnf(format string, v ...any) {}

func (noopLogger) Debugf(format string, v ...any) {}
