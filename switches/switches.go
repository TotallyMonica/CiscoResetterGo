package switches

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"main/common"
	"strconv"
	"strings"
	"time"
)

type SwitchPortConfig struct {
	Port           string
	SwitchportMode string
	Vlan           int
	Shutdown       bool
}

type VlanConfig struct {
	Vlan       int
	Shutdown   bool
	IpAddress  string
	SubnetMask string
}

type SshConfig struct {
	enable   bool
	username string
	password string
	bits     int
}

type SwitchConfig struct {
	Version         float64
	Vlans           []VlanConfig
	Ports           []SwitchPortConfig
	EnablePassword  string
	ConsolePassword string
	Ssh             SshConfig
	Banner          string
	Hostname        string
	DomainName      string
	DefaultGateway  string
}

func ParseFilesToDelete(files [][]byte, debug bool) []string {
	commonPrefixes := []string{"config", "vlan"}
	filesToDelete := make([]string, 0)

	if debug {
		for _, file := range files {
			cleanLine := strings.Split(strings.TrimSpace(string(common.TrimNull(file))), " ")
			if len(cleanLine) > 1 {
				for _, prefix := range commonPrefixes {
					for i := 0; i < len(cleanLine); i++ {
						if len(cleanLine[i]) > 0 && strings.Contains(strings.ToLower(strings.TrimSpace(cleanLine[i])), prefix) {
							delimitedCleanLine := strings.Split(cleanLine[i], "\n")
							filesToDelete = append(filesToDelete, delimitedCleanLine[0])
							fmt.Printf("DEBUG: File %s needs to be deleted (contains substring %s)\n", cleanLine[i], prefix)
						}
					}
				}
			}
		}
	} else {
		for _, file := range files {
			cleanLine := strings.Split(strings.TrimSpace(string(common.TrimNull(file))), " ")
			if len(cleanLine) > 1 {
				for _, prefix := range commonPrefixes {
					if strings.Contains(strings.ToLower(strings.TrimSpace(cleanLine[len(cleanLine)-1])), prefix) {
						filesToDelete[len(filesToDelete)] = cleanLine[len(cleanLine)-1]
					}
				}
			}
		}
	}

	return filesToDelete
}

func Reset(SerialPort string, PortSettings serial.Mode, debug bool) {
	const BUFFER_SIZE = 100
	const RECOVERY_PROMPT = "switches:"
	const CONFIRMATION_PROMPT = "[confirm]"
	const PASSWORD_RECOVERY = "password-recovery"
	const PASSWORD_RECOVERY_DISABLED = "password-recovery mechanism is disabled"
	const PASSWORD_RECOVERY_TRIGGERED = "password-recovery mechanism has been triggered"
	const PASSWORD_RECOVERY_ENABLED = "password-recovery mechanism is enabled"
	const YES_NO_PROMPT = "(y/n)?"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	port.SetReadTimeout(5 * time.Second)

	fmt.Println("Trigger password recovery by following these steps: ")
	fmt.Println("1. Unplug the switches")
	fmt.Println("2. Hold the MODE button on the switches.")
	fmt.Println("3. Plug the switches in while holding the button")
	fmt.Println("4. When you are told, release the MODE button")

	// Wait for switches to startup
	var output []byte
	var parsedOutput string
	if debug {
		for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY)) {
			parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(common.ReadLine(port, 500, debug)))))
			fmt.Printf("\n=============================================\nFROM DEVICE: %s\n", parsedOutput)
			fmt.Printf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
				strings.Contains(parsedOutput, RECOVERY_PROMPT))
			fmt.Printf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED)
			port.Write(common.FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
		fmt.Printf("DEBUG: %s\n", parsedOutput)
	} else {
		for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY)) {
			parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(common.ReadLine(port, 500, debug)))))
			fmt.Printf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
				strings.Contains(parsedOutput, RECOVERY_PROMPT))
			fmt.Printf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED)
			port.Write(common.FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Println("Release the MODE button and press Enter.")
	fmt.Scanln()

	// Ensure we have one of the test cases in the buffer
	if !(strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
		strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		port.Write(common.FormatCommand(""))
		port.Write(common.FormatCommand(""))
		port.Write(common.FormatCommand(""))
		port.Write(common.FormatCommand(""))
		port.Write(common.FormatCommand(""))
		parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(common.ReadLine(port, 500, debug)))))
	}

	// Test to see what we triggered on.
	// Password recovery was disabled
	if strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) {
		fmt.Println("Password recovery was disabled")
		for !(strings.Contains(parsedOutput, YES_NO_PROMPT)) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			port.Write(common.FormatCommand(""))
			output = common.ReadLine(port, 500, debug)
		}

		port.Write(common.FormatCommand("y"))

		for !(strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT)) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			port.Write(common.FormatCommand(""))
			time.Sleep(1 * time.Second)
			output = common.ReadLine(port, 500, debug)
		}

		port.Write(common.FormatCommand("boot"))
		common.ReadLines(port, BUFFER_SIZE, 10, debug)

		// Password recovery was enabled
	} else if strings.Contains(parsedOutput, RECOVERY_PROMPT) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			output = common.ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", common.TrimNull(output))
		}

		// Initialize Flash
		fmt.Println("Entered recovery console, now initializing flash")
		port.Write(common.FormatCommand("flash_init"))
		output = common.ReadLine(port, 500, debug)
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", common.TrimNull(output))
			}
			port.Write(common.FormatCommand(""))
			time.Sleep(1 * time.Second)
			output = common.ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", common.TrimNull(output))
		}

		// Get files
		fmt.Println("Flash has been initialized, now listing directory")
		port.SetReadTimeout(15 * time.Second)
		listing := make([][]byte, 1)
		port.Write(common.FormatCommand("dir flash:"))
		if debug {
			fmt.Printf("TO DEVICE: %s\n", "dir flash:")
		}
		time.Sleep(5 * time.Second)
		line := common.ReadLine(port, 16384, debug)
		listing = append(listing, line)
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))), RECOVERY_PROMPT) {
			line = common.ReadLine(port, 16384, debug)
			listing = append(listing, line)
			if debug {
				fmt.Printf("DEBUG: %s\n", common.TrimNull(line))
			}
			port.Write(common.FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", common.TrimNull(line))
		}

		// Determine the files we need to delete
		// TODO: Debug this section
		fmt.Println("Parsing files to delete...")
		filesToDelete := ParseFilesToDelete(listing, debug)

		// Delete files if necessary
		if len(filesToDelete) == 0 {
			fmt.Println("Switch has been reset already.")
		} else {
			port.SetReadTimeout(1 * time.Second)
			fmt.Println("Deleting files")
			for _, file := range filesToDelete {
				fmt.Println("Deleting " + file)
				if debug {
					fmt.Printf("TO DEVICE: %s\n", "del flash:"+file)
				}
				port.Write(common.FormatCommand("del flash:" + file))
				common.ReadLine(port, 500, debug)
				if debug {
					fmt.Printf("DEBUG: Confirming deletion\n")
				}
				fmt.Printf("TO DEVICE: %s\n", "y")
				port.Write(common.FormatCommand("y"))
				common.ReadLine(port, 500, debug)
			}
			fmt.Println("Switch has been reset")
		}

		fmt.Println("Restarting the switches")
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			output = common.ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", common.TrimNull(output))
		}

		if debug {
			fmt.Printf("TO DEVICE: %s\n", "reset")
		}
		port.Write(common.FormatCommand("reset"))
		common.ReadLine(port, 500, debug)

		if debug {
			fmt.Printf("TO DEVICE: %s\n", "y")
		}
		port.Write(common.FormatCommand("y"))
		common.ReadLines(port, BUFFER_SIZE, 10, debug)
	}

	fmt.Println("Successfully reset!")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config SwitchConfig, debug bool) {
	hostname := "Switch"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	port.SetReadTimeout(1 * time.Second)

	common.WaitForSubstring(port, prompt, debug)
	port.Write(common.FormatCommand(""))
	line := common.ReadLine(port, 500, debug)
	if debug {
		fmt.Printf("OUTPUT: %s", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
	}
	port.Write(common.FormatCommand("enable"))
	prompt = hostname + "#"

	port.Write(common.FormatCommand("conf t"))
	prompt = hostname + "(config)#"

	if len(config.Vlans) > 0 {
		for _, vlan := range config.Vlans {
			fmt.Printf("Configuring vlan %d\n", vlan.Vlan)
			port.Write(common.FormatCommand("inter vlan " + strconv.Itoa(vlan.Vlan)))
			if vlan.IpAddress != "" && vlan.SubnetMask != "" {
				port.Write(common.FormatCommand("ip addr " + vlan.IpAddress + " " + vlan.SubnetMask))
			}
			if vlan.Shutdown {
				port.Write(common.FormatCommand("shutdown"))
			} else {
				port.Write(common.FormatCommand("no shutdown"))
			}
			port.Write(common.FormatCommand("exit"))
		}
	}

	if len(config.Ports) != 0 {
		for _, switchPort := range config.Ports {
			fmt.Printf("Configuring port %s\n", switchPort.Port)
			port.Write(common.FormatCommand("inter " + switchPort.Port))
			if switchPort.SwitchportMode != "" {
				port.Write(common.FormatCommand("switchport mode " + switchPort.SwitchportMode))
			}
			if switchPort.Vlan != 0 && (strings.ToLower(switchPort.SwitchportMode) == "access" || strings.ToLower(switchPort.SwitchportMode) == "trunk") {
				if strings.ToLower(switchPort.SwitchportMode) == "access" {
					port.Write(common.FormatCommand("switchport access vlan " + strconv.Itoa(switchPort.Vlan)))
				} else if strings.ToLower(switchPort.SwitchportMode) == "trunk" {
					port.Write(common.FormatCommand("switchport trunk native vlan " + strconv.Itoa(switchPort.Vlan)))
				} else {
					fmt.Printf("Switch port mode %s is not supported for static vlan assignment\n", switchPort.SwitchportMode)
				}
			}
			port.Write(common.FormatCommand("exit"))
		}
	}

	if config.Banner != "" {
		port.Write(common.FormatCommand("banner motd " + config.Banner))
	}

	if config.ConsolePassword != "" {
		port.Write(common.FormatCommand("line console 0"))
		port.Write(common.FormatCommand("console password " + config.ConsolePassword))
		port.Write(common.FormatCommand("exit"))
	}

	if config.EnablePassword != "" {
		port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
	}

	if config.DefaultGateway != "" {
		port.Write(common.FormatCommand("ip default-gateway " + config.DefaultGateway))
	}

	if config.Hostname != "" {
		port.Write(common.FormatCommand("hostname " + config.Hostname))
		hostname = config.Hostname
	}

	if config.DomainName != "" {
		port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
	}

	if config.Ssh.enable {
		allowSSH := true
		if config.Ssh.username == "" {
			fmt.Println("Warning: SSH username not specified.")
			allowSSH = false
		}
		if config.Ssh.password == "" {
			fmt.Println("Warning: SSH password not specified.")
			allowSSH = false
		}
		if config.DomainName == "" {
			fmt.Println("Warning: Domain name not specified.")
			allowSSH = false
		}
		if config.Hostname == "" {
			fmt.Println("Warning: Hostname not specified.")
			allowSSH = false
		}

		if allowSSH {
			port.Write(common.FormatCommand("username " + config.Ssh.username + " password " + config.Ssh.password))
			port.Write(common.FormatCommand("crypto key gen rsa"))
			if config.Ssh.bits > 0 && config.Ssh.bits < 360 {
				config.Ssh.bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.bits <= 0 {
				config.Ssh.bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.bits > 2048 {
				config.Ssh.bits = 2048 // User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
			}
			port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.bits)))
		}
	}
}
