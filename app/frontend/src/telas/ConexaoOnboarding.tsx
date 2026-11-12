import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { useApp } from '../app/contexto';
import { HOST_PADRAO, iaProntaParaBusca, type EstadoIA } from './ConectarIA';
import { LinkExterno } from '../componentes';
import type { TentativaExterna } from '../app/rascunhoOnboarding';
import '../estilo/conexao-onboarding.css';

const MARCAS = [
  { provedor: 'openai', nome: 'OpenAI', cli: 'codex-cli' },
  { provedor: 'anthropic', nome: 'Claude', cli: 'claude-cli' },
  { provedor: 'xai', nome: 'Grok', cli: '' },
  { provedor: 'google', nome: 'Gemini', cli: 'gemini-cli' },
];

type Marca = typeof MARCAS[number];
type Modo = 'atual' | 'assinatura' | 'chave';

function opcao(estado: EstadoIA, marca: Marca): Modo {
  if ((estado.provedor === marca.provedor || estado.provedor === marca.cli) && iaProntaParaBusca(estado)) return 'atual';
  const cli = estado.clis.find(c => c.provedor === marca.cli);
  return cli?.instalada && cli.pronta !== false && cli.logada !== false ? 'assinatura' : 'chave';
}

export function ConexaoOnboarding({ aoConectar, tentativaExterna = '', registrarTentativa }: {
  aoConectar: (nome:string,verificacao?:string) => void; tentativaExterna?: TentativaExterna;
  registrarTentativa?: (tipo: TentativaExterna) => void;
}) {
  const { api, limpar } = useApp();
  const [estado, setEstado] = useState<EstadoIA | null>(null);
  const [marca, setMarca] = useState(MARCAS[0]);
  const [modo, setModo] = useState<Modo>('chave');
  const [chave, setChave] = useState('');
  const [modelo, setModelo] = useState('');
  const [erro, setErro] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const trava = useRef(false);
  const [tentativa, setTentativa] = useState(0);
  const [copia, setCopia] = useState('');
  const retomarLogin = useRef(Boolean(tentativaExterna));
  const escolher = useCallback((e: EstadoIA, m: Marca) => {
    setMarca(m); setModo(opcao(e, m)); setChave(''); setErro('');
    setModelo(e.provedor === m.provedor && !e.url ? e.modelo ?? '' : '');
  }, []);
  useEffect(() => {
    let aberta = true;
    api('/api/v1/ia').then((e: EstadoIA) => {
      if (!aberta) return;
      const inicial = (retomarLogin.current ? MARCAS.find(m => m.nome === 'Claude') : undefined) ?? MARCAS.find(m => e.provedor === m.provedor || e.provedor === m.cli)
        ?? MARCAS.find(m => opcao(e, m) === 'assinatura') ?? MARCAS.find(m=>m.nome==='Claude')!;
      retomarLogin.current=false;setEstado(e); escolher(e, inicial);
    }).catch(e => { if (aberta) setErro(e.message || 'Falha ao consultar as conexões.'); });
    return () => { aberta = false; };
  }, [api, tentativa, escolher]);

  if (!estado) return <div className="pa-ia-status" role="status">{erro ? <>
    <p>{erro}</p><button type="button" className="pa-secundario" onClick={() => { setErro(''); setTentativa(n => n + 1); }}>Tentar novamente</button>
  </> : 'Verificando conexões neste computador…'}</div>;

  const cli = estado.clis.find(c => c.provedor === marca.cli);
  const assinatura = cli?.instalada && cli.pronta !== false && cli.logada !== false;
  const podeManter = opcao(estado, marca) === 'atual';
  const manterChave = estado.provedor === marca.provedor && !estado.url && Boolean(estado.chaves[marca.provedor]);
  const cliAtual = modo === 'atual' && estado.provedor === marca.cli;
  const destino = modo === 'atual' && estado.url ? estado.url : HOST_PADRAO[marca.provedor];
  async function confirmarProva() {
    const prova=await api('/api/v1/ia/testar', 'POST', {});
    if(prova?.ok !== true) throw new Error('O provedor não confirmou a conexão. Verifique os dados e tente novamente.');
    limpar(); aoConectar(marca.nome,prova.verificacao);
  }
  async function verificarLogin() {
    if(trava.current) return;
    trava.current=true;setOcupado(true);setErro('');
    registrarTentativa?.('login-claude');
    try {
      await api('/api/v1/ia/detectar','POST',{});
      const atual:EstadoIA=await api('/api/v1/ia');
      setEstado(atual);
      const detectada=atual.clis.find(c=>c.provedor==='claude-cli');
      if(!detectada?.instalada || detectada.pronta===false) throw new Error('O Applyer ainda não encontrou uma instalação utilizável do Claude Code.');
      if(detectada.logada===false) throw new Error('O login do Claude ainda não foi concluído neste computador.');
      await api('/api/v1/ia/config','PUT',{provedor:'claude-cli'});
      await confirmarProva();
    } catch(e) {setErro(e instanceof Error ? e.message : 'Não foi possível verificar o login do Claude.');}
    finally {trava.current=false;setOcupado(false);window.dispatchEvent(new Event('applyer:ia-alterada'));}
  }

  async function conectar(ev: FormEvent) {
    ev.preventDefault();
    if (trava.current) return;
    setErro('');
    if (modo === 'chave') {
      if (!manterChave && !chave.trim()) { setErro('Cole a chave de API para continuar.'); return; }
      if (chave.trim() && chave.trim().length < 10) { setErro('Cole a chave de API inteira.'); return; }
      if (!modelo.trim() && marca.provedor !== 'anthropic') { setErro('Informe o identificador do modelo habilitado na sua conta.'); return; }
    }
    trava.current = true; setOcupado(true);
    try {
      if (modo === 'assinatura') await api('/api/v1/ia/config', 'PUT', { provedor: marca.cli });
      if (modo === 'chave') {
        await api('/api/v1/ia/config', 'PUT', { provedor: marca.provedor, modelo: modelo.trim() });
        if (chave.trim()) {
          await api('/api/v1/ia/chaves/' + marca.provedor, 'PUT', { chave: chave.trim() });
          setChave('');
        }
      }
      await confirmarProva();
    } catch (e) {
      // A configuração pode ter sido salva antes de o provedor recusar o teste.
      // Releia o cofre para permitir a tentativa seguinte com a chave já salva.
      try { const atual: EstadoIA = await api('/api/v1/ia'); setEstado(atual); } catch { /* Mostre o erro original. */ }
      setErro((e as Error).message || 'A conexão falhou. Confira os dados e tente novamente.');
    } finally { trava.current = false; setOcupado(false); window.dispatchEvent(new Event('applyer:ia-alterada')); }
  }

  return <form className="pa-conexao" onSubmit={conectar}>
    <div className="pa-provedores" role="group" aria-label="Escolha seu provedor de IA">
      {MARCAS.map(m => <button key={m.provedor} type="button" className={'pa-provedor ' + m.provedor} aria-pressed={marca === m}
        disabled={ocupado} onClick={() => escolher(estado, m)} aria-label={m.nome}>
        <span className="pa-logotipo" aria-hidden="true" /><strong>{m.nome}</strong>
        <span className="pa-marcador" aria-hidden="true">{marca === m && '✓'}</span>
      </button>)}
    </div>
    <div className="pa-ia-painel">
      <h2>Conectar {marca.nome}</h2>
      {marca.nome==='Claude' && (Boolean(tentativaExterna) || (!assinatura && (modo!=='atual' || cliAtual))) && <div className="pa-ajuda-claude">
        {(!cli?.instalada || cli.pronta===false) && <p>Para usar sua assinatura, instale o aplicativo oficial.{' '}
          <span onClickCapture={()=>registrarTentativa?.('instalar-claude')}><LinkExterno url={cli?.instalar || 'https://docs.claude.com/claude-code'}>Instalar Claude Code</LinkExterno></span></p>}
        {!assinatura && <><p>Abra o terminal e entre na sua conta com <code>claude auth login</code>.</p>
          <button type="button" className="pa-link" onClick={async()=>{
            registrarTentativa?.('login-claude');
            try{await navigator.clipboard.writeText('claude auth login');setCopia('Comando copiado.');}
            catch{setCopia('Selecione e copie o comando acima.');}
          }}>Copiar comando</button><span role="status">{copia}</span></>}
        <button type="button" className="pa-secundario" disabled={ocupado} onClick={()=>void verificarLogin()}>Já entrei · verificar</button>
      </div>}
      {(podeManter || assinatura) && <div className="pa-ia-modos" role="group" aria-label="Forma de conexão">
        {podeManter && <button type="button" disabled={ocupado} aria-pressed={modo === 'atual'} onClick={() => setModo('atual')}>Conexão atual</button>}
        {assinatura && !(podeManter && estado.provedor === marca.cli) && <button type="button" disabled={ocupado} aria-pressed={modo === 'assinatura'} onClick={() => setModo('assinatura')}>Minha assinatura</button>}
        <button type="button" disabled={ocupado} aria-pressed={modo === 'chave'} onClick={() => setModo('chave')}>Chave de API</button>
      </div>}
      {modo === 'chave' ? <div className="pa-campos">
        <div className="pa-dupla">
          <label>Chave de API<input type="password" autoComplete="off" spellCheck={false} disabled={ocupado}
            placeholder={manterChave ? 'Chave salva no cofre' : 'Cole sua chave'} value={chave} onChange={e => setChave(e.target.value)} /></label>
          <label>Modelo<input type="text" autoComplete="off" spellCheck={false} disabled={ocupado} value={modelo}
            onChange={e => setModelo(e.target.value)} placeholder={marca.provedor === 'anthropic' ? 'Padrão do Applyer' : 'ID do modelo na sua conta'} /></label>
        </div>
        <p>A chave fica no cofre do sistema. Destino da conexão: <strong>{destino}</strong>.</p>
      </div> : <p>{modo === 'assinatura' || cliAtual ? 'O Applyer vai usar o login de ' + (cli?.nome ?? marca.nome) + ' neste computador.' :
        'O Applyer vai usar a chave salva para ' + destino + '.'}</p>}
    </div>
    {erro && <p className="pa-erro" role="alert">{erro}</p>}
    <footer className="pa-acoes"><button type="submit" className="pa-primario" disabled={ocupado}>
      {ocupado ? 'Verificando conexão…' : modo === 'atual' ? 'Usar conexão atual' : 'Conectar e continuar'} <span aria-hidden="true">→</span>
    </button></footer>
  </form>;
}
