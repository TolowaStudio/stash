-- +goose Up
ALTER TABLE namespaces
    ADD COLUMN merged_into BIGINT NULL REFERENCES namespaces(id),
    ADD COLUMN normalized_slug TEXT NOT NULL DEFAULT '';

-- Backfill normalized_slug (lowercase; strip -, _, /, and .com).
UPDATE namespaces
SET normalized_slug = lower(
    replace(
        replace(
            replace(
                replace(slug, '-', ''),
                '_', ''
            ),
            '/', ''
        ),
        '.com', ''
    )
);

CREATE INDEX namespaces_merged_into_idx ON namespaces (merged_into) WHERE merged_into IS NOT NULL;
CREATE INDEX namespaces_normalized_slug_active_idx
    ON namespaces (normalized_slug)
    WHERE merged_into IS NULL AND normalized_slug <> '';

CREATE TABLE namespace_merges (
    id                  BIGSERIAL   PRIMARY KEY,
    from_namespace_id   BIGINT      NOT NULL REFERENCES namespaces(id),
    into_namespace_id   BIGINT      NOT NULL REFERENCES namespaces(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_namespace_id <> into_namespace_id)
);

CREATE TABLE namespace_merge_moves (
    merge_id    BIGINT  NOT NULL REFERENCES namespace_merges(id) ON DELETE CASCADE,
    table_name  TEXT    NOT NULL,
    row_id      BIGINT  NOT NULL,
    PRIMARY KEY (merge_id, table_name, row_id)
);

CREATE INDEX namespace_merge_moves_merge_id_idx ON namespace_merge_moves (merge_id);

-- +goose Down
DROP TABLE IF EXISTS namespace_merge_moves;
DROP TABLE IF EXISTS namespace_merges;
DROP INDEX IF EXISTS namespaces_normalized_slug_active_idx;
DROP INDEX IF EXISTS namespaces_merged_into_idx;
ALTER TABLE namespaces DROP COLUMN IF EXISTS normalized_slug;
ALTER TABLE namespaces DROP COLUMN IF EXISTS merged_into;