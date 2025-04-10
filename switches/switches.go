package switches

import (
	"errors"
	"fmt"
	"go.bug.st/serial"
	"io"
	"main/common"
	"main/crglogging"
	"os"
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
	Lines           []LineConfig
}

const BUFFER_SIZE = 500
const RECOVERY_PROMPT = "switch:"
const CONFIRMATION_PROMPT = "[confirm]"
const PASSWORD_RECOVERY = "password-recovery"
const PASSWORD_RECOVERY_DISABLED = "password-recovery mechanism is disabled"
const PASSWORD_RECOVERY_TRIGGERED = "password-recovery mechanism has been triggered"
const PASSWORD_RECOVERY_ENABLED = "password-recovery mechanism is enabled"
const YES_NO_PROMPT = "(y/n)?"
const LOW_PRIV_PREFIX = "Switch>"
const ELEVATED_PREFIX = "Switch#"
const INITIAL_CONFIG_PROMPT = "Would you like to enter the initial configuration dialog? [yes/no]:"

var redirectedOutput chan string
var consoleOutput [][]byte

var LoggerName string

func ParseFilesToDelete(files [][]byte, debug bool) []string {
	logger := crglogging.GetLogger(LoggerName)

	commonPrefixes := []string{"config", "vlan"}
	filesToDelete := make([]string, 0)

	for _, file := range files {
		cleanLine := strings.Split(strings.TrimSpace(string(common.TrimNull(file))), " ")
		if len(cleanLine) > 1 {
			for _, prefix := range commonPrefixes {
				for i := 0; i < len(cleanLine); i++ {
					if len(cleanLine[i]) > 0 && strings.Contains(strings.ToLower(strings.TrimSpace(cleanLine[i])), prefix) {
						getRidOfSpacesPlease := strings.TrimSpace(cleanLine[i])
						delimitedCleanLine := strings.Split(getRidOfSpacesPlease, "\n")
						filesToDelete = append(filesToDelete, delimitedCleanLine[0])
						logger.Debugf("DEBUG: File %s needs to be deleted (contains substring %s)\n", cleanLine[i], prefix)
					}
				}
			}
		}
	}

	return filesToDelete
}

func Reset(SerialPort string, PortSettings serial.Mode, backup common.Backup, debug bool, updateChan chan bool) {
	LoggerName = fmt.Sprintf("SwitchResetter%s%d%d%d", SerialPort, PortSettings.BaudRate, PortSettings.StopBits, PortSettings.DataBits)
	resetLogger := crglogging.New(LoggerName)

	var files []string
	currentTime := time.Now()
	backup.Prefix = currentTime.Format(fmt.Sprintf("%d%02d%02d_%02d%02d%02d", currentTime.Year(), currentTime.Month(),
		currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second()))

	if updateChan != nil {
		common.SetOutputChannel(updateChan, LoggerName)
	}

	if debug {
		resetLogger.SetLogLevel(5)
	} else {
		resetLogger.SetLogLevel(4)
	}

	var progress common.Progress
	progress.TotalSteps = 10
	progress.CurrentStep = 0

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		resetLogger.Fatalf("switches.Reset: Error while opening port: %s\n", err)
	}

	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while closing port: %s\n", err)
		}
	}(port)

	common.SetReaderPort(port)

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		resetLogger.Fatalf("switches.Reset: Error while setting read timeout: %s\n", err)
	}

	common.OutputInfo("Trigger password recovery by following these steps: \n")
	common.OutputInfo("1. Unplug the switch\n")
	common.OutputInfo("2. Hold the MODE button on the switch.\n")
	common.OutputInfo("3. Plug the switch in while holding the button\n")
	common.OutputInfo("4. When you are told, release the MODE button\n")
	progress.CurrentStep += 1

	// Wait for switch to startup
	var output []byte
	var parsedOutput string
	for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))
		resetLogger.Debugf("\n=============================================\nFROM DEVICE: %s\n", parsedOutput)
		consoleOutput = append(consoleOutput, output)
		resetLogger.Debugf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
			strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
			strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
			strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
			strings.Contains(parsedOutput, RECOVERY_PROMPT))
		resetLogger.Debugf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED)
		//common.WriteLine(port, "\r", debug)
	}

	common.OutputInfo("Release the mode button now\n")
	// Assumption being made: we are being ran as a CLI app rather than the web gui
	// Allow the user to have time to release the button
	if updateChan == nil {
		common.OutputInfo("Press enter once you've released it")
		_, err := fmt.Scanln()
		if err != nil {
			resetLogger.Fatalf("Error while processing entered string: %s\n", err)
		}
	}

	//err = port.SetReadTimeout(5 * time.Second)
	//if err != nil {
	//	resetLogger.Fatal(err)
	//}

	// Ensure we have one of the test cases in the buffer
	common.OutputInfo("Checking to see if password recovery is enabled\n")
	for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
		strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		for i := 0; i < 5; i++ {
			common.WriteLine(port, "\r", debug)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))
	}

	// Test to see what we triggered on.
	// Password recovery was disabled
	if strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) {
		common.OutputInfo("Password recovery was disabled\n")

		// We can't back up the config if password recovery is disabled
		if backup.Backup {
			common.OutputInfo("Backing up the config is impossible as password recovery is disabled.\n")

			// Assumption being made: we're being ran from the CLI rather than the web gui, so prompt if we want to continue
			if updateChan == nil {
				common.OutputInfo("Would you like to continue? (y/N)\n")
				var userInput string
				_, err := fmt.Scanln(&userInput)
				switch {
				case err != nil:
					resetLogger.Fatalf("Switch not reset\nError while processing input: %s\n", err)
				case strings.ToLower(userInput) == "n" || strings.ToLower(userInput) == "no" || userInput == "":
					common.OutputInfo("Not resetting\n")
					break
				case strings.ToLower(userInput) == "y" || strings.ToLower(userInput) == "yes":
					common.OutputInfo("Continuing with reset.\n")
					backup.Backup = false
					break
				}
				// Assumption being made, we are being run from the web gui.
				// We don't have prompts going (yet), so defaulting to continuing
			} else {
				common.OutputInfo("Continuing with reset.\n")
				backup.Backup = false
			}
		}
		progress.TotalSteps = 4
		progress.CurrentStep += 1
		for !(strings.Contains(parsedOutput, YES_NO_PROMPT)) {
			common.WriteLine(port, "", debug)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
		}

		common.WriteLine(port, "", debug)

		for !(strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT)) {
			common.WriteLine(port, "", debug)
			time.Sleep(1 * time.Second)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
		}
		common.WriteLine(port, "boot", debug)
		_, err = common.ReadLines(port, BUFFER_SIZE, 10, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading lines: %s\n", err)
		}

		// Password recovery was enabled
	} else if strings.Contains(parsedOutput, RECOVERY_PROMPT) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) {
		common.OutputInfo("Password recovery was enabled\n")
		progress.CurrentStep += 1
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			resetLogger.Debugf(fmt.Sprintf("DEBUG: %s\n", common.TrimNull(output)))
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			common.WriteLine(port, "", debug)
			consoleOutput = append(consoleOutput, output)
		}
		resetLogger.Debugf("DEBUG: %s\n", common.TrimNull(output))

		// Initialize Flash
		common.OutputInfo("Entered recovery console, now initializing flash\n")
		progress.CurrentStep += 1
		common.WriteLine(port, "flash_init", debug)
		//time.Sleep(5 * time.Second)
		err = port.SetReadTimeout(1 * time.Second)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while setting the read timeout: %s\n", err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)

		// Loop until it stops getting butchered
		for strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), "unknown cmd: ") ||
			!strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), fmt.Sprintf("%s flash_init", RECOVERY_PROMPT)) {
			common.WriteLine(port, "flash_init", debug)
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			common.WriteLine(port, "", debug)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		// Make sure there's nothing extra left in the buffer
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), RECOVERY_PROMPT) {
			output, err = common.ReadLine(port, 500, debug)
			if errors.Is(err, io.ErrNoProgress) {
				resetLogger.Debugf("INPUT: %s\n", "\\n\\n\\n")
				common.WriteLine(port, "", debug)
				common.WriteLine(port, "", debug)
				common.WriteLine(port, "", debug)
			} else if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			} else {
				resetLogger.Debugf("OUTPUT: %s\n", common.TrimNull(output))
				consoleOutput = append(consoleOutput, output)
			}
		}

		// Clear out buffer
		for !errors.Is(err, io.ErrNoProgress) {
			output, err = common.ReadLine(port, 500, debug)
			if errors.Is(err, io.ErrNoProgress) {
				break
			} else if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			} else {
				resetLogger.Debugf("OUTPUT: %s\n", common.TrimNull(output))
				consoleOutput = append(consoleOutput, output)
			}
		}

		// Get files
		common.OutputInfo("Flash has been initialized, now listing directory\n")
		progress.CurrentStep += 1
		listing := make([][]byte, 1)
		common.WriteLine(port, "dir flash:", debug)
		line, err := common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		//
		//common.WriteLine(port, "dir flash:", debug)
		//line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		//if err != nil {
		//	resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		//}

		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, line)
		listing = append(listing, line)
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))), RECOVERY_PROMPT) {
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			listing = append(listing, line)
			consoleOutput = append(consoleOutput, line)
			common.WriteLine(port, "\r", debug)
		}

		// Determine the files we need to delete
		// TODO: Debug this section
		if backup.Backup {
			common.OutputInfo("Parsing files to move...\n")
		} else {
			common.OutputInfo("Parsing files to delete...\n")
		}
		progress.CurrentStep += 1
		files = ParseFilesToDelete(listing, debug)

		common.WaitForSubstring(port, RECOVERY_PROMPT, debug)

		//err = port.SetReadTimeout(1 * time.Second)
		//if err != nil {
		//	resetLogger.Fatal(err)
		//}

		// Delete files if necessary
		if len(files) == 0 {
			common.OutputInfo("Switch has been reset already.\n")
			progress.TotalSteps -= 1
			progress.CurrentStep += 1
		} else {
			// Clear buffer
			for !errors.Is(err, io.ErrNoProgress) {
				output, err = common.ReadLine(port, 500, debug)
				if errors.Is(err, io.ErrNoProgress) {

					break
				} else if err != nil && !errors.Is(err, io.ErrNoProgress) {
					resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
				} else if strings.Contains(strings.ToLower(strings.TrimSpace(string(output))), "-- more --") {
					resetLogger.Debugf("OUTPUT: %s\n", common.TrimNull(output))
					resetLogger.Debugf("INPUT: %s\n", "\\n")
					common.WriteLine(port, "", debug)
					consoleOutput = append(consoleOutput, output)
				} else {
					resetLogger.Debugf("OUTPUT: %s\n", common.TrimNull(output))
					consoleOutput = append(consoleOutput, output)
				}
			}

			if backup.Backup {
				common.OutputInfo("Moving files\n")
				progress.CurrentStep += 1
				for _, file := range files {
					common.OutputInfo(fmt.Sprintf("Moving file %s to %s-%s\n", strings.TrimSpace(file), backup.Prefix, file))
					common.WriteLine(port, fmt.Sprintf("rename flash:%s flash:%s-%s", strings.TrimSpace(file), backup.Prefix, strings.TrimSpace(file)), debug)
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					resetLogger.Debugf("rename flash:%s flash:%s-%s\n", strings.TrimSpace(file), backup.Prefix, strings.TrimSpace(file))
					resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
				}
			} else {
				common.OutputInfo("Deleting files\n")
				progress.CurrentStep += 1
				for _, file := range files {
					common.OutputInfo(fmt.Sprintf("Deleting %s\n", strings.TrimSpace(file)))
					resetLogger.Debugf("INPUT: %s%s\n", "del flash:", strings.TrimSpace(file))
					common.WriteLine(port, fmt.Sprintf("del flash:%s", strings.TrimSpace(file)), debug)
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
					}
					consoleOutput = append(consoleOutput, output)
					resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(line))))

					time.Sleep(250 * time.Millisecond)

					resetLogger.Debugf("DEBUG: Confirming deletion\n")
					resetLogger.Debugf("INPUT: %s\n", "y")
					common.WriteLine(port, "y", debug)
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
					}
					resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(line))))
					consoleOutput = append(consoleOutput, output)

					time.Sleep(250 * time.Millisecond)
				}
			}
			common.OutputInfo("Switch has been reset\n")
			progress.CurrentStep += 1
		}

		common.OutputInfo("Restarting the switch\n")
		progress.CurrentStep += 1
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if errors.Is(err, io.ErrNoProgress) {
				common.WriteLine(port, "", debug)
			} else if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		common.WriteLine(port, "reset", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)

		time.Sleep(100 * time.Millisecond)

		common.WriteLine(port, "y", debug)
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			resetLogger.Fatalf("switches.Reset: Error while reading lines: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)
	}
	progress.CurrentStep += 1
	common.OutputInfo("Successfully reset!\n")
	if backup.Backup {
		//err = port.SetReadTimeout(serial.NoTimeout)
		//if err != nil {
		//	resetLogger.Fatal(err)
		//}
		if backup.Destination != "" && ((backup.Source == "" && backup.SubnetMask == "") || (backup.Source != "" && backup.SubnetMask != "")) {
			closeTftpServer := make(chan bool)

			// Spin up TFTP server
			if backup.UseBuiltIn {
				go common.BuiltInTftpServer(closeTftpServer)
			}

			// Wait for the switch to start up
			common.OutputInfo("Waiting for switch to start up to back up config\n")

			for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(LOW_PRIV_PREFIX)) {
				resetLogger.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
				resetLogger.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
				resetLogger.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
				if common.IsEmpty(output) {
					resetLogger.Debugf("TO DEVICE: %s\n", "\\r\\n")
					_, err = port.Write([]byte("\r\n"))
					if err != nil {
						resetLogger.Fatalf("switches.Reset: Error while writing bytes to port: %s\n", err)
					}
				}
				if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(INITIAL_CONFIG_PROMPT)) {
					//err = port.SetReadTimeout(1 * time.Second)
					//if err != nil {
					//	resetLogger.Fatalf("Error occurred while changing port timeout to back up config: %s\n", err)
					//}

					resetLogger.Debugf("TO DEVICE: %s\n", "no")
					common.OutputInfo("Getting out of initial configuration dialog\n")
					progress.CurrentStep += 1
					_, err = port.Write(common.FormatCommand("no"))
					if err != nil {
						resetLogger.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
					}
				}
				output, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if err != nil {
					resetLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
				}
			}
			common.OutputInfo("Getting out of initial configuration dialog\n")
			common.OutputInfo("We have booted up now\n")
			progress.CurrentStep += 1
			_, err = port.Write(common.FormatCommand(""))
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
			}
			line, err := common.ReadLine(port, BUFFER_SIZE, debug)

			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			resetLogger.Debugf("INPUT: %s\n", "enable")
			common.OutputInfo("Entering privileged exec.\n")
			_, err = port.Write(common.FormatCommand("enable"))
			if err != nil {
				resetLogger.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)

			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			// Assign IP address
			common.OutputInfo("Assigning vlan 1 an IP address")
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "conf t"))
			_, err = port.Write(common.FormatCommand("conf t"))
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "inter vlan 1"))
			_, err = port.Write(common.FormatCommand("inter vlan 1"))
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			// Make an educated guess if we should be using DHCP
			if backup.Source == "" {
				common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "ip address dhcp"))
				_, err = port.Write(common.FormatCommand("ip address dhcp"))
				if err != nil {
					resetLogger.Fatalf("switches.Reset: Error while sending DHCP to port: %s\n", err)
				}
			} else {
				common.OutputInfo(fmt.Sprintf("INPUT: ip address %s %s\n", backup.Source, backup.SubnetMask))
				_, err = port.Write(common.FormatCommand(fmt.Sprintf("ip address %s %s", backup.Source, backup.SubnetMask)))
				if err != nil {
					resetLogger.Fatal(err)
				}
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			common.OutputInfo(fmt.Sprintf("INPUT: %s\n", "end"))
			_, err = port.Write(common.FormatCommand("end"))
			if err != nil {
				resetLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			resetLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			// Begin copying files to TFTP server
			common.OutputInfo(fmt.Sprintf("Copying %d files to %s.\n", len(files), backup.Destination))
			for _, file := range files {
				filename := fmt.Sprintf("%s-%s", backup.Prefix, file)
				common.OutputInfo(fmt.Sprintf("Backing up file %s to %s.\n", filename, backup.Destination))
				_, err = port.Write(common.FormatCommand(fmt.Sprintf("copy flash:%s tftp://%s/%s", filename, backup.Destination, filename)))
				if err != nil {
					resetLogger.Fatal(err)
				}
			}
			if backup.UseBuiltIn {
				closeTftpServer <- true
			}
		} else {
			// Inform the user of the missing information
			common.OutputInfo("Unable to back up configs to TFTP server as there are missing values\n")
			if backup.Source == "" {
				common.OutputInfo("Source address missing\n")
			}
			if backup.SubnetMask == "" {
				common.OutputInfo("Subnet mask missing\n")
			}
			if backup.Destination == "" {
				common.OutputInfo("Destination address missing\n")
			}
		}
	}

	dumpFile := os.Getenv("DumpConsoleOutput")
	if dumpFile != "" {
		file, err := os.OpenFile(dumpFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			resetLogger.Fatalf("Error while opening file %s to dump console outputs: %s\n", dumpFile, err)
		}

		defer file.Close()

		totalWritten := 0

		for _, line := range consoleOutput {
			written, err := file.Write(line)
			if err != nil {
				resetLogger.Fatalf("Error while writing %v to %s: %s\n", line, dumpFile, err)
			}
			totalWritten += written
		}

		common.OutputInfo(fmt.Sprintf("Wrote %d bytes to %s\n", totalWritten, dumpFile))
	}

	// Send clue that we're at the end
	common.OutputInfo("---EOF---")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config SwitchConfig, debug bool, updateChan chan bool) {
	LoggerName = fmt.Sprintf("SwitchDefaults%s%d%d%d", SerialPort, PortSettings.BaudRate, PortSettings.StopBits, PortSettings.DataBits)
	defaultsLogger := crglogging.New(LoggerName)

	var progress common.Progress
	progress.TotalSteps = 2
	progress.CurrentStep = 0
	progress.TotalSteps += (len(config.Lines) * 5) + 1 + (len(config.Ports) * 4) + 1 + (len(config.Vlans) * 3) + 1
	if len(config.EnablePassword) != 0 {
		progress.TotalSteps += 1
	}
	if len(config.ConsolePassword) != 0 {
		progress.TotalSteps += 1
	}
	if len(config.Banner) != 0 {
		progress.TotalSteps += 1
	}
	if len(config.DomainName) != 0 {
		progress.TotalSteps += 1
	}
	if len(config.Hostname) != 0 {
		progress.TotalSteps += 1
	}
	if config.Ssh.Enable && len(config.Ssh.Password) != 0 && len(config.Ssh.Username) != 0 && len(config.Ssh.Login) != 0 && len(config.Hostname) != 0 && len(config.DomainName) != 0 && config.Ssh.Bits != 0 {
		progress.TotalSteps += 3
	}

	if updateChan != nil {
		common.SetOutputChannel(updateChan, LoggerName)
	}

	if debug {
		defaultsLogger.SetLogLevel(5)
	} else {
		defaultsLogger.SetLogLevel(4)
	}

	hostname := "Switch"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		defaultsLogger.Fatal(err)
	}

	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			defaultsLogger.Fatal(err)
		}
	}(port)

	common.SetReaderPort(port)

	defaultsLogger.Infoln("Waiting for the switch to startup")

	// Try to guess if we've started yet
	output, err := common.ReadLine(port, BUFFER_SIZE, debug)
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		defaultsLogger.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		defaultsLogger.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		defaultsLogger.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
		common.WriteLine(port, "", debug)

		// Sometimes this'll pop up, sometimes this won't, so we can't test exclusively on this
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			defaultsLogger.Debugf(fmt.Sprintf("TO DEVICE: %s\n", "no"))
			defaultsLogger.Infof("Getting out of initial configuration dialog\n")
			progress.CurrentStep += 1
			_, err = port.Write(common.FormatCommand("no"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			defaultsLogger.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
		}
	}

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		defaultsLogger.Fatal(err)
	}

	defaultsLogger.Info("We have booted up now\n")
	progress.CurrentStep += 1
	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	line, err := common.ReadLine(port, BUFFER_SIZE, debug)

	defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
	defaultsLogger.Debugf("INPUT: %s\n", "enable")

	// Elevate our privileges so we can run practical configuration commands
	defaultsLogger.Info("Entering privileged exec.\n")
	_, err = port.Write(common.FormatCommand("enable"))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	prompt = hostname + "#"
	line, err = common.ReadLine(port, BUFFER_SIZE, debug)

	defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
	defaultsLogger.Debugf("INPUT: %s\n", "conf t")
	defaultsLogger.Info("Entering global configuration mode for the switch\n")
	progress.CurrentStep += 1
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		defaultsLogger.Fatal(err)
	}
	prompt = hostname + "(config)#"

	// Begin setting up Vlans
	if len(config.Vlans) > 0 {
		for _, vlan := range config.Vlans {
			defaultsLogger.Infof("Configuring vlan %d\n", vlan.Vlan)
			progress.CurrentStep += 1

			defaultsLogger.Debugf("INPUT: %s\n", "inter vlan "+strconv.Itoa(vlan.Vlan))
			_, err = port.Write(common.FormatCommand("inter vlan " + strconv.Itoa(vlan.Vlan)))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			prompt = hostname + "(config-if)#"

			// Assign a static IP
			// TODO: handle DHCP
			if vlan.IpAddress != "" && vlan.SubnetMask != "" {
				defaultsLogger.Infof("Assigning IP address %s with subnet mask %s to vlan %d\n", vlan.IpAddress, vlan.SubnetMask, vlan.Vlan)
				progress.CurrentStep += 1
				defaultsLogger.Debugf("INPUT: %s\n", "ip addr "+vlan.IpAddress+" "+vlan.SubnetMask)
				_, err = port.Write(common.FormatCommand("ip addr " + vlan.IpAddress + " " + vlan.SubnetMask))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			// Is this redundant?
			if vlan.Shutdown {
				defaultsLogger.Infof("Shutting down vlan %d\n", vlan.Vlan)
				progress.CurrentStep += 1
				defaultsLogger.Debugf("INPUT: %s\n", "shutdown")
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
			} else {
				defaultsLogger.Infof("Bringing up vlan %d\n", vlan.Vlan)
				progress.CurrentStep += 1
				defaultsLogger.Debugf("INPUT: %s\n", "no shutdown")
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			defaultsLogger.Infof("Finished configuring vlan %d\n", vlan.Vlan)
			defaultsLogger.Debugf("INPUT: %s\n", "exit")
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			prompt = hostname + "(config)#"
		}
		defaultsLogger.Info("Finished configuring vlans\n")
	}

	// Configure our physical ports
	if len(config.Ports) != 0 {
		for _, switchPort := range config.Ports {
			defaultsLogger.Infof("Configuring port %s\n", switchPort.Port)
			progress.CurrentStep += 1

			defaultsLogger.Debugf("INPUT: %s\n", "inter "+switchPort.Port)
			_, err = port.Write(common.FormatCommand("inter " + switchPort.Port))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			prompt = hostname + "(config-if)#"

			// Setting intended functionality
			if switchPort.SwitchportMode != "" {
				defaultsLogger.Infof("Setting the switchport mode on port %s to %s\n", switchPort.Port, switchPort.SwitchportMode)
				progress.CurrentStep += 1

				defaultsLogger.Debugf("INPUT: %s\n", "switchport mode "+switchPort.SwitchportMode)
				_, err = port.Write(common.FormatCommand("switchport mode " + switchPort.SwitchportMode))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			// Set the intended vlan
			// TODO: Possible voice vlan stuff? Should this just get pawned off to ansible?
			if switchPort.Vlan != 0 && (strings.ToLower(switchPort.SwitchportMode) == "access" || strings.ToLower(switchPort.SwitchportMode) == "trunk") {
				if strings.ToLower(switchPort.SwitchportMode) == "access" {
					defaultsLogger.Infof("Setting port %s to be an access port on vlan %d\n", switchPort.Port, switchPort.Vlan)
					progress.CurrentStep += 1
					defaultsLogger.Debugf("INPUT: %s\n", "switchport access vlan "+strconv.Itoa(switchPort.Vlan))
					_, err = port.Write(common.FormatCommand("switchport access vlan " + strconv.Itoa(switchPort.Vlan)))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
				} else if strings.ToLower(switchPort.SwitchportMode) == "trunk" {
					defaultsLogger.Infof("Setting port %s to be a trunk port with native vlan %d\n", switchPort.Port, switchPort.Vlan)
					progress.CurrentStep += 1
					defaultsLogger.Debugf("INPUT: %s\n", "switchport trunk native vlan "+strconv.Itoa(switchPort.Vlan))
					_, err = port.Write(common.FormatCommand("switchport trunk native vlan " + strconv.Itoa(switchPort.Vlan)))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
				} else {
					defaultsLogger.Infof("Switch port mode %s is not supported for static vlan assignment\n", switchPort.SwitchportMode)
					progress.CurrentStep += 1
				}
			}

			if switchPort.Shutdown {
				defaultsLogger.Infof("Shutting down port %s\n", switchPort.Port)
				progress.CurrentStep += 1
				defaultsLogger.Debugf("INPUT: %s\n", "shutdown")
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			} else {
				defaultsLogger.Infof("Bringing up port %s\n", switchPort.Port)
				progress.CurrentStep += 1
				defaultsLogger.Debugf("INPUT: %s\n", "no shutdown")
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					defaultsLogger.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
			}

			defaultsLogger.Infof("Finished configuring port %s\n", switchPort.Port)
			progress.CurrentStep += 1
			defaultsLogger.Debugf("INPUT: %s\n", "exit")
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			prompt = hostname + "(config)#"
		}
		defaultsLogger.Info("Finished configuring ports\n")
		progress.CurrentStep += 1
	}

	// Set up the banner
	if config.Banner != "" {
		defaultsLogger.Infof("Setting the banner to %s\n", config.Banner)
		progress.CurrentStep += 1
		defaultsLogger.Debugf("INPUT: %s\n", "banner motd \""+config.Banner+"\"")
		_, err = port.Write(common.FormatCommand("banner motd \"" + config.Banner + "\""))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
	}

	// Set up the console password (old templates only)
	if config.Version < 0.02 && config.ConsolePassword != "" {
		defaultsLogger.Infof("Setting the console password to %s\n", config.ConsolePassword)
		progress.CurrentStep += 1
		defaultsLogger.Debugf("INPUT: %s\n", "line console 0")
		_, err = port.Write(common.FormatCommand("line console 0"))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		prompt = hostname + "(config-line)#"

		defaultsLogger.Debugf("INPUT: %s\n", "password "+config.ConsolePassword)
		_, err = port.Write(common.FormatCommand("password " + config.ConsolePassword))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

		defaultsLogger.Info("Enabling login on the console port\n")
		progress.CurrentStep += 1
		defaultsLogger.Debugf("INPUT: %s\n", "login ")
		_, err = port.Write(common.FormatCommand("login"))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		defaultsLogger.Debugf("INPUT: %s\n", "exit")
		_, err = port.Write(common.FormatCommand("exit"))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		prompt = hostname + "(config)#"

		defaultsLogger.Info("Finished configuring the console port\n")
	}

	// Enable password, defaulting to a secret rather than plain text
	// TODO: Should plain text enable passwords be allowed? Our console passwords are plain text
	if config.EnablePassword != "" {
		defaultsLogger.Infof("Setting the privileged exec password to %s\n", config.EnablePassword)
		progress.CurrentStep += 1
		defaultsLogger.Debugf("INPUT: %s\n", "enable secret "+config.EnablePassword)
		_, err = port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		defaultsLogger.Info("Finished setting the privileged exec password\n")
	}

	// Default gateway
	// TODO: Probably redundant if/when DHCP gets set up, logically speaking could get moved up near vlan configuration
	if config.DefaultGateway != "" {
		defaultsLogger.Infof("Setting the default gateway to %s\n", config.DefaultGateway)
		defaultsLogger.Debugf("INPUT: %s\n", "ip default-gateway "+config.DefaultGateway)
		_, err = port.Write(common.FormatCommand("ip default-gateway " + config.DefaultGateway))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		defaultsLogger.Info("Finished setting the default gateway\n")
	}

	// Hostname configuration
	if config.Hostname != "" {
		defaultsLogger.Infof("Setting the hostname to %s\n", config.Hostname)
		defaultsLogger.Debugf("INPUT: %s\n", "hostname "+config.Hostname)
		_, err = port.Write(common.FormatCommand("hostname " + config.Hostname))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		hostname = config.Hostname
		prompt = hostname + "(config)#"

		defaultsLogger.Info("Finished setting the hostname.\n")
	}

	// Domain name configuration
	// TODO: Should any sort of validation be done for this? Or do we just want to make the switch responsible for this?
	if config.DomainName != "" {
		defaultsLogger.Infof("Setting the domain name of the switch to %s\n", config.DomainName)
		defaultsLogger.Debugf("INPUT: %s\n", "ip domain-name "+config.DomainName)
		_, err = port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))
		defaultsLogger.Info("Finished setting the domain name.\n")
	}

	if config.Ssh.Enable {
		allowSSH := true
		// Ensure SSH prereqs are met
		if config.Ssh.Username == "" {
			defaultsLogger.Info("WARNING: SSH username not specified.\n")
			allowSSH = false
		}
		if config.Ssh.Password == "" {
			defaultsLogger.Info("WARNING: SSH password not specified.\n")
			allowSSH = false
		}
		if config.DomainName == "" {
			defaultsLogger.Info("WARNING: Domain name not specified.\n")
			allowSSH = false
		}
		if config.Hostname == "" {
			defaultsLogger.Info("WARNING: Hostname not specified.\n")
			allowSSH = false
		}

		// Prereqs are met, so we can proceed
		if allowSSH {
			defaultsLogger.Infof("Enabling SSH with username %s and password %s\n", config.Ssh.Username, config.Ssh.Password)
			progress.CurrentStep += 1
			defaultsLogger.Debugf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password)
			_, err = port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			defaultsLogger.Debugf("INPUT: %s\n", "crypto key gen rsa")
			_, err = port.Write(common.FormatCommand("crypto key gen rsa"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			if config.Ssh.Bits > 0 && config.Ssh.Bits < 360 {
				defaultsLogger.Debugf("DEBUG: Requested bit setting of %d is too low, defaulting to 360\n", config.Ssh.Bits)
				config.Ssh.Bits = 360 // User presumably wanted minimum bit setting, 360 is minimum on IOS 12.2
			} else if config.Ssh.Bits <= 0 {
				defaultsLogger.Debugf("DEBUG: Bit setting not provided, defaulting to 512\n")
				config.Ssh.Bits = 512 // Accept default bit setting for non-provided values
			} else if config.Ssh.Bits > 2048 {
				defaultsLogger.Debugf("DEBUG: Requested bit setting of %d is too low, defaulting to 2048\n", config.Ssh.Bits)
				// User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
				// TODO: IOS 15 supports 4096 bit keys, can this get modified on the fly?
				config.Ssh.Bits = 2048
			}

			defaultsLogger.Infof("Generating an SSH key with %d bits big\n", config.Ssh.Bits)
			progress.CurrentStep += 1
			defaultsLogger.Debugf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits))
			_, err = port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line)))))

			// Previous command can take a while, so wait for the prompt
			err = port.SetReadTimeout(10 * time.Second)
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				defaultsLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
				defaultsLogger.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
				defaultsLogger.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
				defaultsLogger.Debugf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output))
				defaultsLogger.Debugf("DEBUG: Expected prompt: %s\n", strings.ToLower(prompt))
				common.WriteLine(port, "", debug)
				output, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if err != nil {
					defaultsLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
				}
			}
			err = port.SetReadTimeout(1 * time.Second)
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			defaultsLogger.Info("Finished generating the SSH key.\n")
			progress.CurrentStep += 1
		}
	}

	// Configure console lines
	if len(config.Lines) != 0 {
		for _, line := range config.Lines {
			if line.Type != "" {
				defaultsLogger.Infof("Configuring %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine)
				progress.CurrentStep += 1
				// Ensure both lines are <= 15
				if line.StartLine > 15 {
					defaultsLogger.Infof("Starting line of %d is invalid, defaulting back to 15\n", line.StartLine)
					line.StartLine = 15
				}
				if line.EndLine > 15 {
					defaultsLogger.Infof("Ending line of %d is invalid, defaulting back to 15\n", line.EndLine)
					line.EndLine = 15
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
				defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

				prompt = hostname + "(config-line)#"
				common.WaitForSubstring(port, prompt, debug)

				// Set the line password
				if line.Password != "" {
					defaultsLogger.Infof("Setting the %s lines %d to %d password to %s\n", line.Type, line.StartLine, line.EndLine, line.Password)
					progress.CurrentStep += 1
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
					defaultsLogger.Infof("Enabling login for %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine)
					progress.CurrentStep += 1
					defaultsLogger.Debugf("INPUT: %s\n", "login "+line.Login)
					_, err = port.Write(common.FormatCommand("login " + line.Login))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						defaultsLogger.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
					}
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					defaultsLogger.Infof("Setting transport input for %s lines %d to %d to %s\n", line.Type, line.StartLine, line.EndLine, line.Transport)
					progress.CurrentStep += 1
					defaultsLogger.Debugf("INPUT: %s\n", "transport input "+line.Transport)
					_, err = port.Write(common.FormatCommand("transport input " + line.Transport))
					if err != nil {
						defaultsLogger.Fatal(err)
					}
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						defaultsLogger.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
					}
					defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
				} else {
					progress.TotalSteps -= 1
				}
			}

			defaultsLogger.Infof("Finished configuring %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine)
			progress.CurrentStep += 1

			defaultsLogger.Debugf("INPUT: %s\n", "exit")
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				defaultsLogger.Fatal(err)
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				defaultsLogger.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
			}
			defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))

			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)

		}
		defaultsLogger.Info("Finished configuring console lines.\n")
		progress.CurrentStep += 1
		_, err = port.Write(common.FormatCommand("end"))
		if err != nil {
			defaultsLogger.Fatal(err)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			defaultsLogger.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		defaultsLogger.Debugf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))))
	}

	defaultsLogger.Info("Settings applied!\n")
	defaultsLogger.Info("Note: Settings have not been made persistent and will be lost upon reboot.\n")
	defaultsLogger.Info("To fix this, run `wr` on the target device.\n") // Should this be ran automatically?
	defaultsLogger.Info("---EOF---")
}
