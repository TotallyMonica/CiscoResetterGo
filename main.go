package main

import (
	"bufio"
	"flag"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

func ReadLine(port serial.Port, buffSize int, debug bool) []byte {
	line := ReadLines(port, buffSize, 1, debug)
	return line[0]
}

func ReadLines(port serial.Port, buffSize int, maxLines int, debug bool) [][]byte {
	output := make([][]byte, maxLines)

	for i := 0; i < maxLines; i++ {
		output[i] = make([]byte, buffSize)
		for {
			// Reads up to buffSize bytes
			n, err := port.Read(output[i])
			if err != nil {
				log.Fatal(err)
			}
			if n == 0 {
				break
			}
			if debug {
				fmt.Printf("%s", output[i][:n])
			}
			if n == '\n' {
				break
			}
		}
	}
	//if debug {
	//	for _, line := range output {
	//		fmt.Printf("FROM DEVICE: %s", string(line))
	//	}
	//}

	return output
}

func WaitForPrefix(port serial.Port, prompt string, debug bool) {
	var output []byte
	if debug {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected prefix: %s\n", prompt)
			fmt.Printf("FROM DEVICE: %s", strings.TrimSpace(string(output)))
			fmt.Printf("TO DEVICE: %s\n", "\\n")
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 4096, debug))

		}
		fmt.Println(output)
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected prefix: %s\n", prompt)
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 4096, debug))

		}
	}
}

func WaitForSubstring(port serial.Port, prompt string, debug bool) {
	var output []byte
	if debug {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected substring: %s\n", prompt)
			fmt.Printf("FROM DEVICE: %s", strings.TrimSpace(string(output)))
			fmt.Printf("TO DEVICE: %s\n", "\\n")
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 4096, debug))
			time.Sleep(1 * time.Second)

		}
		fmt.Println(output)
	} else {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output[:])))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected substring: %s\n", prompt)
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 4096, debug))
			time.Sleep(1 * time.Second)

		}
	}
}

func IsEmpty(output []byte) bool {
	for _, outputByte := range output {
		if outputByte != byte(0) {
			return false
		}
	}
	return true
}

func TrimNull(bytes []byte) []byte {
	friendlyLine := make([]byte, 0)
	if !IsEmpty(bytes) {
		for _, val := range bytes {
			if val != 0x00 {
				friendlyLine = append(friendlyLine, val)
			}
		}
	}
	return friendlyLine
}

func TrimNewLines(unparsed string) string {
	friendlyLine := ""
	for _, val := range unparsed {
		if string(val) != "\r" && string(val) != "\n" {
			friendlyLine = friendlyLine + string(val)
		}
	}
	return friendlyLine
}

func RemoveNonPrintable(output []byte) []byte {
	printable := [255 - 32]byte{}
	for i := 0; i < len(printable); i++ {
		printable[i] = byte(32 + i)
	}
	printableOutput := make([]byte, 0, len(output))
	for _, outputByte := range output {
		for _, printableByte := range printable {
			if outputByte == printableByte {
				printableOutput[len(printable)-1] = outputByte
				break
			}
		}
	}

	return printableOutput
}

func FormatCommand(cmd string) []byte {
	formattedString := []byte(cmd + "\n")
	return formattedString
}

func ParseFilesToDelete(files [][]byte, debug bool) []string {
	commonPrefixes := []string{"config", "vlan"}
	filesToDelete := make([]string, 0)

	if debug {
		for _, file := range files {
			cleanLine := strings.Split(strings.TrimSpace(string(TrimNull(file))), " ")
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
			cleanLine := strings.Split(strings.TrimSpace(string(TrimNull(file))), " ")
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

func SetupSerial() (string, serial.Mode) {
	var userInput string
	var chosenPort string
	isValid := false
	for !isValid {
		ports, err := enumerator.GetDetailedPortsList()
		if err != nil {
			log.Fatal(err)
		}
		if len(ports) == 0 {
			log.Fatal("No serial ports found!")
		}
		for _, port := range ports {
			fmt.Printf("Found port %v\n", port.Name)
			fmt.Printf("\tDescription:\t%s\n", port.Product)
			if port.IsUSB {
				fmt.Printf("\tUSB ID\t\t%s:%s\n", port.VID, port.PID)
				fmt.Printf("\tUSB Serial\t%s\n", port.SerialNumber)
			}
		}

		fmt.Printf("Select a serial port ")
		fmt.Scanln(&userInput)

		for _, port := range ports {
			if strings.ToUpper(userInput) == strings.ToUpper(port.Name) {
				isValid = true
				chosenPort = userInput
			}
		}
	}

	fmt.Println("Default settings are 9600 8N1. Would you like to change these? (y/N)")
	fmt.Scanln(&userInput)

	settings := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	if strings.ToLower(userInput) == "y" {
		var baudRate int
		var dataBits int
		var parityBitInput int
		var stopBitsInput float64
		stopBits := serial.OneStopBit
		parityBit := serial.NoParity

		fmt.Println("Default baud rate is 9600.")
		fmt.Printf("Enter the desired baud rate (Empty for defaults): ")
		fmt.Scanf("%d\n", &baudRate)
		if baudRate == 0 {
			baudRate = 9600
		}

		fmt.Println("Default data bits is 8.")
		fmt.Printf("Enter the desired data bits (Empty for defaults): ")
		fmt.Scanf("%d\n", &dataBits)
		if dataBits == 0 {
			dataBits = 8
		}

		fmt.Println("Default setting for parity bits is none.")
		fmt.Println("Valid options are (1) None, (2) Even, (3) Odd, (4) Mark, or (5) Space.")
		fmt.Printf("Enter the desired parity bits (Empty for defaults): ")
		fmt.Scanf("%d\n", &parityBitInput)
		switch parityBitInput {
		case 1:
		case 0:
			parityBit = serial.NoParity
			break
		case 2:
			parityBit = serial.EvenParity
			break
		case 3:
			parityBit = serial.OddParity
			break
		case 4:
			parityBit = serial.MarkParity
			break
		case 5:
			parityBit = serial.SpaceParity
			break
		default:
			log.Fatal("Invalid parity bit value provided")
		}

		fmt.Println("Default value for stop bits is 1")
		fmt.Println("Valid values for stop bits are 1, 1.5, or 2 stop bits.")
		fmt.Printf("Enter the desired stop bits (Empty for defaults): ")
		fmt.Scanf("%f\n", &stopBitsInput)

		switch stopBitsInput {
		case 0.0:
		case 1.0:
			stopBits = serial.OneStopBit
			break
		case 1.5:
			stopBits = serial.OnePointFiveStopBits
			break
		case 2.0:
			stopBits = serial.TwoStopBits
			break
		default:
			log.Fatal("Invalid stop bits value provided")
		}

		settings = &serial.Mode{
			BaudRate: baudRate,
			DataBits: dataBits,
			Parity:   parityBit,
			StopBits: stopBits,
		}
	}

	return chosenPort, *settings
}

func RouterDefaults(SerialPort string, PortSettings serial.Mode, debug bool) {
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
			output = TrimNull(ReadLine(port, BUFFER_SIZE, debug))
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
			output = TrimNull(ReadLine(port, BUFFER_SIZE, debug))
			time.Sleep(1 * time.Second)
		}
	}

	// In ROMMON
	fmt.Println("We've entered ROMMON, setting the register to 0x2142.")
	commands := []string{"confreg " + RECOVERY_REGISTER, "reset"}

	// TODO: Ensure we're actually at the prompt instead of just assuming
	for _, cmd := range commands {
		fmt.Printf("TO DEVICE: %s\n", cmd)
		port.Write(FormatCommand(cmd))
		output = ReadLine(port, BUFFER_SIZE, debug)
		fmt.Printf("DEBUG: Sent %s to device", cmd)
	}

	// We've made it out of ROMMON
	// Set timeout (does this do anything? idk)
	port.SetReadTimeout(10 * time.Second)
	fmt.Println("We've finished with ROMMON, going back into the regular console")
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), SHELL_PROMPT) {
		fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		fmt.Printf("FROM DEVICE: Output empty? %t\n", IsEmpty(output))
		if IsEmpty(output) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n\\r\\n")
			}
			port.Write([]byte("\r\n\r\n\r\n\r\n\r\n\r\n"))
		}
		time.Sleep(1 * time.Second)
		output = TrimNull(ReadLine(port, BUFFER_SIZE*2, debug))
	}

	fmt.Println("Setting the registers back to regular")
	port.SetReadTimeout(5 * time.Second)
	// We can safely assume we're at the prompt, begin running reset commands
	commands = []string{"enable", "conf t", "config-register " + NORMAL_REGISTER, "end"}
	for _, cmd := range commands {
		if debug {
			fmt.Printf("TO DEVICE: %s\n", cmd)
		}
		port.Write(FormatCommand(cmd))
		ReadLines(port, BUFFER_SIZE, 2, debug)
	}

	// Now reset config and restart
	fmt.Println("Resetting the configuration")
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "erase nvram:")
	}
	port.Write(FormatCommand("erase nvram:"))
	ReadLines(port, BUFFER_SIZE, 2, debug)
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "\\n")
	}
	port.Write(FormatCommand(""))
	ReadLines(port, BUFFER_SIZE, 2, debug)

	fmt.Println("Reloading the router")
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "reload")
	}
	port.Write(FormatCommand("reload"))
	ReadLines(port, BUFFER_SIZE, 2, debug)

	port.Write(FormatCommand("yes"))
	if debug {
		fmt.Printf("TO DEVICE: %s\n", "yes")
	}
	ReadLines(port, BUFFER_SIZE, 2, debug)

	if debug {
		fmt.Printf("TO DEVICE: %s\n", "\\n")
	}
	port.Write(FormatCommand(""))
	ReadLines(port, BUFFER_SIZE, 2, debug)

	fmt.Println("Successfully reset!")
	PrintOutput(port)
}

func SwitchDefaults(SerialPort string, PortSettings serial.Mode, debug bool) {
	const BUFFER_SIZE = 100
	const RECOVERY_PROMPT = "switch:"
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
	fmt.Println("1. Unplug the switch")
	fmt.Println("2. Hold the MODE button on the switch.")
	fmt.Println("3. Plug the switch in while holding the button")
	fmt.Println("4. When you are told, release the MODE button")

	// Wait for switch to startup
	var output []byte
	var parsedOutput string
	if debug {
		for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY)) {
			parsedOutput = strings.ToLower(strings.TrimSpace(string(TrimNull(ReadLine(port, 500, debug)))))
			fmt.Printf("\n=============================================\nFROM DEVICE: %s\n", parsedOutput)
			fmt.Printf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
				strings.Contains(parsedOutput, RECOVERY_PROMPT))
			fmt.Printf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED)
			port.Write(FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
		fmt.Printf("DEBUG: %s\n", parsedOutput)
	} else {
		for !(strings.Contains(parsedOutput, PASSWORD_RECOVERY)) {
			parsedOutput = strings.ToLower(strings.TrimSpace(string(TrimNull(ReadLine(port, 500, debug)))))
			fmt.Printf("Has prefix: %t\n", strings.Contains(parsedOutput, PASSWORD_RECOVERY) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
				strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) ||
				strings.Contains(parsedOutput, RECOVERY_PROMPT))
			fmt.Printf("Expected substrings: %s, %s, %s, %s, or %s\n", RECOVERY_PROMPT, PASSWORD_RECOVERY, PASSWORD_RECOVERY_DISABLED, PASSWORD_RECOVERY_TRIGGERED, PASSWORD_RECOVERY_ENABLED)
			port.Write(FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Println("Release the MODE button and press Enter.")
	fmt.Scanln()

	// Ensure we have one of the test cases in the buffer
	if !(strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) ||
		strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) || strings.Contains(parsedOutput, RECOVERY_PROMPT)) {
		port.Write(FormatCommand(""))
		port.Write(FormatCommand(""))
		port.Write(FormatCommand(""))
		port.Write(FormatCommand(""))
		port.Write(FormatCommand(""))
		parsedOutput = strings.ToLower(strings.TrimSpace(string(TrimNull(ReadLine(port, 500, debug)))))

	}

	// Test to see what we triggered on.
	// Password recovery was disabled
	if strings.Contains(parsedOutput, PASSWORD_RECOVERY_DISABLED) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_TRIGGERED) {
		fmt.Println("Password recovery was disabled")
		for !(strings.Contains(parsedOutput, YES_NO_PROMPT)) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			port.Write(FormatCommand(""))
			output = ReadLine(port, 500, debug)
		}

		port.Write(FormatCommand("y"))

		for !(strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output)))), RECOVERY_PROMPT)) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			port.Write(FormatCommand(""))
			time.Sleep(1 * time.Second)
			output = ReadLine(port, 500, debug)
		}

		port.Write(FormatCommand("boot"))
		ReadLines(port, BUFFER_SIZE, 10, debug)

		// Password recovery was enabled
	} else if strings.Contains(parsedOutput, RECOVERY_PROMPT) || strings.Contains(parsedOutput, PASSWORD_RECOVERY_ENABLED) {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			output = ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", TrimNull(output))
		}

		// Initialize Flash
		fmt.Println("Entered recovery console, now initializing flash")
		port.Write(FormatCommand("flash_init"))
		output = ReadLine(port, 500, debug)
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", TrimNull(output))
			}
			port.Write(FormatCommand(""))
			time.Sleep(1 * time.Second)
			output = ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", TrimNull(output))
		}

		// Get files
		fmt.Println("Flash has been initialized, now listing directory")
		port.SetReadTimeout(15 * time.Second)
		listing := make([][]byte, 1)
		port.Write(FormatCommand("dir flash:"))
		if debug {
			fmt.Printf("TO DEVICE: %s\n", "dir flash:")
		}
		time.Sleep(5 * time.Second)
		line := ReadLine(port, 16384, debug)
		listing = append(listing, line)
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(line)))), RECOVERY_PROMPT) {
			line = ReadLine(port, 16384, debug)
			listing = append(listing, line)
			if debug {
				fmt.Printf("DEBUG: %s\n", TrimNull(line))
			}
			port.Write(FormatCommand(""))
			time.Sleep(1 * time.Second)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", TrimNull(line))
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
				port.Write(FormatCommand("del flash:" + file))
				ReadLine(port, 500, debug)
				if debug {
					fmt.Printf("DEBUG: Confirming deletion\n")
				}
				fmt.Printf("TO DEVICE: %s\n", "y")
				port.Write(FormatCommand("y"))
				ReadLine(port, 500, debug)
			}
			fmt.Println("Switch has been reset")
		}

		fmt.Println("Restarting the switch")
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output)))), RECOVERY_PROMPT) {
			if debug {
				fmt.Printf("DEBUG: %s\n", output)
			}
			output = ReadLine(port, 500, debug)
		}
		if debug {
			fmt.Printf("DEBUG: %s\n", TrimNull(output))
		}

		if debug {
			fmt.Printf("TO DEVICE: %s\n", "reset")
		}
		port.Write(FormatCommand("reset"))
		ReadLine(port, 500, debug)

		if debug {
			fmt.Printf("TO DEVICE: %s\n", "y")
		}
		port.Write(FormatCommand("y"))
		ReadLines(port, BUFFER_SIZE, 10, debug)
	}

	fmt.Println("Successfully reset! Will continue trailing the output, but ^C at any point to exit.")
	PrintOutput(port)
}

func PrintOutput(port serial.Port) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	readOps := 0

	go func() {
		<-c
		port.Close()
		return
	}()
	for true {
		fmt.Printf("%s\n", ReadLine(port, 32768, false)[:80])
		readOps++
		fmt.Println(readOps)
	}
}

func TrailOutput(SerialPort string) {
	mode := &serial.Mode{
		BaudRate: 9600,
	}

	port, err := serial.Open(SerialPort, mode)
	if err != nil {
		log.Fatal(err)
	}

	port.SetReadTimeout(10 * time.Second)

	for {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		userInput := scanner.Text()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("TO DEVICE: %s\n", userInput[:80])
		_, err = port.Write(FormatCommand(userInput))
		if err != nil {
			log.Fatal(err)
		}

		ReadLines(port, 32768, 2, true)
	}
}

func main() {
	var debug bool
	var resetRouter bool
	var resetSwitch bool
	var serialDevice string
	var portSettings serial.Mode

	flag.BoolVar(&debug, "debug", false, "Show debugging messages")
	flag.BoolVar(&resetRouter, "router", false, "Reset a router")
	flag.BoolVar(&resetSwitch, "switch", false, "Reset a switch")
	flag.Parse()

	fmt.Printf("The application was built with the Go version: %s\n", runtime.Version())

	if resetRouter || resetSwitch {
		serialDevice, portSettings = SetupSerial()
	} else {
		log.Fatal("Neither router or switch reset flags provided. Run program with -router and/or -switch")
	}

	if resetRouter {
		RouterDefaults(serialDevice, portSettings, debug)
	}
	if resetSwitch {
		SwitchDefaults(serialDevice, portSettings, debug)
	}
}
