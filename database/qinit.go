// /home/krylon/go/src/github.com/blicero/badnews/database/qinit.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-19 21:02:06 krylon>

package database

var initQueries = []string{
	`
CREATE TABLE feed (
    id                  INTEGER PRIMARY KEY,
    title               TEXT UNIQUE NOT NULL,
    url                 TEXT UNIQUE NOT NULL,
    homepage            TEXT NOT NULL,
    interval            INTEGER NOT NULL DEFAULT 1800,
    last_refresh        INTEGER NOT NULL DEFAULT 0,
    active              INTEGER NOT NULL DEFAULT 1,
    CHECK (interval > 0)
) STRICT
`,
	"CREATE INDEX feed_last_refresh_idx ON feed (last_refresh)",
	"CREATE INDEX feed_active_idx ON feed (active <> 0)",
	`
CREATE TABLE item (
    id                  INTEGER PRIMARY KEY,
    feed_id             INTEGER NOT NULL,
    url                 TEXT UNIQUE NOT NULL,
    timestamp           INTEGER NOT NULL,
    headline            TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (feed_id) REFERENCES feed (id)
) STRICT
`,
	"CREATE INDEX item_feed_idx ON item (feed_id)",
	"CREATE INDEX item_time_idx ON item (timestamp)",
	"CREATE INDEX item_headline_idx ON item (headline)",
}
