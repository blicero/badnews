// /home/krylon/go/src/github.com/blicero/badnews/database/query/query.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-24 18:43:55 krylon>

// Package query provides symbolic constants to identify database queries.
package query

//go:generate stringer -type=ID

// ID represents a database query
type ID uint8

const (
	FeedAdd ID = iota
	FeedGetByID
	FeedGetAll
	FeedGetPending
	FeedUpdateRefresh
	FeedSetActive
	FeedDelete
	ItemAdd
	ItemExists
	ItemGetRecent
	ItemGetByFeed
	ItemGetRated
	ItemRate
	ItemUnrate
)

// AllQueries returns a slice of all queries.
func AllQueries() []ID {
	return []ID{
		FeedAdd,
	}
} // func AllQueries() []ID
