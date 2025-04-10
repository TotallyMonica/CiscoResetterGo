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

var consoleOutput [][]byte
var LoggerName string

func GetLoggerName() string {
	logger := crglogging.GetLogger(LoggerName)
	logger.Debugf("Logger name: %s\n", LoggerName)
	return LoggerName
}

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

func Reset(SerialPort string, PortSettings serial.Mode, backup common.Backup, debug bool, updateChan chan bool) {
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

	if updateChan != nil {
		common.SetOutputChannel(updateChan, LoggerName)
	}

	if debug {
		resetterLog.SetLogLevel(5)
	} else {
		resetterLog.SetLogLevel(4)
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

	resetterLog.Infof("Trigger the recovery sequence by following these steps: \n")
	resetterLog.Infof("1. Turn off the router\n")
	resetterLog.Infof("2. After waiting for the lights to shut off, turn the router back on\n")

	resetterLog.Infof("Sending ^C until we get into ROMMON...\n")
	var output []byte

	// Get to ROMMON
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT+" 1 >") {
		resetterLog.Debugf("Has prefix: %t\n", strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), ROMMON_PROMPT+" 1 >"))
		resetterLog.Debugf("Expected prefix: %s\n", ROMMON_PROMPT+" 1 >")
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
		resetterLog.Debugf("FROM DEVICE: %s\n", strings.ToLower(strings.TrimSpace(string(output[:]))))
		resetterLog.Debugf("TO DEVICE: %s\n", "^c")
		_, err = port.Write([]byte("\x03"))
		if err != nil {
			resetterLog.Fatal(err)
		}
	}
	resetterLog.Debugf("%s\n", output)
	WriteConsoleOutput()

	// In ROMMON
	resetterLog.Infof("We've entered ROMMON, setting the register to 0x2142.\n")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	for idx, cmd := range commands {
		resetterLog.Debugf("TO DEVICE: %s\n", cmd)
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
		resetterLog.Debugf("DEBUG: Sent %s to device\n", cmd)

		for !strings.HasPrefix(strings.ToLower(parsedOutput), fmt.Sprintf("%s %d >", ROMMON_PROMPT, idx+1)) {
			_, err = port.Write([]byte("\r\n"))
			resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			parsedOutput = strings.TrimSpace(string(common.TrimNull(output)))
			consoleOutput = append(consoleOutput, output)
			resetterLog.Debugf("FROM DEVICE: %s\n", output)
		}
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	err = port.SetReadTimeout(10 * time.Second)
	if err != nil {
		resetterLog.Fatal(err)
	}
	resetterLog.Infof("We've finished with ROMMON, going back into the regular console\n")
	WriteConsoleOutput()
	resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
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
		resetterLog.Debugf("FROM DEVICE: %s\n", output)
		resetterLog.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		resetterLog.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}

	// Send new lines until we get to shell prompt
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT+">") {
		resetterLog.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		resetterLog.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		resetterLog.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
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

	resetterLog.Infof("We've made it into the regular console\n")
	WriteConsoleOutput()

	closeTftpServer := make(chan bool)

	// Check if we can and should back up
	if backup.Backup {
		if backup.Destination != "" || (backup.Source == "" && backup.SubnetMask != "") || (backup.Source != "" && backup.SubnetMask == "") {
			backup.Backup = false
		}
		resetterLog.Infof("Unable to back up the config due to missing values\n")
		if backup.Destination == "" {
			resetterLog.Infof("Backup destination is empty\n")
		}
		if backup.Source == "" {
			resetterLog.Infof("Backup source is empty\n")
		}
		if backup.SubnetMask == "" {
			resetterLog.Infof("Subnet mask is empty\n")
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
			resetterLog.Infof("Setting our register back to normal\n")
			prefix = SHELL_PROMPT + "#"
			break
		case "conf t":
			resetterLog.Infof("Entering privileged exec\n")
			prefix = SHELL_PROMPT + "(config)#"
		case "inter g0/0/0":
			resetterLog.Infof("Setting an IP address to back up the config\n")
			prefix = SHELL_PROMPT + "(config-if)#"
			break
		case "end":
			resetterLog.Infof("Finished configuring our console\n")
			prefix = SHELL_PROMPT + "#"
			break
		case fmt.Sprintf("copy startup-config tftp://%s/%s-router-config.txt", backup.Destination, backup.Prefix):
			resetterLog.Infof("Backing up the config to %s\n", backup.Destination)
			prefix = SHELL_PROMPT + "#"
			break
		case "erase nvram:":
			resetterLog.Infof("Erasing the router's config\n")
			prefix = SHELL_PROMPT + "#"
			break
		case "reload":
			resetterLog.Infof("Restarting the switch\n")
			break
		}

		resetterLog.Debugf("TO DEVICE: %s\n", cmd)
		_, err = port.Write(common.FormatCommand(cmd))
		if err != nil {
			resetterLog.Fatal(err)
		}

		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		resetterLog.Debugf("FROM DEVICE: %s\n", output)
		consoleOutput = append(consoleOutput, output)
		WriteConsoleOutput()

		for common.IsSyslog(string(output)) || // Disregard syslog messages
			!(strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), prefix) || // Disregard lines that don't have the prompt we're looking for
				cmd == "conf t" && strings.Contains(strings.ToLower(strings.TrimSpace(string(output))),
					strings.ToLower(strings.TrimSpace("enter configuration commands, one per line.  end with cntl/z.")))) { // Global config specific test

			resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
			err := common.WriteLine(port, "", debug)
			if err != nil {

			}

			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
			}
			resetterLog.Debugf("FROM DEVICE: %s\n", output)
			consoleOutput = append(consoleOutput, output)
			WriteConsoleOutput()
		}
	}

	// Reload the switch
	resetterLog.Debugf("TO DEVICE: %s\n", "reload")

	common.WriteLine(port, "reload", debug)
	output, err = common.ReadLine(port, BUFFER_SIZE, debug)
	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
	}

	consoleOutput = append(consoleOutput, output)
	resetterLog.Debugf("FROM DEVICE: %s\n", output)
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), SAVE_PROMPT) {
		resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
		common.WriteLine(port, "", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
		resetterLog.Debugf("FROM DEVICE: %s\n", output)
	}

	resetterLog.Debugf("TO DEVICE: %s\n", "yes")
	common.WriteLine(port, "yes", debug)
	output, err = common.ReadLine(port, BUFFER_SIZE, debug)
	if err != nil {
		resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
	}
	consoleOutput = append(consoleOutput, output)

	// Send blank new lines until we've reset
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), CONFIRMATION_PROMPT) {
		resetterLog.Debugf("FROM DEVICE: %s\n", output)

		resetterLog.Debugf("TO DEVICE: %s\n", "\\r\\n")
		common.WriteLine(port, "", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetterLog.Fatalf("routers.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}
	resetterLog.Debugf("FROM DEVICE: %s\n", output)

	if backup.UseBuiltIn {
		closeTftpServer <- true
	}

	WriteConsoleOutput()
	resetterLog.Infof("Successfully reset!\n")
	resetterLog.Infof("---EOF---")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config RouterDefaults, debug bool, updateChan chan bool) {
	LoggerName = fmt.Sprintf("RouterDefaults%s%d%d%d", SerialPort, PortSettings.BaudRate, PortSettings.StopBits, PortSettings.DataBits)
	defaultsLogger := crglogging.New(LoggerName)

	if updateChan != nil {
		common.SetOutputChannel(updateChan, LoggerName)
	}

	// Handle debug
	defaultsLogger.SetLogLevel(4)
	if debug {
		defaultsLogger.SetLogLevel(5)
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

	defaultsLogger.Debugf("TO DEVICE: %s\n", "\\r\\n")

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
	defaultsLogger.Infof("Waiting for the router to start up\n")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		defaultsLogger.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		defaultsLogger.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		defaultsLogger.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			defaultsLogger.Debugf("TO DEVICE: %s\n", "no")
			_, err = port.Write(common.FormatCommand("no"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
		} else {
			defaultsLogger.Debugf("TO DEVICE: %s\n", "\\r\\n")
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

	defaultsLogger.Infof("Elevating our privileges\n")

	defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	defaultsLogger.Debugf("INPUT: %s\n", "enable")
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

	defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	defaultsLogger.Debugf("INPUT: %s\n", "conf t")

	defaultsLogger.Infof("Entering global configuration mode\n")
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	prompt = hostname + "(config)#"
	common.WaitForSubstring(port, prompt, debug)

	// Configure router ports
	if len(config.Ports) != 0 {
		defaultsLogger.Infof("Configuring the physical interfaces\n")
		for _, routerPort := range config.Ports {
			defaultsLogger.Infof("Configuring interface %s\n", routerPort.Port)
			defaultsLogger.Debugf("INPUT: %s\n", "inter "+routerPort.Port)
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

			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			// Assign an IP address
			if routerPort.IpAddress != "" && routerPort.SubnetMask != "" {
				defaultsLogger.Infof("Assigning IP %s with subnet mask %s\n", routerPort.IpAddress, routerPort.SubnetMask)
				defaultsLogger.Debugf("INPUT: %s\n", "ip addr "+routerPort.IpAddress+" subnet mask "+routerPort.SubnetMask)
				_, err = port.Write(common.FormatCommand("ip addr " + routerPort.IpAddress + " " + routerPort.SubnetMask))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
			}

			// Decide if the port is up
			if routerPort.Shutdown {
				defaultsLogger.Infof("Shutting down the interface\n")
				defaultsLogger.Debugf("INPUT: %s\n", "shutdown")
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
			} else {
				defaultsLogger.Infof("Brining up the interface\n")
				defaultsLogger.Debugf("INPUT: %s\n", "no shutdown")
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				output, err = common.ReadLine(port, 500, debug)
				if err != nil {
					defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
				}
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
			}

			// Exit out to maintain consistent prompt state
			defaultsLogger.Infof("Finished configuring %s\n", routerPort.Port)
			defaultsLogger.Debugf("INPUT: %s\n", "exit")
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)
		}

		defaultsLogger.Infof("Finished configuring physical interfaces\n")
	}

	// Configure console lines
	// Literally stolen from switches/switches.go
	if len(config.Lines) != 0 {
		defaultsLogger.Infof("Configuring console lines\n")
		for _, line := range config.Lines {
			defaultsLogger.Infof("Configuring line %s %d to %d\n", line.Type, line.StartLine, line.EndLine)
			if line.Type != "" {
				// Ensure both lines are <= 4
				if line.StartLine > 4 {
					defaultsLogger.Infof("Starting line of %d is invalid, defaulting back to 4\n", line.StartLine)
					line.StartLine = 4
				}
				if line.EndLine > 4 {
					defaultsLogger.Infof("Ending line of %d is invalid, defaulting back to 4\n", line.EndLine)
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
				defaultsLogger.Debugf("INPUT: %s\n", command)
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
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

				// Set the line password
				if line.Password != "" {
					defaultsLogger.Infof("Applying the password %s to the line\n", line.Password)
					defaultsLogger.Debugf("INPUT: %s\n", "password "+line.Password)
					_, err = port.Write(common.FormatCommand("password " + line.Password))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

					// In case login type wasn't provided, set that.
					if line.Login != "" && line.Type == "vty" {
						line.Login = "local"
					}
				}

				// Set login method (empty string is valid for line console 0)
				if line.Login != "" || (line.Type == "console" && line.Password != "") {
					defaultsLogger.Infof("Enforcing credential usage on the line\n")
					defaultsLogger.Debugf("INPUT: %s\n", "login "+line.Login)
					_, err = port.Write(common.FormatCommand("login " + line.Login))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, 500, debug)
					if err != nil {
						defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
					}
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					defaultsLogger.Infof("Setting the transport type to %s\n", line.Transport)
					defaultsLogger.Debugf("INPUT: %s\n", "transport input "+line.Transport)
					_, err = port.Write(common.FormatCommand("transport input " + line.Transport))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, 500, debug)
					if err != nil {
						defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
					}
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}
			}

			defaultsLogger.Infof("Configuring line %s %d to %d done\n", line.Type, line.StartLine, line.EndLine)
			defaultsLogger.Debugf("TO DEVICE: %s\n", "exit")
			common.WriteLine(port, "exit", debug)
			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)
		}
	}

	// Set the default route
	if config.DefaultRoute != "" {
		defaultsLogger.Infof("Setting the default route to %s\n", config.DefaultRoute)
		defaultsLogger.Debugf("INPUT: %s\n", "ip route 0.0.0.0 0.0.0.0 "+config.DefaultRoute)
		_, err = port.Write(common.FormatCommand("ip route 0.0.0.0 0.0.0.0 " + config.DefaultRoute))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}

	// Set the domain name
	if config.DomainName != "" {
		defaultsLogger.Infof("Setting the domain name to %s\n", config.DomainName)
		defaultsLogger.Debugf("INPUT: %s\n", "ip domain-name "+config.DomainName)
		_, err = port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}

	// Set the enable password
	if config.EnablePassword != "" {
		defaultsLogger.Infof("Setting the enable password to %s\n", config.EnablePassword)
		defaultsLogger.Debugf("INPUT: %s\n", "enable secret "+config.EnablePassword)
		_, err = port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}

	// Set the hostname
	if config.Hostname != "" {
		defaultsLogger.Debugf("Setting the hostname to %s\n", config.Hostname)
		defaultsLogger.Debugf("INPUT: %s\n", "hostname "+config.Hostname)
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
		defaultsLogger.Infof("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}

	if config.Banner != "" {
		defaultsLogger.Infof("Setting the banner to %s\n", config.Banner)
		defaultsLogger.Debugf("INPUT: %s\"%s\"\n", "banner motd ", config.Banner)
		_, err = port.Write(common.FormatCommand(fmt.Sprintf("banner motd \"%s\"", config.Banner)))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
		}
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}
	if config.Ssh.Enable {
		defaultsLogger.Infof("Determing if SSH can be enabled\n")
		allowSSH := true
		if config.Ssh.Username == "" {
			defaultsLogger.Warningf("WARNING: SSH username not specified.\n")
			allowSSH = false
		}
		if config.Ssh.Password == "" {
			defaultsLogger.Warningf("WARNING: SSH password not specified.\n")
			allowSSH = false
		}
		if config.DomainName == "" {
			defaultsLogger.Warningf("WARNING: Domain name not specified.\n")
			allowSSH = false
		}
		if config.Hostname == "" {
			defaultsLogger.Warningf("WARNING: Hostname not specified.\n")
			allowSSH = false
		}

		if allowSSH {
			defaultsLogger.Debugf("Setting the username to %s and the password to %s\n", config.Ssh.Username, config.Ssh.Password)
			defaultsLogger.Debugf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password)
			_, err = port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			defaultsLogger.Infof("Generating the RSA key\n")
			defaultsLogger.Debugf("INPUT: %s\n", "crypto key gen rsa")
			_, err = port.Write(common.FormatCommand("crypto key gen rsa"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			if config.Ssh.Bits > 0 && config.Ssh.Bits < 360 {
				defaultsLogger.Debugf("DEBUG: Requested bit setting of %d is too low, defaulting to 360\n", config.Ssh.Bits)
				config.Ssh.Bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.Bits <= 0 {
				defaultsLogger.Debugf("DEBUG: Bit setting not provided, defaulting to 512\n")
				config.Ssh.Bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.Bits > 2048 {
				defaultsLogger.Debugf("DEBUG: Requested bit setting of %d is too low, defaulting to 2048\n", config.Ssh.Bits)
				config.Ssh.Bits = 2048 // User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
			}

			defaultsLogger.Debugf("Generating an RSA key %d bits wide\n", config.Ssh.Bits)

			defaultsLogger.Debugf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits))
			_, err = port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				defaultsLogger.Fatalf("routers.Defaults: Error while reading line: %s\n", err)
			}
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			// Previous command can take a while, so wait for the prompt
			err = port.SetReadTimeout(10 * time.Second)
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			common.WaitForSubstring(port, prompt, debug)
		}
	}

	defaultsLogger.Infof("Leaving global exec")

	defaultsLogger.Debugf("INPUT: %s\n", "end")
	common.WriteLine(port, "end", debug)
	prompt = hostname + "#"

	common.WaitForSubstring(port, prompt, debug)

	defaultsLogger.Infof("Settings applied!\n")
	defaultsLogger.Infof("Note: Settings have not been made persistent and will be lost upon reboot.\n")
	defaultsLogger.Infof("To fix this, run `wr` on the target device.\n")
	defaultsLogger.Infof("---EOF---")
}
