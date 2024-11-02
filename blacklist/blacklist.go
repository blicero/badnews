// /home/krylon/go/src/github.com/blicero/badnews/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 01. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-02 21:42:11 krylon>

// Package blacklist provides a way to filter news Items with regular expressions.
package blacklist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

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
	lock sync.RWMutex
	List []*Pattern
}

var (
	instance *Blacklist
	openLock sync.Mutex
)

// New creates a new Blacklist from the given list of patterns
func New(pattern ...string) (*Blacklist, error) {
	var (
		err error
		bl  *Blacklist
	)

	openLock.Lock()
	defer openLock.Unlock()

	if instance != nil {
		return instance, nil
	}

	bl = &Blacklist{
		List: make([]*Pattern, len(pattern)),
	}
	instance = bl

	for idx, pat := range pattern {
		var p = &Pattern{
			ID: int64(idx) + 1,
		}

		if p.Pattern, err = regexp.Compile(pat); err != nil {
			return nil, err
		}

		bl.List[idx] = p
	}

	return bl, nil
} // func New(pattern... string) (Blacklist, error)

// NewFromFile restores a Blacklist from a JSON file.
func NewFromFile(path string) (*Blacklist, error) {
	var (
		err error
		bl  *Blacklist = new(Blacklist)
		fh  *os.File
		buf bytes.Buffer
	)

	openLock.Lock()
	defer openLock.Unlock()

	if instance != nil {
		return bl, nil
	}

	if fh, err = os.Open(path); err != nil {
		if os.IsNotExist(err) {
			return bl, nil
		}
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if _, err = io.Copy(&buf, fh); err != nil {
		return nil, err
	} else if err = json.Unmarshal(buf.Bytes(), &bl); err != nil {
		return nil, err
	}

	instance = bl
	return bl, nil
} // func NewFromFile(path string) (Blacklist, error)

func (bl *Blacklist) Len() int           { return len(bl.List) }
func (bl *Blacklist) Swap(i, j int)      { bl.List[i], bl.List[j] = bl.List[j], bl.List[i] }
func (bl *Blacklist) Less(i, j int) bool { return bl.List[i].Cnt.Load() > bl.List[j].Cnt.Load() }

// Match checks is the given Item is matched by any of the patterns in the Blacklist.
// If a match is found, the list is sorted.
func (bl *Blacklist) Match(i *model.Item) bool {
	bl.lock.RLock()
	defer bl.lock.RUnlock()

	for _, p := range bl.List {
		if p.Match(i) {
			return true
		}
	}

	return false
} // func (bl Blacklist) Match(i *model.Item) bool

func (bl *Blacklist) Sort() {
	bl.lock.Lock()
	defer bl.lock.Unlock()

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

	if data, err = json.Marshal(bl); err != nil {
		return err
	} else if fh, err = os.Create(path); err != nil {
		return err
	}

	defer fh.Close() // nolint: errcheck

	buf = bytes.NewBuffer(data)

	if _, err = io.Copy(fh, buf); err != nil {
		return err
	}

	return nil
} // func (bl Blacklist) Dump(path string) error

// Add adds a Pattern to the Blacklist
func (bl *Blacklist) Add(p *Pattern) {
	bl.lock.Lock()
	bl.List = append(bl.List, p)
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
	bl.List = append(bl.List, p)
	return nil
} // func (bl *Blacklist) AddString(s string) error
