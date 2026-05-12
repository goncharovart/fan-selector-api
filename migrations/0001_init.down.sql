-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS fan_curves_q_range;
DROP TABLE IF EXISTS fan_curves;
DROP TABLE IF EXISTS fan_models;

-- +goose StatementEnd
