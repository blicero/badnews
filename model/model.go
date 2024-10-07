// /home/krylon/go/src/github.com/blicero/badnews/model/model.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-07 13:24:36 krylon>

// Package model provides the data types used across the application.
package model

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
)

// Feed is an RSS feed. Duh.
type Feed struct {
	ID             int64         `json:"id,omitempty"`
	Title          string        `json:"title"`
	URL            *url.URL      `json:"url"`
	Homepage       *url.URL      `json:"homepage"`
	UpdateInterval time.Duration `json:"interval"`
	LastRefresh    time.Time     `json:"last_refresh,omitempty"`
	Active         bool          `json:"active,omitempty"`
}

func (f *Feed) String() string {
	return fmt.Sprintf(`{ ID: %d, Title: %q, URL: %q, Homepage: %q, UpdateInterval: %s, LastRefresh: %s, Active: %t }`,
		f.ID,
		f.Title,
		f.URL.String(),
		f.Homepage.String(),
		f.UpdateInterval,
		f.LastRefresh,
		f.Active)
}

// IsDue returns true if the Feed is due for a refresh.
func (f *Feed) IsDue() bool {
	return time.Now().After(f.LastRefresh.Add(f.UpdateInterval))
} // func (f *Feed) IsDue() bool

// Clone returns a shallow copy of the Feed
func (f *Feed) Clone() *Feed {
	var c = &Feed{
		ID:             f.ID,
		Title:          f.Title,
		URL:            f.URL,
		Homepage:       f.Homepage,
		UpdateInterval: f.UpdateInterval,
		LastRefresh:    f.LastRefresh,
		Active:         f.Active,
	}

	return c
}

// Item is a single news item
type Item struct {
	ID          int64
	FeedID      int64
	URL         *url.URL
	Timestamp   time.Time
	Headline    string
	Description string
	Rating      int8
	Guessed     int8
	_idstr      string
	_plain      string
}

var whitespace *regexp.Regexp = regexp.MustCompile(`[\s\t\n\r]+`)

// EffectiveRating returns the Item's Rating, *if* it has been rated, the guessed
// Rating, if one is stored in the Item, else zero.
func (i *Item) EffectiveRating() int8 {
	if i.Rating != 0 {
		return i.Rating
	} else if i.Guessed != 0 {
		return i.Guessed
	}

	return 0
} // func (i *Item) EffectiveRating() int8

// Plaintext returns the complete text of the Item, cleansed of any HTML.
func (i *Item) Plaintext() string {
	var tmp = make([]string, 2)
	var err error

	if i._plain != "" {
		return i._plain
	}

	if tmp[0], err = html2text.FromString(i.Headline); err != nil {
		tmp[0] = i.Headline
	}

	if tmp[1], err = html2text.FromString(i.Description); err != nil {
		tmp[1] = i.Description
	}

	if tmp[1] == "Comments" { // Hacker News
		tmp[1] = ""
	}

	tmp[0] = whitespace.ReplaceAllString(tmp[0], " ")
	tmp[1] = whitespace.ReplaceAllString(tmp[1], " ")

	i._plain = strings.Join(tmp, " ")
	return i._plain
} // func (i *Item) Plaintext() string

// Returns the ID as a string
func (i *Item) IDString() string {
	if i._idstr != "" {
		return i._idstr
	}

	i._idstr = strconv.FormatInt(i.ID, 10)
	return i._idstr
} // func (i *Item) IDString() string
