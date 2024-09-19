// /home/krylon/go/src/github.com/blicero/badnews/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-19 19:32:16 krylon>

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
}
