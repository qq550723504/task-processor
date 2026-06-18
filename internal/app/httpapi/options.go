package httpapi

import "os"

type Options struct {
	ConfigPath     string
	Port           int
	ShutdownSignal chan os.Signal
}
