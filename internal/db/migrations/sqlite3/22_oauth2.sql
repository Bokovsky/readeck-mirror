-- SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

CREATE TABLE IF NOT EXISTS oauth2_client (
    id               integer  PRIMARY KEY AUTOINCREMENT,
    uid              text     UNIQUE NOT NULL,
    created          datetime NOT NULL,
    name             text     NOT NULL,
    website          text     NULL,
    logo             text     NULL,
    redirect_uris    json     NOT NULL,
    software_id      text     NOT NULL,
    software_version text     NOT NULL
);

ALTER TABLE token ADD COLUMN client_id integer NULL
CONSTRAINT fk_token_oauth2_client REFERENCES oauth2_client(id) ON DELETE CASCADE;
