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
var fatalRegex *regexp.Regexp
var criticalRegex *regexp.Regexp

func compileRegexes() error {
	var err error

	if debugRegex == nil {
		debugRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ DEBUG \d+ \[.* DEBUG Sample Message`)
		if err != nil {
			return err
		}
	}

	if infoRegex == nil {
		infoRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ INFO \d+ \[.* INFO Sample Message`)
		if err != nil {
			return err
		}
	}

	if noticeRegex == nil {
		noticeRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ NOTICE \d+ \[.* NOTICE Sample Message`)
		if err != nil {
			return err
		}
	}

	if errorRegex == nil {
		errorRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ ERROR \d+ \[.* ERROR Sample Message`)
		if err != nil {
			return err
		}
	}

	if fatalRegex == nil {
		fatalRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ FATAL \d+ \[.* FATAL Sample Message`)
		if err != nil {
			return err
		}
	}

	if criticalRegex == nil {
		criticalRegex, err = regexp.Compile(`\d+:\d+:\d+\.\d+ func1 ▶ CRITICAL \d+ \[.* CRITICAL Sample Message`)
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
			filename: fmt.Sprintf("/tmp/DEBUG_File_tst.log"),
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
		want: `\d+:\d+:\d+\.\d+ .* DEBUG \d+ Sample Message`,
	}, {
		name: "LogInfoToMemory",
		args: args{
			level:          logging.INFO,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_info",
		},
		want: `\d+:\d+:\d+\.\d+ .* INFO \d+ Sample Message`,
	}, {
		name: "LogNoticeToMemory",
		args: args{
			level:          logging.NOTICE,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_notice",
		},
		want: `\d+:\d+:\d+\.\d+ .* NOTICE \d+ Sample Message`,
	}, {
		name: "LogWarningToMemory",
		args: args{
			level:          logging.WARNING,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_warning",
		},
		want: `\d+:\d+:\d+\.\d+ .* WARNING \d+ Sample Message`,
	}, {
		name: "LogErrorToMemory",
		args: args{
			level:          logging.ERROR,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_error",
		},
		want: `\d+:\d+:\d+\.\d+ .* ERROR \d+ Sample Message`,
	}, {
		name: "LogCriticalToMemory",
		args: args{
			level:          logging.DEBUG,
			message:        "Sample Message\n",
			memChannelName: "cicd_mem_critical",
		},
		want: `\d+:\d+:\d+\.\d+ .* CRITICAL \d+ Sample Message`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			placeholderChan := make(chan bool)
			lines := make([]string, 0)

			logger := New("cicd_test")
			logger.NewLogTarget(tt.args.memChannelName, placeholderChan, false)
			logger.Debug(placeholderChan, tt.args.level, tt.args.message)

			memBuff, err := logger.GetMemLogContents(tt.args.memChannelName)
			if err != nil {
				t.Errorf("Failed to get mem log for test %s: %v", tt.name, err)
				return
			}

			for line := memBuff.Buff.Head(); line != nil; line = line.Next() {
				lines = append(lines, line.Record.Formatted(0))
			}

			if len(lines) < 0 {
				t.Errorf("%s returned emtpy, expected regex expression %s\n", tt.name, tt.want)
			}

			for _, line := range lines {
				if !debugRegex.MatchString(line) {
					t.Errorf("%s failed, got %s, expected regex expression %s\n", tt.name, line, tt.want)
				}
			}
		})
	}
}
