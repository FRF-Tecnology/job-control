// "Caixa de e-mail", no Perfil (api/openapi.yaml 0.16.0, rota /email/conta). A pessoa conecta a caixa por IMAP com uma
// senha de app; o app só lê (respostas das empresas e alertas de vaga), nunca marca, move, apaga ou envia.
// A senha sai do estado assim que o pedido termina e nunca volta do servidor. Frases fixas: nenhum texto do servidor
// vai para a tela.
import { useEffect, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { LinkExterno } from '../componentes';

const ROTA = '/api/v1/email/conta';
const SENHAS_GOOGLE = 'https://myaccount.google.com/apppasswords';
// Domínios cujo servidor o app já conhece (internal/caixa); os outros pedem o servidor.
const conhecidos = ['gmail.com', 'googlemail.com', 'outlook.com', 'hotmail.com', 'live.com', 'icloud.com', 'me.com', 'mac.com'];
const dominio = (v: string) => v.trim().toLowerCase().split('@')[1] || '';
const enderecoValido = (v: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim());

type Estado = { conectada: boolean; usuario: string };

export function ContaEmail() {
  const app = useApp();
  const [estado, setEstado] = useState<Estado | null>(null);
  const [usuario, setUsuario] = useState('');
  const [senha, setSenha] = useState('');
  const [servidor, setServidor] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState('');
  const [feito, setFeito] = useState('');

  useEffect(() => {
    app.api(ROTA).then(r => setEstado({ conectada: r?.conectada === true, usuario: typeof r?.usuario === 'string' ? r.usuario : '' }))
      .catch(() => setEstado({ conectada: false, usuario: '' }));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const pedeServidor = enderecoValido(usuario) && !conhecidos.includes(dominio(usuario));

  const explicar = (e: unknown) => {
    if (e instanceof ErroApi && e.status === 400) return 'O e-mail não aceitou. Confira o endereço e use uma senha de app (não a senha da sua conta).';
    if (e instanceof ErroApi && e.status === 502) return 'Não consegui falar com o servidor de e-mail. Confira a internet e o servidor e tente de novo.';
    if (e instanceof ErroApi && e.status === 403) return 'Só você pode conectar o e-mail, por esta tela.';
    return 'Não deu para conectar. Tente de novo.';
  };

  async function conectar() {
    if (ocupado) return;
    setErro(''); setFeito('');
    if (!enderecoValido(usuario)) { setErro('Informe um endereço de e-mail válido.'); return; }
    if (!senha.trim()) { setErro('Cole a senha de app.'); return; }
    if (pedeServidor && !servidor.trim()) { setErro('Informe o servidor IMAP do seu e-mail.'); return; }
    setOcupado(true);
    try {
      const corpo: Record<string, string> = { usuario: usuario.trim(), senha };
      if (pedeServidor) corpo.servidor = servidor.trim();
      const r = await app.api(ROTA, 'PUT', corpo);
      setEstado({ conectada: r?.conectada === true, usuario: typeof r?.usuario === 'string' ? r.usuario : usuario.trim() });
      setUsuario(''); setServidor('');
      setFeito('Pronto. O Applyer já consegue ler sua caixa de entrada.');
    } catch (e) { setErro(explicar(e)); }
    finally { setSenha(''); setOcupado(false); }
  }

  async function desconectar() {
    if (ocupado) return;
    setOcupado(true); setErro(''); setFeito('');
    try {
      await app.api(ROTA, 'DELETE');
      setEstado({ conectada: false, usuario: '' });
      setFeito('E-mail desconectado. Para revogar a senha de app, apague-a também na sua conta do Google.');
    } catch { setErro('Não deu para desconectar. Tente de novo.'); }
    finally { setOcupado(false); }
  }

  if (estado === null) return null;

  return (
    <section className="panel pr-card" aria-labelledby="conta-email-titulo">
      <div className="pr-card-title"><span className="pr-tile"><IconeUI nome="send" tamanho={19} /></span><h2 id="conta-email-titulo">Caixa de e-mail</h2></div>
      <p className="pr-note">O Applyer lê as respostas das empresas e os alertas de vaga do LinkedIn e do Indeed. Ele só lê: não marca como lido, não move, não apaga e não envia e-mail.</p>
      {feito && <p className="pr-connected" role="status">{feito}</p>}
      {estado.conectada ? <>
        <p className="pr-connected">Conectado: {estado.usuario}</p>
        <button type="button" className="btn secondary" disabled={ocupado} onClick={desconectar}>{ocupado ? 'Desconectando…' : 'Desconectar'}</button>
      </> : <form className="pr-fields" onSubmit={e => { e.preventDefault(); void conectar(); }}>
        <div className="pr-field pr-wide"><label htmlFor="email-usuario">Endereço da caixa</label>
          <input id="email-usuario" type="email" autoComplete="off" maxLength={254} placeholder="nome@gmail.com" value={usuario} onChange={e => setUsuario(e.target.value)} /></div>
        <div className="pr-field pr-wide"><label htmlFor="email-senha">Senha de app</label>
          <input id="email-senha" type="password" autoComplete="off" maxLength={256} placeholder="xxxx xxxx xxxx xxxx" value={senha} onChange={e => setSenha(e.target.value)} aria-describedby="email-senha-dica" /></div>
        {pedeServidor && <div className="pr-field pr-wide"><label htmlFor="email-servidor">Servidor IMAP</label>
          <input id="email-servidor" type="text" autoComplete="off" maxLength={253} placeholder="imap.exemplo.com" value={servidor} onChange={e => setServidor(e.target.value)} /></div>}
        <p className="pr-wide pr-note" id="email-senha-dica">Use uma senha de app, não a senha da conta. No Gmail, crie uma em <LinkExterno url={SENHAS_GOOGLE}>Senhas de app do Google</LinkExterno> (precisa da verificação em duas etapas ligada). Ela fica no cofre deste computador, e você pode revogá-la quando quiser.</p>
        <div className="pr-wide"><button type="submit" className="btn primary" disabled={ocupado}>{ocupado ? 'Conferindo…' : 'Conectar e-mail'}</button></div>
      </form>}
      {erro && <p role="alert">{erro}</p>}
    </section>
  );
}
