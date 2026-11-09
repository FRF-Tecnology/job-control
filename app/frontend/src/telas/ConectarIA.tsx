// Conectar IA: a pessoa escolhe a IA que vai triar vagas, responder formulários e escrever mensagens.
// Três caminhos: a própria assinatura (CLI oficial no PC), a própria chave de API, ou modelos gratuitos.
import { useCallback, useEffect, useState } from 'react';
import { useApp } from '../app/contexto';
import { Convite } from '../componentes';

type CLI = { provedor: string; comando: string; nome: string; plano: string; instalar: string; verificada: boolean; instalada: boolean; logada: boolean | null; versao?: string };
type EstadoIA = {
  configurada: boolean; provedor?: string; modelo?: string; url?: string;
  chaves: Record<string, string | null>; clis: CLI[]; gratis: { disponivel: boolean };
  uso: { chamadas: number; falhas: number; custoEstimadoUSD: number };
};

const NOMES: Record<string, string> = {
  'claude-cli': 'Claude (sua assinatura)', 'codex-cli': 'ChatGPT (sua assinatura)', 'gemini-cli': 'Gemini (sua assinatura)',
  anthropic: 'Claude (sua chave de API)', openai: 'OpenAI ou compatível (sua chave)', google: 'Gemini (sua chave de API)',
};
const CHAVES = [
  { provedor: 'anthropic', nome: 'Anthropic (Claude)', modelo: 'claude-haiku-5-5', ajuda: 'console.anthropic.com → API Keys' },
  { provedor: 'openai', nome: 'OpenAI ou compatível (ex.: OpenRouter)', modelo: '', ajuda: 'platform.openai.com → API keys, ou a chave do serviço compatível' },
  { provedor: 'google', nome: 'Google (Gemini)', modelo: '', ajuda: 'aistudio.google.com → Get API key' },
];

export function TelaConectarIA() {
  const app = useApp();
  const [ia, setIA] = useState<EstadoIA | null>(null);
  const [ocupado, setOcupado] = useState<string | null>(null);
  const [escolha, setEscolha] = useState('anthropic');
  const [chave, setChave] = useState('');
  const [modelo, setModelo] = useState('');
  const [url, setUrl] = useState('');

  const ler = useCallback(async () => setIA(await app.api('/api/v1/ia')), [app]);
  useEffect(() => { ler().catch(e => app.aviso(e.message, true)); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const fazer = async (rotulo: string, fn: () => Promise<unknown>, msg?: string) => {
    setOcupado(rotulo);
    try { await fn(); if (msg) app.aviso(msg); await ler(); } catch (e) { app.aviso((e as Error).message, true); } finally { setOcupado(null); }
  };
  const usar = (provedor: string, extra: object = {}) => fazer(provedor, () => app.api('/api/v1/ia/config', 'PUT', { provedor, ...extra }), `${NOMES[provedor]} conectado`);
  const testar = () => fazer('testar', async () => {
    const r = await app.api('/api/v1/ia/testar', 'POST', {});
    app.aviso(`A IA respondeu em ${(r.ms / 1000).toFixed(1)} s`);
  });

  if (!ia) return <div><header className="cabecalho"><div><h1>Conectar IA</h1><p>Procurando as IAs deste computador…</p></div></header></div>;
  const ativa = ia.configurada ? NOMES[ia.provedor!] ?? ia.provedor : null;
  const opcaoChave = CHAVES.find(c => c.provedor === escolha)!;

  return (
    <div className="conectar-ia">
      <header className="cabecalho">
        <div><h1>Conectar IA</h1><p>A IA tria vagas, responde formulários e escreve rascunhos de mensagem. Você escolhe qual usar e troca quando quiser.</p></div>
      </header>

      {ativa ? (
        <Convite azul titulo={`Conectada: ${ativa}`} texto={`${ia.modelo ? `Modelo ${ia.modelo}. ` : ''}Últimos 30 dias: ${ia.uso.chamadas} uso(s)${ia.uso.custoEstimadoUSD ? `, cerca de US$ ${ia.uso.custoEstimadoUSD.toFixed(2)}` : ''}.`}>
          <span className="acoes">
            <button className="botao" type="button" disabled={!!ocupado} onClick={testar}>{ocupado === 'testar' ? 'Testando…' : 'Testar conexão'}</button>
            <button className="botao-fantasma" type="button" disabled={!!ocupado} onClick={() => fazer('nenhum', () => app.api('/api/v1/ia/config', 'PUT', { provedor: 'nenhum' }), 'IA desconectada')}>Desconectar</button>
          </span>
        </Convite>
      ) : <Convite titulo="Nenhuma IA conectada" texto="Escolha um dos caminhos abaixo. Sem IA, a Central funciona, mas a triagem e as mensagens ficam por sua conta." />}

      <section className="bloco">
        <header><h2>Usar minha assinatura</h2></header>
        <p className="pequeno mudo">O app usa o programa oficial que você já tem no computador, com o seu login. Nada é cobrado à parte.</p>
        <ul className="lista-cartao">
          {ia.clis.map(c => {
            const pronto = c.instalada && c.logada !== false;
            const estado = !c.instalada ? 'Não instalado' : c.logada === false ? 'Faça login' : c.verificada ? 'Pronto' : 'Pronto · não testado';
            return (
              <li key={c.provedor} className="item-acao">
                <span className={`selo-n${pronto ? '' : ' ambar'}`}>{c.nome[0]}</span>
                <div>
                  <strong>{c.nome}</strong>
                  <span className="sub">{`${c.plano} · ${estado}${c.versao ? ` · ${c.versao}` : ''}`}</span>
                  {c.instalada && c.logada === false && <span className="sub">{`No terminal, rode: ${c.comando} auth login`}</span>}
                </div>
                <div className="acoes">
                  {pronto
                    ? <button className="botao p" type="button" disabled={!!ocupado || ia.provedor === c.provedor} onClick={() => usar(c.provedor)}>{ia.provedor === c.provedor ? 'Em uso' : ocupado === c.provedor ? 'Conectando…' : 'Usar'}</button>
                    : <a className="botao-fantasma p" href={c.instalar} target="_blank" rel="noopener">Como instalar</a>}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="bloco">
        <header><h2>Usar minha chave de API</h2></header>
        <p className="pequeno mudo">A chave fica no cofre do sistema deste computador. A Central nunca mostra a chave inteira e não a envia para nenhum outro lugar.</p>
        <form className="formulario" onSubmit={ev => {
          ev.preventDefault();
          fazer(escolha, async () => {
            if (chave.trim()) await app.api(`/api/v1/ia/chaves/${escolha}`, 'PUT', { chave });
            await app.api('/api/v1/ia/config', 'PUT', { provedor: escolha, modelo, url });
            setChave('');
            app.limpar();
          }, `${NOMES[escolha]} conectado`);
        }}>
          <div className="duas">
            <label className="campo"><span>Provedor</span>
              <select value={escolha} onChange={ev => { setEscolha(ev.target.value); setModelo(''); setUrl(''); }}>
                {CHAVES.map(c => <option key={c.provedor} value={c.provedor}>{c.nome}</option>)}
              </select>
            </label>
            <label className="campo"><span>{`Chave de API${ia.chaves[escolha] ? ` (salva: ${ia.chaves[escolha]})` : ''}`}</span>
              <input type="password" autoComplete="off" value={chave} onChange={ev => setChave(ev.target.value)} placeholder={ia.chaves[escolha] ? 'deixe em branco para manter a salva' : opcaoChave.ajuda} />
            </label>
            <label className="campo"><span>Modelo</span>
              <input type="text" value={modelo} onChange={ev => setModelo(ev.target.value)} placeholder={opcaoChave.modelo || 'nome do modelo na sua conta'} />
            </label>
            {escolha === 'openai' && (
              <label className="campo"><span>Endereço (só para compatíveis)</span>
                <input type="url" value={url} onChange={ev => setUrl(ev.target.value)} placeholder="ex.: https://openrouter.ai/api" />
              </label>
            )}
          </div>
          <div className="acoes">
            <button className="botao" type="submit" disabled={!!ocupado || (!chave.trim() && !ia.chaves[escolha])}>{ocupado === escolha ? 'Salvando…' : 'Salvar e usar'}</button>
            {ia.chaves[escolha] && <button className="botao-fantasma" type="button" disabled={!!ocupado} onClick={() => fazer('apagar', () => app.api(`/api/v1/ia/chaves/${escolha}`, 'DELETE', {}), 'Chave apagada do cofre')}>Apagar chave salva</button>}
          </div>
        </form>
      </section>

      <section className="bloco">
        <header><h2>Modelos gratuitos</h2></header>
        <Convite titulo="Em breve" texto="Para quem não tem assinatura nem chave. Chega junto com a conta da Central." />
      </section>
    </div>
  );
}
