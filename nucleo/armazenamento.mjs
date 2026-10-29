// Guarda o estado em arquivos JSON dentro de dados/. Cada leitura vai ao disco
// (a tela e o Claude gravam nos mesmos arquivos) e cada gravação é atômica.
import { readFileSync, writeFileSync, renameSync, existsSync, mkdirSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { estadoVazio } from './servico.mjs';

const ARQUIVOS = { vagas: 'vagas.json', eventos: 'eventos.json', plataformas: 'plataformas.json', rodadas: 'rodadas.json', perfil: 'perfil.json' };

export function criarArmazem(dir) {
  mkdirSync(dir, { recursive: true });

  function lerArquivo(nome, padrao) {
    const caminho = join(dir, nome);
    if (!existsSync(caminho)) return padrao;
    try {
      return JSON.parse(readFileSync(caminho, 'utf8'));
    } catch (e) {
      throw new Error(`não consegui ler ${nome} (${e.message}). Corrija o arquivo ou restaure de dados/backups/.`);
    }
  }

  function gravarArquivo(nome, valor) {
    const caminho = join(dir, nome);
    const tmp = `${caminho}.${process.pid}.tmp`;
    writeFileSync(tmp, JSON.stringify(valor, null, 2) + '\n', 'utf8');
    renameSync(tmp, caminho);
  }

  function carregar() {
    const vazio = estadoVazio();
    return Object.fromEntries(Object.entries(ARQUIVOS).map(([k, nome]) => [k, lerArquivo(nome, vazio[k])]));
  }

  function salvar(estado, antes) {
    for (const [k, nome] of Object.entries(ARQUIVOS)) {
      if (!antes || JSON.stringify(antes[k]) !== JSON.stringify(estado[k])) gravarArquivo(nome, estado[k]);
    }
  }

  // fn recebe o estado atual e devolve { estado, ...resto }. Se lançar, nada é gravado.
  function transacao(fn) {
    const antes = carregar();
    const resultado = fn(antes);
    salvar(resultado.estado, antes);
    return resultado;
  }

  function backupDiario(hoje) {
    const destino = join(dir, 'backups', hoje);
    if (existsSync(destino)) return false;
    mkdirSync(destino, { recursive: true });
    for (const nome of Object.values(ARQUIVOS)) {
      if (existsSync(join(dir, nome))) copyFileSync(join(dir, nome), join(destino, nome));
    }
    return true;
  }

  return { dir, carregar, salvar, transacao, backupDiario };
}
