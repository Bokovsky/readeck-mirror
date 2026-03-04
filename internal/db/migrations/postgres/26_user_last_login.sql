-- SPDX-FileCopyrightText: © 2026 Olivier Meunier <olivier@neokraft.net>
--
-- SPDX-License-Identifier: AGPL-3.0-only

ALTER TABLE "user" ADD COLUMN last_login timestamptz NOT NULL default '0001-01-01T00:00:00Z';
