package routers

import (
	"encoding/json"
	"go.bug.st/serial"
	"io"
	"log"
	"main/common"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func getPortType() string {
	if runtime.GOOS == "windows" {
		return "COM3"
	} else if runtime.GOOS == "linux" {
		return "/dev/ttyUSB0"
	} else {
		log.Fatalf("Unsupported OS type: %s\n", runtime.GOOS)
	}

	return ""
}

// TODO: Validate reset/defaults
func validateOutput() {

}

func TestReset(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	} else if os.Getenv("SKIP_RESET_TESTS") != "" {
		t.Skip("Skipping all reset tests")
	}

	type args struct {
		SerialPort   string
		PortSettings serial.Mode
		backup       common.Backup
		debug        bool
		progressDest chan string
	}

	tests := make([]struct {
		name string
		args args
	}, 0)
	if os.Getenv("SKIP_VERBOSE_RESET") != "" {
		t.Skip("Skipping verbose output reset tests")
	} else {
		tests = append(tests, struct {
			name string
			args args
		}{
			name: "Reset with verbose output",
			args: args{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				backup: common.Backup{
					Backup: false,
				},
				debug:        true,
				progressDest: make(chan string),
			},
		})
	}

	if os.Getenv("SKIP_LIMITED_RESET") != "" {
		t.Skip("Skipping limited output reset tests")
	} else {
		tests = append(tests, struct {
			name string
			args args
		}{
			name: "Reset without verbose output",
			args: args{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				backup: common.Backup{
					Backup: false,
				},
				debug:        false,
				progressDest: make(chan string),
			},
		})
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
					t.Logf("Test %s passed in %d:%d", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
					canExit = true
				} else {
					t.Logf("Test %s: %s", tt.name, msg)
				}
			case <-timeout:
				t.Fatalf("Test %s timed out in 20 minutes.", tt.name)
			}
			if canExit {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func TestDefaults(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	} else if os.Getenv("SKIP_DEFAULTS") != "" {
		t.Skip("Skipping all defaults tests")
	}

	type args struct {
		SerialPort   string
		PortSettings serial.Mode
		config       RouterDefaults
		debug        bool
		progressDest chan string
	}

	defaultsFile, err := os.OpenFile("router_defaults.json", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("Error while loading defaults file for testing: %s", err)
	}

	defer func(defaultsFile *os.File) {
		err = defaultsFile.Close()
		if err != nil {
			t.Fatalf("Error while closing defaults file for testing: %s", err)
		}
	}(defaultsFile)

	defaults, err := io.ReadAll(defaultsFile)
	if err != nil {
		t.Fatalf("Error while reading defaults file for testing: %s", err)
	}

	defaultsStruct := RouterDefaults{}

	err = json.Unmarshal(defaults, &defaultsStruct)
	if err != nil {
		t.Fatalf("Error while parsing defaults file for testing: %s", err)
	}

	tests := make([]struct {
		name string
		args args
	}, 0)

	if os.Getenv("SKIP_VERBOSE_DEFAULTS") != "" {
		t.Skip("Skipping verbose defaults tests")
	} else {
		tests = append(tests, struct {
			name string
			args args
		}{
			name: "Apply defaults with verbose output",
			args: args{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				config:       defaultsStruct,
				debug:        true,
				progressDest: make(chan string),
			},
		})
	}

	if os.Getenv("SKIP_LIMITED_DEFAULTS") != "" {
		t.Skip("Skipping limited output defaults tests")
	} else {
		tests = append(tests, struct {
			name string
			args args
		}{
			name: "Apply defaults with limited output",
			args: args{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				config:       defaultsStruct,
				progressDest: make(chan string),
			},
		})
	}

	for _, tt := range tests {
		start := time.Now()
		timeout := time.After(20 * time.Minute)

		go t.Run(tt.name, func(t *testing.T) {
			Defaults(tt.args.SerialPort, tt.args.PortSettings, tt.args.config, tt.args.debug, tt.args.progressDest)
		})
		for {
			canExit := false
			select {
			case msg := <-tt.args.progressDest:
				if strings.Contains(msg, "--EOF--") {
					t.Logf("Test %s passed in %d:%d", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
					canExit = true
				} else {
					t.Logf("Test %s: %s", tt.name, msg)
				}
			case <-timeout:
				t.Fatalf("Test %s timed out in 20 minutes.", tt.name)
			}

			if canExit {
				break
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func TestResetAndDefaults(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	} else if os.Getenv("SKIP_RESET_DEFAULTS_TESTS") != "" {
		t.Skip("Skipping reset and defaults tests")
	}

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
		t.Fatalf("Error while loading defaults file for testing: %s", err)
	}

	defer defaultsFile.Close()

	defaults, err := io.ReadAll(defaultsFile)
	if err != nil {
		t.Fatalf("Error while reading defaults file for testing: %s", err)
	}

	defaultsStruct := RouterDefaults{}

	err = json.Unmarshal(defaults, &defaultsStruct)
	if err != nil {
		t.Fatalf("Error while parsing defaults file for testing: %s", err)
	}

	tests := make([]struct {
		name         string
		resetArgs    resetArgs
		defaultsArgs defaultsArgs
	}, 0)

	if os.Getenv("SKIP_VERBOSE_RESET_DEFAULT_TEST") != "" {
		t.Skip("Skipping verbose reset and default tests")
	} else {
		tests = append(tests, struct {
			name         string
			resetArgs    resetArgs
			defaultsArgs defaultsArgs
		}{
			name: "Reset and apply defaults with verbose output",
			resetArgs: resetArgs{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				backup: common.Backup{
					Backup: false,
				},
				debug:        true,
				progressDest: make(chan string),
			},
			defaultsArgs: defaultsArgs{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				config:       defaultsStruct,
				debug:        true,
				progressDest: make(chan string),
			},
		})
	}

	if os.Getenv("SKIP_LIMITED_RESET_DEFAULT_TEST") != "" {
		t.Skip("Skipping limited output reset and default tests")
	} else {
		tests = append(tests, struct {
			name         string
			resetArgs    resetArgs
			defaultsArgs defaultsArgs
		}{
			name: "Reset and apply defaults with limited output",
			resetArgs: resetArgs{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				backup: common.Backup{
					Backup: false,
				},
				debug:        false,
				progressDest: make(chan string),
			},
			defaultsArgs: defaultsArgs{
				SerialPort:   getPortType(),
				PortSettings: serial.Mode{BaudRate: 9600, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit},
				config:       defaultsStruct,
				progressDest: make(chan string),
			},
		})
	}

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
					t.Logf("Reset test %s passed in %d:%d", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
					canExit = true
				} else {
					t.Logf("Reset test %s: %s", tt.name, msg)
				}
			case <-timeout:
				t.Fatalf("Reset test %s timed out in 20 minutes.", tt.name)
			}
			if canExit {
				break
			}
		}

		time.Sleep(5 * time.Second)

		go t.Run(tt.name, func(t *testing.T) {
			Defaults(tt.defaultsArgs.SerialPort, tt.defaultsArgs.PortSettings, tt.defaultsArgs.config, tt.defaultsArgs.debug, tt.defaultsArgs.progressDest)
		})
		for {
			canExit := false
			select {
			case msg := <-tt.defaultsArgs.progressDest:
				if strings.Contains(msg, "--EOF--") {
					t.Logf("Defaults test %s passed in %d:%d", tt.name, int(math.Floor(time.Since(start).Minutes())), int(math.Floor(time.Since(start).Seconds()))%60)
					canExit = true
				} else {
					t.Logf("Defaults test %s: %s", tt.name, msg)
				}
			case <-timeout:
				t.Fatalf("Defaults test %s timed out in 20 minutes.", tt.name)
			}
			if canExit {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}
}
