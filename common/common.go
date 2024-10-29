package common

import (
	"context"
	"fmt"
	"github.com/pin/tftp/v3"
	"go.bug.st/serial"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type Progress struct {
	CurrentStep int
	TotalSteps  int
}

type Backup struct {
	Backup      bool
	Prefix      string
	Source      string
	SubnetMask  string
	Destination string
	UseBuiltIn  bool
}

var lineTimeout time.Duration = 10 * time.Second

func SetReadLineTimeout(t time.Duration) {
	lineTimeout = t
}

func TftpWriteHandler(filename string, wt io.WriterTo) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	recvd, err := wt.WriteTo(file)
	if err != nil {
		return err
	}

	fmt.Printf("TftpWriteHandler: Received %d bytes\n", recvd)

	return nil
}

func BuiltInTftpServer(close chan bool) {
	s := tftp.NewServer(nil, TftpWriteHandler)
	s.SetTimeout(5 * time.Second)
	err := s.ListenAndServe(":69")
	defer s.Shutdown()
	if err != nil {
		fmt.Printf("server: %v\n", err)
		os.Exit(1)
	}
	for <-close {
		return
	}
}

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
				log.Fatalf("WaitForSubstring: Error while writing new line: %s\n", err)
			}
		}
		time.Sleep(1 * time.Second)
		output = TrimNull(ReadLine(port, 500, debug))
	}
}

func FormatCommand(cmd string) []byte {
	if cmd == "" {
		cmd = "\r"
	}
	formattedString := []byte(cmd + "\n")
	return formattedString
}

func WriteLine(port serial.Port, line string, debug bool) {
	bytes, err := port.Write(FormatCommand(line))
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		fmt.Printf("TO DEVICE: sent %d bytes: %s\n", bytes, line+"\\n")
	}
}

func ReadLine(port serial.Port, buffSize int, debug bool) []byte {
	line := ReadLines(port, buffSize, 1, debug)
	if debug {
		fmt.Printf("FROM DEVICE: %s\n", line[0])
	}
	return line[0]
}

func ReadLines(port serial.Port, buffSize int, maxLines int, debug bool) [][]byte {
	output := make([][]byte, maxLines)
	if debug {
		fmt.Printf("\n======================================\nDEBUG: ")
	}
	for i := 0; i < maxLines; i++ {
		var readBytes int

		// Limit how long timer will read for
		readLineCtx, cancel := context.WithTimeout(context.Background(), lineTimeout)

		defer cancel()

		lineOutput := make([]byte, buffSize)

		go func() {
			for {
				lineSoFar := TrimNull(lineOutput)
				// Reads up to buffSize bytes, n is number of bytes read
				n, err := port.Read(lineOutput)
				if err != nil {
					log.Fatalf("WaitForSubstring: Error while reading data from port: %s\n", err)
				}
				if n == 0 {
					break
				}
				lineOutput = []byte(fmt.Sprintf("%s%s", lineSoFar, lineOutput[:n]))
				readBytes += n
				if debug {
					fmt.Printf("Output up to %d bytes: %s\n", readBytes, lineOutput[:readBytes])
				}
				if strings.Contains(string(lineOutput[readBytes-1]), "\n") || readBytes >= buffSize {
					break
				}
			}

		}()

		select {
		case <-readLineCtx.Done():
			output[i] = lineOutput[:readBytes]
		case <-time.After(lineTimeout):
			output[i] = lineOutput[:readBytes]
		}

		output[i] = lineOutput[:readBytes]
		if debug {
			fmt.Printf("DEBUG: parsed %s", output[i][:readBytes])
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
