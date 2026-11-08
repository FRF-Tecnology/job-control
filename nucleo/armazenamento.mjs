// Ponto único para criar o armazém. O núcleo só conhece a interface:
//   { tipo, carregar(), salvar(estado), transacao(fn), backupDiario(hoje) }
// Implementações: arquivos (modo local, hoje) e memoria (testes). Postgres entra na T7.
import { mkdirSync, existsSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { criarArmazemArquivos } from './armazens/arquivos.mjs';
import { criarArmazemMemoria } from './armazens/memoria.mjs';

export { criarArmazemArquivos, criarArmazemMemoria };

// Atalho do modo local (usado pela CLI, pelo servidor e pelos testes antigos).
export const criarArmazem = (dir, opcoes) => criarArmazemArquivos(dir, opcoes);

export function criarArmazemPor({ tipo = 'arquivos', dir, ...opcoes } = {}) {
  if (tipo === 'arquivos') return criarArmazemArquivos(dir, opcoes);
  if (tipo === 'memoria') return criarArmazemMemoria(opcoes);
  throw new Error(`armazém desconhecido: ${tipo}`);
}

// Computador novo: começa com as plataformas conhecidas. Devolve true se preparou algo.
export function prepararPrimeiraVez(dir, modeloDir) {
  mkdirSync(join(dir, 'curriculos'), { recursive: true });
  const destino = join(dir, 'plataformas.json');
  if (existsSync(destino) || !existsSync(join(modeloDir, 'plataformas.json'))) return false;
  copyFileSync(join(modeloDir, 'plataformas.json'), destino);
  return true;
}
