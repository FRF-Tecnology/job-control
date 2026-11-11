// Porta de entrada da conta (Task 7): entrar, criar e confirmar com o código, esqueci a senha e o login com o Google
// (pelo navegador; a tela acompanha por GET /conta a cada 2 s). E a seção Conta do Perfil: sair e excluir.
// A senha digitada fica só na memória deste componente (ruling R10), para o confirmar; some ao entrar ou ao sair da tela.
import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { TERMOS_VERSAO } from '../app/conta';
import termosMd from '../../../../docs/legal/termos.md?raw';
import privacidadeMd from '../../../../docs/legal/privacidade.md?raw';

type Api = (caminho: string, metodo?: string, corpo?: unknown) => Promise<any>;
type Passo = 'entrar' | 'criar' | 'codigo' | 'esqueci' | 'nova-senha' | 'google';
type Origem = 'criar' | 'entrar' | 'recuperar';

const ESPERA_CODIGO = 60; // a nuvem não manda outro código antes disso (e não avisa)
const CADENCIA_GOOGLE = 2000;

// Os termos e a política vão junto no app (funcionam sem internet), na versão que a tela manda ao criar a conta.
export function TextosLegais() {
  const [aberto, setAberto] = useState<'termos' | 'privacidade' | null>(null);
  const alternar = (q: 'termos' | 'privacidade') => setAberto(a => (a === q ? null : q));
  return (
    <div className="textos-legais">
      <div className="acoes">
        <button className="botao-fantasma p" type="button" aria-expanded={aberto === 'termos'} onClick={() => alternar('termos')}>Ler os termos de uso</button>
        <button className="botao-fantasma p" type="button" aria-expanded={aberto === 'privacidade'} onClick={() => alternar('privacidade')}>Ler a política de privacidade</button>
      </div>
      {aberto && <pre className="texto-legal" tabIndex={0}>{aberto === 'termos' ? termosMd : privacidadeMd}</pre>}
    </div>
  );
}

function Campo({ rotulo, tipo = 'text', valor, mudar, auto, entrada }: { rotulo: string; tipo?: string; valor: string; mudar: (v: string) => void; auto?: string; entrada?: React.Ref<HTMLInputElement> }) {
  return (
    <label className="campo"><span>{rotulo}</span>
      <input ref={entrada} type={tipo} value={valor} autoComplete={auto} required onChange={e => mudar(e.target.value)} />
    </label>
  );
}

export function TelaConta({ api, aoEntrar }: { api: Api; aoEntrar: () => void | Promise<void> }) {
  const [passo, setPasso] = useState<Passo>('entrar');
  const [email, setEmail] = useState('');
  const [senha, setSenha] = useState('');
  const [codigo, setCodigo] = useState('');
  const [aceito, setAceito] = useState(false);
  const [erro, setErro] = useState('');
  const [info, setInfo] = useState('');
  const [termos, setTermos] = useState<{ versao: string; mensagem: string; refazer: (v: string) => Promise<void> } | null>(null);
  const [ocupado, setOcupado] = useState(false);
  // Pedir outro código: só 60 s depois (prazo pelo relógio, não pela contagem dos tiques).
  const [ate, setAte] = useState(0);
  const [agora, setAgora] = useState(() => Date.now());
  const espera = Math.max(0, Math.ceil((ate - agora) / 1000));
  const comecarEspera = () => { const n = Date.now(); setAgora(n); setAte(n + ESPERA_CODIGO * 1000); };
  const origem = useRef<Origem>('criar');
  const primeiro = useRef<HTMLInputElement>(null);

  useEffect(() => { primeiro.current?.focus(); }, [passo]);
  useEffect(() => {
    if (ate <= Date.now()) return;
    const t = setInterval(() => { const n = Date.now(); setAgora(n); if (n >= ate) clearInterval(t); }, 1000);
    return () => clearInterval(t);
  }, [ate]);

  const ir = (p: Passo) => { setErro(''); setInfo(''); setTermos(null); setPasso(p); };
  const irParaCodigo = (o: Origem, texto: string) => {
    origem.current = o;
    setCodigo('');
    ir(o === 'recuperar' ? 'nova-senha' : 'codigo');
    setInfo(texto);
    comecarEspera();
  };
  const entrou = async () => {
    setSenha(''); setCodigo('');
    await aoEntrar();
  };

  // O erro da nuvem: os motivos conhecidos mudam o passo; o resto mostra a frase do servidor (429 e 503 inclusive).
  const tratar = (e: unknown, refazer?: (v: string) => Promise<void>) => {
    const motivo = e instanceof ErroApi ? e.dados?.motivo : undefined;
    const frase = (e as Error)?.message || 'Não deu certo. Tente de novo.';
    if (motivo === 'confirmar') return irParaCodigo('entrar', `Sua conta ainda não foi confirmada. Enviamos um código para ${email}.`);
    if (motivo === 'termos' && refazer) { setErro(''); setTermos({ versao: (e as ErroApi).dados?.versao || TERMOS_VERSAO, mensagem: frase, refazer }); return; }
    if (motivo === 'entrar-com-senha') setPasso('entrar');
    setErro(frase);
  };
  const executar = async (fn: () => Promise<void>, refazer?: (v: string) => Promise<void>) => {
    setOcupado(true); setErro('');
    try { await fn(); } catch (e) { tratar(e, refazer); } finally { setOcupado(false); }
  };

  const entrar = (t?: string): Promise<void> => executar(async () => {
    await api('/api/v1/conta/entrar', 'POST', t ? { email, senha, termos: t } : { email, senha });
    await entrou();
  }, entrar);
  const criar = () => {
    if (!aceito) { setErro('Para criar a conta, aceite os termos de uso e a política de privacidade.'); return; }
    return executar(async () => {
      await api('/api/v1/conta/criar', 'POST', { email, senha, termos: TERMOS_VERSAO });
      irParaCodigo('criar', `Enviamos um código para ${email}. Digite-o abaixo.`);
    });
  };
  const confirmar = (t?: string): Promise<void> => executar(async () => {
    await api('/api/v1/conta/confirmar', 'POST', t ? { email, codigo, senha, termos: t } : { email, codigo, senha });
    await entrou();
  }, confirmar);
  const recuperar = () => executar(async () => {
    await api('/api/v1/conta/recuperar', 'POST', { email });
    irParaCodigo('recuperar', `Se ${email} tem conta, enviamos um código para criar uma senha nova.`);
  });
  const novaSenha = () => executar(async () => {
    await api('/api/v1/conta/nova-senha', 'POST', { email, codigo, senha });
    await entrou();
  });
  const reenviar = () => executar(async () => {
    try {
      if (origem.current === 'criar') await api('/api/v1/conta/criar', 'POST', { email, senha, termos: TERMOS_VERSAO });
      else if (origem.current === 'recuperar') await api('/api/v1/conta/recuperar', 'POST', { email });
      else await api('/api/v1/conta/entrar', 'POST', { email, senha });
    } catch (e) {
      if (!(e instanceof ErroApi && e.dados?.motivo === 'confirmar')) throw e; // entrar de novo: a nuvem manda o código
    }
    comecarEspera();
    setInfo(`Enviamos outro código para ${email}.`);
  });
  const google = (t = TERMOS_VERSAO): Promise<void> => executar(async () => {
    await api('/api/v1/conta/google', 'POST', { termos: t });
    ir('google');
  });
  const cancelarGoogle = () => executar(async () => {
    await api('/api/v1/conta/google/cancelar', 'POST', {});
    ir('entrar');
  });

  // Aguardando o Google: GET /conta agora e a cada 2 s até entrar, dar erro ou a pessoa cancelar.
  const conferirGoogle = useCallback(async () => {
    let c: any;
    try { c = await api('/api/v1/conta'); } catch { return; } // tenta de novo no próximo tique
    if (c?.entrada) { await aoEntrar(); return; }
    const g: string = c?.google ?? '';
    if (g === 'aguardando') return;
    setPasso('entrar');
    if (!g.startsWith('erro:')) return; // cancelado em outro lugar
    const frase = g.slice('erro:'.length);
    if (c.google_motivo === 'termos') setTermos({ versao: c.google_versao || TERMOS_VERSAO, mensagem: frase, refazer: v => google(v) });
    else setErro(frase);
  }, [api, aoEntrar]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (passo !== 'google') return;
    let vivo = true;
    const tique = () => { if (vivo) conferirGoogle(); };
    tique();
    const t = setInterval(tique, CADENCIA_GOOGLE);
    return () => { vivo = false; clearInterval(t); };
  }, [passo, conferirGoogle]);

  const formulario = (titulo: string, aoEnviar: () => unknown, corpo: ReactNode) => (
    <form className="formulario" onSubmit={(ev: FormEvent) => { ev.preventDefault(); if (!ocupado) aoEnviar(); }}>
      <header className="cabecalho"><div><h1>{titulo}</h1></div></header>
      {info && <p className="mudo" role="status">{info}</p>}
      <fieldset>{corpo}</fieldset>
    </form>
  );
  const emailCampo = <Campo rotulo="Email" tipo="email" auto="email" valor={email} mudar={setEmail} entrada={primeiro} />;
  const tempo = espera > 0
    ? <p className="pequeno mudo">{`Pode pedir outro código em ${espera} s`}</p>
    : null;

  let conteudo: ReactNode;
  switch (passo) {
    case 'criar':
      conteudo = formulario('Criar conta', criar, <>
        {emailCampo}
        <Campo rotulo="Senha" tipo="password" auto="new-password" valor={senha} mudar={setSenha} />
        <label style={{ display: 'flex', gap: '8px', alignItems: 'start' }}>
          <input type="checkbox" checked={aceito} onChange={e => setAceito(e.target.checked)} />
          <span>Li e aceito os termos de uso e a política de privacidade</span>
        </label>
        <TextosLegais />
        <div className="acoes">
          <button className="botao" type="submit" disabled={ocupado}>Criar conta</button>
          <button className="botao-fantasma" type="button" onClick={() => ir('entrar')}>Já tenho conta</button>
        </div>
      </>);
      break;
    case 'codigo':
    case 'nova-senha':
      conteudo = formulario(passo === 'codigo' ? 'Confirme o seu email' : 'Criar uma senha nova', passo === 'codigo' ? () => confirmar() : novaSenha, <>
        <label className="campo"><span>Código de confirmação</span>
          <input ref={primeiro} type="text" inputMode="numeric" autoComplete="one-time-code" required value={codigo} onChange={e => setCodigo(e.target.value.trim())} />
        </label>
        {passo === 'nova-senha' && <Campo rotulo="Senha nova" tipo="password" auto="new-password" valor={senha} mudar={setSenha} />}
        {tempo}
        <div className="acoes">
          <button className="botao" type="submit" disabled={ocupado}>{passo === 'codigo' ? 'Confirmar' : 'Salvar senha nova'}</button>
          <button className="botao-fantasma" type="button" disabled={ocupado || espera > 0} onClick={reenviar}>Pedir outro código</button>
          <button className="botao-fantasma" type="button" onClick={() => { setSenha(''); ir('entrar'); }}>Voltar</button>
        </div>
      </>);
      break;
    case 'esqueci':
      conteudo = formulario('Esqueci a senha', recuperar, <>
        {emailCampo}
        <div className="acoes">
          <button className="botao" type="submit" disabled={ocupado}>Enviar código</button>
          <button className="botao-fantasma" type="button" onClick={() => ir('entrar')}>Voltar</button>
        </div>
      </>);
      break;
    case 'google':
      conteudo = (
        <div className="formulario">
          <header className="cabecalho"><div><h1>Entrar com Google</h1></div></header>
          <fieldset>
            <p role="status">Continue no navegador para entrar com o Google. Esta tela segue sozinha quando terminar.</p>
            <div className="acoes"><button className="botao-fantasma" type="button" disabled={ocupado} onClick={cancelarGoogle}>Cancelar</button></div>
          </fieldset>
        </div>
      );
      break;
    default:
      conteudo = formulario('Entre na sua conta', () => entrar(), <>
        <p className="mudo" style={{ margin: 0 }}>Entre na sua conta para buscar e se candidatar.</p>
        {emailCampo}
        <Campo rotulo="Senha" tipo="password" auto="current-password" valor={senha} mudar={setSenha} />
        <div className="acoes">
          <button className="botao" type="submit" disabled={ocupado}>Entrar</button>
          <button className="botao-fantasma" type="button" onClick={() => ir('esqueci')}>Esqueci a senha</button>
        </div>
        <div className="acoes">
          <button className="botao-fantasma" type="button" disabled={ocupado} onClick={() => google()}>Entrar com Google</button>
          <button className="botao-fantasma" type="button" onClick={() => ir('criar')}>Criar uma conta</button>
        </div>
        <p className="pequeno mudo" style={{ margin: 0 }}>Ao entrar com o Google pela primeira vez, você aceita os termos de uso e a política de privacidade.</p>
        <TextosLegais />
      </>);
  }

  return (
    <div className="tela-conta">
      {conteudo}
      <div aria-live="assertive">
        {erro && <p className="erro-resposta" role="alert">{erro}</p>}
      </div>
      {termos && (
        <div className="zona" role="group" aria-label="Termos novos">
          <p>{termos.mensagem}</p>
          <TextosLegais />
          <div><button className="botao" type="button" disabled={ocupado} onClick={() => { const t = termos; setTermos(null); t.refazer(t.versao); }}>Aceito os novos termos</button></div>
        </div>
      )}
    </div>
  );
}

// Seção Conta do Perfil: o email, sair e excluir (digitando o email; com a caixa, apaga também os dados daqui).
export function SecaoConta() {
  const app = useApp();
  const c = app.conta;
  const [digitado, setDigitado] = useState('');
  const [apagar, setApagar] = useState(false);
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState('');
  const [feito, setFeito] = useState('');
  if (!c?.ativa || !c.entrada) return null;
  const confere = !!c.email && digitado.trim().toLowerCase() === c.email.toLowerCase();

  const sair = async () => {
    setOcupado(true); setErro('');
    try {
      await app.api('/api/v1/conta/sair', 'POST', {});
      await app.releConta?.();
    } catch (e) { setErro((e as Error).message); } finally { setOcupado(false); }
  };
  const excluir = async (ev: FormEvent) => {
    ev.preventDefault();
    if (!confere || ocupado) return;
    setOcupado(true); setErro('');
    try {
      const r = await app.api('/api/v1/conta/excluir', 'POST', { email: c.email, apagarDados: apagar });
      if (r && typeof r === 'object' && r.contaExcluida) { setErro(r.erro || 'A conta foi excluída, mas algo falhou neste computador.'); return; }
      if (r && typeof r === 'object' && 'fechar' in r) {
        const parcial = Array.isArray(r.faltou) && r.faltou.length > 0;
        setFeito(`Conta excluída. ${parcial ? 'Parte dos dados foi apagada.' : 'Dados apagados.'}${r.fechar ? ' O app vai fechar.' : ''}`);
        return; // com fechar, o servidor fecha o app: não adianta reler
      }
      app.aviso('Conta excluída');
      await app.releConta?.();
    } catch (e) { setErro((e as Error).message); } finally { setOcupado(false); }
  };

  return (
    <section className="seus-dados" aria-labelledby="conta-titulo">
      <h2 id="conta-titulo">Conta</h2>
      <p className="mudo">Você entrou como <b>{c.email}</b></p>
      {feito && <p className="feito" role="status">{feito}</p>}
      <div aria-live="assertive">{erro && <p className="erro-resposta" role="alert">{erro}</p>}</div>
      <div className="zona">
        <div><button className="botao-fantasma" type="button" disabled={ocupado} onClick={sair}>Sair da conta</button></div>
      </div>
      <form className="zona perigo" onSubmit={excluir}>
        <h3>Excluir conta</h3>
        <p className="mudo">Apaga a conta na nuvem, de vez, e sai neste computador.</p>
        <label className="campo"><span>Para confirmar, digite o email da conta</span>
          <input type="email" autoComplete="off" spellCheck={false} value={digitado} onChange={e => setDigitado(e.target.value)} />
        </label>
        <label style={{ display: 'flex', gap: '8px', alignItems: 'start' }}>
          <input type="checkbox" checked={apagar} onChange={e => setApagar(e.target.checked)} />
          <span>Apagar também os dados deste computador</span>
        </label>
        <div><button className="botao-perigo" type="submit" disabled={!confere || ocupado}>{ocupado ? 'Excluindo…' : 'Excluir conta'}</button></div>
      </form>
    </section>
  );
}
