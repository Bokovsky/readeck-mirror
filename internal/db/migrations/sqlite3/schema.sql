-- SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE migration (
    id      integer  PRIMARY KEY,
    name    text     NOT NULL,
    applied datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS user (
    id       integer  PRIMARY KEY AUTOINCREMENT,
    uid      text     UNIQUE NOT NULL,
    created  datetime NOT NULL,
    updated  datetime NOT NULL,
    username text     UNIQUE NOT NULL,
    email    text     UNIQUE NOT NULL,
    password text     NOT NULL,
    `group`  text     NOT NULL DEFAULT "user",
    settings json     NOT NULL DEFAULT "{}",
    seed     integer  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS token (
    id          integer  PRIMARY KEY AUTOINCREMENT,
    uid         text     UNIQUE NOT NULL,
    user_id     integer  NOT NULL,
    created     datetime NOT NULL,
    last_used   datetime NULL,
    expires     datetime NULL,
    is_enabled  integer  NOT NULL DEFAULT 1,
    application text     NOT NULL,
    roles       json     NOT NULL DEFAULT "",

    CONSTRAINT fk_token_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS credential (
    id          integer  PRIMARY KEY AUTOINCREMENT,
    uid         text     UNIQUE NOT NULL,
    user_id     integer  NOT NULL,
    created     datetime NOT NULL,
    last_used   datetime NULL,
    is_enabled  integer  NOT NULL DEFAULT 1,
    name        text     NOT NULL,
    password    text     NOT NULL,
    roles       json     NOT NULL DEFAULT "",

    CONSTRAINT fk_app_password_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bookmark (
    id            integer  PRIMARY KEY AUTOINCREMENT,
    uid           text     UNIQUE NOT NULL,
    user_id       integer  NOT NULL,
    created       datetime NOT NULL,
    updated       datetime NOT NULL,
    is_marked     integer  NOT NULL DEFAULT 0,
    is_archived   integer  NOT NULL DEFAULT 0,
    state         integer  NOT NULL DEFAULT 0,
    url           text     NOT NULL,
    initial_url   text     NOT NULL,
    title         text     NOT NULL,
    domain        text     NOT NULL DEFAULT "",
    site          text     NOT NULL DEFAULT "",
    site_name     text     NOT NULL DEFAULT "",
    published     datetime,
    authors       json     NOT NULL DEFAULT "",
    lang          text     NOT NULL DEFAULT "",
    dir           text     NOT NULL DEFAULT "",
    type          text     NOT NULL DEFAULT "",
    description   text     NOT NULL DEFAULT "",
    text          text     NOT NULL DEFAULT "",
    word_count    integer  NOT NULL DEFAULT 0,
    duration      integer  NOT NULL DEFAULT 0,
    embed         text     NOT NULL DEFAULT "",
    file_path     text     NOT NULL DEFAULT "",
    files         json     NOT NULL DEFAULT "",
    errors        json     NOT NULL DEFAULT "",
    labels        json     NOT NULL DEFAULT "",
    read_progress integer  NOT NULL DEFAULT 0,
    read_anchor   text     NOT NULL DEFAULT "",
    annotations   json     NOT NULL DEFAULT "",
    links         json     NOT NULL DEFAULT "",

    CONSTRAINT fk_bookmark_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX bookmark_created_idx ON "bookmark" (created DESC);
CREATE INDEX bookmark_updated_idx ON "bookmark" (updated DESC);
CREATE INDEX bookmark_url_idx ON "bookmark" (url);
CREATE INDEX bookmark_initial_url_idx ON "bookmark" (initial_url);

CREATE TABLE IF NOT EXISTS bookmark_removed (
    uid     text     NOT NULL,
    user_id integer  NOT NULL,
    deleted datetime NOT NULL,

    CONSTRAINT fk_bookmark_removed_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX bookmark_removed_deleted_idx ON "bookmark_removed" (deleted DESC);

CREATE VIRTUAL TABLE IF NOT EXISTS bookmark_idx USING fts5(
    tokenize='unicode61 remove_diacritics 2',
    content='bookmark',
    content_rowid='id',
    catchall,
    title,
    description,
    text,
    site,
    author,
    label
);

INSERT INTO bookmark_idx(bookmark_idx, rank) VALUES ('rank', 'bm25(0, 12.0, 6.0, 5.0, 2.0, 4.0)');

DROP TRIGGER IF EXISTS bookmark_ai;
CREATE TRIGGER bookmark_ai AFTER INSERT ON bookmark BEGIN
    INSERT INTO bookmark_idx (
        rowid, catchall, title, description, text, site, author, label
    ) VALUES (
        new.id, 'oooooo', new.title, new.description, new.text, new.site_name || ' ' || new.site || ' ' || new.domain, new.authors, new.labels
    );
END;

DROP TRIGGER IF EXISTS bookmark_au;
CREATE TRIGGER bookmark_au AFTER UPDATE ON bookmark BEGIN
    INSERT INTO bookmark_idx(
        bookmark_idx, rowid, catchall, title, description, text, site, author, label
    ) VALUES (
        'delete', old.id, 'oooooo', old.title, old.description, old.text, old.site, old.authors, old.labels
    );
    INSERT INTO bookmark_idx (
        rowid, catchall, title, description, text, site, author, label
    ) VALUES (
        new.id, 'oooooo', new.title, new.description, new.text, new.site_name || ' ' || new.site || ' ' || new.domain, new.authors, new.labels
    );
END;

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

CREATE TABLE IF NOT EXISTS bookmark_collection (
    id          integer  PRIMARY KEY AUTOINCREMENT,
    uid         text     UNIQUE NOT NULL,
    user_id     integer  NOT NULL,
    created     datetime NOT NULL,
    updated     datetime NOT NULL,
    name        text     NOT NULL,
    is_pinned   integer  NOT NULL DEFAULT 0,
    filters     json     NOT NULL DEFAULT "{}",

    CONSTRAINT fk_bookmark_collection_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);
