-- +goose Up
-- A nota fiscal sobrevive à exclusão da conta (obrigação fiscal): sem a chave estrangeira, usuarios pode
-- ser apagado e notas.usuario_id fica com o id antigo (ninguém mais o usa, o id é um uuid novo a cada conta).
alter table notas drop constraint notas_usuario_id_fkey;

-- +goose Down
alter table notas add constraint notas_usuario_id_fkey foreign key (usuario_id) references usuarios (id);
