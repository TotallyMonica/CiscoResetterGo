package crglogging

import (
	"bytes"
	"fmt"
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
	MemBuffers []MemBuffer
	name       string
}

type Backend struct {
	backend logging.LeveledBackend
	name    string
}

type MemBuffer struct {
	buff     bytes.Buffer
	contents []byte
	name     string
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

			fileBackend = logging.NewLogBackend(f, name, 0)
			break
		case io.Writer:
			fileBackend = logging.NewLogBackend(v, name, 0)
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
			fileBackend = logging.NewLogBackend(v, name, 0)
			break
		case chan string:
			if l.MemBuffers == nil || len(l.MemBuffers) == 0 {
				l.MemBuffers = make([]MemBuffer, 0)
			}

			buffContents := make([]byte, 0)

			memLog := MemBuffer{
				buff:     *bytes.NewBuffer(buffContents),
				name:     name,
				contents: buffContents,
			}
			l.MemBuffers = append(l.MemBuffers, memLog)
			fileBackend = logging.NewLogBackend(&memLog.buff, name, 0)
		default:
			l.Errorf("Unknown target type: %T", target)
			return
		}
	}

	backendFormatter := logging.NewBackendFormatter(fileBackend, format)
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

func (l *Crglogging) GetMemLogContents(name string) ([]byte, error) {
	for _, backend := range l.MemBuffers {
		if backend.name == name {
			return backend.buff.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("could not find mem log for %s", name)
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
