// Conectar IA: a pessoa escolhe a IA que vai triar vagas, responder formulários e escrever mensagens.
// Três caminhos: a própria assinatura (CLI oficial no PC), a própria chave de API, ou modelos gratuitos.
import { useCallback, useEffect, useRef, useState } from 'react';
import { useApp } from '../app/contexto';
import { Convite, LinkExterno } from '../componentes';

// pronta/aviso (contrato 0.4.2): instalada num formato que o app não usa (ex.: Claude .cmd no Windows) vem com
// pronta false e um aviso fixo do back. Sem o campo (API antiga), vale como pronta.
type CLI = { provedor: string; comando: string; nome: string; plano: string; instalar: string; verificada: boolean; instalada: boolean; logada: boolean | null; versao?: string; pronta?: boolean; aviso?: string };
const naoPronta = (c?: CLI) => !!c && c.instalada && c.pronta === false;
const AVISO_PADRAO = 'O app não consegue usar esta instalação. Veja como instalar a versão indicada.';
// O aviso do back vem em minúscula (cabe no meio de uma frase); na tela ele é frase própria.
const avisoDe = (c: CLI) => { const t = c.aviso?.trim() || AVISO_PADRAO; return t[0].toUpperCase() + t.slice(1); };
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
// A chave fica presa ao endereço configurado na hora em que é gravada (jc-a2y.27); a tela mostra qual é (jc-a2y.40).
// Endereços padrão iguais aos de internal/ia/provedores.go.
const HOST_PADRAO: Record<string, string> = { anthropic: 'api.anthropic.com', openai: 'api.openai.com', google: 'generativelanguage.googleapis.com' };
// Só o host (sem caminho, usuário ou senha), em minúsculas; "" se não for um endereço https válido.
const hostDe = (endereco?: string) => {
  try { const u = new URL((endereco ?? '').trim()); return u.protocol === 'https:' ? u.host.toLowerCase().replace(/\.$/, '') : ''; } catch { return ''; }
};
const destinoDe = (provedor: string, endereco?: string) => {
  const proprio = (endereco ?? '').trim();
  return proprio ? { host: hostDe(proprio) || 'endereço inválido (use https://)', padrao: false } : { host: HOST_PADRAO[provedor] ?? '', padrao: true };
};

export function TelaConectarIA() {
  const app = useApp();
  const [ia, setIA] = useState<EstadoIA | null>(null);
  const [ocupado, setOcupado] = useState<string | null>(null);
  const [escolha, setEscolha] = useState('anthropic');
  const [chave, setChave] = useState('');
  const [modelo, setModelo] = useState('');
  const [url, setUrl] = useState('');
  const campoChave = useRef<HTMLInputElement>(null);

  const ler = useCallback(async () => { const d: EstadoIA = await app.api('/api/v1/ia'); setIA(d); return d; }, [app]);
  // Na primeira leitura, o formulário já vem com o provedor de chave em uso, o modelo e o endereço dele: salvar sem
  // redigitar o endereço não volta para o padrão (o que apagaria a chave).
  useEffect(() => {
    ler().then(d => {
      if (d.configurada && CHAVES.some(c => c.provedor === d.provedor)) { setEscolha(d.provedor!); setModelo(d.modelo ?? ''); setUrl(d.url ?? ''); }
    }).catch(e => app.aviso(e.message, true));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const escolher = (provedor: string) => {
    const emUso = ia?.configurada && ia.provedor === provedor;
    setEscolha(provedor); setModelo(emUso ? ia!.modelo ?? '' : ''); setUrl(emUso ? ia!.url ?? '' : '');
  };
  const salvarChave = async (ev: React.FormEvent) => {
    ev.preventDefault();
    const nova = chave.trim();
    if (nova && nova.length < 10) { app.aviso('Cole a chave de API inteira.', true); campoChave.current?.focus(); return; }
    setOcupado(escolha);
    try {
      // Endereço antes da chave: a chave é presa ao endereço configurado no momento em que é gravada.
      const r = await app.api('/api/v1/ia/config', 'PUT', { provedor: escolha, modelo, url });
      if (nova) await app.api(`/api/v1/ia/chaves/${escolha}`, 'PUT', { chave: nova });
      setChave('');
      app.limpar();
      if (r?.chaveApagada && !nova) {
        app.aviso(`A chave salva foi apagada porque o endereço mudou para ${destinoDe(escolha, url).host}. Cole a chave de novo.`, true);
        await ler();
        campoChave.current?.focus();
      } else {
        app.aviso(`${NOMES[escolha]} conectado`);
        await ler();
      }
    } catch (e) { app.aviso((e as Error).message, true); } finally { setOcupado(null); }
  };

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
  const cliAtiva = ia.clis.find(c => c.provedor === ia.provedor);
  const opcaoChave = CHAVES.find(c => c.provedor === escolha)!;
  const faltaChave = ia.configurada && CHAVES.some(c => c.provedor === ia.provedor) && !ia.chaves[ia.provedor!];
  const destino = destinoDe(escolha, url);
  const desconectar = <button className="botao-fantasma" type="button" disabled={!!ocupado} onClick={() => fazer('nenhum', () => app.api('/api/v1/ia/config', 'PUT', { provedor: 'nenhum' }), 'IA desconectada')}>Desconectar</button>;

  return (
    <div className="conectar-ia">
      <header className="cabecalho">
        <div><h1>Conectar IA</h1><p>A IA tria vagas, responde formulários e escreve rascunhos de mensagem. Você escolhe qual usar e troca quando quiser.</p></div>
      </header>

      {ativa && naoPronta(cliAtiva) ? (
        <Convite titulo={`Conectada, mas não pronta: ${ativa}`} texto={avisoDe(cliAtiva!)}>
          <span className="acoes">
            <LinkExterno url={cliAtiva!.instalar} className="botao p">Como instalar</LinkExterno>
            {desconectar}
          </span>
        </Convite>
      ) : ativa && faltaChave ? (
        <Convite titulo={`Falta a chave: ${ativa}`} texto={`Cole a chave de API para ${destinoDe(ia.provedor!, ia.url).host}.`}>
          <span className="acoes">
            <button className="botao p" type="button" onClick={() => { escolher(ia.provedor!); campoChave.current?.focus(); }}>Colar a chave</button>
            {desconectar}
          </span>
        </Convite>
      ) : ativa ? (
        <Convite azul titulo={`Conectada: ${ativa}`} texto={`${ia.modelo ? `Modelo ${ia.modelo}. ` : ''}Últimos 30 dias: ${ia.uso.chamadas} uso(s)${ia.uso.custoEstimadoUSD ? `, cerca de US$ ${ia.uso.custoEstimadoUSD.toFixed(2)}` : ''}.`}>
          <span className="acoes">
            <button className="botao" type="button" disabled={!!ocupado} onClick={testar}>{ocupado === 'testar' ? 'Testando…' : 'Testar conexão'}</button>
            {desconectar}
          </span>
        </Convite>
      ) : <Convite titulo="Nenhuma IA conectada" texto="Escolha um dos caminhos abaixo. Sem IA, a Central funciona, mas a triagem e as mensagens ficam por sua conta." />}

      <section className="bloco">
        <header><h2>Usar minha assinatura</h2></header>
        <p className="pequeno mudo">O app usa o programa oficial que você já tem no computador, com o seu login. Nada é cobrado à parte.</p>
        <ul className="lista-cartao">
          {ia.clis.map(c => {
            const pronto = c.instalada && !naoPronta(c) && c.logada !== false;
            const estado = !c.instalada ? 'Não instalado' : naoPronta(c) ? 'Não pronto' : c.logada === false ? 'Faça login' : c.verificada ? 'Pronto' : 'Pronto · não testado';
            return (
              <li key={c.provedor} className="item-acao">
                <span className={`selo-n${pronto ? '' : ' ambar'}`}>{c.nome[0]}</span>
                <div>
                  <strong>{c.nome}</strong>
                  <span className="sub">{`${c.plano} · ${estado}${c.versao ? ` · ${c.versao}` : ''}`}</span>
                  {naoPronta(c) && <span className="sub aviso-cli">{avisoDe(c)}</span>}
                  {c.instalada && !naoPronta(c) && c.logada === false && <span className="sub">{`No terminal, rode: ${c.comando} auth login`}</span>}
                </div>
                <div className="acoes">
                  {pronto
                    ? <button className="botao p" type="button" disabled={!!ocupado || ia.provedor === c.provedor} onClick={() => usar(c.provedor)}>{ia.provedor === c.provedor ? 'Em uso' : ocupado === c.provedor ? 'Conectando…' : 'Usar'}</button>
                    : <LinkExterno url={c.instalar} className="botao-fantasma p">Como instalar</LinkExterno>}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="bloco">
        <header><h2>Usar minha chave de API</h2></header>
        <p className="pequeno mudo">A chave fica no cofre do sistema deste computador. A Central nunca mostra a chave inteira e não a envia para nenhum outro lugar.</p>
        <form className="formulario" onSubmit={salvarChave}>
          <div className="duas">
            <label className="campo"><span>Provedor</span>
              <select value={escolha} onChange={ev => escolher(ev.target.value)}>
                {CHAVES.map(c => <option key={c.provedor} value={c.provedor}>{c.nome}</option>)}
              </select>
            </label>
            <label className="campo"><span>{`Chave de API${ia.chaves[escolha] ? ` (salva: ${ia.chaves[escolha]})` : ''}`}</span>
              <input ref={campoChave} type="password" autoComplete="off" value={chave} onChange={ev => setChave(ev.target.value)} placeholder={ia.chaves[escolha] ? 'deixe em branco para manter a salva' : opcaoChave.ajuda} />
              <small className="destino-chave">A chave vai para <strong>{destino.host}</strong>{destino.padrao ? ' (padrão)' : ''}</small>
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
