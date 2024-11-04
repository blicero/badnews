// /home/krylon/go/src/github.com/blicero/badnews/busybee/busybee.go
// -*- mode: go; coding: utf-8; -*-
// Created on 04. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-04 22:44:56 krylon>

// Package busybee implements ahead-of-time rating and judging of news Items,
// caching the results for (hopefully) improved performance in the web frontend.
package busybee

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"

	bt "go.etcd.io/bbolt" // Use the BoltDB backend

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/faabiosr/cachego"
	"github.com/faabiosr/cachego/bolt"
)

// BusyBee implements background workers that proactively compute suggested
// ratings and Tags for news Items, caching the results.
type BusyBee struct {
	lock   sync.RWMutex
	active atomic.Bool
	log    *log.Logger
	adb    *bt.DB
	jdb    *bt.DB
	acache cachego.Cache
	jcache cachego.Cache
	pool   *database.Pool
}

// Create instantiates a new BusyBee.
func Create() (*BusyBee, error) {
	var (
		err error
		bee = new(BusyBee)
	)

	if bee.log, err = common.GetLogger(logdomain.BusyBee); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to create Logger for BusyBee: %s\n",
			err.Error())
		return nil, err
	} else if bee.adb, err = bt.Open(common.Path(path.Advisor), 0600, nil); err != nil {
		bee.log.Printf("[ERROR] Failed to open Advisor cache at %s: %s\n",
			common.Path(path.Advisor),
			err.Error())
		return nil, err
	} else if bee.jdb, err = bt.Open(common.Path(path.Judge), 0600, nil); err != nil {
		bee.log.Printf("[ERROR] Failed to open Judge cache at %s: %s\n",
			common.Path(path.Judge),
			err.Error())
		return nil, err
	} else if bee.pool, err = database.NewPool(4); err != nil {
		bee.log.Printf("[ERROR] Failed to create database connection pool: %s\n",
			err.Error())
		return nil, err
	}

	bee.acache = bolt.New(bee.adb)
	bee.jcache = bolt.New(bee.jdb)

	return bee, nil
} // func Create() (*BusyBee, error)
