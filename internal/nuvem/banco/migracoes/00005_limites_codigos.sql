-- +goose Up
-- Histórico de envios e de códigos errados por email+tipo: o orçamento de envios (5 em 15 min, 20 em 24 h)
-- e o teto de 20 erros em 24 h sobrevivem a reinícios. A limpeza apaga o que passou de 1 dia.
create table eventos_codigo (
    email  text not null,
    tipo   text not null check (tipo in ('confirmar', 'recuperar')),
    evento text not null check (evento in ('envio', 'falha')),
    em     timestamptz not null
);
create index eventos_codigo_email_tipo on eventos_codigo (email, tipo, evento, em);

create index sessoes_usuario on sessoes (usuario_id);

-- +goose Down
drop index sessoes_usuario;
drop table eventos_codigo;
