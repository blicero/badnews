// /home/krylon/go/src/github.com/blicero/badnews/database/05_search_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-18 22:16:35 krylon>

package database

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/model"
)

const (
	testSearchCnt = 4
)

var (
	testSearches []*model.Search = make([]*model.Search, 0, testSearchCnt)
	testTerms                    = []string{
		"ocean",
		"volcano",
		"coral",
		"bird",
	}
)

func TestSearchAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err    error
		status bool
	)

	db.Begin() // nolint: errcheck
	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
		}
	}()

	for i := 0; i < testSearchCnt; i++ {
		var s = &model.Search{
			Title:       fmt.Sprintf("Search #%02d", i+1),
			TimeCreated: time.Now().Add(time.Hour * -(testSearchCnt - time.Duration(i))),
			Tags:        []int64{1, 2},
			QueryString: testTerms[i],
		}

		if err = db.SearchAdd(s); err != nil {
			t.Fatalf("Failed to add Search %q to database: %s",
				s.Title,
				err.Error())
		} else if s.ID == 0 {
			t.Fatal("Search Object should have a non-zero ID after adding it to Database")
		}
	}

	status = true
} // func TestSearchAdd(t *testing.T)

func TestSearchGetActive01(t *testing.T) {
	// At this point, we have a few search queries stored in the
	// database, but they not been started.
	// SearchGetActive should thus return, without an error, zero results.
	if db == nil {
		t.SkipNow()
	}

	var (
		err      error
		searches []*model.Search
	)

	if searches, err = db.SearchGetActive(); err != nil {
		t.Fatalf("Failed to look for active searches: %s", err.Error())
	} else if len(searches) != 0 {
		t.Fatalf("SearchGetActive should have returned zero results, not %d:\n%+v\n",
			len(searches),
			searches)
	}
} // func TestSearchGetActive01(t *testing.T)

func TestSearchGetByID(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	for _, q := range testSearches {
		var (
			err error
			r   *model.Search
		)

		if r, err = db.SearchGetByID(q.ID); err != nil {
			t.Fatalf("Error looking up Search query %d: %s",
				q.ID,
				err.Error())
		} else if r == nil {
			t.Fatalf("SearchGetByID returned no value for Search Query #%d",
				q.ID)
		}
	}
} // func TestSearchGetByID(t *testing.T)

// 2024-11-18, 18:47
// I am now at a point where I think I can test running a Search.
// To do so, I need a bunch of Items in the database.
// The easiest - but most tedious - way is to create a bunch more Items
// or use a database filled with existing data.
// I think I will do just that.

var searchdb *Database

func TestSearchSampleDBOpen(t *testing.T) {
	// First, we copy the testing database to our testing folder
	// Then we open the copy.
	const srcpath = "./testdata/badnews.db"
	var (
		err      error
		src, dst *os.File
		dstpath  = filepath.Join(
			common.BaseDir,
			"searchtest.db",
		)
	)

	if src, err = os.Open(srcpath); err != nil {
		t.Fatalf("Failed to open sample database %s for copying: %s",
			srcpath,
			err.Error())
	}

	defer src.Close() // nolint: errcheck

	if dst, err = os.Create(dstpath); err != nil {
		t.Fatalf("Failed to create destination file for testing database %s: %s",
			dstpath,
			err.Error())
	}

	defer dst.Close() // nolint: errcheck

	if _, err = io.Copy(dst, src); err != nil {
		t.Fatalf("Failed to copy contents of sample database %s to testing folder %s: %s",
			srcpath,
			dstpath,
			err.Error())
	} else if err = dst.Close(); err != nil {
		t.Fatalf("Failed to close filehandle for testing db %s: %s",
			dstpath,
			err.Error())
	}

	if searchdb, err = Open(dstpath); err != nil {
		searchdb = nil
		t.Fatalf("Failed to open test db %s: %s",
			dstpath,
			err.Error())
	}
} // func TestSearchSampleDBOpen(t *testing.T)

var sampleSearches []*model.Search = []*model.Search{
	{
		Title:       "Fußball",
		TimeCreated: time.Now(),
		QueryString: "Fußball",
	},
	{
		Title:       "Astronomy",
		TimeCreated: time.Now(),
		Tags:        []int64{84},
	},
	{
		Title:       "Unix desktops",
		TimeCreated: time.Now(),
		Tags:        []int64{4, 9, 109, 110, 119, 120},
		QueryString: "(?:KDE|GNOME|Plasma)",
		Regex:       true,
	},
	{
		// In my sample database, this should return 3 items, 685, 544, 680.
		Title:          "By Period",
		TimeCreated:    time.Now(),
		FilterByPeriod: true,
		FilterPeriod: [2]time.Time{
			// 2024-10-01 00:00:00 -- 2024-10-06 23:59:59
			time.Unix(1727733600, 0),
			time.Unix(1728251999, 0),
		},
		QueryString: "BSD",
	},
}

func TestSearchSampleDBSearchAdd(t *testing.T) {
	if searchdb == nil {
		t.SkipNow()
	}

	var (
		err    error
		status bool
	)

	if err = searchdb.Begin(); err != nil {
		t.Fatalf("Failed to start transaction in Search DB: %s",
			err.Error())
	} else {
		defer func() {
			if status {
				searchdb.Commit() // nolint: errcheck
			} else {
				searchdb.Rollback() // nolint: errcheck
			}
		}()
	}

	for _, s := range sampleSearches {
		if err = searchdb.SearchAdd(s); err != nil {
			t.Fatalf("Failed to add search query to DB: %s",
				err.Error())
		}
	}

	status = true
} // func TestSearchSampleDBSearchAdd(t *testing.T)

func TestSearchExecute(t *testing.T) {
	if searchdb == nil {
		t.SkipNow()
	}

	var (
		err error
	)

	for _, s := range sampleSearches {
		if err = searchdb.SearchStart(s); err != nil {
			t.Fatalf("Error marking Search %s (%d) as started: %s",
				s.Title,
				s.ID,
				err.Error())
		} else if err = searchdb.SearchExecute(s); err != nil {
			t.Fatalf("Failed to execute Search %s (%d): %s",
				s.Title,
				s.ID,
				err.Error())
		} else if !s.Status {
			t.Errorf("Search %s (%d) did not execute successfully: %s",
				s.Title,
				s.ID,
				s.Message)
		} else if len(s.Results) == 0 {
			t.Errorf("Search %s (%d) did not produce results, but it should have",
				s.Title,
				s.ID)
		}
	}
} // func TestSearchExecute(t *testing.T)
