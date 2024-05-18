package common

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"strings"
	"time"
)

func WaitForSubstring(port serial.Port, prompt string, debug bool) {
	var output []byte
	if debug {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected substring: %s\n", prompt)
			fmt.Printf("FROM DEVICE: %s", strings.TrimSpace(string(output)))
			fmt.Printf("TO DEVICE: %s\n", "\\n")
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 500, debug))
			time.Sleep(1 * time.Second)

		}
		fmt.Println(output)
	} else {
		for !strings.Contains(strings.ToLower(strings.TrimSpace(string(TrimNull(output[:])))), prompt) {
			fmt.Printf("Has prefix: %t\n", strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
			fmt.Printf("Expected substring: %s\n", prompt)
			port.Write(FormatCommand(""))
			output = TrimNull(ReadLine(port, 500, debug))
			time.Sleep(1 * time.Second)

		}
	}
}

func FormatCommand(cmd string) []byte {
	formattedString := []byte(cmd + "\n")
	return formattedString
}

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

	return output
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

func IsEmpty(output []byte) bool {
	for _, outputByte := range output {
		if outputByte != byte(0) {
			return false
		}
	}
	return true
}
