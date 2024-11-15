// /home/krylon/go/src/github.com/blicero/badnews/database/search.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-15 21:52:56 krylon>

package database

import (
	"fmt"

	"github.com/blicero/badnews/model"
	"github.com/blicero/krylib"
)

// I outsourced (so to speak) the Search process into a separate file. I will
// likely break it up into several methods and don't want to clutter up the
// database.go file more then it already is. (One sunny day, I should probably do
// that with the other sections of that file. But one thing after the other.)

// SearchExecute runs a Search. If everything goes well, it fills in the
// results (if there are any) and sets the Status, and TimeFinished fields,
// both in the Search object AND the database.
// If something goes wrong, it stores the relevant error message in the
// object and the database record.
func (db *Database) SearchExecute(s *model.Search) error {
	var (
		err   error
		items []*model.Item
	)

	if len(s.Tags) > 0 {
		// In this case, we load the Items by the associated Tags
		if items, err = db.searchLoadByTags(s); err != nil {
			// do that thing
		}
	} else {
	}

	return krylib.ErrNotImplemented
} // func (db *Database) SearchExecute(s *model.Search) error

func (db *Database) searchLoadByTags(s *model.Search) ([]*model.Item, error) {
	var (
		err   error
		items []*model.Item
	)

	if !s.TagsAll {
		var (
			tag      *model.Tag
			ok       bool
			tagItems []*model.Item
			union    = make(map[int64]*model.Item)
		)

		for _, tid := range s.Tags {
			// First, get the Tag
			if tag, err = db.TagGetByID(tid); err != nil {
				db.log.Printf("[ERROR] Failed to load Tag #%d: %s\n",
					tid,
					err.Error())
				return nil, err
			} else if tag == nil {
				err = fmt.Errorf("No Tag with ID = %d was found in the database",
					tid)
				db.log.Printf("[ERROR] %s\n",
					err.Error())
				return nil, err
			} else if tagItems, err = db.TagLinkGetByTag(tag); err != nil {
				db.log.Printf("[ERROR] Failed to load Items for Tag %s (%d): %s\n",
					tag.Name,
					tag.ID,
					err.Error())
				return nil, err
			}

			for _, item := range tagItems {
				if _, ok = union[item.ID]; !ok {
					union[item.ID] = item
				}
			}
		}

		items = make([]*model.Item, 0, len(union))

		for _, item := range union {
			items = append(items, item)
		}
	} else if len(s.Tags) == 0 {
		// Just skip this part?
	} else {
		var (
			tag       *model.Tag
			intersect map[int64]*model.Item
			tagItems  []*model.Item
		)

		// If we have an AND clause, the plan is to start loading Items linked
		// to the first Tag, then loop over the subsequent tags and discard Items
		// that do not appear in those lists.

		if tag, err = db.TagGetByID(s.Tags[0]); err != nil {
			db.log.Printf("[ERROR] Failed to load Tag #%d: %s\n",
				s.Tags[0],
				err.Error())
			return nil, err
		} else if tag == nil {
			err = fmt.Errorf("No Tag with ID = %d was found in the database",
				s.Tags[0])
			db.log.Printf("[ERROR] %s\n",
				err.Error())
			return nil, err
		} else if tagItems, err = db.TagLinkGetByTag(tag); err != nil {
			db.log.Printf("[ERROR] Failed to load Items linked to Tag %s (%d): %s\n",
				tag.Name,
				tag.ID,
				err.Error())
			return nil, err
		}

		intersect = make(map[int64]*model.Item, len(tagItems))

		for _, item := range tagItems {
			intersect[item.ID] = item
		}

		for _, tid := range s.Tags[1:] {
			// First, get the Tag
			if tag, err = db.TagGetByID(tid); err != nil {
				db.log.Printf("[ERROR] Failed to load Tag #%d: %s\n",
					tid,
					err.Error())
				return nil, err
			} else if tag == nil {
				err = fmt.Errorf("No Tag with ID = %d was found in the database",
					tid)
				db.log.Printf("[ERROR] %s\n",
					err.Error())
				return nil, err
			} else if tagItems, err = db.TagLinkGetByTag(tag); err != nil {
				db.log.Printf("[ERROR] Failed to load Items for Tag %s (%d): %s\n",
					tag.Name,
					tag.ID,
					err.Error())
				return nil, err
			}

		}
	}

	// ...

	// At long last:
	return items, nil
} // func (db *Database) searchLoadByTags(s *model.Search) ([]*model.Item, error)
