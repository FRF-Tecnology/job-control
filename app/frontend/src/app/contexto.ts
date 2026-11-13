// O que as telas recebem da casca do app: o estado e as ações (porte do objeto `app` de web/app.mjs).
import { createContext, useContext } from 'react';
import { EVENTO_TRAVA, fraseDaTrava, type Situacao } from './conta';

// O estado vem de GET /api/v1/estado (vagas, eventos, plataformas, rodadas, perfil, pedidos, emails...).
export type Estado = any;

export interface Pergunta {
  titulo: string;
  texto?: string;
  itens?: string[]; // lista mostrada como está, para a pessoa conferir (ex.: vagas a aprovar de uma vez)
  opcoes?: string[];
  campo?: string;
  valor?: string;
  confirmar?: string;
  multilinha?: boolean;
}

export interface App {
  estado: Estado;
  conexaoIA?: { nome: string; verificada: boolean };
  hoje: string;
  foco: string;
  rota: string;
  vaga: string | null;
  mudarFoco(foco: string): Promise<void>;
  // cabecalhos: só os da confirmação de uma proposta de pedido (X-Pedido, X-Impressao; jc-a2y.7.1).
  api(caminho: string, metodo?: string, corpo?: unknown, cabecalhos?: Record<string, string>): Promise<any>;
  agir(fn: () => Promise<unknown>, mensagem?: string): Promise<boolean>;
  mover(id: string, etapa: string, extras?: Record<string, unknown>, mensagem?: string): Promise<boolean>;
  abrirVaga(id: string): void;
  navegar(rota: string): void;
  aviso(texto: string, erro?: boolean): void;
  perguntar(p: Pergunta): Promise<string | null>;
  recarregar(): Promise<void>;
  limpar(): void; // nada mais por salvar na tela (a atualização automática volta)
  // Conta (Task 7). podeAgir false: a trava das ações está ligada (as telas ficam só para ver). Ausentes: tudo liberado.
  podeAgir?: boolean;
  conta?: Situacao | null;
  releConta?(): Promise<void>;
}

export const Contexto = createContext<App | null>(null);

export function useApp(): App {
  const app = useContext(Contexto);
  if (!app) throw new Error('useApp fora do App');
  return app;
}

export class ErroApi extends Error {
  constructor(mensagem: string, public status: number, public dados: any) {
    super(mensagem);
  }
}

// Chamada à API local. Erro de validação vira mensagem com a lista de erros.
export async function chamarApi(caminho: string, metodo = 'GET', corpo?: unknown, cabecalhos?: Record<string, string>) {
  const multipart = corpo instanceof FormData;
  const headers = multipart ? cabecalhos : { ...cabecalhos, 'content-type': 'application/json' };
  const body = multipart ? corpo : corpo !== undefined ? JSON.stringify(corpo) : undefined;
  const r = await fetch(caminho, { method: metodo, headers, body });
  // 204 (sair, cancelar o Google, excluir a conta): sem corpo, mesmo que o servidor diga json.
  const dados = r.status === 204 ? '' : r.headers.get('content-type')?.includes('json') ? await r.json() : await r.text();
  // 402: a trava das ações (o teste acabou, sem assinatura, sem internet ou sem conta). A casca relê a conta e mostra a faixa.
  if (r.status === 402) window.dispatchEvent(new CustomEvent(EVENTO_TRAVA, { detail: dados?.motivo ?? '' }));
  if (!r.ok) throw new ErroApi(dados?.erros?.join('; ') || dados?.erro || `erro ${r.status}`, r.status, dados);
  return dados;
}

// id do texto da faixa da conta: os botões travados apontam para ele (aria-describedby).
export const ID_FAIXA = 'faixa-conta-texto';

export interface PropsTrava { disabled: boolean; title?: string; 'aria-describedby'?: string }

// useTrava (ruling A13): só as ações de ESCRITA usam. Com a trava ligada (podeAgir false), o botão fica desabilitado
// com a frase da faixa; ver, filtrar, abrir vagas, pausar e sair continuam livres. O que escapar, o servidor pega (402).
// trava(outro): junta com o disabled que o botão já tinha. Sem contexto (ou sem conta), nada muda.
export function useTrava() {
  const app = useContext(Contexto);
  const travado = app?.podeAgir === false;
  const frase = travado ? fraseDaTrava(app?.conta?.motivo || '') : '';
  const trava = (outro = false): PropsTrava => (travado ? { disabled: true, title: frase, 'aria-describedby': ID_FAIXA } : { disabled: outro });
  return Object.assign(trava, { travado, frase });
}
