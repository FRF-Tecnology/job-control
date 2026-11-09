// Perguntas que só a pessoa responde na página (P5, jc-a2y.6.3.3): a tela usa a mesma regra do servidor
// (internal/dominio/sensivel.go, perguntaSensivel). Os casos são os do sensivel_test.go. Só dados fictícios.
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { NOME_MOTIVO_SENSIVEL, perguntaSensivel } from './sensivel';

// Vetores compartilhados com o Go (jc-a2y.6.2.6): o mesmo arquivo que o TestVetoresSensiveis lê. O motivo lá é o do
// rótulo; para a pergunta do pacote, termos e captcha valem "" (a página bloqueia na hora).
const vetores: { casos: { texto: string; motivo: string }[] } =
  JSON.parse(readFileSync(resolve(process.cwd(), '../../internal/dominio/testdata/sensivel.json'), 'utf8'));

describe('perguntaSensivel: vetores compartilhados com o servidor (PT, ES, EN)', () => {
  it('o arquivo tem casos', () => expect(vetores.casos.length).toBeGreaterThan(100));
  it.each(vetores.casos.map(c => [c.texto, c.motivo === 'termos' || c.motivo === 'captcha' ? '' : c.motivo]))(
    '%j → %j', (texto, esperado) => expect(perguntaSensivel(texto)).toBe(esperado));
});

describe('nomes dos motivos (LATAM)', () => {
  it('não usam a sigla brasileira', () => {
    expect(NOME_MOTIVO_SENSIVEL.cpf).toBe('número fiscal');
    expect(NOME_MOTIVO_SENSIVEL.rg).toBe('documento de identidade');
  });
});

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
