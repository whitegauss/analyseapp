-- +goose Up
-- Stage 2 of the projects migration (PDR.md section 5): every experiment
-- belongs to exactly one project.
--
-- Done in four steps rather than as one "add not null column", which fails
-- on any database that already holds experiments -- the dev database does,
-- and so will anything restored from it. The column arrives nullable, the
-- rows it needs are backfilled, and only then is the constraint tightened.
alter table experiments
    add column if not exists project_id uuid references projects (id) on delete cascade;

-- One "未分類" (uncategorised) project per user, created only for users who
-- actually have experiments that predate projects. The partial unique index
-- below keeps this to exactly one per user from here on, so the backfill
-- and the API agree on which project that is.
create unique index if not exists projects_user_default_idx
    on projects (user_id, title)
    where title = '未分類';

insert into projects (user_id, title, description)
select distinct e.user_id, '未分類', 'プロジェクト機能の導入前からある実験の置き場所です。'
from experiments e
where e.project_id is null
on conflict do nothing;

-- Written as a scalar subquery rather than "update ... from projects": a
-- user who already had a project called 未分類 before this migration would
-- match twice in a join, and Postgres would pick one of them arbitrarily.
update experiments e
set project_id = (
    select p.id
    from projects p
    where p.user_id = e.user_id and p.title = '未分類'
    order by p.created_at, p.id
    limit 1
)
where e.project_id is null;

alter table experiments alter column project_id set not null;

create index if not exists experiments_project_id_idx on experiments (project_id);

-- +goose Down
drop index if exists experiments_project_id_idx;
alter table experiments drop column if exists project_id;
drop index if exists projects_user_default_idx;
-- The 未分類 projects themselves are deliberately left behind: once the
-- column is gone there is nothing left to say which experiments they held,
-- and deleting by title would take projects the user has since renamed,
-- filled by hand, or created deliberately.
