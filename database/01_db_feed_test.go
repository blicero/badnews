// /home/krylon/go/src/github.com/blicero/badnews/database/01_db_feed_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 20. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-20 21:44:31 krylon>

package database

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/model"
)

var (
	db    *Database
	feeds []model.Feed
)

func TestDBOpen(t *testing.T) {
	var (
		err    error
		dbpath string
	)

	dbpath = common.Path(path.Database)

	if db, err = Open(dbpath); err != nil {
		db = nil
		t.Fatalf("Failed to open database at %s: %s",
			dbpath,
			err.Error())
	}
} // func TestDBOpen(t *testing.T)

func TestDBQueryPrepare(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err error
	)

	for qid := range dbQueries {
		if _, err = db.getQuery(qid); err != nil {
			t.Errorf("Failed to prepare query %s: %s",
				qid,
				err.Error())
		}
	}
} // func TestDBQueryPrepare(t *testing.T)

func TestDBFeedAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	const feedCnt = 32

	type testCase struct {
		f           model.Feed
		expectError bool
	}

	var (
		err       error
		hplink    *url.URL
		testCases = make([]testCase, feedCnt*2)
	)

	hplink, _ = url.Parse("https://www.example.org/news")

	for i := 0; i < feedCnt; i++ {
		var u *url.URL

		u, _ = url.Parse(fmt.Sprintf(
			"https://www.example.org/news/feed%03d.rss",
			i+1))

		testCases[i] = testCase{
			f: model.Feed{
				Title:          fmt.Sprintf("Feed %03d", i+1),
				URL:            u,
				Homepage:       hplink,
				UpdateInterval: time.Second * 3600,
			},
		}

		testCases[i+feedCnt] = testCase{
			f:           testCases[i].f,
			expectError: true,
		}
	}

	feeds = make([]model.Feed, 0, feedCnt)

	for _, c := range testCases {
		if err = db.FeedAdd(&c.f); err != nil {
			if !c.expectError {
				t.Fatalf("Unexpected error while adding feed %s: %s",
					c.f.Title,
					err.Error())
			}
		} else if !c.expectError {
			feeds = append(feeds, c.f)
		}
	}
} // func TestDBFeedAdd(t *testing.T)
