// /home/krylon/go/src/github.com/blicero/badnews/database/01_db_feed_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 20. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-12 18:31:33 krylon>

package database

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/blicero/badnews/model"
)

func TestDBFeedAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

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
				Active:         true,
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
		} else if c.f.ID == 0 {
			t.Fatalf("After adding Feed %s, ID is still zero",
				c.f.Title)
		} else if !c.expectError {
			feeds = append(feeds, c.f)
		}
	}
} // func TestDBFeedAdd(t *testing.T)

func TestDBFeedGetByID(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var err error

	for _, f1 := range feeds {
		var f2 *model.Feed

		if f2, err = db.FeedGetByID(f1.ID); err != nil {
			t.Fatalf("Error getting Feed %s (%d) from database: %s",
				f1.Title,
				f1.ID,
				err.Error())
		} else if f2 == nil {
			t.Fatalf("Feed %s (%d) was not found in database",
				f1.Title, f1.ID)
		} else if !feedEqual(&f1, f2) {
			t.Fatalf("Feed from database not equal to original Feed:\nOriginal: %s\nDatabase: %s",
				&f1,
				f2)
		}
	}
} // func TestDBFeedGetByID(t *testing.T)

func TestDBFeedGetPending(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err     error
		pending []model.Feed
	)

	if pending, err = db.FeedGetPending(); err != nil {
		t.Fatalf("Failed to fetch pending Feeds: %s", err.Error())
	} else if len(pending) != len(feeds) {
		t.Fatalf("Unexpected number of results from FeedGetPending: %d (expected %d)",
			len(pending),
			len(feeds))
	}
} // func TestDBFeedGetPending(t *testing.T)

func TestDBFeedUpdateRefresh(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err error
		now = time.Now()
	)

	for i := range feeds {
		if err = db.FeedUpdateRefresh(&feeds[i], now); err != nil {
			t.Fatalf("Error setting update timestamp on Feed %s (%d): %s",
				feeds[i].Title,
				feeds[i].ID,
				err.Error())
		}
	}

	var pending []model.Feed

	if pending, err = db.FeedGetPending(); err != nil {
		t.Fatalf("Failed to get pending Feeds: %s", err.Error())
	} else if len(pending) != 0 {
		t.Fatalf("Unexpected number of pending Feeds: %d (expected 0)",
			len(pending))
	}
} // func TestDBFeedUpdateRefresh(t *testing.T)
