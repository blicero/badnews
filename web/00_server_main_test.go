// /home/krylon/go/src/github.com/blicero/scrollmaster/server/00_database_main_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 05. 06. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-29 18:19:16 krylon>

package web

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/blicero/badnews/common"
)

const testPort = common.Port + 2

var (
	srv    *Server
	addr   string
	client http.Client
)

func TestMain(m *testing.M) {
	var (
		err     error
		result  int
		opts    *cookiejar.Options
		baseDir = time.Now().Format("/tmp/badnews_web_test_20060102_150405")
	)

	opts = &cookiejar.Options{}

	if client.Jar, err = cookiejar.New(opts); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to create Cookiejar for Client: %s\n",
			err.Error())
		os.Exit(128)
	} else if err = common.SetBaseDir(baseDir); err != nil {
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
