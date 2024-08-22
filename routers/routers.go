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
	if redirectedOutput == nil {
		fmt.Printf(data)
	} else {
		redirectedOutput <- data
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

	if err != nil {
		log.Fatal(err)
	}

	err = port.SetReadTimeout(2 * time.Second)
	if err != nil {
		return
	}
	if err != nil {
		log.Fatal(err)
	}

	outputInfo("Trigger the recovery sequence by following these steps: \n")
	outputInfo("1. Turn off the router\n")
	outputInfo("2. After waiting for the lights to shut off, turn the router back \n")
	outputInfo("3. Press enter here once this has been completed\n")
	_, err = fmt.Scanln()
	if err != nil {
		return
	}

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
			outputInfo(fmt.Sprintf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT)))
			outputInfo(fmt.Sprintf("Expected prefix: %s\n", ROMMON_PROMPT))
			_, err = port.Write([]byte("\x03\x03\x03\x03\x03\x03\x03\x03\x03\x03"))
			if err != nil {
				log.Fatal(err)
			}
			output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE, debug))
			time.Sleep(1 * time.Second)
		}
	}

	// In ROMMON
	outputInfo("We've entered ROMMON, setting the register to 0x2142.\n")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	// TODO: Ensure we're actually at the prompt instead of just assuming
	for _, cmd := range commands {
		outputInfo(fmt.Sprintf("TO DEVICE: %s\n", cmd))
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			log.Fatal(err)
		}
		output = common.ReadLine(port, BUFFER_SIZE, debug)
		outputInfo(fmt.Sprintf("DEBUG: Sent %s to device", cmd))
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	err = port.SetReadTimeout(10 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	outputInfo("We've finished with ROMMON, going back into the regular console\n")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT) {
		outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
		outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
		outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
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
}

func Defaults(SerialPort string, PortSettings serial.Mode, config RouterDefaults, debug bool, progressDest chan string) {
	redirectedOutput = progressDest

	hostname := "Switch"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	output := common.TrimNull(common.ReadLine(port, 500, debug))
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
		outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
		outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
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
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		log.Fatal(err)
	}
	prompt = hostname + "(config)#"

	// Configure router ports
	if len(config.Ports) != 0 {
		for _, routerPort := range config.Ports {
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
	}

	// Configure console lines
	// Literally stolen from switches/switches.go
	if len(config.Lines) != 0 {
		for _, line := range config.Lines {
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
}
