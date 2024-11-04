// /home/krylon/go/src/github.com/blicero/badnews/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 01. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-04 18:48:37 krylon>

// Package blacklist provides a way to filter news Items with regular expressions.
package blacklist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
)

// Pattern is a single item, it includes both the regular expression and a
// counter for how many times it matched news Items.
// This information is used in sorting the blacklist, so Patterns with more matches
// move to the front of the list.
type Pattern struct {
	ID      int64          `json:"id"`
	Pattern *regexp.Regexp `json:"pattern"`
	Cnt     atomic.Int64   `json:"cnt"`
}

// Match checks if the receiver Pattern matches the given Item.
func (p *Pattern) Match(i *model.Item) bool {
	if p.Pattern.MatchString(i.Plaintext()) {
		p.Cnt.Add(1)
		return true
	}
	return false
} // func (p *Pattern) Match(i *model.Item) bool

func (p *Pattern) MarshalJSON() ([]byte, error) {
	var s = fmt.Sprintf(`{ "id": %d, "pattern": %q, "cnt": %d }`,
		p.ID,
		p.Pattern.String(),
		p.Cnt.Load(),
	)

	return []byte(s), nil
} // func (p *Pattern) MarshalJSON() ([]byte, error)

func (p *Pattern) UnmarshalJSON(b []byte) error {
	type dummy struct {
		ID      int64  `json:"id"`
		Pattern string `json:"pattern"`
		Cnt     int64  `json:"cnt"`
	}

	var (
		err error
		d   dummy
	)

	if err = json.Unmarshal(b, &d); err != nil {
		return err
	}

	p.ID = d.ID
	p.Cnt.Store(d.Cnt)
	if p.Pattern, err = regexp.Compile(d.Pattern); err != nil {
		return err
	}

	return nil
} // func (p *Pattern) UnmarshalJSON(b []byte) error

// Blacklist is a collection of Patterns.
type Blacklist struct {
	lock    sync.RWMutex
	log     *log.Logger
	changed atomic.Bool
	List    []*Pattern
}

var (
	instance *Blacklist
	openLock sync.Mutex
)

// NewFromFile restores a Blacklist from a JSON file.
func NewFromFile(path string) (*Blacklist, error) {
	var (
		err    error
		bl     *Blacklist = new(Blacklist)
		logger *log.Logger
		fh     *os.File
		buf    bytes.Buffer
	)

	openLock.Lock()
	defer openLock.Unlock()

	if instance != nil {
		instance.log.Println("[DEBUG] Use existing Blacklist Singleton")
		return instance, nil
	} else if logger, err = common.GetLogger(logdomain.Blacklist); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to create Logger for Blacklist: %s\n",
			err.Error(),
		)
		return nil, err
	}

	logger.Printf("[INFO] Restore Blacklist from %s\n",
		path)

	if fh, err = os.Open(path); err != nil {
		if os.IsNotExist(err) {
			bl.List = make([]*Pattern, 0, 8)
			bl.log = logger
			return bl, nil
		}
		logger.Printf("[ERROR] Cannot open blacklist dump at %s: %s\n",
			path,
			err.Error())
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if _, err = io.Copy(&buf, fh); err != nil {
		logger.Printf("[ERROR] Failed to load serialized Blacklist from %s: %s\n",
			path,
			err.Error())
		return nil, err
	} else if err = json.Unmarshal(buf.Bytes(), &bl); err != nil {
		logger.Printf("[ERROR] Failed to de-serialize Blacklist: %s\n",
			err.Error())
		return nil, err
	}

	bl.log = logger
	instance = bl
	return bl, nil
} // func NewFromFile(path string) (Blacklist, error)

func (bl *Blacklist) Len() int           { return len(bl.List) }
func (bl *Blacklist) Swap(i, j int)      { bl.List[i], bl.List[j] = bl.List[j], bl.List[i] }
func (bl *Blacklist) Less(i, j int) bool { return bl.List[i].Cnt.Load() > bl.List[j].Cnt.Load() }

// Changed returns the Blacklist's change flag. A return value of true indicates the
// contents of the Blacklist have changed since it was created / last saved to disk.
func (bl *Blacklist) Changed() bool { return bl.changed.Load() }

// Match checks is the given Item is matched by any of the patterns in the Blacklist.
// If a match is found, the list is sorted.
func (bl *Blacklist) Match(i *model.Item) bool {
	bl.lock.RLock()
	defer bl.lock.RUnlock()

	for _, p := range bl.List {
		if p.Match(i) {
			bl.log.Printf("[DEBUG] Blacklist Item %q matches Item %q\n",
				p.Pattern,
				i.Headline)
			bl.changed.Store(true)
			return true
		}
	}

	return false
} // func (bl Blacklist) Match(i *model.Item) bool

// Sort sorts the Patterns of the Blacklist so that Patterns with larger match
// counts move to the front.
func (bl *Blacklist) Sort() {
	bl.lock.Lock()
	defer bl.lock.Unlock()

	bl.log.Println("[DEBUG] Sorting Blacklist")

	sort.Sort(bl)
} // func (bl *Blacklist) Sort()

// Dump serializes the Blacklist to JSON and writes it to a file at the given path.
func (bl *Blacklist) Dump(path string) error {
	var (
		err  error
		fh   *os.File
		data []byte
		buf  *bytes.Buffer
	)

	bl.Sort()
	bl.lock.RLock()
	defer bl.lock.RUnlock()

	bl.log.Printf("[INFO] Dumping Blacklist to %s\n",
		path)

	if data, err = json.Marshal(bl); err != nil {
		bl.log.Printf("[ERROR] Failed to serialize Blacklist: %s\n",
			err.Error())
		return err
	} else if fh, err = os.Create(path); err != nil {
		bl.log.Printf("[ERROR] Failed to open Blacklist file at %s: %s\n",
			path,
			err.Error())
		return err
	}

	defer fh.Close() // nolint: errcheck

	buf = bytes.NewBuffer(data)

	if _, err = io.Copy(fh, buf); err != nil {
		bl.log.Printf("[ERROR] Failed to write serialized Blacklist to %s: %s\n",
			path,
			err.Error())
		return err
	}

	bl.changed.Store(false)

	return nil
} // func (bl Blacklist) Dump(path string) error

// Add adds a Pattern to the Blacklist
func (bl *Blacklist) Add(p *Pattern) {
	bl.lock.Lock()
	bl.List = append(bl.List, p)
	bl.changed.Store(true)
	bl.lock.Unlock()
}

// AddString creates a new Pattern from the given string and adds it to the Blacklist.
func (bl *Blacklist) AddString(s string) error {
	var p = new(Pattern)
	var err error

	if p.Pattern, err = regexp.Compile(s); err != nil {
		return err
	}

	bl.lock.Lock()
	defer bl.lock.Unlock()
	bl.changed.Store(true)
	bl.List = append(bl.List, p)
	return nil
} // func (bl *Blacklist) AddString(s string) error
