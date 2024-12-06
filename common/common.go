package common

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/pin/tftp/v3"
	"go.bug.st/serial"
	"io"
	"log"
	"os"
	"regexp"
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

var LineTimeout time.Duration = 10 * time.Second

var reader *bufio.Reader

func SetReaderPort(port io.Reader) {
	reader = bufio.NewReader(port)
}

func SetReadLineTimeout(t time.Duration) {
	LineTimeout = t
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
			output, err = ReadLine(port, 500, debug)
			if err != nil {
				log.Fatal(err)
			}
		}
		fmt.Println(output)
	} else {
		for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
			_, err := port.Write(FormatCommand(""))
			if err != nil {
				log.Fatal(err)
			}
			output, err = ReadLine(port, 500, debug)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func WaitForSubstring(port serial.Port, prompt string, debug bool) {
	output, err := ReadLine(port, 500, debug)
	if err != nil && !errors.Is(err, io.ErrNoProgress) {
		log.Fatalf("Error while waiting for substring: %s\n", err.Error())
	} else if errors.Is(err, io.ErrNoProgress) {
		if debug {
			fmt.Printf("TO DEVICE: %s\n", "\\r\\n")
		}
		WriteLine(port, "", debug)
	}
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		if debug {
			fmt.Printf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
			fmt.Printf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
			fmt.Printf("FROM DEVICE: Output empty? %t\n", IsEmpty(output))
		}
		output, err = ReadLine(port, 500, debug)
		if err != nil && !errors.Is(err, io.ErrNoProgress) {
			log.Fatalf("Error while waiting for substring: %s\n", err.Error())
		} else if errors.Is(err, io.ErrNoProgress) {
			if debug {
				fmt.Printf("TO DEVICE: %s\n", "\\r\\n")
			}
			WriteLine(port, "", debug)
		}
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
	if line == "\r\n" || line == "\r" || line == "\n" || line == "" || line == "\n\r" {
		//log.Printf("Note: quietly discarding command\n")
		//return
	}
	bytes, err := port.Write(FormatCommand(line))
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		fmt.Printf("TO DEVICE: sent %d bytes: %s\n", bytes, line+"\\n")
	}
}

func ReadLine(port serial.Port, buffSize int, debug bool) ([]byte, error) {
	line, err := ReadLines(port, buffSize, 1, debug)
	if debug {
		fmt.Printf("FROM DEVICE: %s\n", line[0])
	}
	return line[0], err
}

func ReadLines(port serial.Port, buffSize int, maxLines int, debug bool) ([][]byte, error) {
	output := make([][]byte, maxLines)
	if debug {
		fmt.Printf("\n======================================\nDEBUG: \n")
	}
	for i := 0; i < maxLines; i++ {
		//scanner := bufio.NewScanner(port)

		res, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		output[i] = []byte(res)

		//for scanner.Scan() {
		//	output[i] = append(output[i], scanner.Bytes()...)
		//	if len(output[i]) > 2 && strings.Contains(string(output[i][1:]), "\r") {
		//		break
		//	}
		//}

		if debug {
			fmt.Printf("DEBUG: parsed %s\n", output[i])
		}
	}

	return output, nil
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

func IsSyslog(output string) bool {
	compile, err := regexp.Compile(`\w{3}\s((\s\d|\d{2})\s)((\s\d|\d{2}):){2}\d{2}\.\d{3}:\s%(\w|-)*:\s.*`)
	if err != nil {
		log.Fatalf("common.IsSyslog: Could not compile Syslog regexp: %s\n", err)
	}

	return compile.MatchString(output)
}
