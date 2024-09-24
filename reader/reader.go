// /home/krylon/go/src/github.com/blicero/badnews/reader/reader.go
// -*- mode: go; coding: utf-8; -*-
// Created on 24. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-24 15:18:50 krylon>

package reader

import (
	"log"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
)

// TODO Set to reasonable value when testing is done.
const (
	checkInterval = time.Second * 30
	workerCnt     = 8
)

type Reader struct {
	log  *log.Logger
	pool *database.Pool
	q    chan model.Feed
}

func New() (*Reader, error) {
	var (
		err error
		rdr = &Reader{
			q: make(chan model.Feed, workerCnt),
		}
	)

	if rdr.log, err = common.GetLogger(logdomain.Reader); err != nil {
		return nil, err
	} else if rdr.pool, err = database.NewPool(workerCnt); err != nil {
		rdr.log.Printf("[ERROR] Cannot open database Pool: %s\n",
			err.Error())
		return nil, err
	}

	return rdr, nil
} // func New() (*Reader, error)
