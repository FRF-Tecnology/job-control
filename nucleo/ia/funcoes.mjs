// Funções de negócio da IA: contrato de entrada e saída (JSON), prompt de sistema e mensagem.
// O núcleo só conhece estas funções; o provedor (Claude, GPT, Gemini, assistente local) é detalhe.
import { MOTIVOS_PULO } from '../modelo.mjs';

const texto = { type: 'string' };
const textos = { type: 'array', items: texto };
const objeto = { type: 'object' };
// Saída: todo objeto fecha com additionalProperties:false (exigência do formato estruturado dos provedores).
const saida = (properties, required = Object.keys(properties)) => ({ type: 'object', properties, required, additionalProperties: false });
const entrada = (properties, required = Object.keys(properties)) => ({ type: 'object', properties, required });

const REGRAS_FIXAS = [
  'Nunca escreva, peça ou repita CPF, RG, senhas ou dados bancários.',
  'Nunca invente experiência: só vale o que está no currículo, em respostasPadrao ou no perfil.',
  'Na dúvida, seja conservador.',
].join('\n');

// Só o que a IA precisa do perfil (menos tokens, nada de caminhos de arquivo).
const CAMPOS_PERFIL = ['cargo', 'experienciaAnos', 'stackPrincipal', 'stackSecundaria', 'ingles', 'formacao', 'cidade', 'contratos',
  'pretensaoCLT', 'pretensaoPJ', 'pretensaoUSD', 'inicio', 'filtros', 'prioridadeVagas', 'respostasPadrao', 'nome', 'linkedin', 'github'];
export function resumoPerfil(perfil = {}) {
  return Object.fromEntries(CAMPOS_PERFIL.filter(k => perfil[k] != null && perfil[k] !== '').map(k => [k, perfil[k]]));
}

const json = o => JSON.stringify(o, null, 1);

export const FUNCOES = {
  triarVaga: {
    entrada: entrada({ vaga: objeto, perfil: objeto }),
    saida: saida({
      decisao: { type: 'string', enum: ['aplicar', 'pular', 'perguntar'] },
      aderencia: { type: 'string', enum: ['A', 'B', 'C'] },
      motivo: texto,
      motivoPulo: { type: 'string', enum: MOTIVOS_PULO },
    }, ['decisao', 'aderencia', 'motivo']),
    sistema: `Você faz a triagem de vagas para uma pessoa candidata.
Compare a vaga com o perfil (nível, modelo de trabalho, stack, inglês, formação, filtros.pularSe, prioridadeVagas).
aderencia: A = encaixa bem, B = encaixa em parte, C = encaixa pouco.
decisao: "aplicar" só se está dentro dos filtros; "pular" se bate em filtros.pularSe (informe motivoPulo); "perguntar" na dúvida.
motivoPulo: "Sênior" = nível acima do perfil (Staff, Principal, Lead, Master, 10+ anos); "Stack" = tecnologia principal fora do perfil;
"Presencial/Local" = exige presencial, híbrido ou morar numa cidade; "Inglês", "Diploma", "Documento" quando for isso; "Outro" no resto
(ex.: pagamento abaixo da pretensão, trabalho de treinar/avaliar IA que não é vaga, exigência de domínio que a pessoa não tem).
Vaga que exige CPF antes de aplicar, conta com senha ou teste pessoal não é "aplicar" direto: use "perguntar".
motivo: uma frase curta.`,
    mensagem: e => `Vaga:\n${json(e.vaga)}\n\nPerfil:\n${json(resumoPerfil(e.perfil))}`,
  },

  responderPergunta: {
    entrada: entrada({
      pergunta: texto,
      tipoCampo: { type: 'string', enum: ['texto', 'numero', 'opcao', 'sim-nao'] },
      opcoes: textos,
      perfil: objeto,
      curriculo: texto,
    }, ['pergunta', 'tipoCampo', 'perfil']),
    saida: saida({
      resposta: texto,
      fonte: { type: 'string', enum: ['curriculo', 'respostasPadrao', 'perfil', 'nenhuma'] },
      confianca: { type: 'number' },
      parar: { type: 'boolean' },
      motivoParar: texto,
    }, ['resposta', 'fonte', 'confianca']),
    extra: s => (s.confianca < 0 || s.confianca > 1 ? ['confianca deve estar entre 0 e 1'] : []),
    sistema: `Você responde perguntas de formulário de candidatura em nome da pessoa.
Use só o currículo, respostasPadrao e o perfil. Diga a fonte usada; se não achou, fonte "nenhuma".
Experiência que não aparece em nenhuma fonte: numérico "0", sim/não "Não" (ou "No"), texto "sem experiência".
Campo "numero": apenas dígitos. Campo "opcao": copie exatamente uma das opções. Campo "sim-nao": Sim/Não (ou Yes/No).
Pretensão: CLT ou PJ conforme o rótulo (cooperado = PJ).
Se a pergunta pede CPF, RG, senha, dados bancários ou criar conta/login: parar = true, resposta vazia e motivoParar.
confianca entre 0 e 1.`,
    mensagem: e => [
      `Pergunta: ${e.pergunta}`,
      `Tipo do campo: ${e.tipoCampo}`,
      e.opcoes?.length ? `Opções: ${json(e.opcoes)}` : '',
      `Perfil:\n${json(resumoPerfil(e.perfil))}`,
      `Currículo:\n${e.curriculo || '(não informado)'}`,
    ].filter(Boolean).join('\n\n'),
  },

  revisarAntesDeEnviar: {
    entrada: entrada({ formulario: { type: 'array', items: entrada({ pergunta: texto }, ['pergunta']) }, perfil: objeto, curriculo: texto }, ['formulario', 'perfil']),
    saida: saida({
      ok: { type: 'boolean' },
      problemas: { type: 'array', items: saida({ pergunta: texto, valorAtual: texto, sugerido: texto, motivo: texto }) },
    }),
    sistema: `Você revisa um formulário de candidatura já preenchido antes do envio.
Aponte respostas erradas: experiência maior que a do currículo, pré-preenchimento errado, número com texto em campo numérico,
pretensão no regime errado, campo de CPF/RG/senha preenchido (sugerido vazio).
ok = true só se não houver problemas. Valores sempre como texto.`,
    mensagem: e => `Formulário:\n${json(e.formulario)}\n\nPerfil:\n${json(resumoPerfil(e.perfil))}\n\nCurrículo:\n${e.curriculo || '(não informado)'}`,
  },

  classificarEmail: {
    entrada: entrada({ email: entrada({ de: texto, assunto: texto, texto }), vagas: { type: 'array', items: objeto } }),
    saida: saida({
      tipo: { type: 'string', enum: ['confirmacao', 'rejeicao', 'entrevista', 'teste', 'oferta', 'outro'] },
      vagaId: { type: ['string', 'null'] },
      resumo: texto,
      proximoPasso: texto,
    }, ['tipo', 'vagaId', 'resumo']),
    sistema: `Você classifica emails de processos seletivos.
tipo: confirmacao (recebemos sua candidatura), rejeicao, entrevista (convite/agendamento), teste, oferta, outro.
vagaId: o id de uma das vagas listadas, ou null se não der para saber. Nunca invente id.
resumo: uma frase. proximoPasso: o que a pessoa precisa fazer, se houver.`,
    mensagem: e => `Email:\n${json(e.email)}\n\nVagas em andamento:\n${json(e.vagas.map(v => ({ id: v.id, empresa: v.empresa, titulo: v.titulo })))}`,
  },

  avaliarEntrevista: {
    entrada: entrada({ transcript: texto, vaga: objeto }),
    saida: saida({ pontosFortes: textos, melhorar: textos, perguntasFeitas: textos, nota: { type: 'integer' } }),
    extra: s => (s.nota < 1 || s.nota > 5 ? ['nota deve ser de 1 a 5'] : []),
    sistema: `Você revisa uma entrevista de emprego já feita, a partir do transcript ou das notas.
Liste pontos fortes, o que melhorar (concreto, acionável) e as perguntas feitas. nota de 1 a 5.
Baseie-se só no texto; não invente falas.`,
    mensagem: e => `Vaga:\n${json(e.vaga)}\n\nTranscript/notas:\n${e.transcript}`,
  },

  escreverMensagem: {
    entrada: entrada({ tipo: { type: 'string', enum: ['follow-up', 'agradecimento', 'recrutador'] }, vaga: objeto, perfil: objeto, idioma: texto }),
    saida: saida({ assunto: texto, texto }, ['texto']),
    sistema: `Você escreve um rascunho curto de mensagem (follow-up, agradecimento ou contato com recrutador) no idioma pedido.
Tom profissional e direto, sem exageros. Não afirme experiência que não está no perfil.
É só um rascunho: a pessoa revisa e envia.`,
    mensagem: e => `Tipo: ${e.tipo}\nIdioma: ${e.idioma}\n\nVaga:\n${json(e.vaga)}\n\nPerfil:\n${json(resumoPerfil(e.perfil))}`,
  },
};

// Prompt completo: regras fixas + instrução da função + esquema (o assistente local não tem saída estruturada).
export function montarPrompt(funcao, ent) {
  const f = FUNCOES[funcao];
  return {
    sistema: `${f.sistema}\n\nRegras que não mudam:\n${REGRAS_FIXAS}\n\nResponda apenas com um objeto JSON válido neste esquema:\n${JSON.stringify(f.saida)}`,
    mensagem: f.mensagem(ent),
    esquema: f.saida,
    nomeEsquema: funcao,
  };
}
