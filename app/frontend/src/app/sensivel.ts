// Perguntas que só a pessoa responde na página (P5, jc-a2y.6.3.3). Mesma regra de internal/dominio/sensivel.go
// (perguntaSensivel): texto normalizado (sem acento, em minúsculas) e os mesmos padrões. O servidor recusa com 400 uma
// resposta dessas no pacote; a tela nem oferece e avisa ao lado da resposta. Se a regra do servidor mudar, mude aqui.
import { normalizar } from '../dominio/modelo';

const PADROES: [motivo: string, re: RegExp][] = [
  ['cpf', /\bcpf\b|cadastro de pessoa fisica/],
  ['rg', /\brg\b|registro geral|carteira de identidade|(numero|documento|cedula) de identidade|\borgao emissor\b/],
  ['senha', /\bsenha\b|\bpassword\b|\bpasscode\b|\bpin\b/],
  ['dado-bancario', /\bagencia\b|\bconta (corrente|bancaria|poupanca|salario)\b|numero da conta|dados bancarios|\bchave pix\b|\bpix\b|\biban\b|\bswift\b|routing number|bank account|account number|\bbanco (para|do) (deposito|pagamento)\b/],
  ['documento', /\bcnh\b|carteira (de motorista|nacional de habilitacao|de trabalho)|\bctps\b|comprovante|certidao|titulo de eleitor|\bpis\b|\bpasep\b|\bnis\b|reservista|passaporte|\bpassport\b|driver'?s license|proof of (address|residence)/],
  ['dado-sensivel', /\braca\b|\betnia\b|\betnic|cor ou raca|cor\/raca|raca\/cor|deficiencia|\bpcd\b|\bpne\b|genero|\bsexo\b|orientacao sexual|identidade de genero|\bsaude\b|\bdoenca|religiao|\breligios|\bcrenca\b|\bgender\b|\brace\b|ethnicity|disabilit|sexual orientation|\bveteran|religion|pronome|pronoun|\btrans(genero)?\b|lgbt/],
];

export const NOME_MOTIVO_SENSIVEL: Record<string, string> = {
  cpf: 'CPF', rg: 'RG', senha: 'senha', 'dado-bancario': 'dado bancário', documento: 'documento', 'dado-sensivel': 'dado sensível',
};

// Motivo ("cpf", "senha"…) quando a resposta a esta pergunta não pode ir no pacote; "" quando pode.
// Aceite de termos e captcha não entram: não são respostas de texto, a página bloqueia na hora.
export function perguntaSensivel(pergunta: string): string {
  const s = normalizar(pergunta ?? '');
  if (!s) return '';
  return PADROES.find(([, re]) => re.test(s))?.[0] ?? '';
}
