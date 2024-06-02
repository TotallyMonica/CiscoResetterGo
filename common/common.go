package common

import (
	"fmt"
	"go.bug.st/serial"
	"log"
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
			_, err := port.Write(FormatCommand(""))
			if err != nil {
				log.Fatal(err)
			}
			output = TrimNull(ReadLine(port, 500, debug))
		}
		fmt.Println(output)
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			_, err := port.Write(FormatCommand(""))
			if err != nil {
				log.Fatal(err)
			}
			output = TrimNull(ReadLine(port, 500, debug))
		}
	}
}

func WaitForSubstring(port serial.Port, prompt string, debug bool) {
	output := TrimNull(ReadLine(port, 500, debug))
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		if debug {
			fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
			fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
			fmt.Printf("FROM DEVICE: Output empty? %t\n", IsEmpty(output))
		}
		if IsEmpty(output) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n")
			}
			_, err := port.Write([]byte("\r\n"))
			if err != nil {
				log.Fatal(err)
			}
		}
		time.Sleep(1 * time.Second)
		output = TrimNull(ReadLine(port, 500, debug))
	}
}

func FormatCommand(cmd string) []byte {
	formattedString := []byte(cmd + "\n")
	return formattedString
}

func WriteLine(port serial.Port, line string, debug bool) {
	_, err := port.Write(FormatCommand(line))
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		fmt.Printf("TO DEVICE: %s\n", line+"\\n")
	}
}

func ReadLine(port serial.Port, buffSize int, debug bool) []byte {
	line := ReadLines(port, buffSize, 1, debug)
	return line[0]
}

func ReadLines(port serial.Port, buffSize int, maxLines int, debug bool) [][]byte {
	output := make([][]byte, maxLines)
	if debug {
		fmt.Printf("\n======================================\nDEBUG: ")
	}
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

		if debug {
			fmt.Printf("DEBUG: parsed %s", TrimNull(output[i]))
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
