// Perguntas que só a pessoa responde na página (P5, jc-a2y.6.3.3): a tela usa a mesma regra do servidor
// (internal/dominio/sensivel.go, perguntaSensivel). Os casos são os do sensivel_test.go. Só dados fictícios.
import { describe, expect, it } from 'vitest';
import { perguntaSensivel } from './sensivel';

describe('perguntaSensivel (igual ao servidor)', () => {
  it.each([
    ['CPF *', 'cpf'], ['Informe seu CPF', 'cpf'], ['Cadastro de Pessoa Física', 'cpf'],
    ['Número do RG', 'rg'], ['Órgão emissor', 'rg'],
    ['Senha', 'senha'], ['Crie uma senha', 'senha'], ['Password', 'senha'],
    ['Agência', 'dado-bancario'], ['Conta corrente', 'dado-bancario'], ['Chave PIX', 'dado-bancario'], ['Bank account number', 'dado-bancario'],
    ['Anexe sua CNH', 'documento'], ['Comprovante de residência', 'documento'], ['Passaporte', 'documento'], ['Carteira de trabalho (CTPS)', 'documento'],
    ['Raça/cor', 'dado-sensivel'], ['Você é PcD?', 'dado-sensivel'], ['Possui alguma deficiência?', 'dado-sensivel'],
    ['Identidade de gênero', 'dado-sensivel'], ['Orientação sexual', 'dado-sensivel'], ['Religião', 'dado-sensivel'], ['Gender', 'dado-sensivel'], ['Pronouns', 'dado-sensivel'],
  ])('%s → %s', (pergunta, motivo) => expect(perguntaSensivel(pergunta)).toBe(motivo));

  it.each(['Anos com React?', 'Disponibilidade para começar?', 'Nível de inglês', 'Pretensão salarial', 'Por que esta vaga?', ''])(
    '%j não é sensível', pergunta => expect(perguntaSensivel(pergunta)).toBe(''));

  it('aceite de termos e captcha não contam como pergunta do pacote (a página bloqueia na hora)', () => {
    expect(perguntaSensivel('Li e aceito os termos')).toBe('');
    expect(perguntaSensivel('Não sou um robô')).toBe('');
  });
});
