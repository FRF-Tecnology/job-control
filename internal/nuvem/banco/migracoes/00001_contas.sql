-- +goose Up
-- Esquema `applyer` (criado por banco.Abrir). E-mails são gravados sempre normalizados (minúsculas, sem espaços).
create table usuarios (
    id                  uuid primary key default gen_random_uuid(),
    email               text not null unique,
    nome                text not null default '',
    senha_hash          text,
    google_sub          text unique,
    email_confirmado_em timestamptz,
    teste_inicio        timestamptz,
    teste_fim           timestamptz,
    cortesia            boolean not null default false,
    cortesia_motivo     text not null default '',
    stripe_cliente      text,
    termos_versao       text not null default '',
    termos_aceitos_em   timestamptz,
    falhas_login        integer not null default 0,
    bloqueado_ate       timestamptz,
    criado_em           timestamptz not null default now()
);

create table codigos (
    id         uuid primary key default gen_random_uuid(),
    email      text not null,
    tipo       text not null check (tipo in ('confirmar', 'recuperar')),
    hash       text not null,
    expira_em  timestamptz not null,
    tentativas integer not null default 0,
    usado_em   timestamptz,
    criado_em  timestamptz not null default now()
);
create index codigos_email_tipo on codigos (email, tipo);

create table sessoes (
    id               uuid primary key default gen_random_uuid(),
    usuario_id       uuid not null references usuarios (id) on delete cascade,
    familia          uuid not null,
    hash_renovacao   text not null unique,
    dispositivo_hash text not null default '',
    criada_em        timestamptz not null default now(),
    usada_em         timestamptz not null default now(),
    revogada_em      timestamptz
);
create index sessoes_familia on sessoes (familia);

create table computadores_teste (
    hash       text primary key,
    usuario_id uuid references usuarios (id) on delete set null,
    usado_em   timestamptz not null default now()
);

-- Espelho do Stripe, atualizado pelos webhooks.
create table assinaturas (
    usuario_id        uuid primary key references usuarios (id) on delete cascade,
    stripe_assinatura text not null default '',
    estado            text not null,
    periodo_fim       timestamptz,
    atualizada_em     timestamptz not null default now()
);

-- Para processar cada webhook uma vez só.
create table eventos_stripe (
    event_id    text primary key,
    recebido_em timestamptz not null default now()
);

-- Notas fiscais: sem cascade, a nota sobrevive à conta (obrigação fiscal).
create table notas (
    invoice_id text primary key,
    usuario_id uuid references usuarios (id),
    numero     text not null default '',
    estado     text not null,
    link_pdf   text not null default '',
    erro       text not null default '',
    tentativas integer not null default 0,
    criada_em  timestamptz not null default now()
);

-- +goose Down
drop table notas;
drop table eventos_stripe;
drop table assinaturas;
drop table computadores_teste;
drop table sessoes;
drop table codigos;
drop table usuarios;
