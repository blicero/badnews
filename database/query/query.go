// /home/krylon/go/src/github.com/blicero/badnews/database/query/query.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-13 21:05:21 krylon>

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
	ItemDeleteByFeed
	ItemExists
	ItemGetRecent
	ItemGetRecentPaged
	ItemGetByID
	ItemGetByFeed
	ItemGetByPeriod
	ItemGetRated
	ItemGetAll
	ItemRate
	ItemUnrate
	TagAdd
	TagGetByID
	TagGetChildren
	TagGetAll
	TagGetSorted
	TagGetItemCnt
	TagRename
	TagSetParent
	TagUpdate
	TagDelete
	TagLinkAdd
	TagLinkDelete
	TagLinkDeleteByFeed
	TagLinkGetByItem
	TagLinkGetByTag
	TagLinkGetByTagHierarchy
	SearchAdd
	SearchDelete
	SearchGetByID
	SearchGetActive
	SearchGetNextPending
	SearchGetAll
	SearchStart
	SearchFinish
)

// AllQueries returns a slice of all queries.
func AllQueries() []ID {
	return []ID{
		FeedAdd,
		FeedGetByID,
		FeedGetAll,
		FeedGetPending,
		FeedUpdateRefresh,
		FeedSetActive,
		FeedDelete,
		ItemAdd,
		ItemDeleteByFeed,
		ItemExists,
		ItemGetRecent,
		ItemGetRecentPaged,
		ItemGetByID,
		ItemGetByFeed,
		ItemGetByPeriod,
		ItemGetRated,
		ItemGetAll,
		ItemRate,
		ItemUnrate,
		TagAdd,
		TagGetByID,
		TagGetChildren,
		TagGetAll,
		TagGetSorted,
		TagRename,
		TagSetParent,
		TagUpdate,
		TagDelete,
		TagLinkAdd,
		TagLinkDelete,
		TagLinkDeleteByFeed,
		TagLinkGetByItem,
		TagLinkGetByTag,
		TagLinkGetByTagHierarchy,
		SearchAdd,
		SearchDelete,
		SearchGetByID,
		SearchGetActive,
		SearchGetAll,
		SearchStart,
		SearchFinish,
	}
} // func AllQueries() []ID
