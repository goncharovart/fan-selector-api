-- +goose Up
-- +goose StatementBegin

CREATE TABLE fan_models (
    id              text PRIMARY KEY,
    manufacturer    text NOT NULL,
    series          text NOT NULL,
    size            text NOT NULL,
    rpm             integer NOT NULL,
    impeller_d_mm   integer,
    metadata        jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE fan_curves (
    fan_id          text PRIMARY KEY REFERENCES fan_models(id) ON DELETE CASCADE,
    q_min_m3h       real NOT NULL,
    q_max_m3h       real NOT NULL,
    p_coeffs        real[] NOT NULL,
    n_coeffs        real[] NOT NULL,
    fitted_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (q_min_m3h < q_max_m3h),
    CHECK (array_length(p_coeffs, 1) >= 1),
    CHECK (array_length(n_coeffs, 1) >= 1)
);

-- Range index lets the prefilter cheaply find fans whose envelope brackets a target Q.
CREATE INDEX fan_curves_q_range
    ON fan_curves
    USING gist (numrange(q_min_m3h::numeric, q_max_m3h::numeric, '[]'));

-- +goose StatementEnd
