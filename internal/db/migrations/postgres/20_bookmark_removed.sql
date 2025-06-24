-- SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE IF NOT EXISTS bookmark_removed (
    uid     varchar(32) NOT NULL,
    user_id integer     NOT NULL,
    deleted timestamptz NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_bookmark_removed_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX bookmark_removed_deleted_idx ON "bookmark_removed" (deleted DESC);

CREATE OR REPLACE FUNCTION bookmark_after_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO bookmark_removed (
        uid, user_id
    ) VALUES (
        OLD.uid, OLD.user_id
    );

    RETURN OLD;
END;
$$;

CREATE TRIGGER bookmark_ad AFTER DELETE on bookmark
    FOR EACH ROW EXECUTE PROCEDURE bookmark_after_delete();
