// /home/krylon/go/src/github.com/blicero/badnews/database/04_db_tag_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 10. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-12 21:00:53 krylon>

package database

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/blicero/badnews/model"
)

func TestTagAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	tags = make([]*model.Tag, 0, tagCnt)

	var status bool

	db.Begin() // nolint: errcheck
	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
			t.Error("Failed to add Tags")
		}
	}()

	for i := 1; i <= tagCnt; i++ {
		var (
			err error
			tag = &model.Tag{
				Name: fmt.Sprintf("Tag%03d", i),
			}
		)

		if err = db.TagAdd(tag); err != nil {
			t.Errorf("Failed to add Tag %s: %s",
				tag.Name,
				err.Error())
			return
		}

		tags = append(tags, tag)
	}

	status = true
} // func TestTagAdd(t *testing.T)

func TestTagLinkAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	const linkPerTag = (itemCnt * feedCnt) / 8

	var status bool

	db.Begin() // nolint: errcheck
	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
			t.Error("Failed to add Tags")
		}
	}()

	for _, tag := range tags {
		var indices = rand.Perm(len(items))
		for i := 0; i < linkPerTag; i++ {
			var (
				err  error
				item = items[indices[i]]
			)

			if err = db.TagLinkAdd(item, tag); err != nil {
				t.Errorf("Error linking tag %s to Item %d: %s",
					tag.Name,
					item.ID,
					err.Error())
			}
		}
	}

	status = true
} // func TestTagLinkAdd(t *testing.T)

func TestTagItemCnt(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err error
		cnt map[int64]int64
	)

	if cnt, err = db.TagGetItemCnt(); err != nil {
		t.Fatalf("Failed to query Item count per Tag: %s",
			err.Error())
	} else if len(cnt) != len(tags) {
		t.Fatalf("Unexpected number of Tags in Item count: %d (expected %d)",
			len(cnt),
			len(tags))
	}
} // func TestTagItemCnt(t *testing.T)
