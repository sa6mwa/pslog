package pslog

type noopLogger struct{}

func (noopLogger) Trace(string, ...any)            {}
func (noopLogger) Debug(string, ...any)            {}
func (noopLogger) Info(string, ...any)             {}
func (noopLogger) Warn(string, ...any)             {}
func (noopLogger) Error(string, ...any)            {}
func (noopLogger) Fatal(string, ...any)            {}
func (noopLogger) Panic(string, ...any)            {}
func (noopLogger) Log(Level, string, ...any)       {}
func (n noopLogger) With(...any) Logger            { return n }
func (n noopLogger) WithLogLevel() Logger          { return n }
func (n noopLogger) LogLevel(Level) Logger         { return n }
func (n noopLogger) LogLevelFromEnv(string) Logger { return n }
