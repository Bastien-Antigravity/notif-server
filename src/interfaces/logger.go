package interfaces

// -----------------------------------------------------------------------------
// Logger defines the standard logging methods required by the server.
// It is decoupled from the specific implementation details.
type Logger interface {
	// -------------------------------------------------------------------------
	// Debug logs a message at Debug level.
	Debug(msg string)

	// -------------------------------------------------------------------------
	// Info logs a message at Info level.
	Info(msg string)

	// -------------------------------------------------------------------------
	// Warning logs a message at Warning level.
	Warning(msg string)

	// -------------------------------------------------------------------------
	// Error logs a message at Error level.
	Error(msg string)

	// -------------------------------------------------------------------------
	// Critical logs a message at Critical level.
	Critical(msg string)

	// -------------------------------------------------------------------------
	// Close flushes any buffered logs and closes the handler.
	Close()
}
