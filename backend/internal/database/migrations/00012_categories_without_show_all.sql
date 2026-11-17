-- +goose Up

-- The Categories block already has "Load more"; a second "Show all" button under it
-- was redundant (eneba.com has only "Load more"). A content change: no frontend change.
UPDATE home_blocks SET show_all_url = NULL WHERE type = 'category_grid';

-- +goose Down
UPDATE home_blocks SET show_all_url = '/store' WHERE type = 'category_grid';
