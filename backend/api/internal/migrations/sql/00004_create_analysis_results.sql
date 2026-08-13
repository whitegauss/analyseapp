-- +goose Up
create table if not exists analysis_results (
    id uuid primary key default gen_random_uuid(),
    experiment_id uuid not null references experiments (id) on delete cascade,
    analysis_type text not null,
    parameters jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create index if not exists analysis_results_experiment_id_idx on analysis_results (experiment_id);

-- +goose Down
drop table if exists analysis_results;
