// /home/krylon/go/src/github.com/blicero/badnews/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-17 16:12:33 krylon>

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
	query.TagAdd: `
INSERT INTO tag (name, parent)
         VALUES (   ?,      ?)
RETURNING id
`,
	query.TagGetByID: "SELECT name, parent FROM tag WHERE id = ?",
	// TODO Recursively fetch grandchildren etc.
	query.TagGetChildren: `
SELECT
    id,
    name
FROM tag
WHERE parent = ?
`,
	// TODO Can I order the tags hierarchically right here?
	query.TagGetAll: `
SELECT
    id,
    parent,
    name
FROM tag
ORDER BY COALESCE(parent, 0), id
`,
	query.TagGetItemCnt: `
WITH cnt_list (tag_id, cnt) AS (
    SELECT
        tag_id,
        COUNT(tag_id)
    FROM tag_link
    GROUP BY tag_id
)

SELECT
  t.id,
  COALESCE(c.cnt, 0)
FROM tag t
LEFT OUTER JOIN cnt_list c ON t.id = c.tag_id
`,
	query.TagRename:    "UPDATE tag SET name = ? WHERE id = ?",
	query.TagSetParent: "UPDATE tag SET parent = ? WHERE id = ?",
	query.TagUpdate:    "UPDATE tag SET name = ?, parent = ? WHERE id = ?",
	query.TagDelete:    "DELETE FROM tag WHERE id = ?",
	query.TagLinkAdd: `
INSERT INTO tag_link (tag_id, item_id)
              VALUES (     ?,       ?)
`,
	query.TagLinkDelete: "DELETE FROM tag_link WHERE tag_id = ? AND item_id = ?",
	query.TagLinkGetByItem: `
SELECT
    t.id,
    t.parent,
    t.name
FROM tag_link l
INNER JOIN tag t ON l.tag_id = t.id
WHERE l.item_id = ?
`,
	query.TagLinkGetByTag: `
SELECT
    i.id,
    i.feed_id,
    i.url,
    i.timestamp,
    i.headline,
    i.description,
    i.rating
FROM tag_link l
INNER JOIN item i ON l.item_id = i.id
WHERE tag_id = ?
`,
}

/*
WITH RECURSIVE children(id, name, description, lvl, root, parent, full_name) AS (
    SELECT
        id,
        name,
        description,
        0 AS lvl,
        id AS root,
        COALESCE(parent, 0) AS parent,
        name AS full_name
    FROM tag WHERE parent IS NULL
    UNION ALL
    SELECT
        tag.id,
        tag.name,
        tag.description,
        lvl + 1 AS lvl,
        children.root,
        tag.parent,
        full_name || '/' || tag.name AS full_name
    FROM tag, children
    WHERE tag.parent = children.id
)

SELECT
        id,
        name,
        description,
        parent,
        lvl,
        full_name
FROM children
ORDER BY full_name;
*/
