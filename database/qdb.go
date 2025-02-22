// /home/krylon/go/src/github.com/blicero/badnews/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-10 23:11:28 krylon>

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
ORDER BY title
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
	query.ItemDeleteByFeed: "DELETE FROM item WHERE feed_id = ?",
	query.ItemExists:       "SELECT COUNT(id) FROM item WHERE url = ?",
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
	query.ItemGetByPeriod: `
SELECT
    id,
    feed_id,
    url,
    timestamp,
    headline,
    description,
    rating
FROM item
WHERE timestamp BETWEEN ? AND ?
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
	query.ItemGetAll: `
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
	query.TagGetSorted: `
WITH RECURSIVE children(id, name, lvl, root, parent, full_name) AS (
    SELECT
        id,
        name,
        0 AS lvl,
        id AS root,
        COALESCE(parent, 0) AS parent,
        name AS full_name
    FROM tag WHERE parent IS NULL
    UNION ALL
    SELECT
        tag.id,
        tag.name,
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
        parent,
        lvl,
        full_name
FROM children
ORDER BY full_name
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
	query.TagLinkDeleteByFeed: `
-- This probably is not the most efficient way to do this.
-- But a) we most likely won't be doing this very often, and
-- b) it should be good enough to get started.
WITH links (link_id, item_id, feed_id) AS (
     SELECT l.id,
            l.item_id,
            i.feed_id
     FROM tag_link l
     INNER JOIN item i ON l.item_id = i.id
)

DELETE FROM tag_link
WHERE item_id IN (SELECT item_id FROM links WHERE feed_id = ?)
`,
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
	query.TagLinkGetByTagHierarchy: `
WITH RECURSIVE children(id, name, lvl, root, parent, full_name) AS (
    SELECT
        id,
        name,
        0 AS lvl,
        id AS root,
        COALESCE(parent, 0) AS parent,
        name AS full_name
    FROM tag WHERE parent IS NULL
    UNION ALL
    SELECT
        tag.id,
        tag.name,
        lvl + 1 AS lvl,
        children.root,
        tag.parent,
        full_name || '/' || tag.name AS full_name
    FROM tag, children
    WHERE tag.parent = children.id
)

SELECT DISTINCT
    i.id,
    i.feed_id,
    i.url,
    i.timestamp,
    i.headline,
    i.description,
    i.rating
FROM tag_link l
INNER JOIN item i ON l.item_id = i.id
WHERE l.tag_id IN (SELECT id FROM children WHERE root = ?)
ORDER BY i.timestamp;
`,
	query.SearchAdd: `
INSERT INTO search (title, time_created, tags, tags_all, query_string, regex)
            VALUES (    ?,            ?,    ?,        ?,            ?,     ?)
RETURNING id
`,
	query.SearchDelete: "DELETE FROM search WHERE id = ?",
	query.SearchGetByID: `
SELECT
    title,
    time_created,
    time_started,
    time_finished,
    status,
    msg,
    tags,
    tags_all,
    query_string,
    regex,
    results
FROM search
WHERE id = ?
`,
	query.SearchGetActive: `
SELECT
    id,
    title,
    time_created,
    time_started,
    status,
    msg,
    tags,
    tags_all,
    query_string,
    regex
FROM search
WHERE time_started IS NOT NULL AND time_finished IS NULL
ORDER BY time_started
`,
	query.SearchGetNextPending: `
SELECT
    id,
    title,
    time_created,
    tags,
    tags_all,
    filter_by_period,
    filter_period_begin,
    filter_period_end,
    query_string,
    regex
FROM search
WHERE time_started IS NULL
ORDER BY time_created
LIMIT 1
`,
	query.SearchGetAll: `
SELECT
    id,
    title,
    time_created,
    time_started,
    time_finished,
    status,
    msg,
    tags,
    tags_all,
    filter_by_period,
    filter_period_begin,
    filter_period_end,
    query_string,
    regex,
    results
FROM search
ORDER BY time_created
`,
	query.SearchStart: "UPDATE search SET time_started = ?, time_finished = NULL WHERE id = ?",
	query.SearchFinish: `
UPDATE search
SET time_finished = ?,
    status = ?,
    msg = ?,
    results = ?
WHERE id = ?
`,
}
