// /home/krylon/go/src/github.com/blicero/badnews/busybee/busybee.go
// -*- mode: go; coding: utf-8; -*-
// Created on 04. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-05 21:16:21 krylon>

// Package busybee implements ahead-of-time rating and judging of news Items,
// caching the results for (hopefully) improved performance in the web frontend.
package busybee

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	// Use the BoltDB backend

	"github.com/blicero/badnews/advisor"
	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/judge"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
)

const (
	runInterval  = time.Second * 30 // TODO Increase after testing/debugging is done
	checkPeriod  = time.Second * 86400 * 2
	errTmp       = "resource temporarily unavailable"
	backoffDelay = time.Millisecond * 25
)

func backOff() {
	time.Sleep(backoffDelay)
}

// BusyBee implements background workers that proactively compute suggested
// ratings and Tags for news Items, caching the results.
type BusyBee struct {
	lock   sync.RWMutex // nolint: unused
	active atomic.Bool
	log    *log.Logger
	adv    *advisor.Advisor
	jdg    *judge.Judge
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
	} else if bee.adv, err = advisor.NewAdvisor(); err != nil {
		bee.log.Printf("[ERROR] Failed to create Advisor: %s\n",
			err.Error())
		return nil, err
	} else if bee.jdg, err = judge.New(); err != nil {
		bee.log.Printf("[ERROR] Failed to create Judge: %s\n",
			err.Error())
		return nil, err
	} else if bee.pool, err = database.NewPool(4); err != nil {
		bee.log.Printf("[ERROR] Failed to create database connection pool: %s\n",
			err.Error())
		return nil, err
	}

	return bee, nil
} // func Create() (*BusyBee, error)

// IsActive returns the BusyBee's active flag
func (bee *BusyBee) IsActive() bool {
	return bee.active.Load()
} // func (bee *BusyBee) IsActive() bool

// Stop clears the BusyBee's active flag
func (bee *BusyBee) Stop() {
	bee.active.Store(false)
} // func (bee *BusyBee) Stop()

// Run executes the BusyBee's main loop.
func (bee *BusyBee) Run() {
	var (
		err    error
		ticker *time.Ticker
	)

	ticker = time.NewTicker(runInterval)
	defer ticker.Stop()

	bee.active.Store(true)

	for bee.active.Load() {
		<-ticker.C

		if err = bee.preComputeAdvice(checkPeriod); err != nil {
			bee.log.Printf("[ERROR] Failed to precompute Advice/Ratings: %s\n",
				err.Error())
			continue
		}
	}
} // func (bee *BusyBee) Run()

func (bee *BusyBee) preComputeAdvice(period time.Duration) error {
	const suggCnt = 10
	var (
		err   error
		items []*model.Item
		db    *database.Database
	)

	if period > 0 {
		period = -period
	}

	bee.log.Printf("[INFO] Precomputing advice for Items from the last %s\n",
		-period)

	db = bee.pool.Get()
	defer bee.pool.Put(db)

	if items, err = db.ItemGetRecent(time.Now().Add(period)); err != nil {
		bee.log.Printf("[ERROR] Failed to load Items for last %s: %s\n",
			-period,
			err.Error())
		return err
	}

	bee.log.Printf("[DEBUG] Processing %d Items\n", len(items))

	var (
		acnt, jcnt int
	)

	defer func() {
		bee.log.Printf("[DEBUG] Precomputed Tags for %d Items, Ratings for %d Items\n",
			acnt,
			jcnt)
	}()

	for _, i := range items {
		if !bee.active.Load() {
			bee.log.Println("[TRACE] BusyBee has been stopped, aborting processing.")
			break
		}

	JCACHE:
		if !bee.jdg.InCache(i) {
			if _, err = bee.jdg.Rate(i); err != nil {
				if err.Error() == errTmp {
					backOff()
					goto JCACHE
				}
				bee.log.Printf("[ERROR] Failed to rate Item %d (%q): %s\n",
					i.ID,
					i.Headline,
					err.Error())
				return err
			}
			jcnt++
		}

		if !bee.adv.InCache(i) {
			var sugg = bee.adv.Suggest(i, suggCnt)
			if len(sugg) != 10 {
				bee.log.Printf("[INFO] Unexpected number of suggestions for Item %d (%q): %d (expected %d)\n",
					i.ID,
					i.Headline,
					len(sugg),
					suggCnt)
			}
			acnt++
		}
	}

	return nil
} // func preComputeAdvice(period time.Duration) error
