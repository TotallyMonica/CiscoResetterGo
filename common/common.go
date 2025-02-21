package common

import (
	"bufio"
	"errors"
	"github.com/pin/tftp/v3"
	"go.bug.st/serial"
	"io"
	"main/crglogging"
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

var logger *crglogging.Crglogging
var updateChan chan bool

func SetOutputChannel(c chan bool, loggerName string) {
	updateChan = c

	logger = crglogging.GetLogger(loggerName)
	logger.NewLogTarget("WebHandler", c, false)
}

func OutputInfo(data string) {
	logger.Info(data)
	updateChan <- true
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
	tftpLogger := crglogging.GetLogger("TftpLogger")

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	recvd, err := wt.WriteTo(file)
	if err != nil {
		return err
	}

	tftpLogger.Infof("TftpWriteHandler: Received %d bytes\n", recvd)

	return nil
}

func BuiltInTftpServer(close chan bool) {
	tftpLogger := crglogging.New("tftpLogger")

	s := tftp.NewServer(nil, TftpWriteHandler)
	s.SetTimeout(5 * time.Second)
	err := s.ListenAndServe(":69")
	defer s.Shutdown()
	if err != nil {
		tftpLogger.Errorf("server: Built in TFTP server encountered an error: %v\n", err)
		return
	}
	for <-close {
		return
	}
}

func WaitForPrefix(port serial.Port, prompt string, debug bool) error {
	prefixLogger := crglogging.GetLogger("prefixLogger")
	if prefixLogger == nil {
		prefixLogger = crglogging.New("prefixLogger")
	}

	// Handle debug
	prefixLogger.SetLogLevel(4)
	if debug {
		prefixLogger.SetLogLevel(5)
	}

	var output []byte
	for !strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt) {
		prefixLogger.Debugf("Has prefix: %t\n", strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(output[:]))), prompt))
		prefixLogger.Debugf("Expected prefix: %s\n", prompt)
		prefixLogger.Debugf("FROM DEVICE: %s", strings.TrimSpace(string(output)))
		prefixLogger.Debugf("TO DEVICE: %s\n", "\\n")
		_, err := port.Write(FormatCommand(""))
		if err != nil {
			return err
		}
		output, err = ReadLine(port, 500, debug)
		if err != nil {
			return err
		}
	}
	prefixLogger.Debugf("%+v\n", output)

	return nil
}

func WaitForSubstring(port serial.Port, prompt string, debug bool) error {
	substringLogger := crglogging.GetLogger("SubstringLogger")
	if substringLogger == nil {
		substringLogger = crglogging.New("SubstringLogger")
	}

	// Handle debug
	substringLogger.SetLogLevel(4)
	if debug {
		substringLogger.SetLogLevel(5)
	}

	WriteLine(port, "", debug)
	output, err := ReadLine(port, 500, debug)
	if err != nil && !errors.Is(err, io.ErrNoProgress) {
		substringLogger.Fatalf("Error while waiting for substring: %s\n", err.Error())
	} else if errors.Is(err, io.ErrNoProgress) {
		substringLogger.Debugf("TO DEVICE: %s\n", "\\r\\n")
		WriteLine(port, "", debug)
		WriteLine(port, "", debug)
		WriteLine(port, "", debug)
	}
	for !strings.Contains(strings.ToLower(strings.TrimSpace(string(output[:]))), strings.ToLower(prompt)) {
		substringLogger.Debugf("FROM DEVICE: %s\n", output) // We don't really need all 32k bytes
		substringLogger.Debugf("FROM DEVICE: Output size: %d\n", len(strings.TrimSpace(string(output))))
		substringLogger.Debugf("FROM DEVICE: Output empty? %t\n", IsEmpty(output))
		output, err = ReadLine(port, 500, debug)
		if err != nil && !errors.Is(err, io.ErrNoProgress) {
			return err
		} else if errors.Is(err, io.ErrNoProgress) {
			substringLogger.Debugf("TO DEVICE: %s\n", "\\r\\n")
			WriteLine(port, "", debug)
			WriteLine(port, "", debug)
			WriteLine(port, "", debug)
		}
	}

	return nil
}

func FormatCommand(cmd string) []byte {
	if cmd == "" {
		cmd = "\r"
	}
	formattedString := []byte(cmd + "\n")
	return formattedString
}

func WriteLine(port serial.Port, line string, debug bool) error {
	writeLineLogger := crglogging.GetLogger("WriteLineLogger")
	if writeLineLogger == nil {
		writeLineLogger = crglogging.New("WriteLineLogger")
	}

	// Handle debug
	writeLineLogger.SetLogLevel(4)
	if debug {
		writeLineLogger.SetLogLevel(5)
	}

	if line == "\r\n" || line == "\r" || line == "\n" || line == "" || line == "\n\r" {
		//writeLineLogger.Debugf("Note: quietly discarding command\n")
		//return
	}
	bytes, err := port.Write(FormatCommand(line))
	if err != nil {
		return err
	}
	writeLineLogger.Debugf("TO DEVICE: sent %d bytes: %s\n", bytes, line+"\\n")

	return nil
}

func ReadLine(port serial.Port, buffSize int, debug bool) ([]byte, error) {
	line, err := ReadLines(port, buffSize, 1, debug)
	if err != nil {
		return nil, err
	}
	return line[0], err
}

func ReadLines(port serial.Port, buffSize int, maxLines int, debug bool) ([][]byte, error) {
	readLinesLogger := crglogging.GetLogger("ReadLinesLogger")
	if readLinesLogger == nil {
		readLinesLogger = crglogging.New("ReadLinesLogger")
	}

	// Handle debug
	readLinesLogger.SetLogLevel(4)
	if debug {
		readLinesLogger.SetLogLevel(5)
	}

	output := make([][]byte, maxLines)
	readLinesLogger.Debugf("\n======================================\nDEBUG: \n")
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

		readLinesLogger.Debugf("DEBUG: parsed %s\n", output[i])
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
	syslogParserLogger := crglogging.GetLogger("SyslogParserLogger")
	if syslogParserLogger == nil {
		syslogParserLogger = crglogging.New("SyslogParserLogger")
	}

	// Handle debug
	syslogParserLogger.SetLogLevel(4)
	if debug {
		syslogParserLogger.SetLogLevel(5)
	}

	compile, err := regexp.Compile(`\w{3}\s((\s\d|\d{2})\s)((\s\d|\d{2}):){2}\d{2}\.\d{3}:\s%(\w|-)*:\s.*`)
	if err != nil {
		syslogParserLogger.Fatalf("common.IsSyslog: Could not compile Syslog regexp: %s\n", err)
	}

	return compile.MatchString(output)
}
