-- +goose Up
create table if not exists api_keys (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references profiles (id) on delete cascade,
    key_hash text not null,
    name text not null,
    created_at timestamptz not null default now(),
    last_used_at timestamptz
);

create index if not exists api_keys_user_id_idx on api_keys (user_id);

-- +goose Down
drop table if exists api_keys;
