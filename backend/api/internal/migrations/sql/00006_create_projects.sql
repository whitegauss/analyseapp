-- +goose Up
create table if not exists projects (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references profiles (id) on delete cascade,
    title text not null,
    description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists projects_user_id_idx on projects (user_id);

-- +goose Down
drop table if exists projects;
