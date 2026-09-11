package logger

// Level type
type Level int

// Log level definitions
const (
	// DebugLevel usually only enabled when debugging. Very verbose logging.
	DebugLevel Level = 10
	// InfoLevel general operational entries about what's going on inside the application.
	InfoLevel Level = 20
	// WarnLevel non-critical entries that deserve eyes.
	WarnLevel Level = 30
	// ErrorLevel used for errors that should definitely be noted.
	ErrorLevel Level = 40
)

// Config options for the logger.
type Config struct {
	LogThreshold Level
	Formatter    string
}
