// /home/krylon/go/src/github.com/blicero/badnews/database/search.go
// -*- mode: go; coding: utf-8; -*-
// Created on 15. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-16 16:19:15 krylon>

package database

import (
	"fmt"
	"time"

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
		err    error
		items  []*model.Item
		status bool
		msg    string
	)

	defer func() {
		s.TimeFinished = time.Now()
		s.Status = status
		if !status {
			s.Message = msg
		}
	}()

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
				msg = err.Error()
				return nil, err
			} else if tag == nil {
				err = fmt.Errorf("No Tag with ID = %d was found in the database",
					tid)
				msg = err.Error()
				db.log.Printf("[ERROR] %s\n",
					err.Error())
				return nil, err
			} else if tagItems, err = db.TagLinkGetByTag(tag); err != nil {
				msg = fmt.Sprintf("Failed to load Items for Tag %s (%d): %s\n",
					tag.Name,
					tag.ID,
					err.Error())
				db.log.Printf("[ERROR] %s\n",
					msg)
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
		// Nope! In this case I have to load potentially all Items, filtered only by
		// their Timestamp if s.FilterByPeriod is true.
		// It's gonna get a bit chonky.
		if s.FilterByPeriod {
			if items, err = db.ItemGetByPeriod(s.FilterPeriod[0], s.FilterPeriod[1]); err != nil {
				msg = fmt.Sprintf("Failed to load Items by period [%s, %s] -- %s",
					s.FilterPeriod[0].Format(time.DateTime),
					s.FilterPeriod[1].Format(time.DateTime),
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return nil, err
			}
		} else {
			// Make a channel, spawn a goroutine, call db.ItemGetFiltered
			var itemQ = make(chan *model.Item)
			go db.ItemGetFiltered(itemQ, s.Match)
			items = make([]*model.Item, 0, 16)

			for i := range itemQ {
				items = append(items, i)
			}
		}
	} else {
		var (
			tag       *model.Tag
			intersect map[int64]*model.Item
			tagMap    map[int64]*model.Item
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
		} else if intersect, err = db.TagLinkGetByTagMap(tag); err != nil {
			db.log.Printf("[ERROR] Failed to load Items linked to Tag %s (%d): %s\n",
				tag.Name,
				tag.ID,
				err.Error())
			return nil, err
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
			} else if tagMap, err = db.TagLinkGetByTagMap(tag); err != nil {
				db.log.Printf("[ERROR] Failed to load Items for Tag %s (%d): %s\n",
					tag.Name,
					tag.ID,
					err.Error())
				return nil, err
			}

			for _, item := range intersect {
				var ok bool
				if _, ok = tagMap[item.ID]; !ok {
					delete(intersect, item.ID)
				}
			}

			if len(intersect) == 0 {
				break
			}
		}

		if len(intersect) == 0 {
			return nil, nil
		}

		items = make([]*model.Item, 0, len(intersect))

		for _, i := range intersect {
			items = append(items, i)
		}
	}

	if len(items) == 0 {
		return nil, nil
	}

	var results = make([]*model.Item, 0, len(items))

	for _, i := range items {
		if s.Match(i) {
			results = append(results, i)
		}
	}

	// At long last:
	return results, nil
} // func (db *Database) searchLoadByTags(s *model.Search) ([]*model.Item, error)
