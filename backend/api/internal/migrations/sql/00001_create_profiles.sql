-- +goose Up
create extension if not exists pgcrypto;

create table if not exists profiles (
    id uuid primary key references auth.users (id) on delete cascade,
    created_at timestamptz not null default now()
);

-- +goose Down
drop table if exists profiles;
