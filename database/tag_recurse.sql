-- /home/krylon/go/src/github.com/blicero/badnews/database/tag_recurse.sql
-- Time-stamp: <2024-12-13 21:03:29 krylon>
-- created on 11. 12. 2024 by Benjamin Walkenhorst
-- (c) 2024 Benjamin Walkenhorst
-- Use at your own risk!

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
    i.id,
    i.feed_id,
    -- i.url,
    datetime(i.timestamp, 'unixepoch') AS timestamp,
    i.headline
    -- i.description,
    -- i.rating
FROM tag_link l
INNER JOIN item i ON l.item_id = i.id
WHERE l.tag_id IN (SELECT id FROM children WHERE full_name LIKE 'IT/Operating Systems%')
ORDER BY i.timestamp;
