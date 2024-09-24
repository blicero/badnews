// /home/krylon/go/src/github.com/blicero/badnews/reader/reader.go
// -*- mode: go; coding: utf-8; -*-
// Created on 24. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-24 20:02:35 krylon>

// Package reader implements the fetching and parsing of RSS/Atom feeds.
package reader

import (
	"log"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
	"github.com/mmcdole/gofeed"
)

// TODO Set to reasonable value when testing is done.
const (
	checkInterval = time.Second * 30 // nolint: unused
)

// Reader provides fetching and parsing of RSS feeds.
type Reader struct {
	log       *log.Logger
	pool      *database.Pool
	q         chan model.Feed
	active    atomic.Bool
	workerCnt int
}

// New creates a new Reader. Duh.
func New(workers int) (*Reader, error) {
	var (
		err error
		rdr = &Reader{
			q:         make(chan model.Feed, workers),
			workerCnt: workers,
		}
	)

	if rdr.log, err = common.GetLogger(logdomain.Reader); err != nil {
		return nil, err
	} else if rdr.pool, err = database.NewPool(workers); err != nil {
		rdr.log.Printf("[ERROR] Cannot open database Pool: %s\n",
			err.Error())
		return nil, err
	}

	return rdr, nil
} // func New() (*Reader, error)

// IsActive returns the Reader's active flag
func (r *Reader) IsActive() bool {
	return r.active.Load()
} // func (r *Reader) IsActive() bool

// Stop tells the Reader to stop.
func (r *Reader) Stop() {
	r.active.Store(false)
} // func (r *Reader) Stop()

// Start starts the Reader's worker goroutines.
func (r *Reader) Start() {
	r.active.Store(true)
	go r.feeder()
	for i := 0; i < r.workerCnt; i++ {
		go r.worker(i + 1)
	}
}

func (r *Reader) getPendingFeeds() ([]model.Feed, error) {
	var db = r.pool.Get()
	defer r.pool.Put(db)

	return db.FeedGetPending()
} // func (r *Reader) getPendingFeeds() ([]model.Feed, error)

// This method could have used a better name, but I just could not resist the pun.
func (r *Reader) feeder() {
	var ticker = time.NewTicker(checkInterval)
	defer ticker.Stop()

	for r.IsActive() {
		<-ticker.C
		r.checkFeeds()
	}
} // func (r *Reader) feeder()

func (r *Reader) checkFeeds() {
	var (
		err   error
		feeds []model.Feed
	)

	if feeds, err = r.getPendingFeeds(); err != nil {
		r.log.Printf("[ERROR] Failed to load feeds that are due for a refresh: %s\n",
			err.Error())
		return
	}

	for _, f := range feeds {
		r.q <- f
	}
} // func (r *Reader) checkFeeds()

func (r *Reader) worker(n int) {
	defer r.log.Printf("[INFO] Reader/worker_%02d stopping.\n", n)

	var ticker = time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for r.IsActive() {
		var (
			err error
			f   model.Feed
		)

		select {
		case f = <-r.q:
			if err = r.process(f); err != nil {
				r.log.Printf("[ERROR] Error processing Feed %s (%d): %s\n",
					f.Title,
					f.ID,
					err.Error())
			}
		case <-ticker.C:
			continue
		}
	}
} // func (r *Reader) worker()

func (r *Reader) process(f model.Feed) error {
	var (
		err  error
		db   *database.Database
		fp   = gofeed.NewParser()
		feed *gofeed.Feed
	)

	if feed, err = fp.ParseURL(f.URL.String()); err != nil {
		return err
	}

	db = r.pool.Get()
	defer r.pool.Put(db)

	r.log.Printf("[DEBUG] Processing Feed %s, %d items\n",
		feed.Title,
		len(feed.Items))

	// Should I use a transaction for adding new Items? :tinking-emoji:

	for _, fitem := range feed.Items {
		var item = model.Item{
			FeedID:      f.ID,
			Headline:    fitem.Title,
			Description: fitem.Description,
		}

		if item.URL, err = url.Parse(fitem.Link); err != nil {
			r.log.Printf("[ERROR] Cannot parse URL of Item %q (%s): %s\n",
				fitem.Title,
				fitem.Link,
				err.Error())
			continue
		} else if fitem.UpdatedParsed != nil {
			item.Timestamp = *fitem.UpdatedParsed
		} else if fitem.PublishedParsed != nil {
			item.Timestamp = *fitem.PublishedParsed
		} else {
			item.Timestamp = time.Now()
		}

		var exists bool

		if exists, err = db.ItemExists(&item); err != nil {
			r.log.Printf("[ERROR] Failed to check for Item %q: %s\n",
				item.URL,
				err.Error())
			continue
		} else if exists {
			r.log.Printf("[DEBUG] Item %q already exists in database.\n",
				item.URL)
			continue
		} else if err = db.ItemAdd(&item); err != nil {
			r.log.Printf("[ERROR] Failed to add item %q (%s) to database: %s\n",
				item.URL,
				item.Headline,
				err.Error())
			continue
		}
	}

	return nil
} // func (r *Reader) process(f model.Feed)
