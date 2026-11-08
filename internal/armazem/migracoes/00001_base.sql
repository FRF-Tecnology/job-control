-- +goose Up
-- Contador de versão: sobe a cada gravação; a tela consulta para saber se recarrega.
CREATE TABLE versao (
  id    INTEGER PRIMARY KEY CHECK (id = 1),
  valor INTEGER NOT NULL
);
INSERT INTO versao (id, valor) VALUES (1, 0);

-- Configurações soltas do app (chave → valor JSON).
CREATE TABLE meta (
  chave TEXT PRIMARY KEY,
  valor TEXT NOT NULL
);

-- +goose Down
DROP TABLE meta;
DROP TABLE versao;
