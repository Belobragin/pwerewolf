-- +goose Up
-- +goose StatementBegin
CREATE schema IF NOT EXISTS wfn AUTHORIZATION wf;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS wfn;
-- +goose StatementEnd
