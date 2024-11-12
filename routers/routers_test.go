package routers

import (
	"encoding/json"
	"go.bug.st/serial"
	"io"
	"main/common"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// TODO: Validate reset/defaults
func validateOutput() {

}

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
				if strings.Contains(msg, "--EOF--") {
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
			if strings.Contains(msg, "--EOF--") {
				t.Logf("Test %s passed in %d:%d\n", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
			} else {
				t.Logf("Test %s: %s\n", tt.name, msg)
			}
		case <-timeout:
			t.Fatalf("Test %s timed out in 20 minutes.\n", tt.name)
		}
	}
}

func TestResetAndDefaults(t *testing.T) {
	type resetArgs struct {
		SerialPort   string
		PortSettings serial.Mode
		backup       common.Backup
		debug        bool
		progressDest chan string
	}
	type defaultsArgs struct {
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
		name         string
		resetArgs    resetArgs
		defaultsArgs defaultsArgs
	}{{
		name: "Reset and apply defaults with verbose output",
		resetArgs: resetArgs{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			backup: common.Backup{
				Backup: false,
			},
			debug:        true,
			progressDest: make(chan string),
		},
		defaultsArgs: defaultsArgs{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			config:       defaultsStruct,
			debug:        true,
			progressDest: make(chan string),
		},
	}, {
		name: "Reset and apply defaults with limited output",
		resetArgs: resetArgs{
			SerialPort:   "COM3",
			PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
			backup: common.Backup{
				Backup: false,
			},
			debug:        false,
			progressDest: make(chan string),
		},
		defaultsArgs: defaultsArgs{
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
			Reset(tt.resetArgs.SerialPort, tt.resetArgs.PortSettings, tt.resetArgs.backup, tt.resetArgs.debug, tt.resetArgs.progressDest)
		})

		for {
			canExit := false
			select {
			case msg := <-tt.resetArgs.progressDest:
				if strings.Contains(msg, "--EOF--") {
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

		go t.Run(tt.name, func(t *testing.T) {
			Defaults(tt.defaultsArgs.SerialPort, tt.defaultsArgs.PortSettings, tt.defaultsArgs.config, tt.defaultsArgs.debug, tt.defaultsArgs.progressDest)
		})
		select {
		case msg := <-tt.defaultsArgs.progressDest:
			if strings.Contains(msg, "--EOF--") {
				t.Logf("Test %s passed in %d:%d\n", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
			} else {
				t.Logf("Test %s: %s\n", tt.name, msg)
			}
		case <-timeout:
			t.Fatalf("Test %s timed out in 20 minutes.\n", tt.name)
		}
	}
}
