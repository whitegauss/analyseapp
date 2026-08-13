-- +goose Up
create table if not exists experiments (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references profiles (id) on delete cascade,
    title text not null,
    raw_data jsonb not null default '{}'::jsonb,
    config jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists experiments_user_id_idx on experiments (user_id);

-- +goose Down
drop table if exists experiments;
