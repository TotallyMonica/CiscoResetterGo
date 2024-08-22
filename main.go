package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
	"main/routers"
	"main/switches"
	"main/web"
	"os"
	"runtime"
	"strings"
)

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
		_, err = fmt.Scanln(&userInput)
		if err != nil {
			log.Fatal(err)
		}

		for _, port := range ports {
			if strings.ToUpper(userInput) == strings.ToUpper(port.Name) {
				isValid = true
				chosenPort = userInput
			}
		}
	}

	fmt.Println("Default settings are 9600 8N1. Would you like to change these? (y/N)")
	_, err := fmt.Scanln(&userInput)
	if err != nil {
		log.Fatal(err)
	}

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
		_, err = fmt.Scanf("%d\n", &baudRate)
		if err != nil {
			log.Fatal(err)
		}
		if baudRate == 0 {
			baudRate = 9600
		}

		fmt.Println("Default data bits is 8.")
		fmt.Printf("Enter the desired data bits (Empty for defaults): ")
		_, err = fmt.Scanf("%d\n", &dataBits)
		if err != nil {
			log.Fatal(err)
		}
		if dataBits == 0 {
			dataBits = 8
		}

		fmt.Println("Default setting for parity bits is none.")
		fmt.Println("Valid options are (1) None, (2) Even, (3) Odd, (4) Mark, or (5) Space.")
		fmt.Printf("Enter the desired parity bits (Empty for defaults): ")
		_, err = fmt.Scanf("%d\n", &parityBitInput)
		if err != nil {
			log.Fatal(err)
		}
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
		_, err = fmt.Scanf("%f\n", &stopBitsInput)
		if err != nil {
			log.Fatal(err)
		}

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

func main() {
	var debug bool
	var resetRouter bool
	var resetSwitch bool
	var serialDevice string
	var switchDefaults string
	var skipReset bool
	var webServer bool
	var portSettings serial.Mode

	flag.BoolVar(&debug, "debug", false, "Show debugging messages")
	flag.BoolVar(&resetRouter, "router", false, "Reset a router")
	flag.BoolVar(&resetSwitch, "switch", false, "Reset a switch")
	flag.StringVar(&switchDefaults, "switch-defaults", "", "Set default settings on a switch")
	flag.BoolVar(&skipReset, "skip-reset", false, "Skip resetting devices")
	flag.BoolVar(&webServer, "web-server", false, "Use the web server")
	flag.Parse()

	fmt.Printf("The application was built with the Go version: %s\n", runtime.Version())

	if webServer {
		web.ServeWeb()
	}

	if resetRouter || resetSwitch {
		serialDevice, portSettings = SetupSerial()
	} else {
		log.Fatal("Neither router or switch reset flags provided. Run program with -router and/or -switch")
	}

	if resetRouter && !skipReset {
		routers.Reset(serialDevice, portSettings, debug, nil)
	}
	if resetSwitch && !skipReset {
		switches.Reset(serialDevice, portSettings, debug, nil)
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
		switches.Defaults(serialDevice, portSettings, defaults, debug, nil)
	} else {
		fmt.Println("File path not provided, not setting defaults on switch")
	}
}
