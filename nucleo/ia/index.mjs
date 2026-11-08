// Adaptador de IA: o núcleo chama funções de negócio; aqui se escolhe o provedor (rota por função),
// cai para a reserva se o provedor falhar, passa a saída pela guarda e mede cada chamada.
import * as anthropic from './provedores/anthropic.mjs';
import * as openai from './provedores/openai.mjs';
import * as google from './provedores/google.mjs';
import * as assistenteLocal from './provedores/assistente-local.mjs';
import { FUNCOES, montarPrompt } from './funcoes.mjs';
import { guardar, validarEntrada, REGRAS } from './guarda.mjs';
import { custoEstimado, PRECOS } from './precos.mjs';
import { ErroIA } from './erro.mjs';

export { ErroIA, REGRAS, FUNCOES, PRECOS };

const PROVEDORES = { anthropic, openai, google, 'assistente-local': assistenteLocal };

// Resposta do assistente local para um pedido pendente: passa pela mesma guarda.
export function concluirPendente(pendente, resposta) {
  if (!pendente?.pendente || !FUNCOES[pendente.funcao]) throw new ErroIA('entrada', 'pedido pendente inválido');
  return guardar(pendente.funcao, pendente.entrada, resposta).saida;
}

export function criarIA({ config = {}, fetch = globalThis.fetch, registrarUso, agora = () => new Date().toISOString(), env = process.env, precos = PRECOS } = {}) {
  const rotaDe = funcao => config.rotas?.[funcao] ?? config.rotas?.padrao ?? { provedor: 'assistente-local' };

  async function executar(funcao, entrada, rota, prompt) {
    const inicio = performance.now();
    const reg = { funcao, provedor: rota.provedor, modelo: rota.modelo ?? null, tokensEntrada: null, tokensSaida: null, custoEstimadoUSD: null, ms: 0, ok: false, data: agora() };
    const vagaId = entrada.vaga?.id ?? entrada.vagaId;
    if (vagaId) reg.vagaId = vagaId;
    try {
      const prov = PROVEDORES[rota.provedor];
      if (!prov) throw new ErroIA('config', `provedor desconhecido: ${rota.provedor}`);
      if (prov !== assistenteLocal && !rota.modelo) throw new ErroIA('config', `rota de ${funcao} sem modelo para ${rota.provedor}`);
      const opcoes = config.provedores?.[rota.provedor] ?? {};
      const chave = opcoes.chave ?? (prov.variavelChave ? env[prov.variavelChave] : undefined);
      const r = await prov.chamar({ fetch, chave, modelo: rota.modelo, opcoes, funcao, entrada, ...prompt });
      if (r.pendente) {
        Object.assign(reg, { ok: true, pendente: true, custoEstimadoUSD: 0 });
        return r;
      }
      Object.assign(reg, { tokensEntrada: r.tokensEntrada, tokensSaida: r.tokensSaida, custoEstimadoUSD: custoEstimado(rota.modelo, r.tokensEntrada, r.tokensSaida, precos) });
      const { saida, aplicadas } = guardar(funcao, entrada, r.texto);
      reg.ok = true;
      if (aplicadas.length) reg.regras = aplicadas;
      return saida;
    } catch (e) {
      reg.erro = e.tipo ? `${e.tipo}${e.regra ? `:${e.regra}` : ''}` : 'erro';
      throw e;
    } finally {
      reg.ms = Math.round(performance.now() - inicio);
      // Falha ao medir não pode derrubar a resposta da IA.
      try { await registrarUso?.(reg); } catch (e) { console.error(`medição de uso falhou: ${e.message}`); }
    }
  }

  async function rodar(funcao, entrada) {
    validarEntrada(funcao, entrada);
    const prompt = montarPrompt(funcao, entrada);
    const rota = rotaDe(funcao);
    try {
      return await executar(funcao, entrada, rota, prompt);
    } catch (e) {
      // reserva só quando o provedor caiu; bloqueio da guarda não é contornado trocando de modelo
      if (!(e.tipo === 'provedor' && e.reintentavel && rota.reserva)) throw e;
      return executar(funcao, entrada, rota.reserva, prompt);
    }
  }

  const ia = Object.fromEntries(Object.keys(FUNCOES).map(f => [f, entrada => rodar(f, entrada)]));
  ia.concluir = concluirPendente;
  return ia;
}

// Soma registros de uso por um campo: 'vagaId' (candidatura), 'funcao', 'provedor', 'modelo'.
export function somarUso(registros, porCampo = 'funcao') {
  const total = {};
  for (const r of registros) {
    const t = (total[r[porCampo] ?? '(sem)'] ??= { chamadas: 0, falhas: 0, tokensEntrada: 0, tokensSaida: 0, custoEstimadoUSD: 0, custoDesconhecido: 0, ms: 0 });
    t.chamadas++;
    if (!r.ok) t.falhas++;
    t.tokensEntrada += r.tokensEntrada ?? 0;
    t.tokensSaida += r.tokensSaida ?? 0;
    t.ms += r.ms ?? 0;
    if (r.custoEstimadoUSD == null) t.custoDesconhecido++;
    else t.custoEstimadoUSD = Number((t.custoEstimadoUSD + r.custoEstimadoUSD).toFixed(6));
  }
  return total;
}
