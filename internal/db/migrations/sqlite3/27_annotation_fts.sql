-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

DROP VIEW IF EXISTS bookmark_index_view;
CREATE VIEW bookmark_index_view (
    id,
    title,
    description,
    "text",
    site,
    author,
    label,
    note
)
AS SELECT
    id,
    title,
    description,
    "text",
    site_name || ' ' || site || ' ' || domain AS site,
    (
        SELECT group_concat(x.value, ' ')
        FROM json_each(
            CASE json_type(CASE json_valid(authors) WHEN TRUE THEN authors ELSE '0' END)
            WHEN 'array' THEN authors ELSE '[]' END
        ) AS x
    ) AS author,
    (
      SELECT group_concat(x.value, ' ')
      FROM json_each(
          CASE json_type(CASE json_valid(labels) WHEN TRUE THEN labels ELSE '0' END)
          WHEN 'array' THEN labels ELSE '[]' END
      ) AS x
    ) AS label,
    (
      SELECT group_concat(json_extract(x.value, '$.note'), ' ')
      FROM json_each(
          CASE json_type(CASE json_valid(annotations) WHEN TRUE THEN annotations ELSE '0' END)
          WHEN 'array' THEN annotations
          ELSE '[]' END
      ) AS x
      WHERE json_extract(x.value, '$.note') != ''
    ) AS note
FROM bookmark;

DROP TABLE bookmark_idx;
CREATE VIRTUAL TABLE IF NOT EXISTS bookmark_idx USING fts5(
    tokenize='unicode61 remove_diacritics 2',
    content='',
    contentless_delete=1,
    catchall,
    title,
    description,
    "text",
    site,
    author,
    label,
    note
);

INSERT INTO bookmark_idx(bookmark_idx, rank) VALUES ('rank', 'bm25(0, 12.0, 6.0, 5.0, 2.0, 4.0, 6.0, 10.0)');

DROP TRIGGER IF EXISTS bookmark_ai;
CREATE TRIGGER bookmark_ai AFTER INSERT ON bookmark
BEGIN
    INSERT INTO bookmark_idx (rowid, catchall, title, description, "text", site, author, label, note)
    SELECT id, 'oooooo', title, description, "text", site, author, label, note
    FROM bookmark_index_view
    WHERE id = new.id;
END;

DROP TRIGGER IF EXISTS bookmark_au;
CREATE TRIGGER bookmark_au AFTER UPDATE ON bookmark
BEGIN
    DELETE FROM bookmark_idx WHERE rowid = old.id;

    INSERT INTO bookmark_idx (rowid, catchall, title, description, "text", site, author, label, note)
    SELECT id, 'oooooo', title, description, "text", site, author, label, note
    FROM bookmark_index_view
    WHERE id = new.id;
END;

DROP TRIGGER IF EXISTS bookmark_ad;
CREATE TRIGGER IF NOT EXISTS bookmark_ad AFTER DELETE ON bookmark
BEGIN
    DELETE FROM bookmark_idx WHERE rowid = old.id;

    INSERT INTO bookmark_removed(
        uid, user_id, deleted
    ) VALUES (
        old.uid, old.user_id, datetime('now', 'subsec')
    );
END;

-- Re-index all data
UPDATE bookmark set id = id;