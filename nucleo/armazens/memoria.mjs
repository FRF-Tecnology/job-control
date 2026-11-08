// Armazém em memória: mesma interface do de arquivos. Serve para testes e
// prova que o núcleo não depende de onde os dados moram (Postgres entra na T7).
import { estadoVazio } from '../servico.mjs';

export function criarArmazemMemoria({ estadoInicial } = {}) {
  let estado = structuredClone({ ...estadoVazio(), ...(estadoInicial ?? {}) });
  const backups = new Map();

  const carregar = () => structuredClone(estado);
  const salvar = novo => { estado = structuredClone(novo); };

  // Igual ao de arquivos: fn recebe o estado e devolve { estado, ...resto }. Se lançar, nada muda.
  function transacao(fn) {
    const resultado = fn(carregar());
    salvar(resultado.estado);
    return resultado;
  }

  function backupDiario(hoje) {
    if (backups.has(hoje)) return false;
    backups.set(hoje, carregar());
    return true;
  }

  return { tipo: 'memoria', carregar, salvar, transacao, backupDiario };
}
