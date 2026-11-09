// O que as telas recebem da casca do app: o estado e as ações (porte do objeto `app` de web/app.mjs).
import { createContext, useContext } from 'react';

// O estado vem de GET /api/v1/estado (vagas, eventos, plataformas, rodadas, perfil, pedidos, emails...).
export type Estado = any;

export interface Pergunta {
  titulo: string;
  texto?: string;
  opcoes?: string[];
  campo?: string;
  valor?: string;
  confirmar?: string;
  multilinha?: boolean;
}

export interface App {
  estado: Estado;
  hoje: string;
  foco: string;
  rota: string;
  vaga: string | null;
  mudarFoco(foco: string): Promise<void>;
  api(caminho: string, metodo?: string, corpo?: unknown): Promise<any>;
  agir(fn: () => Promise<unknown>, mensagem?: string): Promise<boolean>;
  mover(id: string, etapa: string, extras?: Record<string, unknown>, mensagem?: string): Promise<boolean>;
  abrirVaga(id: string): void;
  navegar(rota: string): void;
  aviso(texto: string, erro?: boolean): void;
  perguntar(p: Pergunta): Promise<string | null>;
  recarregar(): Promise<void>;
  limpar(): void; // nada mais por salvar na tela (a atualização automática volta)
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
export async function chamarApi(caminho: string, metodo = 'GET', corpo?: unknown) {
  const r = await fetch(caminho, { method: metodo, headers: { 'content-type': 'application/json' }, body: corpo !== undefined ? JSON.stringify(corpo) : undefined });
  const dados = r.headers.get('content-type')?.includes('json') ? await r.json() : await r.text();
  if (!r.ok) throw new ErroApi(dados?.erros?.join('; ') || dados?.erro || `erro ${r.status}`, r.status, dados);
  return dados;
}
