package crglogging

import (
	"github.com/op/go-logging"
	"io"
	"os"
)

var format = logging.MustStringFormatter(`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level} %{id:03x}%{color:reset} %{message}`)
var Instances []Instance

type Crglogging struct {
	DebugCount int
	InfoCount  int
	WarnCount  int
	ErrorCount int
	FatalCount int
	Backends   []Backend
	logger     *logging.Logger
}

type Backend struct {
	backend logging.LeveledBackend
	name    string
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
	backendFormatter := logging.NewBackendFormatter(backend, format)
	leveledBackend := logging.AddModuleLevel(backendFormatter)
	logging.SetBackend(leveledBackend)

	// Retain backend to allow for modification later
	backends := make([]Backend, 0)
	backends = append(backends, Backend{
		backend: leveledBackend,
		name:    "Standard Error",
	})

	// Save list of instances
	if len(Instances) == 0 {
		Instances = make([]Instance, 0)
	}

	// Add logger to instance and save it
	l.logger = logger
	Instances = append(Instances, Instance{
		Name:     name,
		Instance: l,
	})

	return l
}

func (l *Crglogging) NewLogTarget(name string, target interface{}, file bool) {
	if file {
		var fileBackend logging.Backend

		// Check if a file name or file pointer was passed
		switch v := target.(type) {
		case string:
			f, err := os.OpenFile(v, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				l.Errorf("Failed to open log file: %v", err)
			}
			defer f.Close()

			fileBackend = logging.NewLogBackend(f, name, 0)
			break
		case io.Writer:
			fileBackend = logging.NewLogBackend(v, name, 0)
			break
		default:
			l.Errorf("Unknown target type: %T", target)
			return
		}

		backendFormatter := logging.NewBackendFormatter(fileBackend, format)
		leveledBackend := logging.AddModuleLevel(backendFormatter)

		l.Backends = append(l.Backends, Backend{
			backend: leveledBackend,
			name:    name,
		})
	} else {
		// Ensure only a writer object was passed
		switch v := target.(type) {
		case io.Writer:
			// Create writer and add to backend list
			backend := logging.NewLogBackend(v, name, 0)
			backendFormatter := logging.NewBackendFormatter(backend, format)
			leveledBackend := logging.AddModuleLevel(backendFormatter)
			l.Backends = append(l.Backends, Backend{
				backend: leveledBackend,
				name:    name,
			})
			break
		default:
			l.Errorf("Unknown target type: %T", target)
			return
		}
	}

	backends := make([]logging.Backend, 0)

	for _, backend := range l.Backends {
		backends = append(backends, backend.backend)
	}

	logging.SetBackend(backends...)
}

func GetLogger(name string) *Crglogging {
	for _, instance := range Instances {
		if instance.Name == name {
			return instance.Instance
		}
	}

	return nil
}
