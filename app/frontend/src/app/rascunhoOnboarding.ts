import { useEffect, useRef, useState } from 'react';
import { ErroApi, type App } from './contexto';

export type PDFSalvo = { arquivoId: string; nomeExibicao: string; tamanho: number };
export type TentativaExterna = '' | 'instalar-claude' | 'login-claude';
export type Rascunho = { revisao: number; etapa: number; dados: Record<string, unknown>; editados: string[]; curriculo: PDFSalvo | null; tentativaExterna?: TentativaExterna };
type Alteracao = { etapa?: number; dados?: Record<string, unknown>; editados?: string[]; tentativaExterna?: TentativaExterna };
type Situacao = 'abrindo' | 'pronto' | 'salvando' | 'salvo' | 'falha' | 'conflito';
const caminho = '/api/v1/onboarding/';
const janela = { 'X-Applyer-Onboarding': '1' };

function juntar(anterior: Alteracao, atual: Alteracao): Alteracao {
  return { ...anterior, ...atual, dados: { ...anterior.dados, ...atual.dados } };
}

export function useRascunhoOnboarding(api: App['api'], restaurar: (r: Rascunho) => void) {
  const [situacao, indicar] = useState<Situacao>('abrindo');
  const [mensagem, informar] = useState('');
  const revisao = useRef<number | null>(null);
  const pendente = useRef<Alteracao | null>(null);
  const conflito = useRef(false);
  const fila = useRef<Promise<unknown>>(Promise.resolve());
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const versaoLeitura = useRef(0);
  const aplicar = useRef(restaurar);
  aplicar.current = restaurar;

  async function carregar() {
    const pedido = ++versaoLeitura.current;
    indicar('abrindo');
    try {
      const remoto: Rascunho = await api(caminho + 'rascunho');
      if (pedido !== versaoLeitura.current) return;
      if (!Number.isInteger(remoto.revisao)) throw new Error('O aplicativo não confirmou a revisão do rascunho.');
      clearTimeout(timer.current); pendente.current = null; conflito.current = false;
      revisao.current = remoto.revisao;
      aplicar.current(remoto);
      indicar(remoto.revisao ? 'salvo' : 'pronto'); informar('');
    } catch (e) {
      if (pedido !== versaoLeitura.current) return;
      informar(e instanceof Error ? e.message : 'Não foi possível abrir o rascunho.'); indicar(conflito.current ? 'conflito' : 'falha');
    }
  }
  useEffect(() => {
    void carregar();
    return () => { versaoLeitura.current++; clearTimeout(timer.current); };
  }, [api]);

  async function enviarCampos() {
    const pacote = pendente.current;
    if (!pacote) return;
    pendente.current = null;
    try {
      const r = await api(caminho + 'rascunho', 'PATCH', { ...pacote, revisao: revisao.current }, janela);
      revisao.current = r.revisao;
    } catch (e) {
      pendente.current = juntar(pacote, pendente.current ?? {});
      throw e;
    }
  }
  function enfileirar<T>(acao: () => Promise<T>): Promise<T> {
    const proxima = fila.current.then(async () => {
      if (conflito.current || revisao.current == null) throw new Error('Carregue o rascunho salvo antes de continuar.');
      try {
        const resultado = await acao();
        if (!pendente.current) { indicar('salvo'); informar(''); }
        return resultado;
      } catch (e) {
        const mudou = e instanceof ErroApi && e.status === 409;
        conflito.current = mudou;
        indicar(mudou ? 'conflito' : 'falha');
        informar(mudou ? 'O rascunho mudou em outra janela. Suas edições continuam nesta tela.' : e instanceof Error ? e.message : 'Não foi possível salvar o rascunho.');
        throw e;
      }
    });
    fila.current = proxima.catch(() => {});
    return proxima;
  }
  function gravar() {
    clearTimeout(timer.current);
    return enfileirar(enviarCampos);
  }
  function alterar(alteracao: Alteracao) {
    pendente.current = juntar(pendente.current ?? {}, alteracao);
    clearTimeout(timer.current);
    if (conflito.current) return;
    indicar('salvando');
    timer.current = setTimeout(() => { void gravar().catch(() => {}); }, 350);
  }
  function aoSairDoCampo() { if (pendente.current && !conflito.current) void gravar().catch(() => {}); }
  async function documento(metodo: 'PUT' | 'DELETE', corpo: Record<string, unknown>) {
    clearTimeout(timer.current);
    indicar('salvando');
    return enfileirar(async () => {
      await enviarCampos();
      const resposta = await api(caminho + 'curriculo', metodo, { ...corpo, revisao: revisao.current }, janela);
      revisao.current = resposta.revisao;
      return metodo === 'PUT' ? resposta as PDFSalvo : null;
    });
  }
  function repetir() {
    if (revisao.current == null || conflito.current) return carregar();
    return gravar().catch(() => {});
  }
  function avisarConflito() {
    conflito.current = true; indicar('conflito'); informar('O rascunho mudou em outra janela. Suas edições continuam nesta tela.');
  }
  return { situacao, mensagem, revisao, carregar, alterar, gravar, aoSairDoCampo, documento, repetir, avisarConflito };
}
