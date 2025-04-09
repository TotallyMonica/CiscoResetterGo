package crglogging

import (
	"fmt"
	"github.com/op/go-logging"
	"io"
	"os"
)

var Format = logging.MustStringFormatter(`%{time:15:04:05.000} %{shortfunc} ▶ %{level} %{id:03x} %{message}`)
var Instances []Instance

type Crglogging struct {
	DebugCount int
	InfoCount  int
	WarnCount  int
	ErrorCount int
	FatalCount int
	Backends   []Backend
	logger     *logging.Logger
	MemBuffers []MemBuffer
	name       string
}

type Backend struct {
	backend logging.LeveledBackend
	name    string
}

type MemBuffer struct {
	Buff *logging.MemoryBackend
	Name string
}

type Instance struct {
	Name     string
	Instance *Crglogging
}

func New(name string) *Crglogging {
	l := &Crglogging{}

	// Create backend
	logger := logging.MustGetLogger("CiscoResetterGo")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, Format)
	leveledBackend := logging.AddModuleLevel(backendFormatter)
	logger.SetBackend(leveledBackend)

	// Retain backend to allow for modification later
	l.Backends = make([]Backend, 0)
	l.Backends = append(l.Backends, Backend{
		backend: leveledBackend,
		name:    "Standard Error",
	})

	// Save list of instances
	if len(Instances) == 0 {
		Instances = make([]Instance, 0)
	}

	// Add logger to instance and save it
	l.logger = logger
	l.name = name
	Instances = append(Instances, Instance{
		Name:     name,
		Instance: l,
	})

	return l
}

func (l *Crglogging) NewLogTarget(name string, target interface{}, file bool) {
	var fileBackend logging.Backend

	if file {
		// Check if a file name or file pointer was passed
		switch v := target.(type) {
		case string:
			f, err := os.OpenFile(v, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				l.Errorf("Failed to open log file: %v", err)
			}
			defer f.Close()

			fileBackend = logging.NewLogBackend(f, "", 0)
			break
		case io.Writer:
			fileBackend = logging.NewLogBackend(v, "", 0)
			break
		default:
			l.Errorf("Unknown target type: %T", target)
			return
		}
	} else {
		// Ensure only a writer object was passed
		switch v := target.(type) {
		case io.Writer:
			// Create writer and add to backend list
			fileBackend = logging.NewLogBackend(v, "", 0)
			break
		case chan bool:
			buff := MemBuffer{
				Name: name,
				Buff: logging.NewMemoryBackend(2 << 16),
			}
			fileBackend = buff.Buff
			l.MemBuffers = append(l.MemBuffers, buff)
			break
		default:
			l.Errorf("Unknown target type: %T", target)
			return
		}
	}

	backendFormatter := logging.NewBackendFormatter(fileBackend, Format)
	leveledBackend := logging.AddModuleLevel(backendFormatter)

	l.Backends = append(l.Backends, Backend{
		backend: leveledBackend,
		name:    name,
	})

	backends := make([]logging.Backend, 0)

	for _, backend := range l.Backends {
		backends = append(backends, backend.backend)
	}

	l.logger.SetBackend(logging.MultiLogger(backends...))
}

func (l *Crglogging) GetMemLogContents(name string) (MemBuffer, error) {
	for _, backend := range l.MemBuffers {
		if backend.Name == name {
			return backend, nil
		}
	}

	return MemBuffer{}, fmt.Errorf("could not find mem log for %s", name)
}

func GetLogger(name string) *Crglogging {
	for _, instance := range Instances {
		if instance.Name == name {
			return instance.Instance
		}
	}

	return nil
}

func (l *Crglogging) GetLoggerName() string {
	return l.name
}

func (l *Crglogging) SetLogLevel(level int) {
	backends := make([]logging.Backend, 0)

	for _, backend := range l.Backends {
		switch level {
		case int(logging.DEBUG):
			backend.backend.SetLevel(logging.DEBUG, "")
			break
		case int(logging.INFO):
			backend.backend.SetLevel(logging.INFO, "")
			break
		case int(logging.NOTICE):
			backend.backend.SetLevel(logging.NOTICE, "")
			break
		case int(logging.WARNING):
			backend.backend.SetLevel(logging.WARNING, "")
			break
		case int(logging.ERROR):
			backend.backend.SetLevel(logging.ERROR, "")
			break
		case int(logging.CRITICAL):
			backend.backend.SetLevel(logging.CRITICAL, "")
			break
		}
		backends = append(backends, backend.backend)
	}

	l.logger.SetBackend(logging.MultiLogger(backends...))
}
