// /home/krylon/go/src/github.com/blicero/badnews/sleuth/sleuth.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-30 22:14:49 krylon>

// Package sleuth handles the scheduling and dispatching of Search Queries.
package sleuth

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/blicero/badnews/common"
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
	}

	return s, nil
} // func Create() (*Sleuth, error)

// IsActive returns the Sleuth's active flag
func (s *Sleuth) IsActive() bool {
	return s.active.Load()
}

func (s *Sleuth) Run() {
	s.active.Store(true)
	defer s.active.Store(false)

	var ticker = time.NewTicker(pulse)
	defer ticker.Stop()

	for s.IsActive() {

	}
} // func (s *Sleuth) Run()
