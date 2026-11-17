-- +goose Up
-- Quando o token de renovação foi trocado por um novo. Um token já trocado que volta é reuso
-- (revoga a família); usada_em sozinha não basta, porque nasce preenchida.
alter table sessoes add column substituida_em timestamptz;

-- +goose Down
alter table sessoes drop column substituida_em;
