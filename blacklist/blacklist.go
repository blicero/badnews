// /home/krylon/go/src/github.com/blicero/badnews/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 01. 11. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-01 16:44:45 krylon>

// Package blacklist provides a way to filter news Items with regular expressions.
package blacklist

import (
	"regexp"
	"sort"

	"github.com/blicero/badnews/model"
)

// Pattern is a single item, it includes both the regular expression and a
// counter for how many times it matched news Items.
// This information is used in sorting the blacklist, so Patterns with more matches
// move to the front of the list.
type Pattern struct {
	ID      int64
	Pattern *regexp.Regexp
	Cnt     int64
}

// Match checks if the receiver Pattern matches the given Item.
func (p *Pattern) Match(i *model.Item) bool {
	if p.Pattern.MatchString(i.Plaintext()) {
		p.Cnt++
		return true
	}
	return false
} // func (p *Pattern) Match(i *model.Item) bool

// Blacklist is a collection of Patterns.
type Blacklist []*Pattern

func (bl Blacklist) Len() int           { return len(bl) }
func (bl Blacklist) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }
func (bl Blacklist) Less(i, j int) bool { return bl[i].Cnt > bl[j].Cnt }

// Match checks is the given Item is matched by any of the patterns in the Blacklist.
// If a match is found, the list is sorted.
func (bl Blacklist) Match(i *model.Item) bool {
	for _, p := range bl {
		if p.Match(i) {
			sort.Sort(bl)
			return true
		}
	}

	return false
} // func (bl Blacklist) Match(i *model.Item) bool
