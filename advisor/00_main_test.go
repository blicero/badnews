// /home/krylon/go/src/ticker/advisor/00_main_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 27. 05. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2024-10-22 17:14:43 krylon>

package advisor

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
		baseDir = time.Now().Format("/tmp/ticker_advisor_test_20060102_150405")
	)

	if err = common.SetBaseDir(baseDir); err != nil {
		fmt.Printf("Cannot set base directory to %s: %s\n",
			baseDir,
			err.Error())
		os.Exit(1)
	} else if result = m.Run(); result == 0 {
		// If any test failed, we keep the test directory (and the
		// database inside it) around, so we can manually inspect it
		// if needed.
		// If all tests pass, OTOH, we can safely remove the directory.
		fmt.Printf("Removing BaseDir %s\n",
			baseDir)
		_ = os.RemoveAll(baseDir)
	} else {
		fmt.Printf(">>> TEST DIRECTORY: %s\n", baseDir)
	}

	os.Exit(result)
} // func TestMain(m *testing.M)
