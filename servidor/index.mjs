// Sobe a Central em http://localhost:4321 (ou a próxima porta livre).
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarServidor } from './app.mjs';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { hojeISO } from '../nucleo/modelo.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const dir = process.env.CENTRAL_DADOS || join(raiz, 'dados');
const inicial = Number(process.env.PORT) || 4321;

criarArmazem(dir).backupDiario(hojeISO());

function tentar(porta, restantes) {
  const srv = criarServidor({ dir });
  srv.once('error', e => {
    if (e.code === 'EADDRINUSE' && restantes > 0) return tentar(porta + 1, restantes - 1);
    console.error(`Não consegui abrir a Central: ${e.message}`);
    process.exit(1);
  });
  srv.listen(porta, '127.0.0.1', () => {
    console.log(`Central de Candidaturas aberta em http://localhost:${porta}`);
    console.log(`Dados em ${dir}. Feche esta janela para desligar.`);
    if (process.env.CENTRAL_ABRIR) import('node:child_process').then(({ exec }) => exec(process.platform === 'win32' ? `start "" http://localhost:${porta}` : `${process.platform === 'darwin' ? 'open' : 'xdg-open'} http://localhost:${porta}`));
  });
}
tentar(inicial, 10);
