-- +goose Up

-- Accented letters used to be thrown away by the search simplification:
-- "God of War Ragnarök" became "god of war ragnar k", and typing "ragnarok" could not
-- match it. unaccent replaces them with their plain letters instead (ö -> o, é -> e,
-- ß -> ss, æ -> ae, ł -> l). Neon supports the extension.
CREATE EXTENSION IF NOT EXISTS unaccent;

-- unaccent() itself is only STABLE: its result depends on a dictionary that could in
-- theory be changed, so PostgreSQL refuses it in generated columns. The usual
-- solution is this wrapper, which names the dictionary explicitly and promises
-- IMMUTABLE. The promise holds as long as nobody edits the unaccent rules file on
-- the server, which we never do. The schema is written out ("public.") because a
-- function used by a generated column must not depend on the search_path setting.
-- +goose StatementBegin
CREATE FUNCTION immutable_unaccent(input text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE RETURNS NULL ON NULL INPUT
AS $$
    SELECT public.unaccent('public.unaccent'::regdictionary, input)
$$;
-- +goose StatementEnd

-- Same rules as before, with accents removed first. The Go side (NormalizeQuery)
-- does the same, and a test compares the two.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION normalize_search(input text) RETURNS text
LANGUAGE sql IMMUTABLE RETURNS NULL ON NULL INPUT
AS $$
    SELECT btrim(regexp_replace(translate(lower(immutable_unaccent(input)), '''’', ''), '[^a-z0-9]+', ' ', 'g'))
$$;
-- +goose StatementEnd

-- Stored generated columns are computed when a row is written, so the existing rows
-- still hold the old values. Writing every row again (to the same name) makes
-- PostgreSQL recompute search_name and search_initials with the new function.
UPDATE products SET name = name;

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION normalize_search(input text) RETURNS text
LANGUAGE sql IMMUTABLE RETURNS NULL ON NULL INPUT
AS $$
    SELECT btrim(regexp_replace(translate(lower(input), '''’', ''), '[^a-z0-9]+', ' ', 'g'))
$$;
-- +goose StatementEnd
UPDATE products SET name = name;
DROP FUNCTION immutable_unaccent(text);
DROP EXTENSION IF EXISTS unaccent;
