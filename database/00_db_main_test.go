// /home/krylon/go/src/github.com/blicero/badnews/database/00_db_main_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 20. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-12 19:49:37 krylon>

package database

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/model"
)

const (
	feedCnt = 32
	itemCnt = 32
	tagCnt  = 16
)

var (
	db    *Database
	feeds []model.Feed
	items []*model.Item
	tags  []*model.Tag
)

func TestMain(m *testing.M) {
	var (
		err     error
		result  int
		baseDir = time.Now().Format("/tmp/badnews_db_test_20060102_150405")
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

// Helpers

func feedEqual(f1, f2 *model.Feed) bool {
	return f1.ID == f2.ID &&
		f1.Title == f2.Title &&
		f1.URL.String() == f2.URL.String() &&
		f1.Homepage.String() == f2.Homepage.String() &&
		f1.UpdateInterval == f2.UpdateInterval &&
		f1.Active == f2.Active
} // func feedEqual(f1, f2 *model.Feed) bool

func purl(ustr string) *url.URL {
	var (
		err error
		u   *url.URL
	)

	if u, err = url.Parse(ustr); err != nil {
		panic(u)
	}

	return u
} // func purl(ustr string) *url.URL
