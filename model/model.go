// /home/krylon/go/src/github.com/blicero/badnews/model/model.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-19 17:46:07 krylon>

// Package model provides the data types used across the application.
package model

import (
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

// Item is a single news item
type Item struct {
	ID          int64
	FeedID      int64
	URL         *url.URL
	Timestamp   time.Time
	Headline    string
	Description string
}
