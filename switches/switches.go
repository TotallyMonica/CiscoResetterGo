package switches

import (
	"errors"
	"fmt"
	"go.bug.st/serial"
	"io"
	"log"
	"main/common"
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

func outputInfo(data string) {
	if redirectedOutput == nil {
		fmt.Printf(data)
	} else {
		redirectedOutput <- data
	}
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
							log.Printf("Found file %s that matches prefix %s\n", strings.ToLower(strings.TrimSpace(cleanLine[i])), prefix)
							getRidOfSpacesPlease := strings.TrimSpace(cleanLine[i])
							delimitedCleanLine := strings.Split(getRidOfSpacesPlease, "\n")
							filesToDelete = append(filesToDelete, delimitedCleanLine[0])
							outputInfo(fmt.Sprintf("DEBUG: File %s needs to be deleted (contains substring %s)\n", cleanLine[i], prefix))
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
					for i := 0; i < len(cleanLine); i++ {
						if len(cleanLine[i]) > 0 && strings.Contains(strings.ToLower(strings.TrimSpace(cleanLine[i])), prefix) {
							delimitedCleanLine := strings.Split(cleanLine[i], "\n")
							filesToDelete = append(filesToDelete, delimitedCleanLine[0])
							outputInfo(fmt.Sprintf("DEBUG: File %s needs to be deleted (contains substring %s)\n", cleanLine[i], prefix))
						}
					}
				}
			}
		}
	}

	return filesToDelete
}

func Reset(SerialPort string, PortSettings serial.Mode, backup common.Backup, debug bool, progressDest chan string) {
	var files []string
	currentTime := time.Now()
	backup.Prefix = currentTime.Format(fmt.Sprintf("%d%02d%02d_%02d%02d%02d", currentTime.Year(), currentTime.Month(),
		currentTime.Day(), currentTime.Hour(), currentTime.Minute(), currentTime.Second()))
	redirectedOutput = progressDest

	var progress common.Progress
	progress.TotalSteps = 10
	progress.CurrentStep = 0

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatalf("switches.Reset: Error while opening port: %s\n", err)
	}

	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			log.Fatalf("switches.Reset: Error while closing port: %s\n", err)
		}
	}(port)

	common.SetReaderPort(port)

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		log.Fatalf("switches.Reset: Error while setting read timeout: %s\n", err)
	}

	outputInfo("Trigger password recovery by following these steps: \n")
	outputInfo("1. Unplug the switch\n")
	outputInfo("2. Hold the MODE button on the switch.\n")
	outputInfo("3. Plug the switch in while holding the button\n")
	outputInfo("4. When you are told, release the MODE button\n")
	progress.CurrentStep += 1

	// Wait for switch to startup
	var output []byte
	var parsedOutput string
	for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))
		if debug {
			outputInfo(fmt.Sprintf("\n=============================================\nFROM DEVICE: %s\n", parsedOutput))
			consoleOutput = append(consoleOutput, output)
			outputInfo(fmt.Sprintf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
				strings.Contains(parsedOutput, RECOVERY_PROMPT)))
			outputInfo(fmt.Sprintf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED))
		}
		//common.WriteLine(port, "\r", debug)
	}

	outputInfo("Release the mode button now\n")
	// Assumption being made: we are being ran as a CLI app rather than the web gui
	// Allow the user to have time to release the button
	if progressDest == nil {
		outputInfo("Press enter once you've released it")
		_, err := fmt.Scanln()
		if err != nil {
			log.Fatalf("Error while processing entered string: %s\n", err)
		}
	}

	//err = port.SetReadTimeout(5 * time.Second)
	//if err != nil {
	//	log.Fatal(err)
	//}

	// Ensure we have one of the test cases in the buffer
	outputInfo("Checking to if password recovery is enabled\n")
	for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
		strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		for i := 0; i < 5; i++ {
			common.WriteLine(port, "\r", debug)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		parsedOutput = strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))
	}

	// Test to see what we triggered on.
	// Password recovery was disabled
	if strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) {
		outputInfo("Password recovery was disabled\n")

		// We can't back up the config if password recovery is disabled
		if backup.Backup {
			outputInfo("Backing up the config is impossible as password recovery is disabled.\n")

			// Assumption being made: we're being ran from the CLI rather than the web gui, so prompt if we want to continue
			if progressDest == nil {
				outputInfo("Would you like to continue? (y/N)\n")
				var userInput string
				_, err := fmt.Scanln(&userInput)
				switch {
				case err != nil:
					log.Fatalf("Switch not reset\nError while processing input: %s\n", err)
				case strings.ToLower(userInput) == "n" || strings.ToLower(userInput) == "no" || userInput == "":
					outputInfo("Not resetting\n")
					break
				case strings.ToLower(userInput) == "y" || strings.ToLower(userInput) == "yes":
					outputInfo("Continuing with reset.\n")
					backup.Backup = false
					break
				}
				// Assumption being made, we are being run from the web gui.
				// We don't have prompts going (yet), so defaulting to continuing
			} else {
				outputInfo("Continuing with reset.\n")
				backup.Backup = false
			}
		}
		progress.TotalSteps = 4
		progress.CurrentStep += 1
		for !(strings.Contains(parsedOutput, YES_NO_PROMPT)) {
			common.WriteLine(port, "", debug)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
		}

		common.WriteLine(port, "", debug)

		for !(strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT)) {
			common.WriteLine(port, "", debug)
			time.Sleep(1 * time.Second)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
		}
		common.WriteLine(port, "boot", debug)
		_, err = common.ReadLines(port, BUFFER_SIZE, 10, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading lines: %s\n", err)
		}

		// Password recovery was enabled
	} else if strings.Contains(parsedOutput, RECOVERY_PROMPT) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) {
		outputInfo("Password recovery was enabled\n")
		progress.CurrentStep += 1
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				outputInfo(fmt.Sprintf("DEBUG: %s\n", common.TrimNull(output)))
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			common.WriteLine(port, "", debug)
			consoleOutput = append(consoleOutput, output)
		}
		if debug {
			outputInfo(fmt.Sprintf("DEBUG: %s\n", common.TrimNull(output)))
		}

		// Initialize Flash
		outputInfo("Entered recovery console, now initializing flash\n")
		progress.CurrentStep += 1
		common.WriteLine(port, "flash_init", debug)
		//time.Sleep(5 * time.Second)
		err = port.SetReadTimeout(1 * time.Second)
		if err != nil {
			log.Fatalf("switches.Reset: Error while setting the read timeout: %s\n", err)
		}
		output, err = common.ReadLine(port, 500, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		consoleOutput = append(consoleOutput, output)

		// Loop until it stops getting butchered
		for strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), "unknown cmd: ") ||
			!strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), fmt.Sprintf("%s flash_init", RECOVERY_PROMPT)) {
			common.WriteLine(port, "flash_init", debug)
			output, err = common.ReadLine(port, 500, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			common.WriteLine(port, "", debug)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		// Make sure there's nothing extra left in the buffer
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output))), RECOVERY_PROMPT) {
			output, err = common.ReadLine(port, 500, debug)
			if errors.Is(err, io.ErrNoProgress) {
				common.WriteLine(port, "", debug)
				common.WriteLine(port, "", debug)
				common.WriteLine(port, "", debug)
			} else if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			} else {
				consoleOutput = append(consoleOutput, output)
			}
		}

		// Clear out buffer
		common.WriteLine(port, "", debug)
		_, err = common.ReadLine(port, 500, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		_, err = common.ReadLine(port, 500, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}

		// Get files
		outputInfo("Flash has been initialized, now listing directory\n")
		progress.CurrentStep += 1
		listing := make([][]byte, 1)
		common.WriteLine(port, "dir flash:", debug)
		line, err := common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		//
		//common.WriteLine(port, "dir flash:", debug)
		//line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		//if err != nil {
		//	log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		//}

		err = port.SetReadTimeout(15 * time.Second)
		if err != nil {
			log.Fatalf("switches.Reset: Error while setting the read timeout: %s\n", err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
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
			outputInfo("Parsing files to move...\n")
		} else {
			outputInfo("Parsing files to delete...\n")
		}
		progress.CurrentStep += 1
		files = ParseFilesToDelete(listing, debug)

		common.WaitForSubstring(port, RECOVERY_PROMPT, debug)

		//err = port.SetReadTimeout(1 * time.Second)
		//if err != nil {
		//	log.Fatal(err)
		//}

		// Delete files if necessary
		if len(files) == 0 {
			outputInfo("Switch has been reset already.\n")
			progress.TotalSteps -= 1
			progress.CurrentStep += 1
		} else {
			common.WaitForSubstring(port, RECOVERY_PROMPT, debug)
			if backup.Backup {
				outputInfo("Moving files\n")
				progress.CurrentStep += 1
				for _, file := range files {
					outputInfo(fmt.Sprintf("Moving file %s to %s-%s\n", strings.TrimSpace(file), backup.Prefix, file))
					common.WriteLine(port, fmt.Sprintf("rename flash:%s flash:%s-%s", strings.TrimSpace(file), backup.Prefix, strings.TrimSpace(file)), debug)
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if debug {
						outputInfo(fmt.Sprintf("rename flash:%s flash:%s-%s\n", strings.TrimSpace(file), backup.Prefix, strings.TrimSpace(file)))
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
					}
				}
			} else {
				outputInfo("Deleting files\n")
				progress.CurrentStep += 1
				for _, file := range files {
					outputInfo(fmt.Sprintf("Deleting %s\n", strings.TrimSpace(file)))
					common.WriteLine(port, fmt.Sprintf("del flash:%s", strings.TrimSpace(file)), debug)
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
					}
					consoleOutput = append(consoleOutput, output)
					if debug {
						outputInfo(fmt.Sprintf("DEBUG: Confirming deletion\n"))
					}
					common.WriteLine(port, "y", debug)
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
					}
					consoleOutput = append(consoleOutput, output)
				}
			}
			outputInfo("Switch has been reset\n")
			progress.CurrentStep += 1
		}

		outputInfo("Restarting the switch\n")
		progress.CurrentStep += 1
		for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), RECOVERY_PROMPT) {
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		for strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), fmt.Sprintf("%s %s", RECOVERY_PROMPT, "reset")) {
			common.WriteLine(port, "reset", debug)
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			consoleOutput = append(consoleOutput, output)
		}

		for strings.Contains(strings.ToLower(strings.TrimSpace(string(common.TrimNull(output)))), strings.ToLower("Are you sure you want to reset the system (y/n)?y")) {
			common.WriteLine(port, "y", debug)
			outputLines, err := common.ReadLines(port, BUFFER_SIZE, 10, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading lines: %s\n", err)
			}
			for _, output := range outputLines {
				consoleOutput = append(consoleOutput, output)
			}
		}
	}
	progress.CurrentStep += 1
	outputInfo("Successfully reset!\n")
	if backup.Backup {
		//err = port.SetReadTimeout(serial.NoTimeout)
		//if err != nil {
		//	log.Fatal(err)
		//}
		if backup.Destination != "" && ((backup.Source == "" && backup.SubnetMask == "") || (backup.Source != "" && backup.SubnetMask != "")) {
			closeTftpServer := make(chan bool)

			// Spin up TFTP server
			if backup.UseBuiltIn {
				go common.BuiltInTftpServer(closeTftpServer)
			}

			// Wait for the switch to start up
			outputInfo("Waiting for switch to start up to back up config\n")

			for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(LOW_PRIV_PREFIX)) {
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
						log.Fatalf("switches.Reset: Error while writing bytes to port: %s\n", err)
					}
				}
				if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(INITIAL_CONFIG_PROMPT)) {
					//err = port.SetReadTimeout(1 * time.Second)
					//if err != nil {
					//	log.Fatalf("Error occurred while changing port timeout to back up config: %s\n", err)
					//}

					if debug {
						outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "no"))
					}
					outputInfo("Getting out of initial configuration dialog\n")
					progress.CurrentStep += 1
					_, err = port.Write(common.FormatCommand("no"))
					if err != nil {
						log.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
					}
				}
				output, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if err != nil {
					log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
				}
			}
			outputInfo("Getting out of initial configuration dialog\n")
			outputInfo("We have booted up now\n")
			progress.CurrentStep += 1
			_, err = port.Write(common.FormatCommand(""))
			if err != nil {
				log.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
			}
			line, err := common.ReadLine(port, BUFFER_SIZE, debug)

			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
				outputInfo(fmt.Sprintf("INPUT: %s\n", "enable"))
			}
			outputInfo("Entering privileged exec.\n")
			_, err = port.Write(common.FormatCommand("enable"))
			if err != nil {
				log.Fatalf("switches.Reset: Error while writing command to port: %s\n", err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)

			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			// Assign IP address
			outputInfo("Assigning vlan 1 an IP address")
			outputInfo(fmt.Sprintf("INPUT: %s\n", "conf t"))
			_, err = port.Write(common.FormatCommand("conf t"))
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}
			outputInfo(fmt.Sprintf("INPUT: %s\n", "inter vlan 1"))
			_, err = port.Write(common.FormatCommand("inter vlan 1"))
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			// Make an educated guess if we should be using DHCP
			if backup.Source == "" {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "ip address dhcp"))
				_, err = port.Write(common.FormatCommand("ip address dhcp"))
				if err != nil {
					log.Fatalf("switches.Reset: Error while sending DHCP to port: %s\n", err)
				}
			} else {
				outputInfo(fmt.Sprintf("INPUT: ip address %s %s\n", backup.Source, backup.SubnetMask))
				_, err = port.Write(common.FormatCommand(fmt.Sprintf("ip address %s %s", backup.Source, backup.SubnetMask)))
				if err != nil {
					log.Fatal(err)
				}
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}
			outputInfo(fmt.Sprintf("INPUT: %s\n", "end"))
			_, err = port.Write(common.FormatCommand("end"))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			// Begin copying files to TFTP server
			outputInfo(fmt.Sprintf("Copying %d files to %s.\n", len(files), backup.Destination))
			for _, file := range files {
				filename := fmt.Sprintf("%s-%s", backup.Prefix, file)
				outputInfo(fmt.Sprintf("Backing up file %s to %s.\n", filename, backup.Destination))
				_, err = port.Write(common.FormatCommand(fmt.Sprintf("copy flash:%s tftp://%s/%s", filename, backup.Destination, filename)))
				if err != nil {
					log.Fatal(err)
				}
			}
			if backup.UseBuiltIn {
				closeTftpServer <- true
			}
		} else {
			// Inform the user of the missing information
			outputInfo("Unable to back up configs to TFTP server as there are missing values\n")
			if backup.Source == "" {
				outputInfo("Source address missing\n")
			}
			if backup.SubnetMask == "" {
				outputInfo("Subnet mask missing\n")
			}
			if backup.Destination == "" {
				outputInfo("Destination address missing\n")
			}
		}
	}

	dumpFile := os.Getenv("DumpConsoleOutput")
	if dumpFile != "" {
		file, err := os.OpenFile(dumpFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalf("Error while opening file %s to dump console outputs: %s\n", dumpFile, err)
		}

		defer file.Close()

		totalWritten := 0

		for _, line := range consoleOutput {
			written, err := file.Write(line)
			if err != nil {
				log.Fatalf("Error while writing %v to %s: %s\n", line, dumpFile, err)
			}
			totalWritten += written
		}

		outputInfo(fmt.Sprintf("Wrote %d bytes to %s\n", totalWritten, dumpFile))
	}

	// Send clue that we're at the end
	outputInfo("---EOF---")
}

func Defaults(SerialPort string, PortSettings serial.Mode, config SwitchConfig, debug bool, progressDest chan string) {
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

	redirectedOutput = progressDest
	hostname := "Switch"
	prompt := hostname + ">"

	port, err := serial.Open(SerialPort, &PortSettings)

	if err != nil {
		log.Fatal(err)
	}

	defer func(port serial.Port) {
		err := port.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(port)

	common.SetReaderPort(port)

	outputInfo("Waiting for the switch to startup\n")

	// Try to guess if we've started yet
	output, err := common.ReadLine(port, BUFFER_SIZE, debug)
	for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		if debug {
			outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
			outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
			outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
		}
		common.WriteLine(port, "", debug)

		// Sometimes this'll pop up, sometimes this won't, so we can't test exclusively on this
		if strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower("Would you like to enter the initial configuration dialog? [yes/no]:")) {
			if debug {
				outputInfo(fmt.Sprintf("TO DEVICE: %s\n", "no"))
			}
			outputInfo("Getting out of initial configuration dialog\n")
			progress.CurrentStep += 1
			_, err = port.Write(common.FormatCommand("no"))
			if err != nil {
				log.Fatal(err)
			}
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
		}
	}

	err = port.SetReadTimeout(1 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	outputInfo("We have booted up now\n")
	progress.CurrentStep += 1
	_, err = port.Write(common.FormatCommand(""))
	if err != nil {
		log.Fatal(err)
	}
	line, err := common.ReadLine(port, BUFFER_SIZE, debug)

	if debug {
		outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		outputInfo(fmt.Sprintf("INPUT: %s\n", "enable"))
	}

	// Elevate our privileges so we can run practical configuration commands
	outputInfo("Entering privileged exec.\n")
	_, err = port.Write(common.FormatCommand("enable"))
	if err != nil {
		log.Fatal(err)
	}
	prompt = hostname + "#"
	line, err = common.ReadLine(port, BUFFER_SIZE, debug)

	if debug {
		outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		outputInfo(fmt.Sprintf("INPUT: %s\n", "conf t"))
	}
	outputInfo("Entering global configuration mode for the switch\n")
	progress.CurrentStep += 1
	_, err = port.Write(common.FormatCommand("conf t"))
	if err != nil {
		log.Fatal(err)
	}
	prompt = hostname + "(config)#"

	// Begin setting up Vlans
	if len(config.Vlans) > 0 {
		for _, vlan := range config.Vlans {
			outputInfo(fmt.Sprintf("Configuring vlan %d\n", vlan.Vlan))
			progress.CurrentStep += 1

			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "inter vlan "+strconv.Itoa(vlan.Vlan)))
			}
			_, err = port.Write(common.FormatCommand("inter vlan " + strconv.Itoa(vlan.Vlan)))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			prompt = hostname + "(config-if)#"

			// Assign a static IP
			// TODO: handle DHCP
			if vlan.IpAddress != "" && vlan.SubnetMask != "" {
				outputInfo(fmt.Sprintf("Assigning IP address %s with subnet mask %s to vlan %d\n", vlan.IpAddress, vlan.SubnetMask, vlan.Vlan))
				progress.CurrentStep += 1
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "ip addr "+vlan.IpAddress+" "+vlan.SubnetMask))
				}
				_, err = port.Write(common.FormatCommand("ip addr " + vlan.IpAddress + " " + vlan.SubnetMask))
				if err != nil {
					log.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
				}
			}

			// Is this redundant?
			if vlan.Shutdown {
				outputInfo(fmt.Sprintf("Shutting down vlan %d\n", vlan.Vlan))
				progress.CurrentStep += 1
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "shutdown"))
				}
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					log.Fatal(err)
				}
			} else {
				outputInfo(fmt.Sprintf("Bringing up vlan %d\n", vlan.Vlan))
				progress.CurrentStep += 1
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "no shutdown"))
				}
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					log.Fatal(err)
				}
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			outputInfo(fmt.Sprintf("Finished configuring vlan %d\n", vlan.Vlan))
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
			}
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			prompt = hostname + "(config)#"
		}
		outputInfo("Finished configuring vlans\n")
	}

	// Configure our physical ports
	if len(config.Ports) != 0 {
		for _, switchPort := range config.Ports {
			outputInfo(fmt.Sprintf("Configuring port %s\n", switchPort.Port))
			progress.CurrentStep += 1

			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "inter "+switchPort.Port))
			}
			_, err = port.Write(common.FormatCommand("inter " + switchPort.Port))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}
			prompt = hostname + "(config-if)#"

			// Setting intended functionality
			if switchPort.SwitchportMode != "" {
				outputInfo(fmt.Sprintf("Setting the switchport mode on port %s to %s\n", switchPort.Port, switchPort.SwitchportMode))
				progress.CurrentStep += 1

				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "switchport mode "+switchPort.SwitchportMode))
				}
				_, err = port.Write(common.FormatCommand("switchport mode " + switchPort.SwitchportMode))
				if err != nil {
					log.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
				}
			}

			// Set the intended vlan
			// TODO: Possible voice vlan stuff? Should this just get pawned off to ansible?
			if switchPort.Vlan != 0 && (strings.ToLower(switchPort.SwitchportMode) == "access" || strings.ToLower(switchPort.SwitchportMode) == "trunk") {
				if strings.ToLower(switchPort.SwitchportMode) == "access" {
					outputInfo(fmt.Sprintf("Setting port %s to be an access port on vlan %d\n", switchPort.Port, switchPort.Vlan))
					progress.CurrentStep += 1
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "switchport access vlan "+strconv.Itoa(switchPort.Vlan)))
					}
					_, err = port.Write(common.FormatCommand("switchport access vlan " + strconv.Itoa(switchPort.Vlan)))
					if err != nil {
						log.Fatal(err)
					}
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
					}
				} else if strings.ToLower(switchPort.SwitchportMode) == "trunk" {
					outputInfo(fmt.Sprintf("Setting port %s to be a trunk port with native vlan %d\n", switchPort.Port, switchPort.Vlan))
					progress.CurrentStep += 1
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "switchport trunk native vlan "+strconv.Itoa(switchPort.Vlan)))
					}
					_, err = port.Write(common.FormatCommand("switchport trunk native vlan " + strconv.Itoa(switchPort.Vlan)))
					if err != nil {
						log.Fatal(err)
					}
					line, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
					}
				} else {
					outputInfo(fmt.Sprintf("Switch port mode %s is not supported for static vlan assignment\n", switchPort.SwitchportMode))
					progress.CurrentStep += 1
				}
			}

			if switchPort.Shutdown {
				outputInfo(fmt.Sprintf("Shutting down port %s\n", switchPort.Port))
				progress.CurrentStep += 1
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "shutdown"))
				}
				_, err = port.Write(common.FormatCommand("shutdown"))
				if err != nil {
					log.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
				}
			} else {
				outputInfo(fmt.Sprintf("Bringing up port %s\n", switchPort.Port))
				progress.CurrentStep += 1
				if debug {
					outputInfo(fmt.Sprintf("INPUT: %s\n", "no shutdown"))
				}
				_, err = port.Write(common.FormatCommand("no shutdown"))
				if err != nil {
					log.Fatal(err)
				}
				line, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
				}
			}

			outputInfo(fmt.Sprintf("Finished configuring port %s\n", switchPort.Port))
			progress.CurrentStep += 1
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
			}
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			prompt = hostname + "(config)#"
		}
		outputInfo("Finished configuring ports\n")
		progress.CurrentStep += 1
	}

	// Set up the banner
	if config.Banner != "" {
		outputInfo(fmt.Sprintf("Setting the banner to %s\n", config.Banner))
		progress.CurrentStep += 1
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "banner motd \""+config.Banner+"\""))
		}
		_, err = port.Write(common.FormatCommand("banner motd \"" + config.Banner + "\""))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
	}

	// Set up the console password (old templates only)
	if config.Version < 0.02 && config.ConsolePassword != "" {
		outputInfo(fmt.Sprintf("Setting the console password to %s\n", config.ConsolePassword))
		progress.CurrentStep += 1
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "line console 0"))
		}
		_, err = port.Write(common.FormatCommand("line console 0"))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		prompt = hostname + "(config-line)#"

		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "password "+config.ConsolePassword))
		}
		_, err = port.Write(common.FormatCommand("password " + config.ConsolePassword))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}

		outputInfo("Enabling login on the console port\n")
		progress.CurrentStep += 1
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "login "))
		}
		_, err = port.Write(common.FormatCommand("login"))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
		}
		_, err = port.Write(common.FormatCommand("exit"))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		prompt = hostname + "(config)#"

		outputInfo("Finished configuring the console port\n")
	}

	// Enable password, defaulting to a secret rather than plain text
	// TODO: Should plain text enable passwords be allowed? Our console passwords are plain text
	if config.EnablePassword != "" {
		outputInfo(fmt.Sprintf("Setting the privileged exec password to %s\n", config.EnablePassword))
		progress.CurrentStep += 1
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "enable secret "+config.EnablePassword))
		}
		_, err = port.Write(common.FormatCommand("enable secret " + config.EnablePassword))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		outputInfo("Finished setting the privileged exec password\n")
	}

	// Default gateway
	// TODO: Probably redundant if/when DHCP gets set up, logically speaking could get moved up near vlan configuration
	if config.DefaultGateway != "" {
		outputInfo(fmt.Sprintf("Setting the default gateway to %s\n", config.DefaultGateway))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "ip default-gateway "+config.DefaultGateway))
		}
		_, err = port.Write(common.FormatCommand("ip default-gateway " + config.DefaultGateway))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		outputInfo("Finished setting the default gateway\n")
	}

	// Hostname configuration
	if config.Hostname != "" {
		outputInfo(fmt.Sprintf("Setting the hostname to %s\n", config.Hostname))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "hostname "+config.Hostname))
		}
		_, err = port.Write(common.FormatCommand("hostname " + config.Hostname))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		hostname = config.Hostname
		prompt = hostname + "(config)#"

		outputInfo("Finished setting the hostname.\n")
	}

	// Domain name configuration
	// TODO: Should any sort of validation be done for this? Or do we just want to make the switch responsible for this?
	if config.DomainName != "" {
		outputInfo(fmt.Sprintf("Setting the domain name of the switch to %s\n", config.DomainName))
		if debug {
			outputInfo(fmt.Sprintf("INPUT: %s\n", "ip domain-name "+config.DomainName))
		}
		_, err = port.Write(common.FormatCommand("ip domain-name " + config.DomainName))
		if err != nil {
			log.Fatal(err)
		}
		line, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
		}
		outputInfo("Finished setting the domain name.\n")
	}

	if config.Ssh.Enable {
		allowSSH := true
		// Ensure SSH prereqs are met
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

		// Prereqs are met, so we can proceed
		if allowSSH {
			outputInfo(fmt.Sprintf("Enabling SSH with username %s and password %s\n", config.Ssh.Username, config.Ssh.Password))
			progress.CurrentStep += 1
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "username "+config.Ssh.Username+" password "+config.Ssh.Password))
			}
			_, err = port.Write(common.FormatCommand("username " + config.Ssh.Username + " password " + config.Ssh.Password))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
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
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
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
				// User presumably wanted highest allowed bit setting, 2048 is max on IOS 12.2
				// TODO: IOS 15 supports 4096 bit keys, can this get modified on the fly?
				config.Ssh.Bits = 2048
			}

			outputInfo(fmt.Sprintf("Generating an SSH key with %d bits big\n", config.Ssh.Bits))
			progress.CurrentStep += 1
			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", strconv.Itoa(config.Ssh.Bits)))
			}
			_, err = port.Write(common.FormatCommand(strconv.Itoa(config.Ssh.Bits)))
			if err != nil {
				log.Fatal(err)
			}
			line, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(line))))))
			}

			// Previous command can take a while, so wait for the prompt
			err = port.SetReadTimeout(10 * time.Second)
			if err != nil {
				log.Fatal(err)
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
			}
			for !strings.HasSuffix(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
				if debug {
					outputInfo(fmt.Sprintf("FROM DEVICE: %s\n", output)) // We don't really need all 32k bytes
					outputInfo(fmt.Sprintf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output)))))
					outputInfo(fmt.Sprintf("FROM DEVICE: Output empty? %t\n", common.IsEmpty(output)))
					outputInfo(fmt.Sprintf("DEBUG: Expected prompt: %s\n", strings.ToLower(prompt)))
				}
				common.WriteLine(port, "", debug)
				output, err = common.ReadLine(port, BUFFER_SIZE, debug)
				if err != nil {
					log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
				}
			}
			err = port.SetReadTimeout(1 * time.Second)
			if err != nil {
				log.Fatal(err)
			}
			outputInfo("Finished generating the SSH key.\n")
			progress.CurrentStep += 1
		}
	}

	// Configure console lines
	if len(config.Lines) != 0 {
		for _, line := range config.Lines {
			if line.Type != "" {
				outputInfo(fmt.Sprintf("Configuring %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine))
				progress.CurrentStep += 1
				// Ensure both lines are <= 15
				if line.StartLine > 15 {
					outputInfo(fmt.Sprintf("Starting line of %d is invalid, defaulting back to 15\n", line.StartLine))
					line.StartLine = 15
				}
				if line.EndLine > 15 {
					outputInfo(fmt.Sprintf("Ending line of %d is invalid, defaulting back to 15\n", line.EndLine))
					line.EndLine = 15
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
				if debug {
					outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
				}

				prompt = hostname + "(config-line)#"
				common.WaitForSubstring(port, prompt, debug)

				// Set the line password
				if line.Password != "" {
					outputInfo(fmt.Sprintf("Setting the %s lines %d to %d password to %s\n", line.Type, line.StartLine, line.EndLine, line.Password))
					progress.CurrentStep += 1
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
					outputInfo(fmt.Sprintf("Enabling login for %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine))
					progress.CurrentStep += 1
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "login "+line.Login))
					}
					_, err = port.Write(common.FormatCommand("login " + line.Login))
					if err != nil {
						log.Fatal(err)
					}
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						log.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
					}
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				}

				if line.Transport != "" && line.Type == "vty" { // console 0 can't use telnet or ssh
					outputInfo(fmt.Sprintf("Setting transport input for %s lines %d to %d to %s\n", line.Type, line.StartLine, line.EndLine, line.Transport))
					progress.CurrentStep += 1
					if debug {
						outputInfo(fmt.Sprintf("INPUT: %s\n", "transport input "+line.Transport))
					}
					_, err = port.Write(common.FormatCommand("transport input " + line.Transport))
					if err != nil {
						log.Fatal(err)
					}
					output, err = common.ReadLine(port, BUFFER_SIZE, debug)
					if err != nil {
						log.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
					}
					if debug {
						outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
					}
				} else {
					progress.TotalSteps -= 1
				}
			}

			outputInfo(fmt.Sprintf("Finished configuring %s lines %d to %d\n", line.Type, line.StartLine, line.EndLine))
			progress.CurrentStep += 1

			if debug {
				outputInfo(fmt.Sprintf("INPUT: %s\n", "exit"))
			}
			_, err = port.Write(common.FormatCommand("exit"))
			if err != nil {
				log.Fatal(err)
			}
			output, err = common.ReadLine(port, BUFFER_SIZE, debug)
			if err != nil {
				log.Fatalf("switches.Defaults: Error while reading line: %s\n", err)
			}
			if debug {
				outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
			}

			prompt = hostname + "(config)#"
			common.WaitForSubstring(port, prompt, debug)

		}
		outputInfo("Finished configuring console lines.\n")
		progress.CurrentStep += 1
		_, err = port.Write(common.FormatCommand("end"))
		if err != nil {
			log.Fatal(err)
		}
		output, err = common.ReadLine(port, BUFFER_SIZE, debug)
		if err != nil {
			log.Fatalf("switches.Reset: Error while reading line: %s\n", err)
		}
		if debug {
			outputInfo(fmt.Sprintf("OUTPUT: %s\n", strings.ToLower(strings.TrimSpace(string(common.TrimNull(output))))))
		}
	}

	outputInfo("Settings applied!\n")
	outputInfo("Note: Settings have not been made persistent and will be lost upon reboot.\n")
	outputInfo("To fix this, run `wr` on the target device.\n") // Should this be ran automatically?
	outputInfo("---EOF---")
}
