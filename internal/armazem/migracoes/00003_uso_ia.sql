-- +goose Up
-- Cada chamada de IA (provedor, modelo, tokens, custo estimado, tempo, regras aplicadas). Nada do conteúdo.
CREATE TABLE uso_ia (
  id    INTEGER PRIMARY KEY AUTOINCREMENT,
  data  TEXT NOT NULL,
  dados TEXT NOT NULL
);

-- +goose Down
DROP TABLE uso_ia;
