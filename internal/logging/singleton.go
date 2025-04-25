package logging

import (
	"sync"
)

var (
	instance *Logger
	once     sync.Once
	mu       sync.RWMutex
	logConfig *Config
)

// Configure sets the logging configuration.
// This should be called before any logger usage.
func Configure(config *Config) {
	mu.Lock()
	defer mu.Unlock()
	logConfig = config
}

// GetLogger returns the singleton logger instance.
// If the logger hasn't been initialized yet, it initializes it with the provided config.
// If no config was provided via Configure(), it panics.
func GetLogger() *Logger {
	once.Do(func() {
		mu.Lock()
		defer mu.Unlock()

		if logConfig == nil {
			panic("logger configuration not set - call logging.Configure() first")
		}

		var err error
		instance, err = NewLogger(logConfig)
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
	})

	return instance
}