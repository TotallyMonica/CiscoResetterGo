package common

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"strings"
	"time"
)

func WaitForSubstring(port serial.Port, prompt string, debug bool) {
	output := TrimNull(ReadLine(port, 500, debug))
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		fmt.Printf("FROM DEVICE: Output empty? %t\n", IsEmpty(output))
		if IsEmpty(output) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n")
			}
			port.Write([]byte("\r\n"))
		}
		time.Sleep(1 * time.Second)
		output = TrimNull(ReadLine(port, 500, debug))
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
