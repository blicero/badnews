// /home/krylon/go/src/github.com/blicero/badnews/reader/00_reader_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 24. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-27 17:04:04 krylon>

package reader

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blicero/badnews/common"
)

func TestMain(m *testing.M) {
	var (
		err     error
		result  int
		baseDir = time.Now().Format("/tmp/badnews_rssreader_test_20060102_150405")
	)

	if err = common.SetBaseDir(baseDir); err != nil {
		fmt.Printf("Cannot set base directory to %s: %s\n",
			baseDir,
			err.Error())
		os.Exit(1)
	} else if err = prepare(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to prepare database: %s\n",
			err.Error(),
		)
		os.Exit(1)
	} else if result = m.Run(); result == 0 {
		// If any test failed, we keep the test directory (and the
		// database inside it) around, so we can manually inspect it
		// if needed.
		// If all tests pass, OTOH, we can safely remove the directory.
		fmt.Printf("NOT Removing BaseDir %s\n",
			baseDir)
		// _ = os.RemoveAll(baseDir)
	} else {
		fmt.Printf(">>> TEST DIRECTORY: %s\n", baseDir)
	}

	os.Exit(result)
} // func TestMain(m *testing.M)
