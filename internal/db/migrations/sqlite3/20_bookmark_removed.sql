-- SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE IF NOT EXISTS bookmark_removed (
    uid     text     NOT NULL,
    user_id integer  NOT NULL,
    deleted datetime NOT NULL,

    CONSTRAINT fk_bookmark_removed_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX bookmark_removed_deleted_idx ON "bookmark_removed" (deleted DESC);

DROP TRIGGER IF EXISTS bookmark_ad;
CREATE TRIGGER IF NOT EXISTS bookmark_ad AFTER DELETE ON bookmark BEGIN
    INSERT INTO bookmark_idx(
        bookmark_idx, rowid, catchall, title, description, text, site, author, label
    ) VALUES (
        'delete', old.id, 'oooooo', old.title, old.description, old.text, old.site, old.authors, old.labels
    );

    INSERT INTO bookmark_removed(
        uid, user_id, deleted
    ) VALUES (
        old.uid, old.user_id, datetime('now', 'subsec')
    );
END;
