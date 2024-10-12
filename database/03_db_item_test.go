// /home/krylon/go/src/github.com/blicero/badnews/database/03_db_item_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 10. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-12 19:48:26 krylon>

package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/blicero/badnews/model"
)

func TestItemAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	items = make([]*model.Item, 0, itemCnt*feedCnt)

	var status bool

	db.Begin() // nolint: errcheck
	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
			t.Error("Adding Items failed!")
		}
	}()

	for _, f := range feeds {
		for idx := 1; idx <= itemCnt; idx++ {
			var (
				err  error
				ustr = fmt.Sprintf("https://feeds.example.com/feed%03d/item%03d.html",
					f.ID,
					idx)
				item = &model.Item{
					FeedID:    f.ID,
					URL:       purl(ustr),
					Timestamp: time.Now(),
					Headline: fmt.Sprintf("News Item %d/%d",
						f.ID,
						idx),
					Description: "Bla",
				}
			)

			if err = db.ItemAdd(item); err != nil {
				t.Errorf("Failed to add Item %d/%d: %s",
					f.ID,
					idx,
					err.Error())
				return
			}

			items = append(items, item)
		}
	}

	status = true
} // func TestItemAdd(t *testing.T)
