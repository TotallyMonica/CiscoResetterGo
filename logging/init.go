package crglogging

import (
	"github.com/op/go-logging"
	"os"
)

var Logger *logging.Logger
var format = logging.MustStringFormatter(`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level} %{id:03x}%{color:reset} %{message}`)

type Crglogging struct {
	DebugCount int
	InfoCount  int
	WarnCount  int
	ErrorCount int
	FatalCount int
}

func New() Crglogging {
	l := Crglogging{}
	Logger = logging.MustGetLogger("CiscoResetterGo")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	leveledBackend := logging.AddModuleLevel(backendFormatter)
	logging.SetBackend(leveledBackend)
	return l
}
