package routers

import (
	"encoding/json"
	"go.bug.st/serial"
	"io"
	"main/common"
	"math"
	"os"
	"testing"
	"time"
)

func TestReset(t *testing.T) {
	type args struct {
		SerialPort   string
		PortSettings serial.Mode
		backup       common.Backup
		debug        bool
		progressDest chan string
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "Reset with verbose output",
		args: args{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			backup: common.Backup{
				Backup: false,
			},
			debug:        true,
			progressDest: make(chan string),
		},
	}, {
		name: "Reset without verbose output",
		args: args{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			backup: common.Backup{
				Backup: false,
			},
			debug:        false,
			progressDest: make(chan string),
		},
	},
	}
	for _, tt := range tests {
		start := time.Now()
		timeout := time.After(20 * time.Minute)
		go t.Run(tt.name, func(t *testing.T) {
			Reset(tt.args.SerialPort, tt.args.PortSettings, tt.args.backup, tt.args.debug, tt.args.progressDest)
		})

		for {
			canExit := false
			select {
			case msg := <-tt.args.progressDest:
				if msg == "---EOF---" {
					t.Logf("Test %s passed in %d:%d\n", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
					canExit = true
				} else {
					t.Logf("Test %s: %s\n", tt.name, msg)
				}
			case <-timeout:
				t.Fatalf("Test %s timed out in 20 minutes.\n", tt.name)
			}
			if canExit {
				break
			}
		}
	}
}

func TestDefaults(t *testing.T) {
	type args struct {
		SerialPort   string
		PortSettings serial.Mode
		config       RouterDefaults
		debug        bool
		progressDest chan string
	}

	defaultsFile, err := os.OpenFile("router_defaults.json", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Error while loading defaults file for testing: %s\n", err)
	}

	defer defaultsFile.Close()

	defaults, err := io.ReadAll(defaultsFile)
	if err != nil {
		t.Fatalf("Error while reading defaults file for testing: %s\n", err)
	}

	defaultsStruct := RouterDefaults{}

	err = json.Unmarshal(defaults, &defaultsStruct)
	if err != nil {
		t.Fatalf("Error while parsing defaults file for testing: %s\n", err)
	}

	tests := []struct {
		name string
		args args
	}{{
		name: "Apply defaults with verbose output",
		args: args{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			config:       defaultsStruct,
			debug:        true,
			progressDest: make(chan string),
		},
	}, {
		name: "Apply defaults with limited output",
		args: args{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			config:       defaultsStruct,
			progressDest: make(chan string),
		},
	}}
	for _, tt := range tests {
		start := time.Now()
		timeout := time.After(20 * time.Minute)

		go t.Run(tt.name, func(t *testing.T) {
			Defaults(tt.args.SerialPort, tt.args.PortSettings, tt.args.config, tt.args.debug, tt.args.progressDest)
		})
		select {
		case msg := <-tt.args.progressDest:
			if msg == "---EOF---" {
				t.Logf("Test %s passed in %d:%d\n", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
			} else {
				t.Logf("Test %s: %s\n", tt.name, msg)
			}
		case <-timeout:
			t.Fatalf("Test %s timed out in 20 minutes.\n", tt.name)
		}
	}
}
