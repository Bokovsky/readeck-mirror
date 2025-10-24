-- SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE IF NOT EXISTS oauth2_client (
    id               SERIAL        PRIMARY KEY,
    uid              varchar(32)   UNIQUE NOT NULL,
    created          timestamptz   NOT NULL,
    name             varchar(128)  NOT NULL,
    website          varchar(256)  NULL,
    logo             text          NULL,
    redirect_uris    jsonb         NOT NULL,
    software_id      varchar(128)  NOT NULL,
    software_version varchar(128)  NOT NULL
);

ALTER TABLE token ADD COLUMN client_id integer NULL
CONSTRAINT fk_token_oauth2_client REFERENCES oauth2_client(id) ON DELETE CASCADE;
