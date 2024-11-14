// /home/krylon/go/src/github.com/blicero/badnews/database/05_search_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 14. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-14 18:14:57 krylon>

package database

import (
	"fmt"
	"testing"
	"time"

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
