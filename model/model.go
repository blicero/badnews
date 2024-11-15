// /home/krylon/go/src/github.com/blicero/badnews/model/model.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-15 17:21:28 krylon>

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
	ID          int64     `json:"id"`
	FeedID      int64     `json:"feed_id"`
	URL         *url.URL  `json:"url"`
	Timestamp   time.Time `json:"timestamp"`
	Headline    string    `json:"headline"`
	Description string    `json:"description"`
	Rating      int8      `json:"rating"`
	Guessed     int8      `json:"guessed"`
	Tags        []*Tag    `json:"tags"`
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

// HasTag returns true if the Tag with the given id is found in the Item's list of Tags.
func (i *Item) HasTag(id int64) bool {
	for _, t := range i.Tags {
		if t.ID == id {
			return true
		}
	}

	return false
} // func (i *Item) HasTag(id int64) bool

// IDString returns the ID as a string
func (i *Item) IDString() string {
	if i._idstr != "" {
		return i._idstr
	}

	i._idstr = strconv.FormatInt(i.ID, 10)
	return i._idstr
} // func (i *Item) IDString() string

// Tag is a label that can be attached to an Item. A Tag can also have
// a Parent Tag, which allows to organize them in a hierarchy.
type Tag struct {
	ID       int64  `json:"id"`
	Parent   int64  `json:"parent,omitempty"`
	Name     string `json:"name"`
	Level    int64  `json:"level"`
	FullName string `json:"full_name"`
}

// Search represents the parameters of a search query.
// Regex, if true, indicates the Query text should be handled as a regular
// expression.
// TagsAll, if true, indicates the query is looking for Items that have ALL the
// supplied Tags linked to them.
type Search struct {
	ID             int64        `json:"id"`
	Title          string       `json:"title"`
	TimeCreated    time.Time    `json:"time_created"`
	TimeStarted    time.Time    `json:"time_started"`
	TimeFinished   time.Time    `json:"time_finished"`
	Status         bool         `json:"status"`
	Message        string       `json:"message"`
	Tags           []int64      `json:"tags"`
	TagsAll        bool         `json:"tags_all"`
	FilterByPeriod bool         `json:"filter_by_period"`
	FilterPeriod   [2]time.Time `json:"filter_period"`
	QueryString    string       `json:"query_string"`
	Regex          bool         `json:"regexp"`
	Results        []*Item      `json:"results"`
}

// IsFinished returns true if the Search query has a Finished timestamp that is
// later than its Started timestamp.
func (s *Search) IsFinished() bool {
	return s.TimeFinished.After(s.TimeStarted)
} // func (s *Search) IsFinished() bool
