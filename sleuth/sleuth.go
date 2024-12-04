// /home/krylon/go/src/github.com/blicero/badnews/sleuth/sleuth.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-03 18:17:19 krylon>

// Package sleuth handles the scheduling and dispatching of Search Queries.
package sleuth

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
)

// TODO Once I am done debugging/testing, I need to raise this
const pulse = time.Millisecond * 2500

// Sleuth treats search requests kinda like a batch queue
type Sleuth struct {
	log     *log.Logger
	db      *database.Database
	searchQ chan *model.Search
	active  atomic.Bool
}

// Create creates and returns a new instance of Sleuth.
func Create() (*Sleuth, error) {
	var (
		err error
		s   = new(Sleuth)
	)

	if s.log, err = common.GetLogger(logdomain.Search); err != nil {
		return nil, err
	} else if s.db, err = database.Open(common.Path(path.Database)); err != nil {
		s.log.Printf("[CRITICAL] Cannot open database at %s: %s\n",
			common.Path(path.Database),
			err.Error())
		return nil, err
	}

	s.searchQ = make(chan *model.Search)

	return s, nil
} // func Create() (*Sleuth, error)

// IsActive returns the Sleuth's active flag
func (s *Sleuth) IsActive() bool {
	return s.active.Load()
}

// Run executes the Sleuth's main loop, it waits for new search queries
// and executes them.
func (s *Sleuth) Run() {
	s.active.Store(true)
	defer s.active.Store(false)

	s.log.Println("[INFO] Sleuth main loop starting up")
	defer s.log.Println("[INFO] Sleuth main loop finishing")

	var ticker = time.NewTicker(pulse)
	defer ticker.Stop()

	go s.feeder()

	for s.IsActive() {
		var (
			err error
			q   *model.Search
		)
		select {
		case q = <-s.searchQ:
			// do something
			if err = s.db.SearchStart(q); err != nil {
				s.log.Printf("[ERROR] Failed to mark Search Query %s (%d) as started: %s\n",
					q.Title,
					q.ID,
					err.Error())
				continue
			} else if err = s.db.SearchExecute(q); err != nil {
				s.log.Printf("[ERROR] Failed to execute Search query %s (%d): %s\n",
					q.Title,
					q.ID,
					err.Error())
			}
		case <-ticker.C:
			continue
		}
	}
} // func (s *Sleuth) Run()

func (s *Sleuth) feeder() {
	var (
		err        error
		searchList []*model.Search
	)

	s.log.Println("[INFO] Sleuth feeder loop starting up.")
	defer s.log.Println("[INFO] Sleeth feeder loop is quitting.")

	if searchList, err = s.db.SearchGetActive(); err != nil {
		s.log.Printf("[ERROR] Failed to load active search queries: %s\n",
			err.Error())
		return
	}

	for _, q := range searchList {
		s.searchQ <- q
	}

	for s.IsActive() {
		var q *model.Search

		// s.log.Println("[INFO] Sleuth feeder loop fetching one Query from database.")

		if q, err = s.db.SearchGetNextPending(); err != nil {
			s.log.Printf("[ERROR] Failed to load pending search queries: %s\n",
				err.Error())
			return
		} else if q != nil {
			s.searchQ <- q
		} else {
			// s.log.Printf("[INFO] No pending queries were found, sleeping for %s\n",
			// 	pulse)
			time.Sleep(pulse)
		}
	}
} // func (s *Sleuth) feeder()
