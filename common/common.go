// /home/krylon/go/src/github.com/blicero/badnews/common/common.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-04 18:34:40 krylon>

package common

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/logdomain"
	"github.com/google/uuid"
	"github.com/hashicorp/logutils"
)

//go:generate ./build_time_stamp.pl

// Debug indicates whether to emit additional log messages and perform
// additional sanity checks.
// Version is the version number to display.
// AppName is the name of the application.
// TimestampFormat is the format string used to render datetime values.
// HeartBeat is the interval for worker goroutines to wake up and check
// their status.
const (
	Debug                    = true
	Version                  = "0.11.0"
	AppName                  = "BadNews"
	TimestampFormat          = "2006-01-02 15:04:05"
	TimestampFormatMinute    = "2006-01-02 15:04"
	TimestampFormatSubSecond = "2006-01-02 15:04:05.0000 MST"
	TimestampFormatDate      = "2006-01-02"
	TimestampFormatForm      = "2006-01-02T15:04:05"
	HeartBeat                = time.Millisecond * 500
	RCTimeout                = time.Millisecond * 10
	Port                     = 8086
	WorkerCntReader          = 4
)

// LogLevels are the names of the log levels supported by the logger.
var LogLevels = []logutils.LogLevel{
	"TRACE",
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"CRITICAL",
	"CANTHAPPEN",
	"SILENT",
}

// PackageLevels defines minimum log levels per package.
var PackageLevels = make(map[logdomain.ID]logutils.LogLevel, len(LogLevels))

// MinLogLevel is the minimum log level
const MinLogLevel = "TRACE"

// SuffixPattern is a regular expression that matches the suffix of a file name.
// For "text.txt", it should match ".txt" and capture "txt".
var SuffixPattern = regexp.MustCompile("([.][^.]+)$")

func init() {
	for _, id := range logdomain.AllDomains() {
		PackageLevels[id] = MinLogLevel
	}
} // func init()

func SetLogLevel(lvl logutils.LogLevel) {
	if slices.Index(LogLevels, lvl) == -1 {
		fmt.Fprintf(
			os.Stderr,
			"Invalid loglevel: %s\n", lvl)
		return
	}

	for _, id := range logdomain.AllDomains() {
		PackageLevels[id] = lvl
	}
} // func SetLogLevel(lvl string)

// Path looks up the given path.Path and returns the full path of the file or directory.
func Path(p path.Path) string {
	switch p {
	case path.Base:
		return BaseDir
	case path.Log:
		return filepath.Join(
			BaseDir,
			fmt.Sprintf("%s.log", strings.ToLower(AppName)))
	case path.Database:
		return filepath.Join(
			BaseDir,
			fmt.Sprintf("%s.db", strings.ToLower(AppName)))
	case path.AgentConfig:
		return filepath.Join(
			BaseDir,
			"agent.json")
	case path.SessionStore:
		return filepath.Join(
			BaseDir,
			"sessions.dat")
	case path.Cookiejar:
		return filepath.Join(
			BaseDir,
			"cookiejar.dat")
	case path.Judge:
		return filepath.Join(
			BaseDir,
			"judge")
	case path.JudgeCache:
		return filepath.Join(
			BaseDir,
			"jcache.db")
	case path.Advisor:
		return filepath.Join(
			BaseDir,
			"advisor",
		)
	case path.AdviceCache:
		return filepath.Join(
			BaseDir,
			"advice_cache",
		)
	case path.Blacklist:
		return filepath.Join(
			BaseDir,
			"blacklist.json",
		)
	default:
		panic(fmt.Sprintf("Invalid Path value: %s", p))
	}
} // func Path(p path.Path) string

// BaseDir is the folder where all application-specific files (database,
// log files, etc) are stored.
// LogPath is the file to the log path.
// DbPath is the path of the main database.
// HostCachePath is the path to the IP cache.
// XfrDbgPath is the path of the folder where data on DNS zone transfers
// are stored.
var (
	BaseDir = filepath.Join(os.Getenv("HOME"), fmt.Sprintf(".%s.d", strings.ToLower(AppName)))
	// LogPath          = filepath.Join(BaseDir, fmt.Sprintf("%s.log", strings.ToLower(AppName)))
	// DbPath           = filepath.Join(BaseDir, fmt.Sprintf("%s.db", strings.ToLower(AppName)))
	// AgentConfPath    = filepath.Join(BaseDir, "agent.json")
	// SessionStorePath = filepath.Join(BaseDir, "sessions.dat")
)

// SetBaseDir sets the BaseDir and related variables.
func SetBaseDir(base string) error {
	fmt.Printf("Setting BASE_DIR to %s\n", base)

	BaseDir = base
	// LogPath = filepath.Join(BaseDir, fmt.Sprintf("%s.log", strings.ToLower(AppName)))
	// DbPath = filepath.Join(BaseDir, fmt.Sprintf("%s.db", strings.ToLower(AppName)))
	// AgentConfPath = filepath.Join(BaseDir, "agent.json")

	if err := InitApp(); err != nil {
		fmt.Printf("Error initializing application environment: %s\n", err.Error())
		return err
	}

	return nil
} // func SetBaseDir(path string)

// GetLogger Tries to create a named logger instance and return it.
// If the directory to hold the log file does not exist, try to create it.
func GetLogger(dom logdomain.ID) (*log.Logger, error) {
	var err error
	err = InitApp()
	if err != nil {
		return nil, fmt.Errorf("Error initializing application environment: %s", err.Error())
	}

	logName := fmt.Sprintf("%s.%s ",
		AppName,
		dom)

	var logfile *os.File
	logfile, err = os.OpenFile(
		Path(path.Log),
		os.O_RDWR|os.O_APPEND|os.O_CREATE,
		0644)
	if err != nil {
		msg := fmt.Sprintf("Error opening log file: %s\n", err.Error())
		fmt.Println(msg)
		return nil, errors.New(msg)
	}

	writer := io.MultiWriter(os.Stdout, logfile)

	fwriter := &logutils.LevelFilter{
		Levels:   LogLevels,
		MinLevel: PackageLevels[dom],
		Writer:   writer,
	}

	logger := log.New(fwriter, logName, log.Ldate|log.Ltime|log.Lshortfile)
	return logger, nil
} // func GetLogger(name string) (*log.logger, error)

// InitApp performs some basic preparations for the application to run.
// Currently, this means creating the BASE_DIR folder.
func InitApp() error {
	err := os.Mkdir(BaseDir, 0755)
	if err != nil {
		if !os.IsExist(err) {
			msg := fmt.Sprintf("Error creating BASE_DIR %s: %s", BaseDir, err.Error())
			return errors.New(msg)
		}
	} else if err = os.Mkdir(Path(path.SessionStore), 0700); err != nil && !os.IsExist(err) {
		fmt.Printf("Error creating session store %s: %s",
			Path(path.SessionStore),
			err.Error())
		return err
	} else if err = os.Mkdir(Path(path.Judge), 0700); err != nil && !os.IsExist(err) {
		fmt.Printf("Error creating Judge store %s: %s",
			Path(path.Judge),
			err.Error())
		return err
	} else if err = os.Mkdir(Path(path.Advisor), 0700); err != nil && !os.IsExist(err) {
		fmt.Printf("Error creating Advisor cache %s: %s",
			Path(path.Judge),
			err.Error())
		return err
	}

	return nil
} // func InitApp() error

// GetUUID returns a randomized UUID
func GetUUID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
} // func GetUUID() string

// TimeEqual returns true if the two timestamps are less than one second apart.
func TimeEqual(t1, t2 time.Time) bool {
	var delta = t1.Sub(t2)

	if delta < 0 {
		delta = -delta
	}

	return delta < time.Second
} // func TimeEqual(t1, t2 time.Time) bool

// GetChecksum computes the SHA512 checksum of the given data.
func GetChecksum(data []byte) (string, error) {
	var err error
	var hash = sha512.New()

	if _, err = hash.Write(data); err != nil {
		fmt.Fprintf( // nolint: errcheck
			os.Stderr,
			"Error computing checksum: %s\n",
			err.Error(),
		)
		return "", err
	}

	var checkSumBinary = hash.Sum(nil)
	var checkSumText = fmt.Sprintf("%x", checkSumBinary)

	return checkSumText, nil
} // func GetChecksum(data []byte) (string, error)

// Fibonacci computes the nth Fibonacci number
func Fibonacci(n int64) int64 {
	var (
		a int64 = 1
		b int64 = 1
		i int64
	)

	if n <= 2 {
		return 1
	}

	for i = 1; i < n; i++ {
		var tmp = a
		a = b
		b = a + tmp
	}

	return a
} // func Fibonacci(n int64) int64
