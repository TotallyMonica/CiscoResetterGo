package routers

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"main/common"
	"strconv"
	"strings"
	"time"
)

type RouterPorts struct {
	Port       string
	Shutdown   bool
	IpAddress  string
	SubnetMask string
}

type SshConfig struct {
	Enable   bool
	Username string
	Password string
	Login    string
	Bits     int
}

type LineConfig struct {
	Type      string
	StartLine int
	EndLine   int
	Login     string
	Transport string
	Password  string
}

type RouterDefaults struct {
	Version        float64
	Ports          []RouterPorts
	Ssh            SshConfig
	Lines          []LineConfig
	EnablePassword string
	Banner         string
	Hostname       string
	DomainName     string
	DefaultRoute   string
}

var redirectedOutput chan string

func outputInfo(data string) {
	current := time.Now()
	if redirectedOutput == nil && !strings.HasSuffix(data, "---EOF---") {
		fmt.Printf("<%d-%02d-%02d %02d:%02d:%02d> %s", current.Year(), current.Month(), current.Day(),
			current.Hour(), current.Minute(), current.Second(), data)
	} else if redirectedOutput != nil {
		redirectedOutput <- fmt.Sprintf("<%d-%02d-%02d %02d:%02d:%02d> %s", current.Year(), current.Month(),
			current.Day(), current.Hour(), current.Minute(), current.Second(), data)
	}
}

func Reset(SerialPort string, PortSettings serial.Mode, debug bool, progressDest chan string) {
	const BUFFER_SIZE = 4096
	const SHELL_PROMPT = "router"
	const ROMMON_PROMPT = "rommon"
	const CONFIRMATION_PROMPT = "[confirm]"
	const RECOVERY_REGISTER = "0x2142"
	const NORMAL_REGISTER = "0x2102"
	const SAVE_PROMPT = "[yes/no]: "
	const SHELL_CUE = "press return to get started!"

	redirectedOutput = progressDest

	port, err := serial.Open(SerialPort, &PortSettings)
	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			log.Fatalf("routers.Reset: Error while closing port %s: %s\n", SerialPort, err)
		}
	}(port)

	if err != nil {
		log.Fatalf("routers.Reset: Error while opening port %s: %s\n", SerialPort, err)
	}

	err = port.SetReadTimeout(2 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	outputInfo("Trigger the recovery sequence by following these steps: \n")
	outputInfo("1. Turn off the router\n")
	outputInfo("2. After waiting for the lights to shut off, turn the router back on\n")

	outputInfo("Sending ^C until we get into ROMMON...\n")
	var output []byte

	// Get to ROMMON
	if debug {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT) {
			outputInfo(fmt.Sprintf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT)))
			outputInfo(fmt.Sprintf("Expected prefix: %s\n", ROMMON_PROMPT))
			output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE, debug))
			outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", strings.ToLower(strings.TrimSpace(string(output[:])))))
			outputInfo(fmt.Sprintf("TO DEVICE: %s%s%s%s%s%s%s%s%s%s\n", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c"))
			_, err = port.Write([]byte("\x03\x03\x03\x03\x03\x03\x03\x03\x03\x03"))
			if err != nil {
				log.Fatal(err)
			}
			time.Sleep(1 * time.Second)
		}
		outputInfo(fmt.Sprintf("%s\n", output))
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT) {
			output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE, debug))
			_, err = port.Write([]byte("\x03\x03\x03\x03\x03\x03\x03\x03\x03\x03"))
			if err != nil {
				log.Fatal(err)
			}
			time.Sleep(1 * time.Second)
		}
	}

	// In ROMMON
	outputInfo("We've entered ROMMON, setting the register to 0x2142.\n")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	// TODO: Ensure we're actually at the prompt instead of just assuming
	for _, cmd := range commands {
		if debug {
			outputInfo(fmt.Sprintf("TO DEVICE: %s\n", cmd))
		}
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("DEBUG: Sent %s to device", cmd))
		}
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	err = port.SetReadTimeout(10 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	outputInfo("We've finished with ROMMON, going back into the regular console\n")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT) {
		if debug {
			outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
			outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		if common.IsEmpty(output) {
			if debug {
				outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n"))
			}
			_, err = port.Write([]byte("\r\n\r\n\r\n\r\n\r\n\r\n"))
			if err != nil {
				log.Fatal(err)
			}
		}
		time.Sleep(1 * time.Second)
		output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE*2, debug))
	}

	outputInfo("Setting the registers back to regular\n")
	err = port.SetReadTimeout(5 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	// We can safely assume we're at the prompt, begin running reset commands
	commands = []string{"enable", "conf t", "config-register " + NORMAL_REGISTER, "end"}
	for _, cmd := range commands {
		if debug {
			outputInfo(fmt.Sprintf("TO DEVICE: %s\n", cmd))
		}
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			log.Fatal(err)
		}
		common.ReadLines(port, BUFFER_SIZE, 2, debug)
	}

	// Now reset config and restart
	outputInfo("Resetting the configuration\n")
	if debug {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "erase nvram:"))
	}
	_, err = port.Write(common.FormatCommand("erase nvram:"))
	if err != nil {
		log.Fatal(err)
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)
	if debug {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\n"))
	}
	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		log.Fatal(err)
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	outputInfo("Reloading the router\n")
	if debug {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "reload"))
	}
	_, err = port.Write(common.FormatCommand("reload"))
	if err != nil {
		log.Fatal(err)
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	_, err = port.Write(common.FormatCommand("yes"))
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "yes"))
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	if debug {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\n"))
	}
	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		log.Fatal(err)
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	outputInfo("Successfully reset!\n")
	outputInfo("---EOF---")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config RouterDefaults, debug bool, progressDest chan string) {
	redirectedOutput = progressDest

	hostname := "Router"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)
	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			log.Fatalf("routers.Defaults: Error while closing port %s: %s\n", SerialPort, err)
		}
	}(port)

	if err != nil {
		log.Fatalf("routers.Defaults: Error while opening port %s: %s\n", SerialPort, err)
	}

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	output := common.TrimNull(common.ReadLine(port, 500, debug))
	outputInfo("Waiting for the router to start up")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		if debug {
			outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
			outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		if common.IsEmpty(output) {
			if debug {
				outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
			}
			_, err = port.Write([]byte("\r\n"))
			if err != nil {
				log.Fatal(err)
			}
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			if debug {
				outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "no"))
			}
			_, err = port.Write(common.FormatCommand("no"))
			if err != nil {
				log.Fatal(err)
			}
		}
		time.Sleep(1 * time.Second)
		output = common.TrimNull(common.ReadLine(port, 500, debug))
	}
	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		log.Fatal(err)
	}
	line := common.ReadLine(port, 500, debug)

	outputInfo("Elevating our privileges")

	if debug {
		outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		outputInfo(fmt.Sprintf("INPUT: %s\n", "enable"))
	}
	_, err = port.Write(common.FormatCommand("enable"))
	if err != nil {
		log.Fatal(err)
	}
	prompt = hostname + "#"
	line = common.ReadLine(port, 500, debug)

	if debug {
		outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		outputInfo(fmt.Sprintf("INPUT: %s\n", "conf t"))
	}

	outputInfo("Entering global configuration mode")
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		log.Fatal(err)
	}
	prompt = hostname + "(config)#"

	// Configure router ports
	if len(config.Ports) != 0 {
		outputInfo("Configuring the interface")
		for _, routerPort := range config.Ports {
			outputInfo(fmt.Sprintf("Configuring interface %s\n", routerPort.Port))
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "inter "+routerPort.Port))
			}
			_, err = port.Write(common.FormatCommand("inter " + routerPort.Port))
			if err != nil {
				log.Fatal(err)
			}
			output = common.TrimNull(common.ReadLine(port, 500, debug))
			prompt = hostname + "(config-if)#"
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			// Assign an IP address
			if routerPort.IpAddress != "" && routerPort.SubnetMask != "" {
				outputInfo(fmt.Sprintf("Assigning IP %s with subnet mask %s\n", routerPort.IpAddress, routerPort.SubnetMask))
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "ip addr "+routerPort.IpAddress+" subnet mask "+routerPort.SubnetMask))
				}
				_, err = port.Write(common.FormatCommand("ip addr " + routerPort.IpAddress + " " + routerPort.SubnetMask))
				if err != nil {
					log.Fatal(err)
				}
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			}

			// Decide if the port is up
			if routerPort.Shutdown {
				outputInfo(fmt.Sprintf("Shutting down the interface\n"))
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "shutdown"))
				}
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					log.Fatal(err)
				}
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			} else {
				outputInfo(fmt.Sprintf("Brining up the interface\n"))
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "no shutdown"))
				}
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					log.Fatal(err)
				}
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			}

			// Exit out to maintain consistent prompt state
			outputInfo(fmt.Sprintf("Finished configuring %s\n", routerPort.Port))
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
			}
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				log.Fatal(err)
			}
			output = common.TrimNull(common.ReadLine(port, 500, debug))
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			prompt = hostname + "(config)#"
		}

		outputInfo("Finished configuring physical interfaces\n")
	}

	// Configure console lines
	// Literally stolen from switches/switches.go
	if len(config.Lines) != 0 {
		outputInfo("Configuring console lines\n")
		for _, line := range config.Lines {
			outputInfo(fmt.Sprintf("Configuring line %s %d to %d", line.Type, line.StartLine, line.EndLine))
			if line.Type != "" {
				// Ensure both lines are <= 4
				if line.StartLine > 4 {
					outputInfo(fmt.Sprintf("Starting line of %d is invalid, defaulting back to 4\n", line.StartLine))
					line.StartLine = 4
				}
				if line.EndLine > 4 {
					outputInfo(fmt.Sprintf("Ending line of %d is invalid, defaulting back to 4\n", line.EndLine))
					line.EndLine = 4
				}

				// Figure out line ranges
				command := ""
				if line.StartLine == line.EndLine { // Check if start line = end line
					command = "line " + line.Type + " " + strconv.Itoa(line.StartLine)
				} else if line.StartLine < line.EndLine { // Make sure starting line < end line
					command = "line " + line.Type + " " + strconv.Itoa(line.StartLine) + " " + strconv.Itoa(line.EndLine)
				} else { // Check if invalid ranges were given
					log.Fatalln("Start line is greater than end line.")
				}
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", command))
				}
				_, err = port.Write(common.FormatCommand(command))
				if err != nil {
					log.Fatal(err)
				}
				output = common.ReadLine(port, 500, debug)
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}

				// Set the line password
				if line.Password != "" {
					outputInfo(fmt.Sprintf("Applying the password %s to the line", line.Password))
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "password "+line.Password))
					}
					_, err = port.Write(common.FormatCommand("password " + line.Password))
					if err != nil {
						log.Fatal(err)
					}
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}

					// In case login type wasn't provided, set that.
					if line.Login != "" && line.Type == "vty" {
						line.Login = "local"
					}
				}

				// Set login method (empty string is valid for line console 0)
				if line.Login != "" || (line.Type == "console" && line.Password != "") {
					outputInfo("Enforcing credential usage on the line\n")
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "login "+line.Login))
					}
					_, err = port.Write(common.FormatCommand("login " + line.Login))
					if err != nil {
						log.Fatal(err)
					}
					output = common.ReadLine(port, 500, debug)
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					outputInfo(fmt.Sprintf("Setting the transport type to %s\n", line.Transport))
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "transport input "+line.Transport))
					}
					_, err = port.Write(common.FormatCommand("transport input " + line.Transport))
					if err != nil {
						log.Fatal(err)
					}
					output = common.ReadLine(port, 500, debug)
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				}
			}
		}
	}

	// Set the default route
	if config.DefaultRoute != "" {
		outputInfo(fmt.Sprintf("Setting the default route to %s\n", config.DefaultRoute))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "ip route 0.0.0.0 0.0.0.0 "+config.DefaultRoute))
		}
		_, err = port.Write(common.FormatCommand("ip route 0.0.0.0 0.0.0.0 " + config.DefaultRoute))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, 500, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the domain name
	if config.DomainName != "" {
		outputInfo(fmt.Sprintf("Setting the domain name to %s\n", config.DomainName))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "ip domain-name "+config.DomainName))
		}
		_, err = port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, 500, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the enable password
	if config.EnablePassword != "" {
		outputInfo(fmt.Sprintf("Setting the enable password to %s\n", config.EnablePassword))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "enable secret "+config.EnablePassword))
		}
		_, err = port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, 500, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the hostname
	if config.Hostname != "" {
		outputInfo(fmt.Sprintf("Setting the hostname to %s\n", config.Hostname))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "hostname "+config.Hostname))
		}
		_, err = port.Write(common.FormatCommand("hostname " + config.Hostname))
		if err != nil {
			log.Fatal(err)
		}
		hostname = config.Hostname
		prompt = hostname + "(config)"
		output = common.ReadLine(port, 500, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	if config.Banner != "" {
		outputInfo(fmt.Sprintf("Setting the banner to %s\n", config.Banner))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "banner motd "+config.Banner))
		}
		_, err = port.Write(common.FormatCommand("banner motd " + config.Banner))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, 500, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}
	if config.Ssh.Enable {
		outputInfo("Determing if SSH can be enabled\n")
		allowSSH := true
		if config.Ssh.Username == "" {
			outputInfo("WARNING: SSH username not specified.\n")
			allowSSH = false
		}
		if config.Ssh.Password == "" {
			outputInfo("WARNING: SSH password not specified.\n")
			allowSSH = false
		}
		if config.DomainName == "" {
			outputInfo("WARNING: Domain name not specified.\n")
			allowSSH = false
		}
		if config.Hostname == "" {
			outputInfo("WARNING: Hostname not specified.\n")
			allowSSH = false
		}

		if allowSSH {
			outputInfo(fmt.Sprintf("Setting the username to %s and the password to %s\n", config.Ssh.Username, config.Ssh.Password))
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password))
			}
			_, err = port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			if err != nil {
				log.Fatal(err)
			}
			line = common.ReadLine(port, 500, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			outputInfo("Generating the RSA key\n")
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "crypto key gen rsa"))
			}
			_, err = port.Write(common.FormatCommand("crypto key gen rsa"))
			if err != nil {
				log.Fatal(err)
			}
			line = common.ReadLine(port, 500, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			if config.Ssh.Bits > 0 && config.Ssh.Bits < 360 {
				if debug {
					outputInfo(fmt.Sprintf("DEBUG: Requested bit setting of %d is too low, defaulting to 360\n", config.Ssh.Bits))
				}
				config.Ssh.Bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.Bits <= 0 {
				if debug {
					outputInfo(fmt.Sprintf("DEBUG: Bit setting not provided, defaulting to 512\n"))
				}
				config.Ssh.Bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.Bits > 2048 {
				if debug {
					outputInfo(fmt.Sprintf("DEBUG: Requested bit setting of %d is too low, defaulting to 2048\n", config.Ssh.Bits))
				}
				config.Ssh.Bits = 2048 // User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
			}

			outputInfo(fmt.Sprintf("Generating an RSA key %d bits wide\n", config.Ssh.Bits))

			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits)))
			}
			_, err = port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			if err != nil {
				log.Fatal(err)
			}
			line = common.ReadLine(port, 500, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			// Previous command can take a while, so wait for the prompt
			err = port.SetReadTimeout(10 * time.Second)
			if err != nil {
				log.Fatal(err)
			}
			common.WaitForSubstring(port, prompt, debug)
		}
	}

	outputInfo("Settings applied!\n")
	outputInfo("Note: Settings have not been made persistent and will be lost upon reboot.\n")
	outputInfo("To fix this, run `wr` on the target device.\n")
	outputInfo("---EOF---")
}
