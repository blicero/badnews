// /home/krylon/go/src/github.com/blicero/badnews/model/model.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-24 20:04:54 krylon>

// Package model provides the data types used across the application.
package model

import (
	"fmt"
	"net/url"
	"time"
)

// Feed is an RSS feed. Duh.
type Feed struct {
	ID             int64
	Title          string
	URL            *url.URL
	Homepage       *url.URL
	UpdateInterval time.Duration
	LastRefresh    time.Time
	Active         bool
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
}
