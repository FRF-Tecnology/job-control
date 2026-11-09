-- +goose Up
-- Dados da conta: cada coleção (vagas, eventos, plataformas, rodadas, pedidos, emails, perfil)
-- é uma lista de objetos JSON, na mesma ordem do Node. perfil é a coleção com um item só (ordem 0).
CREATE TABLE itens (
  colecao TEXT    NOT NULL,
  ordem   INTEGER NOT NULL,
  dados   TEXT    NOT NULL,
  PRIMARY KEY (colecao, ordem)
) WITHOUT ROWID;

-- +goose Down
DROP TABLE itens;
