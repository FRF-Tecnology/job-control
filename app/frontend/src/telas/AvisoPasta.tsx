// Aviso da pasta de dados (jc-mub; rota GET /dados/pasta do jc-etv.14, #117). O aviso é o desta abertura do app:
// pasta-antiga-ficou (as duas pastas existem; o app usa a Applyer e não juntou nada) ou migracao-adiada (a antiga
// estava em uso; o app a usou desta vez e renomeia na próxima abertura). Frases fixas: a resposta não traz caminho nem
// nome de usuário, e a tela não mostra nada dela além do código. "Entendi" esconde até o app abrir de novo.
import { useEffect, useState } from 'react';
import { useApp } from '../app/contexto';

const VISTO = 'applyer:aviso-pasta-visto';
const FRASES: Record<string, { titulo: string; texto: string }> = {
  'pasta-antiga-ficou': {
    titulo: 'A pasta antiga continua no computador',
    texto: 'O app usa só a pasta nova (Applyer) e não juntou nada. A antiga (Central de Candidaturas) ficou ao lado dela. Se os dados de lá não fazem mais falta, apague a pasta antiga à mão.',
  },
  'migracao-adiada': {
    titulo: 'A pasta de dados ainda não mudou de nome',
    texto: 'A pasta antiga (Central de Candidaturas) estava em uso por outro programa, então o app a usou desta vez. Feche o app antigo, a CLI e o assistente (MCP); na próxima abertura o app troca para a pasta Applyer.',
  },
};

export function AvisoPasta() {
  const app = useApp();
  const [aviso, setAviso] = useState<string | null>(null);
  const [visto, setVisto] = useState(() => { try { return sessionStorage.getItem(VISTO); } catch { return null; } });
  useEffect(() => {
    // Servidor sem a rota (404) ou erro: sem aviso. Só os códigos da lista fechada viram aviso.
    // Só chave própria da lista: "toString" ou "constructor" passariam num "in" (nota do CyberSec no #118).
    app.api('/api/v1/dados/pasta').then(r => setAviso(typeof r?.aviso === 'string' && Object.prototype.hasOwnProperty.call(FRASES, r.aviso) ? r.aviso : null)).catch(() => setAviso(null));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (!aviso || visto === aviso) return null;
  const f = FRASES[aviso];
  const entendi = () => { try { sessionStorage.setItem(VISTO, aviso); } catch { /* sem armazenamento: some só agora */ } setVisto(aviso); };
  return (
    <section className="bloco" aria-labelledby="aviso-pasta-titulo">
      <div className="convite ambar" role="status">
        <div><strong id="aviso-pasta-titulo">{f.titulo}</strong><p style={{ margin: 0 }}>{f.texto}</p></div>
        <button className="botao-fantasma" type="button" onClick={entendi}>Entendi</button>
      </div>
    </section>
  );
}
