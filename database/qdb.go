// /home/krylon/go/src/github.com/blicero/badnews/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-04 19:15:03 krylon>

package database

import "github.com/blicero/badnews/database/query"

var dbQueries = map[query.ID]string{
	query.FeedAdd: `
INSERT INTO feed (title, url, homepage, interval)
          VALUES (    ?,   ?,        ?,        ?)
RETURNING id
`,
	query.FeedGetByID: `
SELECT
    title,
    url,
    homepage,
    interval,
    last_refresh,
    active
FROM feed
WHERE id = ?
`,
	query.FeedGetAll: `
SELECT
    id,
    title,
    url,
    homepage,
    interval,
    last_refresh,
    active
FROM feed
`,
	query.FeedGetPending: `
SELECT
    id,
    title,
    url,
    homepage,
    interval,
    last_refresh,
    active
FROM feed
WHERE (active <> 0) AND (last_refresh + interval < unixepoch())
`,
	query.FeedUpdateRefresh: `
UPDATE feed
SET last_refresh = ?
WHERE id = ?
`,
	query.FeedSetActive: `
UPDATE feed
SET active = ?
WHERE id = ?
`,
	query.FeedDelete: "DELETE FROM feed WHERE id = ?",
	query.ItemAdd: `
INSERT INTO item (feed_id, url, timestamp, headline, description)
          VALUES (      ?,   ?,         ?,        ?,           ?)
RETURNING id
`,
	query.ItemExists: "SELECT COUNT(id) FROM item WHERE url = ?",
	query.ItemGetRecent: `
SELECT
    id,
    feed_id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
WHERE timestamp > ?
ORDER BY timestamp DESC
`,
	query.ItemGetRecentPaged: `
SELECT
    id,
    feed_id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
ORDER BY timestamp DESC
LIMIT ?
OFFSET ?
`,
	query.ItemGetByID: `
SELECT
    feed_id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
WHERE id = ?
`,
	query.ItemGetByFeed: `
SELECT
    id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
WHERE feed_id = ?
ORDER BY timestamp DESC
LIMIT ?
OFFSET ?
`,
	query.ItemGetRated: `
SELECT
    id,
    feed_id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
WHERE rating <> 0
ORDER BY timestamp DESC
`,
	query.ItemRate:   "UPDATE item SET rating = ? WHERE id = ?",
	query.ItemUnrate: "UPDATE item SET rating = 0 WHERE id = ?",
}
