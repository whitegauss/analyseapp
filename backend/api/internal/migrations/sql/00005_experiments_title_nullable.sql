-- +goose Up
alter table experiments alter column title drop not null;

-- +goose Down
alter table experiments alter column title set not null;
