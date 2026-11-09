// Perguntas que só a pessoa responde na página (P5, jc-a2y.6.3.3 e .6.3.4). Mesma regra de internal/dominio/sensivel.go
// (perguntaSensivel), em português, espanhol e inglês: texto normalizado (sem acento, em minúsculas, aparado) e os
// mesmos padrões, na mesma ordem. O servidor recusa com 400 uma resposta dessas no pacote; a tela nem oferece e avisa
// ao lado da resposta. O sensivel.test.ts confere contra internal/dominio/testdata/sensivel.json, o mesmo arquivo do Go.
// Termos e captcha ficam de fora: no Go vêm depois destes e, para a pergunta do pacote, valem "".
import { normalizar } from '../dominio/modelo';

const PADROES: [motivo: string, re: RegExp][] = [
  ['cpf', /\bcpf\b|\bcnpj\b|cadastro de pessoa (fisica|juridica)|\brfc\b|\bcuit\b|\bcuil\b|\brut\b|\bruc\b|\bnit\b|\bnif\b|\bssn\b|social security|seguro social|seguridad social|\bitin\b|tax (id|identification|number)|taxpayer|identificacion (fiscal|tributaria)|numero (de identificacion )?(fiscal|tributario)|registro federal de contribuyentes/],
  ['rg', /\brg\b|registro geral|carteira de identidade|(numero|documento|cedula) de identidade|\borgao emissor\b|\bdni\b|documento nacional de identidad|\bcurp\b|clave unica de registro|\bcedula\b|documento de identidad|carnet de identidad|\bcredencial (para votar|de elector)\b|numero de documento|national (id|identity|identification)|identity (card|document|number)|\bid (card|number)\b|government[- ]issued id/],
  ['senha', /\bsenha\b|\bpassword\b|\bpasscode\b|\bpin\b|\bcontrasena\b|\bclave (de acceso|secreta|personal)\b|^clave\b[\s*:]*$/],
  ['dado-bancario', /\bagencia\b|\bconta (corrente|bancaria|poupanca|salario)\b|numero da conta|dados bancarios|\bchave pix\b|\bpix\b|\biban\b|\bswift\b|\bbic\b|routing number|\baba number\b|sort code|bank account|account number|bank details|\bbanco (para|do) (deposito|pagamento)\b|cuenta (bancaria|corriente|de ahorros?)|numero de cuenta|datos bancarios|\bclabe\b|\bcbu\b|\bcvu\b|\bsucursal bancaria\b/],
  ['documento', /\bcnh\b|carteira (de motorista|nacional de habilitacao|de trabalho)|\bctps\b|comprovante|comprobante|certidao|certificado de antecedentes|antecedentes (penales|criminais)|titulo de eleitor|\bpis\b|\bpasep\b|\bnis\b|reservista|passaporte|pasaporte|\bpassport\b|driver'?s licen[cs]e|licencia de (conducir|manejo)|proof of (address|residence)/],
  ['dado-sensivel', /\braca\b|\braza\b|\betnia\b|\betnic|cor ou raca|cor\/raca|raca\/cor|deficiencia|discapacidad|\bpcd\b|\bpne\b|genero|\bsexo\b|orientacao sexual|orientacion sexual|identidade de genero|identidad de genero|\bsaude\b|\bsalud\b|\bdoenca|enfermedad|embaraz|gravidez|gestante|pregnan|religiao|religion|\breligios|\bcrenca\b|\bcreencia|\bgender\b|\brace\b|ethnicity|disabilit|sexual orientation|\bveteran|pronome|pronoun|\btrans(genero)?\b|lgbt|data de nascimento|fecha de nacimiento|date of birth|birth ?date|\bdob\b|\bidade\b|\bedad\b|how old are you|\byour age\b|^age\b[\s*:]*$|estado civil|marital status/],
];

// Nomes para quem é do Brasil e do resto da América Latina: o motivo "cpf" cobre RFC, CUIT, RUT, SSN…, e "rg" cobre
// DNI, CURP, cédula… Por isso a tela não usa a sigla brasileira.
export const NOME_MOTIVO_SENSIVEL: Record<string, string> = {
  cpf: 'número fiscal', rg: 'documento de identidade', senha: 'senha', 'dado-bancario': 'dado bancário', documento: 'documento', 'dado-sensivel': 'dado sensível',
};

// Motivo ("cpf", "senha"…) quando a resposta a esta pergunta não pode ir no pacote; "" quando pode.
// Aceite de termos e captcha não entram: não são respostas de texto, a página bloqueia na hora.
export function perguntaSensivel(pergunta: string): string {
  const s = normalizar(pergunta ?? '');
  if (!s) return '';
  return PADROES.find(([, re]) => re.test(s))?.[0] ?? '';
}
