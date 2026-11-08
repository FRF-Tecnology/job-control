// Armazém em arquivos JSON dentro de dados/ (modo local). Cada leitura vai ao disco
// (a tela e o Claude gravam nos mesmos arquivos) e cada gravação é atômica.
import { readFileSync, writeFileSync, renameSync, existsSync, mkdirSync, copyFileSync, openSync, closeSync, unlinkSync, statSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { estadoVazio } from '../servico.mjs';
import { hojeISO } from '../modelo.mjs';

const ARQUIVOS = { vagas: 'vagas.json', eventos: 'eventos.json', plataformas: 'plataformas.json', rodadas: 'rodadas.json', perfil: 'perfil.json', pedidos: 'pedidos.json' };

// Espera síncrona curta (usada só enquanto outro processo grava).
const dormir = ms => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);

export function criarArmazemArquivos(dir, { esperaTravaMs = 5000, travaVelhaMs = 15000, hoje = () => hojeISO() } = {}) {
  mkdirSync(dir, { recursive: true });
  const arqTrava = join(dir, '.trava');

  // Uma gravação por vez entre a tela (servidor) e o Claude (CLI).
  function travar() {
    const limite = Date.now() + esperaTravaMs;
    for (;;) {
      try {
        closeSync(openSync(arqTrava, 'wx'));
        return;
      } catch (e) {
        if (e.code !== 'EEXIST') throw e;
        try {
          if (Date.now() - statSync(arqTrava).mtimeMs > travaVelhaMs) { rmSync(arqTrava, { force: true }); continue; }
        } catch {}
        if (Date.now() > limite) throw new Error('dados ocupados por outra gravação (Central ou Claude). Tente de novo em alguns segundos.');
        dormir(40);
      }
    }
  }
  const destravar = () => { try { unlinkSync(arqTrava); } catch {} };

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
    travar();
    try {
      const antes = carregar();
      const resultado = fn(antes);
      backupDiario(hoje()); // guarda o estado de antes da primeira gravação do dia
      salvar(resultado.estado, antes);
      return resultado;
    } finally {
      destravar();
    }
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

  return { tipo: 'arquivos', dir, carregar, salvar, transacao, backupDiario };
}
