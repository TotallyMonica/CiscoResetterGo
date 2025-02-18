package crglogging

import (
	"bufio"
	"fmt"
	"github.com/op/go-logging"
	"os"
	"regexp"
	"testing"
)

var debugRegex *regexp.Regexp
var infoRegex *regexp.Regexp
var errorRegex *regexp.Regexp
var noticeRegex *regexp.Regexp
var warningRegex *regexp.Regexp
var fatalRegex *regexp.Regexp
var criticalRegex *regexp.Regexp

func compileRegexes() error {
	var err error

	if debugRegex == nil {
		debugRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ DEBUG [0-9a-f]* \[DEBUG Sample Message`)
		if err != nil {
			return err
		}
	}

	if infoRegex == nil {
		infoRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ INFO [0-9a-f]* \[(INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	if noticeRegex == nil {
		noticeRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ NOTICE [0-9a-f]* \[\[(NOTICE|INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	if warningRegex == nil {
		warningRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ WARNING [0-9a-f]* \[\[(WARNING|NOTICE|INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	if errorRegex == nil {
		errorRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ ERROR [0-9a-f]* \[(ERROR|WARNING|NOTICE|INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	if fatalRegex == nil {
		fatalRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ FATAL [0-9a-f]* \[(FATAL|ERROR|WARNING|NOTICE|INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	if criticalRegex == nil {
		criticalRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ CRITICAL [0-9a-f]* \[(CRITICAL|FATAL|ERROR|WARNING|NOTICE|INFO|DEBUG) Sample Message`)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestLogToFile(t *testing.T) {
	err := compileRegexes()
	if err != nil {
		t.Fatalf("Failed to compile regexes: %v", err)
	}

	tmp, err := os.CreateTemp("", "CICD_Log_File-*.log")
	if err != nil {
		t.Errorf("Failed to create temporary file: %v", err)
		return
	}
	filePattern := tmp.Name()
	tmp.Close()
	os.Remove(filePattern)

	type args struct {
		filename string
		level    logging.Level
		message  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "LogDebugToFile",
		args: args{
			filename: fmt.Sprintf("DEBUG_%s", filePattern),
			level:    logging.DEBUG,
			message:  "Sample Message\n",
		},
		want: `\d+:\d+:\d+\.\d+ .* DEBUG 001 Sample Message`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New("cicd_test")
			logger.NewLogTarget("file", tt.args.filename, true)
			logger.Debug(tt.args.filename, tt.args.level, tt.args.message)

			file, err := os.Open(tt.args.filename)
			if err != nil {
				t.Fatalf("Failed to open log file: %v", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lines := make([]string, 0)

			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			if len(lines) < 1 || lines[0] != tt.want {
				t.Errorf("%s got %v, want %v", tt.name, scanner.Text(), tt.want)
			}
		})
	}
}

func TestLogToMemory(t *testing.T) {
	err := compileRegexes()
	if err != nil {
		t.Fatalf("Failed to compile regexes: %v", err)
	}

	type args struct {
		level          logging.Level
		message        string
		memChannelName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "LogDebugToMemory",
		args: args{
			level:          logging.DEBUG,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_debug",
		},
		want: debugRegex.String(),
	}, {
		name: "LogInfoToMemory",
		args: args{
			level:          logging.INFO,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_info",
		},
		want: infoRegex.String(),
	}, {
		name: "LogNoticeToMemory",
		args: args{
			level:          logging.NOTICE,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_notice",
		},
		want: noticeRegex.String(),
	}, {
		name: "LogWarningToMemory",
		args: args{
			level:          logging.WARNING,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_warning",
		},
		want: warningRegex.String(),
	}, {
		name: "LogErrorToMemory",
		args: args{
			level:          logging.ERROR,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_error",
		},
		want: errorRegex.String(),
	}, {
		name: "LogCriticalToMemory",
		args: args{
			level:          logging.CRITICAL,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_critical",
		},
		want: criticalRegex.String(),
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			placeholderChan := make(chan bool)
			lines := make([]string, 0)

			logger := New("cicd_test")
			logger.NewLogTarget(tt.args.memChannelName, placeholderChan, false)
			logger.SetLogLevel(int(tt.args.level))
			logger.Debug(tt.args.level, tt.args.message)
			logger.Info(tt.args.level, tt.args.message)
			logger.Warning(tt.args.level, tt.args.message)
			logger.Error(tt.args.level, tt.args.message)
			//logger.Fatal(tt.args.level, tt.args.message)

			memBuff, err := logger.GetMemLogContents(tt.args.memChannelName)
			if err != nil {
				t.Errorf("Failed to get mem log for test %s: %v", tt.name, err)
				return
			}

			for line := memBuff.Buff.Head(); line != nil; line = line.Next() {
				formattedLine := line.Record.Formatted(0)
				lines = append(lines, formattedLine)
			}

			if len(lines) < 0 {
				t.Errorf("%s returned emtpy, expected regex expression %s\n", tt.name, tt.want)
			}

			for _, line := range lines {
				if !(debugRegex.MatchString(line) || infoRegex.MatchString(line) || errorRegex.MatchString(line) || fatalRegex.MatchString(line) || warningRegex.MatchString(line)) {
					t.Errorf("%s failed, got %s, expected regex expression %s\n", tt.name, line, tt.want)
				}
			}
		})
	}
}
