-- +goose Up
-- Os webhooks acham o usuário pelo cliente Stripe: busca rápida e um cliente para um usuário só.
create unique index usuarios_stripe_cliente on usuarios (stripe_cliente);

-- +goose Down
drop index usuarios_stripe_cliente;
