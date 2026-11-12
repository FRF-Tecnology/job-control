-- +goose Up
-- Uma conta com o perfil único anterior continua usando apenas a linha de perfil.
CREATE TABLE perfis_busca (
  ordem INTEGER PRIMARY KEY,
  dados TEXT NOT NULL
);
CREATE UNIQUE INDEX perfis_busca_id ON perfis_busca(json_extract(dados, '$.id'));

-- +goose Down
DROP TABLE perfis_busca;
