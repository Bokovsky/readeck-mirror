-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE bookmark_search ADD COLUMN note tsvector NULL;

DROP INDEX bookmark_search_all_idx;
CREATE INDEX bookmark_search_all_idx  ON bookmark_search USING GIN((title || description || "text" || site || "label" || note));
CREATE INDEX bookmark_search_note_idx ON bookmark_search USING GIN (note);

CREATE OR REPLACE FUNCTION bookmark_search_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM bookmark_search WHERE bookmark_id = OLD.id;

    IF tg_op = 'UPDATE' OR tg_op = 'INSERT' THEN
        INSERT INTO bookmark_search (
            bookmark_id, title, description, "text", site, author, "label", note
        ) VALUES (
            NEW.id,
            setweight(to_tsvector('ts', NEW.title), 'A'),
            to_tsvector('ts', NEW.description),
            to_tsvector('ts', NEW."text"),
            to_tsvector('ts',
                NEW.site_name || ' ' || NEW.domain || ' ' ||
                REGEXP_REPLACE(NEW.site, '^www\.', '') || ' ' ||
                REPLACE(NEW.domain, '.', ' ') ||
                REPLACE(REGEXP_REPLACE(NEW.site, '^www\.', ''), '.', ' ')
            ),
            coalesce(jsonb_to_tsvector('ts', NEW.authors, '["string"]'), ''),
            setweight(coalesce(jsonb_to_tsvector('ts', NEW.labels, '["string"]'), ''), 'A'),
            setweight((
                SELECT coalesce(jsonb_to_tsvector('ts', jsonb_agg(x->>'note'), '["string"]'), '')
                FROM jsonb_array_elements(
                    CASE jsonb_typeof(NEW.annotations)
                    WHEN 'array' then NEW.annotations
                    ELSE '[]' END
                ) as x
                WHERE x->>'note' <> ''
            ), 'A')
        );
    END IF;
    RETURN NEW;
END;
$$;

-- Re-index all data
UPDATE bookmark SET id = id;
