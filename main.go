package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
	"main/common"
	"main/routers"
	"main/switches"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

func WaitForPrefix(port serial.Port, prompt string, debug bool) {
	var output []byte
	if debug {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected prefix: %s\n", prompt)
			fmt.Printf("FROM DEVICE: %s", strings.TrimSpace(string(output)))
			fmt.Printf("TO DEVICE: %s\n", "\\n")
			port.Write(common.FormatCommand(""))
			output = common.TrimNull(common.ReadLine(port, 4096, debug))

		}
		fmt.Println(output)
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected prefix: %s\n", prompt)
			port.Write(common.FormatCommand(""))
			output = common.TrimNull(common.ReadLine(port, 4096, debug))
		}
	}
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
		fmt.Printf("%s\n", common.ReadLine(port, 32768, false)[:80])
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
		_, err = port.Write(common.FormatCommand(userInput))
		if err != nil {
			log.Fatal(err)
		}

		common.ReadLines(port, 32768, 2, true)
	}
}

func main() {
	var debug bool
	var resetRouter bool
	var resetSwitch bool
	var serialDevice string
	var switchDefaults string
	var skipReset bool
	var portSettings serial.Mode

	flag.BoolVar(&debug, "debug", false, "Show debugging messages")
	flag.BoolVar(&resetRouter, "router", false, "Reset a router")
	flag.BoolVar(&resetSwitch, "switch", false, "Reset a switch")
	flag.StringVar(&switchDefaults, "switch-defaults", "", "Set default settings on a switch")
	flag.BoolVar(&skipReset, "skip-reset", false, "Skip resetting devices")
	flag.Parse()

	fmt.Printf("The application was built with the Go version: %s\n", runtime.Version())

	if resetRouter || resetSwitch {
		serialDevice, portSettings = SetupSerial()
	} else {
		log.Fatal("Neither router or switch reset flags provided. Run program with -router and/or -switch")
	}

	if resetRouter && !skipReset {
		routers.Reset(serialDevice, portSettings, debug)
	}
	if resetSwitch && !skipReset {
		switches.Reset(serialDevice, portSettings, debug)
	}

	//if resetRouter && setDefaults {
	//	routers.Defaults(serialDevice, portSettings, debug)
	//}

	if resetSwitch && switchDefaults != "" {
		// Load the provided json file
		file, err := os.ReadFile(switchDefaults)
		if err != nil {
			log.Fatal(err)
		}

		// Parse the provided json
		var defaults switches.SwitchConfig
		err = json.Unmarshal(file, &defaults)
		if err != nil {
			log.Fatal(err)
		}
		switches.Defaults(serialDevice, portSettings, defaults, debug)
	} else {
		fmt.Println("File path not provided, not setting defaults on switch")
	}
}
