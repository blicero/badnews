// /home/krylon/go/src/github.com/blicero/badnews/database/qinit.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-13 20:06:14 krylon>

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
    rating              INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (feed_id) REFERENCES feed (id),
    CHECK (rating IN (-1, 0, 1))
) STRICT
`,
	"CREATE INDEX item_feed_idx ON item (feed_id)",
	"CREATE INDEX item_time_idx ON item (timestamp)",
	"CREATE INDEX item_headline_idx ON item (headline)",
	"CREATE INDEX item_rating_idx ON item (rating)",

	`
CREATE TABLE tag (
    id		INTEGER PRIMARY KEY,
    parent	INTEGER,
    name	TEXT NOT NULL,
    FOREIGN KEY (parent) REFERENCES tag (id)
       ON UPDATE RESTRICT
       ON DELETE CASCADE,
    UNIQUE (name, parent),
    CHECK (name <> ''),
    CHECK (parent <> id)
) STRICT`,
	"CREATE INDEX tag_parent_idx ON tag (parent)",

	`
CREATE TABLE tag_link (
    id		INTEGER PRIMARY KEY,
    tag_id	INTEGER NOT NULL,
    item_id	INTEGER NOT NULL,
    FOREIGN KEY (tag_id) REFERENCES tag (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES item (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,
    UNIQUE (tag_id, item_id)
) STRICT
`,
	"CREATE INDEX tl_tag_idx ON tag_link (tag_id)",
	"CREATE INDEX tl_item_idx ON tag_link (item_id)",
}
