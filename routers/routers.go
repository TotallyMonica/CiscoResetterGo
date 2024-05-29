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

func Reset(SerialPort string, PortSettings serial.Mode, debug bool) {
	const BUFFER_SIZE = 4096
	const SHELL_PROMPT = "router"
	const ROMMON_PROMPT = "rommon"
	const CONFIRMATION_PROMPT = "[confirm]"
	const RECOVERY_REGISTER = "0x2142"
	const NORMAL_REGISTER = "0x2102"
	const SAVE_PROMPT = "[yes/no]: "
	const SHELL_CUE = "press return to get started!"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	port.SetReadTimeout(2 * time.Second)

	fmt.Println("Trigger the recovery sequence by following these steps: ")
	fmt.Println("1. Turn off the router")
	fmt.Println("2. After waiting for the lights to shut off, turn the router back on")
	fmt.Println("3. Press enter here once this has been completed")
	fmt.Scanln()

	fmt.Println("Sending ^C until we get into ROMMON...")
	var output []byte

	// Get to ROMMON
	if debug {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT))
			fmt.Printf("Expected prefix: %s\n", ROMMON_PROMPT)
			output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE, debug))
			fmt.Printf("FROM DEVICE: %s\n", strings.ToLower(strings.TrimSpace(string(output[:]))))
			fmt.Printf("TO DEVICE: %s%s%s%s%s%s%s%s%s%s\n", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c", "^c")
			port.Write([]byte("\x03\x03\x03\x03\x03\x03\x03\x03\x03\x03"))
			time.Sleep(1 * time.Second)
		}
		fmt.Println(output)
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT))
			fmt.Printf("Expected prefix: %s\n", ROMMON_PROMPT)
			port.Write([]byte("\x03\x03\x03\x03\x03\x03\x03\x03\x03\x03"))
			output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE, debug))
			time.Sleep(1 * time.Second)
		}
	}

	// In ROMMON
	fmt.Println("We've entered ROMMON, setting the register to 0x2142.")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	// TODO: Ensure we're actually at the prompt instead of just assuming
	for _, cmd := range commands {
		fmt.Printf("TO DEVICE: %s\n", cmd)
		port.Write(common.FormatCommand(cmd))
		output = common.ReadLine(port, BUFFER_SIZE, debug)
		fmt.Printf("DEBUG: Sent %s to device", cmd)
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	port.SetReadTimeout(10 * time.Second)
	fmt.Println("We've finished with ROMMON, going back into the regular console")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT) {
		fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		fmt.Printf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		if common.IsEmpty(output) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n")
			}
			port.Write([]byte("\r\n\r\n\r\n\r\n\r\n\r\n"))
		}
		time.Sleep(1 * time.Second)
		output = common.TrimNull(common.ReadLine(port, BUFFER_SIZE*2, debug))
	}

	fmt.Println("Setting the registers back to regular")
	port.SetReadTimeout(5 * time.Second)
	// We can safely assume we're at the prompt, begin running reset commands
	commands = []string{"enable", "conf t", "config-register " + NORMAL_REGISTER, "end"}
	for _, cmd := range commands {
		if debug {
			fmt.Printf("TO DEVICE: %s\n", cmd)
		}
		port.Write(common.FormatCommand(cmd))
		common.ReadLines(port, BUFFER_SIZE, 2, debug)
	}

	// Now reset config and restart
	fmt.Println("Resetting the configuration")
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "erase nvram:")
	}
	port.Write(common.FormatCommand("erase nvram:"))
	common.ReadLines(port, BUFFER_SIZE, 2, debug)
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "\\n")
	}
	port.Write(common.FormatCommand(""))
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	fmt.Println("Reloading the router")
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "reload")
	}
	port.Write(common.FormatCommand("reload"))
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	port.Write(common.FormatCommand("yes"))
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "yes")
	}
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	if debug {
		fmt.Printf("TO DEVICE: %s\n", "\\n")
	}
	port.Write(common.FormatCommand(""))
	common.ReadLines(port, BUFFER_SIZE, 2, debug)

	fmt.Println("Successfully reset!")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config RouterDefaults, debug bool) {
	hostname := "Switch"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	port.SetReadTimeout(1 * time.Second)

	output := common.TrimNull(common.ReadLine(port, 500, debug))
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		fmt.Printf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		if common.IsEmpty(output) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n")
			}
			port.Write([]byte("\r\n"))
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "no")
			}
			port.Write(common.FormatCommand("no"))
		}
		time.Sleep(1 * time.Second)
		output = common.TrimNull(common.ReadLine(port, 500, debug))
	}
	port.Write(common.FormatCommand(""))
	line := common.ReadLine(port, 500, debug)

	if debug {
		fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		fmt.Printf("INPUT: %s\n", "enable")
	}
	port.Write(common.FormatCommand("enable"))
	prompt = hostname + "#"
	line = common.ReadLine(port, 500, debug)

	if debug {
		fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		fmt.Printf("INPUT: %s\n", "conf t")
	}
	port.Write(common.FormatCommand("conf t"))
	prompt = hostname + "(config)#"

	// Configure router ports
	if len(config.Ports) != 0 {
		for _, routerPort := range config.Ports {
			if debug {
				fmt.Printf("INPUT: %s\n", "inter "+routerPort.Port)
			}
			port.Write(common.FormatCommand("inter " + routerPort.Port))
			output = common.TrimNull(common.ReadLine(port, 500, debug))
			prompt = hostname + "(config-if)#"
			if debug {
				fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
			}

			// Assign an IP address
			if routerPort.IpAddress != "" && routerPort.SubnetMask != "" {
				if debug {
					fmt.Printf("INPUT: %s\n", "ip addr "+routerPort.IpAddress+" subnet mask "+routerPort.SubnetMask)
				}
				port.Write(common.FormatCommand("ip addr " + routerPort.IpAddress + " " + routerPort.SubnetMask))
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}
			}

			// Decide if the port is up
			if routerPort.Shutdown {
				if debug {
					fmt.Printf("INPUT: %s\n", "shutdown")
				}
				port.Write(common.FormatCommand("shutdown"))
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}
			} else {
				if debug {
					fmt.Printf("INPUT: %s\n", "no shutdown")
				}
				port.Write(common.FormatCommand("no shutdown"))
				output = common.TrimNull(common.ReadLine(port, 500, debug))
				if debug {
					fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}
			}

			// Exit out to maintain consistent prompt state
			if debug {
				fmt.Printf("INPUT: %s\n", "exit")
			}
			port.Write(common.FormatCommand("exit"))
			output = common.TrimNull(common.ReadLine(port, 500, debug))
			if debug {
				fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
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
					fmt.Printf("Starting line of %d is invalid, defaulting back to 4\n", line.StartLine)
					line.StartLine = 4
				}
				if line.EndLine > 4 {
					fmt.Printf("Ending line of %d is invalid, defaulting back to 4\n", line.EndLine)
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
					fmt.Printf("INPUT: %s\n", command)
				}
				port.Write(common.FormatCommand(command))
				output = common.ReadLine(port, 500, debug)
				if debug {
					fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}

				// Set the line password
				if line.Password != "" {
					if debug {
						fmt.Printf("INPUT: %s\n", "password "+line.Password)
					}
					port.Write(common.FormatCommand("password " + line.Password))
					if debug {
						fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
					}

					// In case login type wasn't provided, set that.
					if line.Login != "" && line.Type == "vty" {
						line.Login = "local"
					}
				}

				// Set login method (empty string is valid for line console 0)
				if line.Login != "" || (line.Type == "console" && line.Password != "") {
					if debug {
						fmt.Printf("INPUT: %s\n", "login "+line.Login)
					}
					port.Write(common.FormatCommand("login " + line.Login))
					output = common.ReadLine(port, 500, debug)
					if debug {
						fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
					}
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					if debug {
						fmt.Printf("INPUT: %s\n", "transport input "+line.Transport)
					}
					port.Write(common.FormatCommand("transport input " + line.Transport))
					output = common.ReadLine(port, 500, debug)
					if debug {
						fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
					}
				}
			}
		}
	}

	// Set the default route
	if config.DefaultRoute != "" {
		if debug {
			fmt.Printf("INPUT: %s\n", "ip route 0.0.0.0 0.0.0.0 "+config.DefaultRoute)
		}
		port.Write(common.FormatCommand("ip route 0.0.0.0 0.0.0.0 " + config.DefaultRoute))
		output = common.ReadLine(port, 500, debug)
		if debug {
			fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
		}
	}

	// Set the domain name
	if config.DomainName != "" {
		if debug {
			fmt.Printf("INPUT: %s\n", "ip domain-name "+config.DomainName)
		}
		port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		output = common.ReadLine(port, 500, debug)
		if debug {
			fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
		}
	}

	// Set the enable password
	if config.EnablePassword != "" {
		if debug {
			fmt.Printf("INPUT: %s\n", "enable secret "+config.EnablePassword)
		}
		port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		output = common.ReadLine(port, 500, debug)
		if debug {
			fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
		}
	}

	// Set the hostname
	if config.Hostname != "" {
		if debug {
			fmt.Printf("INPUT: %s\n", "hostname "+config.Hostname)
		}
		port.Write(common.FormatCommand("hostname " + config.Hostname))
		hostname = config.Hostname
		prompt = hostname + "(config)"
		output = common.ReadLine(port, 500, debug)
		if debug {
			fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
		}
	}

	if config.Banner != "" {
		if debug {
			fmt.Printf("INPUT: %s\n", "banner motd "+config.Banner)
		}
		port.Write(common.FormatCommand("banner motd " + config.Banner))
		output = common.ReadLine(port, 500, debug)
		if debug {
			fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
		}
	}
	if config.Ssh.Enable {
		allowSSH := true
		if config.Ssh.Username == "" {
			fmt.Println("WARNING: SSH username not specified.")
			allowSSH = false
		}
		if config.Ssh.Password == "" {
			fmt.Println("WARNING: SSH password not specified.")
			allowSSH = false
		}
		if config.DomainName == "" {
			fmt.Println("WARNING: Domain name not specified.")
			allowSSH = false
		}
		if config.Hostname == "" {
			fmt.Println("WARNING: Hostname not specified.")
			allowSSH = false
		}

		if allowSSH {
			if debug {
				fmt.Printf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password)
			}
			port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			line = common.ReadLine(port, 500, debug)
			if debug {
				fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			if debug {
				fmt.Printf("INPUT: %s\n", "crypto key gen rsa")
			}
			port.Write(common.FormatCommand("crypto key gen rsa"))
			line = common.ReadLine(port, 500, debug)
			if debug {
				fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			if config.Ssh.Bits > 0 && config.Ssh.Bits < 360 {
				if debug {
					fmt.Printf("DEBUG: Requested bit setting of %d is too low, defaulting to 360\n", config.Ssh.Bits)
				}
				config.Ssh.Bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.Bits <= 0 {
				if debug {
					fmt.Printf("DEBUG: Bit setting not provided, defaulting to 512\n")
				}
				config.Ssh.Bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.Bits > 2048 {
				if debug {
					fmt.Printf("DEBUG: Requested bit setting of %d is too low, defaulting to 2048\n", config.Ssh.Bits)
				}
				config.Ssh.Bits = 2048 // User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
			}

			if debug {
				fmt.Printf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits))
			}
			port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			line = common.ReadLine(port, 500, debug)
			if debug {
				fmt.Printf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			// Previous command can take a while, so wait for the prompt
			port.SetReadTimeout(10 * time.Second)
			common.WaitForSubstring(port, prompt, debug)
		}
	}
}
