package routers

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"main/common"
	"strings"
	"time"
)

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
