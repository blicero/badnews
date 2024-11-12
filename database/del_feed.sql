-- /home/krylon/go/src/github.com/blicero/badnews/database/del_feed.sql
-- Time-stamp: <2024-11-12 14:58:31 krylon>
-- created on 11. 11. 2024 by Benjamin Walkenhorst
-- (c) 2024 Benjamin Walkenhorst
-- Use at your own risk!

WITH links (link_id, item_id, feed_id) AS (
     SELECT l.id,
            l.item_id,
            i.feed_id
     FROM tag_link l
     INNER JOIN item i ON l.item_id = i.id
)

DELETE FROM tag_link
WHERE item_id IN (SELECT item_id FROM links WHERE feed_id = ?)


