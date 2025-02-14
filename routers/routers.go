package routers

import (
	"fmt"
	"go.bug.st/serial"
	"main/common"
	"main/crglogging"
	"os"
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
var consoleOutput [][]byte
var LoggerName string

func WriteConsoleOutput() error {
	dumpFile := os.Getenv("DumpConsoleOutput")
	if dumpFile != "" {
		file, err := os.OpenFile(dumpFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}

		defer file.Close()

		totalWritten := 0

		for _, line := range consoleOutput {
			written, err := file.Write(line)
			if err != nil {
				return err
			}
			totalWritten += written
		}

		common.OutputInfo(fmt.Sprintf("Wrote %d bytes to %s\n", totalWritten, dumpFile))
	}

	return nil
}

func Reset(SerialPort string, PortSettings serial.Mode, backup common.Backup, debug bool, progressDest chan string) {
	LoggerName = fmt.Sprintf("RouterResetter%s%d%d%d", SerialPort, PortSettings.BaudRate, PortSettings.StopBits, PortSettings.DataBits)
	resetterLog := crglogging.New(LoggerName)

	const BUFFER_SIZE = 4096
	const SHELL_PROMPT = "router"
	const ROMMON_PROMPT = "rommon"
	const CONFIRMATION_PROMPT = "[confirm]"
	const RECOVERY_REGISTER = "0x2142"
	const NORMAL_REGISTER = "0x2102"
	const SAVE_PROMPT = "[yes/no]:"
	const SHELL_CUE = "press return to get started!"

	redirectedOutput = progressDest
	if redirectedOutput != nil {
		common.SetOutputChannel(redirectedOutput)
	}

	currentTime := time.Now()
	backup.Prefix = currentTime.Format(fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", currentTime.Year(), currentTime.Month(),
		currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second()))

	port, err := serial.Open(SerialPort, &PortSettings)
	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while closing port %s: %s\n", SerialPort, err)
		}
	}(port)

	common.SetReaderPort(port)

	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while opening port %s: %s\n", SerialPort, err)
	}

	err = port.SetReadTimeout(2 * time.Second)
	if err != nil {
		resetterLog.Fatal(err)
	}

	common.OutputInfo("Trigger the recovery sequence by following these steps: \n")
	common.OutputInfo("1. Turn off the router\n")
	common.OutputInfo("2. After waiting for the lights to shut off, turn the router back on\n")

	common.OutputInfo("Sending ^C until we get into ROMMON...\n")
	var output []byte

	// Get to ROMMON
	if debug {
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT+" 1 >") {
			common.OutputInfo(fmt.Sprintf("Has prefix: %t\n", strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT+" 1 >")))
			common.OutputInfo(fmt.Sprintf("Expected prefix: %s\n", ROMMON_PROMPT+" 1 >"))
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", strings.ToLower(strings.TrimSpace(string(output[:])))))
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "^c"))
			_, err = port.Write([]byte("\x03"))
			if err != nil {
				resetterLog.Fatal(err)
			}
		}
		common.OutputInfo(fmt.Sprintf("%s\n", output))
	} else {
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT+" 1 >") {
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
			_, err = port.Write([]byte("\x03"))
			if err != nil {
				resetterLog.Fatal(err)
			}
		}
	}
	WriteConsoleOutput()

	// In ROMMON
	common.OutputInfo("We've entered ROMMON, setting the register to 0x2142.\n")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	for idx, cmd := range commands {
		if debug {
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", cmd))
		}
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			resetterLog.Fatal(err)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		parsedOutput := strings.TrimSpace(string(common.TrimNull(output)))
		consoleOutput = append(consoleOutput, output)
		if debug {
			common.OutputInfo(fmt.Sprintf("DEBUG: Sent %s to device\n", cmd))
		}

		for !strings.HasPrefix(strings.ToLower(parsedOutput), fmt.Sprintf("%s %d >", ROMMON_PROMPT, idx+1)) {
			_, err = port.Write([]byte("\r\n"))
			if debug {
				common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			parsedOutput = strings.TrimSpace(string(common.TrimNull(output)))
			consoleOutput = append(consoleOutput, output)
			if debug {
				common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
			}
		}
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	err = port.SetReadTimeout(10 * time.Second)
	if err != nil {
		resetterLog.Fatal(err)
	}
	common.OutputInfo("We've finished with ROMMON, going back into the regular console\n")
	WriteConsoleOutput()
	if debug {
		common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
	}
	_, err = port.Write([]byte("\r\n"))
	if err != nil {
		resetterLog.Fatal(err)
	}
	output, err = common.ReadLine(port, BUFFER_SIZE, debug)
	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
	}
	consoleOutput = append(consoleOutput, output)

	// Wait until we get clue that we're ready for input, intentionally not sending anything
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), SHELL_CUE) {
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}

	// Send new lines until we get to shell prompt
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT+">") {
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
		}
		_, err = port.Write([]byte("\r\n"))
		if err != nil {
			resetterLog.Fatal(err)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}

	common.OutputInfo("We've made it into the regular console\n")
	WriteConsoleOutput()

	closeTftpServer := make(chan bool)

	// Check if we can and should back up
	if backup.Backup {
		if backup.Destination != "" || (backup.Source == "" && backup.SubnetMask != "") || (backup.Source != "" && backup.SubnetMask == "") {
			backup.Backup = false
		}
		common.OutputInfo("Unable to back up the config due to missing values\n")
		if backup.Destination == "" {
			common.OutputInfo("Backup destination is empty\n")
		}
		if backup.Source == "" {
			common.OutputInfo("Backup source is empty\n")
		}
		if backup.SubnetMask == "" {
			common.OutputInfo("Subnet mask is empty\n")
		}
	}

	err = port.SetReadTimeout(5 * time.Second)
	if err != nil {
		resetterLog.Fatal(err)
	}
	// We can safely assume we're at the prompt, begin running commands to restore registers, back up, and reset
	commands = []string{"enable", "conf t", "config-register " + NORMAL_REGISTER}

	// Add in the relevant commands to back up if we are
	if backup.Backup {
		ip := ""
		if backup.Source == "" && backup.SubnetMask == "" {
			ip = "dhcp"
		} else {
			ip = fmt.Sprintf("%s %s", backup.Source, backup.SubnetMask)
		}
		commands = append(commands, "inter g0/0/0", fmt.Sprintf("ip addr %s", ip), "no shutdown")

		// Begin the built-in TFTP server if chosen
		if backup.UseBuiltIn {
			go common.BuiltInTftpServer(closeTftpServer)
		}
	}

	// We're no longer needed in global config, so queue command to get out of that
	commands = append(commands, "end")

	// Add in some more backup-oriented commands
	if backup.Backup {
		commands = append(commands, fmt.Sprintf("copy startup-config tftp://%s/%s-router-config.txt", backup.Destination, backup.Prefix))
	}

	// Queue command to erase the NVRAM and reload
	commands = append(commands, "erase nvram:", "")

	// Execute the commands
	for _, cmd := range commands {
		prefix := ""
		switch cmd {
		case "enable":
			common.OutputInfo("Setting our register back to normal\n")
			prefix = SHELL_PROMPT + "#"
			break
		case "conf t":
			common.OutputInfo("Entering privileged exec\n")
			prefix = SHELL_PROMPT + "(config)#"
		case "inter g0/0/0":
			common.OutputInfo("Setting an IP address to back up the config\n")
			prefix = SHELL_PROMPT + "(config-if)#"
			break
		case "end":
			common.OutputInfo("Finished configuring our console\n")
			prefix = SHELL_PROMPT + "#"
			break
		case fmt.Sprintf("copy startup-config tftp://%s/%s-router-config.txt", backup.Destination, backup.Prefix):
			common.OutputInfo(fmt.Sprintf("Backing up the config to %s\n", backup.Destination))
			prefix = SHELL_PROMPT + "#"
			break
		case "erase nvram:":
			common.OutputInfo("Erasing the router's config\n")
			prefix = SHELL_PROMPT + "#"
			break
		case "reload":
			common.OutputInfo("Restarting the switch\n")
			break
		}

		if debug {
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", cmd))
		}
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			resetterLog.Fatal(err)
		}

		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
		}
		consoleOutput = append(consoleOutput, output)
		WriteConsoleOutput()

		for common.IsSyslog(string(output)) || // Disregard syslog messages
			!(strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), prefix) || // Disregard lines that don't have the prompt we're looking for
				cmd == "conf t" && strings.Contains(strings.ToLower(strings.TrimSpace(string(output))),
					strings.ToLower(strings.TrimSpace("enter configuration commands, one per line.  end with cntl/z.")))) { // Global config specific test

			if debug {
				common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
			}
			err := common.WriteLine(port, "", debug)
			if err != nil {

			}

			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			if debug {
				common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
			}
			consoleOutput = append(consoleOutput, output)
			WriteConsoleOutput()
		}
	}

	// Reload the switch
	if debug {
		common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "reload"))
	}
	common.WriteLine(port, "reload", debug)
	output, err = common.ReadLine(port, BUFFER_SIZE, debug)
	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
	}
	consoleOutput = append(consoleOutput, output)
	if debug {
		common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
	}
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), SAVE_PROMPT) {
		if debug {
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
		}
		common.WriteLine(port, "", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
		}
	}

	if debug {
		common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "yes"))
	}
	common.WriteLine(port, "yes", debug)
	output, err = common.ReadLine(port, BUFFER_SIZE, debug)
	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
	}
	consoleOutput = append(consoleOutput, output)

	// Send blank new lines until we've reset
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), CONFIRMATION_PROMPT) {
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
		}

		if debug {
			common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
		}
		common.WriteLine(port, "", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}
	if debug {
		common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output))
	}

	if backup.UseBuiltIn {
		closeTftpServer <- true
	}

	WriteConsoleOutput()
	common.OutputInfo("Successfully reset!\n")
	common.OutputInfo("---EOF---")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config RouterDefaults, debug bool, progressDest chan string) {
	LoggerName = fmt.Sprintf("RouterDefaults%s%d%d%d", SerialPort, PortSettings.BaudRate, PortSettings.StopBits, PortSettings.DataBits)
	defaultsLogger := crglogging.New(LoggerName)

	redirectedOutput = progressDest
	if redirectedOutput != nil {
		common.SetOutputChannel(redirectedOutput)
	}

	hostname := "Router"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)
	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while closing port %s: %s\n", SerialPort, err)
		}
	}(port)

	if err != nil {
		defaultsLogger.Errorf("routers.Defaults: Error while opening port %s: %s\n", SerialPort, err)
	}

	common.SetReaderPort(port)

	err = port.SetReadTimeout(1 * time.Minute)
	if err != nil {
		defaultsLogger.Errorf("An error occurred while setting the timeout: %s\n", err)
	}

	if debug {
		common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
	}
	_, err = port.Write([]byte("\r\n"))
	if err != nil {
		defaultsLogger.Errorf("An error occurred while writing a new line: %s\n", err)
		return
	}
	output, err := common.ReadLine(port, 500, debug)
	if err != nil {
		defaultsLogger.Errorf("routers.Defaults: Error while reading line: %s\n", err)
		return
	}
	common.OutputInfo("Waiting for the router to start up\n")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		if debug {
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			common.OutputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			if debug {
				common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "no"))
			}
			_, err = port.Write(common.FormatCommand("no"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
		} else {
			if debug {
				common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "\\r\\n"))
			}
			_, err = port.Write([]byte("\r\n"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
		}
		time.Sleep(1 * time.Second)
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
	}
	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		defaultsLogger.Fatal(err)
	}

	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	output, err = common.ReadLine(port, 500, debug)
	if err != nil {
		defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
	}

	common.OutputInfo("Elevating our privileges\n")

	if debug {
		common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "enable"))
	}
	_, err = port.Write(common.FormatCommand("enable"))
	if err != nil {
		defaultsLogger.Fatal(err)
	}

	prompt = hostname + "#"
	common.WaitForSubstring(port, prompt, debug)

	output, err = common.ReadLine(port, 500, debug)
	if err != nil {
		defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
	}

	if debug {
		common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "conf t"))
	}

	common.OutputInfo("Entering global configuration mode\n")
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	prompt = hostname + "(config)#"
	common.WaitForSubstring(port, prompt, debug)

	// Configure router ports
	if len(config.Ports) != 0 {
		common.OutputInfo("Configuring the physical interfaces\n")
		for _, routerPort := range config.Ports {
			common.OutputInfo(fmt.Sprintf("Configuring interface %s\n", routerPort.Port))
			if debug {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "inter "+routerPort.Port))
			}
			_, err = port.Write(common.FormatCommand("inter " + routerPort.Port))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			prompt = hostname + "(config-if)#"
			common.WaitForSubstring(port, prompt, debug)

			if debug {
				common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			// Assign an IP address
			if routerPort.IpAddress != "" && routerPort.SubnetMask != "" {
				common.OutputInfo(fmt.Sprintf("Assigning IP %s with subnet mask %s\n", routerPort.IpAddress, routerPort.SubnetMask))
				if debug {
					common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "ip addr "+routerPort.IpAddress+" subnet mask "+routerPort.SubnetMask))
				}
				_, err = port.Write(common.FormatCommand("ip addr " + routerPort.IpAddress + " " + routerPort.SubnetMask))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				if debug {
					common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			}

			// Decide if the port is up
			if routerPort.Shutdown {
				common.OutputInfo(fmt.Sprintf("Shutting down the interface\n"))
				if debug {
					common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "shutdown"))
				}
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				if debug {
					common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			} else {
				common.OutputInfo(fmt.Sprintf("Brining up the interface\n"))
				if debug {
					common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "no shutdown"))
				}
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				if debug {
					common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}
			}

			// Exit out to maintain consistent prompt state
			common.OutputInfo(fmt.Sprintf("Finished configuring %s\n", routerPort.Port))
			if debug {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
			}
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			if debug {
				common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)
		}

		common.OutputInfo("Finished configuring physical interfaces\n")
	}

	// Configure console lines
	// Literally stolen from switches/switches.go
	if len(config.Lines) != 0 {
		common.OutputInfo("Configuring console lines\n")
		for _, line := range config.Lines {
			common.OutputInfo(fmt.Sprintf("Configuring line %s %d to %d\n", line.Type, line.StartLine, line.EndLine))
			if line.Type != "" {
				// Ensure both lines are <= 4
				if line.StartLine > 4 {
					common.OutputInfo(fmt.Sprintf("Starting line of %d is invalid, defaulting back to 4\n", line.StartLine))
					line.StartLine = 4
				}
				if line.EndLine > 4 {
					common.OutputInfo(fmt.Sprintf("Ending line of %d is invalid, defaulting back to 4\n", line.EndLine))
					line.EndLine = 4
				}

				// Figure out line ranges
				command := ""
				if line.StartLine == line.EndLine { // Check if start line = end line
					command = "line " + line.Type + " " + strconv.Itoa(line.StartLine)
				} else if line.StartLine < line.EndLine { // Make sure starting line < end line
					command = "line " + line.Type + " " + strconv.Itoa(line.StartLine) + " " + strconv.Itoa(line.EndLine)
				} else { // Check if invalid ranges were given
					defaultsLogger.Fatalln("Start line is greater than end line.")
				}
				if debug {
					common.OutputInfo(fmt.Sprintf("INPUT: %s\n", command))
				}
				_, err = port.Write(common.FormatCommand(command))
				if err != nil {
					defaultsLogger.Fatal(err)
				}

				prompt = hostname + "(config-line)#"
				common.WaitForSubstring(port, prompt, debug)

				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				if debug {
					common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}

				// Set the line password
				if line.Password != "" {
					common.OutputInfo(fmt.Sprintf("Applying the password %s to the line\n", line.Password))
					if debug {
						common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "password "+line.Password))
					}
					_, err = port.Write(common.FormatCommand("password " + line.Password))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					if debug {
						common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}

					// In case login type wasn't provided, set that.
					if line.Login != "" && line.Type == "vty" {
						line.Login = "local"
					}
				}

				// Set login method (empty string is valid for line console 0)
				if line.Login != "" || (line.Type == "console" && line.Password != "") {
					common.OutputInfo("Enforcing credential usage on the line\n")
					if debug {
						common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "login "+line.Login))
					}
					_, err = port.Write(common.FormatCommand("login " + line.Login))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, 500, debug)
					if err != nil {
						defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
					}
					if debug {
						common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					common.OutputInfo(fmt.Sprintf("Setting the transport type to %s\n", line.Transport))
					if debug {
						common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "transport input "+line.Transport))
					}
					_, err = port.Write(common.FormatCommand("transport input " + line.Transport))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, 500, debug)
					if err != nil {
						defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
					}
					if debug {
						common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				}
			}

			common.OutputInfo(fmt.Sprintf("Configuring line %s %d to %d done\n", line.Type, line.StartLine, line.EndLine))
			if debug {
				common.OutputInfo(fmt.Sprintf("TO DEVICE: %s\n", "exit"))
			}
			common.WriteLine(port, "exit", debug)
			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)
		}
	}

	// Set the default route
	if config.DefaultRoute != "" {
		common.OutputInfo(fmt.Sprintf("Setting the default route to %s\n", config.DefaultRoute))
		if debug {
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "ip route 0.0.0.0 0.0.0.0 "+config.DefaultRoute))
		}
		_, err = port.Write(common.FormatCommand("ip route 0.0.0.0 0.0.0.0 " + config.DefaultRoute))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the domain name
	if config.DomainName != "" {
		common.OutputInfo(fmt.Sprintf("Setting the domain name to %s\n", config.DomainName))
		if debug {
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "ip domain-name "+config.DomainName))
		}
		_, err = port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the enable password
	if config.EnablePassword != "" {
		common.OutputInfo(fmt.Sprintf("Setting the enable password to %s\n", config.EnablePassword))
		if debug {
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "enable secret "+config.EnablePassword))
		}
		_, err = port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	// Set the hostname
	if config.Hostname != "" {
		common.OutputInfo(fmt.Sprintf("Setting the hostname to %s\n", config.Hostname))
		if debug {
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "hostname "+config.Hostname))
		}
		_, err = port.Write(common.FormatCommand("hostname " + config.Hostname))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		hostname = config.Hostname
		prompt = hostname + "(config)"
		common.WaitForSubstring(port, prompt, debug)
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	if config.Banner != "" {
		common.OutputInfo(fmt.Sprintf("Setting the banner to %s\n", config.Banner))
		if debug {
			common.OutputInfo(fmt.Sprintf("INPUT: %s\"%s\"\n", "banner motd ", config.Banner))
		}
		_, err = port.Write(common.FormatCommand(fmt.Sprintf("banner motd \"%s\"", config.Banner)))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		if debug {
			common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}
	if config.Ssh.Enable {
		common.OutputInfo("Determing if SSH can be enabled\n")
		allowSSH := true
		if config.Ssh.Username == "" {
			common.OutputInfo("WARNING: SSH username not specified.\n")
			allowSSH = false
		}
		if config.Ssh.Password == "" {
			common.OutputInfo("WARNING: SSH password not specified.\n")
			allowSSH = false
		}
		if config.DomainName == "" {
			common.OutputInfo("WARNING: Domain name not specified.\n")
			allowSSH = false
		}
		if config.Hostname == "" {
			common.OutputInfo("WARNING: Hostname not specified.\n")
			allowSSH = false
		}

		if allowSSH {
			common.OutputInfo(fmt.Sprintf("Setting the username to %s and the password to %s\n", config.Ssh.Username, config.Ssh.Password))
			if debug {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password))
			}
			_, err = port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			if debug {
				common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			common.OutputInfo("Generating the RSA key\n")
			if debug {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "crypto key gen rsa"))
			}
			_, err = port.Write(common.FormatCommand("crypto key gen rsa"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			if debug {
				common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			if config.Ssh.Bits > 0 && config.Ssh.Bits < 360 {
				if debug {
					common.OutputInfo(fmt.Sprintf("DEBUG: Requested bit setting of %d is too low, defaulting to 360\n", config.Ssh.Bits))
				}
				config.Ssh.Bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.Bits <= 0 {
				if debug {
					common.OutputInfo(fmt.Sprintf("DEBUG: Bit setting not provided, defaulting to 512\n"))
				}
				config.Ssh.Bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.Bits > 2048 {
				if debug {
					common.OutputInfo(fmt.Sprintf("DEBUG: Requested bit setting of %d is too low, defaulting to 2048\n", config.Ssh.Bits))
				}
				config.Ssh.Bits = 2048 // User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
			}

			common.OutputInfo(fmt.Sprintf("Generating an RSA key %d bits wide\n", config.Ssh.Bits))

			if debug {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits)))
			}
			_, err = port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			if debug {
				common.OutputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			// Previous command can take a while, so wait for the prompt
			err = port.SetReadTimeout(10 * time.Second)
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			common.WaitForSubstring(port, prompt, debug)
		}
	}

	common.OutputInfo("Leaving global exec")

	if debug {
		common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "end"))
	}
	common.WriteLine(port, "end", debug)
	prompt = hostname + "#"

	common.WaitForSubstring(port, prompt, debug)

	common.OutputInfo("Settings applied!\n")
	common.OutputInfo("Note: Settings have not been made persistent and will be lost upon reboot.\n")
	common.OutputInfo("To fix this, run `wr` on the target device.\n")
	common.OutputInfo("---EOF---")
}
