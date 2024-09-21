// /home/krylon/go/src/github.com/blicero/badnews/database/01_db_create_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 21. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-21 20:50:12 krylon>

package database

import (
	"testing"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
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
