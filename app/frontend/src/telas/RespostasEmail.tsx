// "Respostas por e-mail", no Quadro de candidaturas (api/openapi.yaml 0.18.0, rotas /email/conta, /email/ler e
// /emails/{idMensagem}/confirmar|ligar|descartar|criar-vaga). A pessoa pede a leitura; o app lê a caixa conectada, registra as
// respostas e mostra:
// - "Para confirmar": recusa, oferta, ou convite de remetente fora do domínio da vaga. A vaga só muda quando a pessoa
//   confirma; "Não mudar a vaga" (descartar) tira da lista sem mexer na vaga.
// - "Sem vaga" (só convite, teste e oferta): a pessoa cria a vaga com a empresa e o cargo sugeridos (rota criar-vaga),
//   diz de qual vaga é (a etapa muda) ou ignora (descartar).
// Cada grupo traz uma frase que explica o que os botões fazem. Descartado conta como lido: não volta na próxima leitura.
// Sem e-mail conectado, só um convite para conectar em Perfil e IA. Frases fixas: nenhum texto do servidor na tela;
// o resumo da IA aparece como texto, nunca como link.
import { useEffect, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { foiEnviada } from '../dominio/modelo';
import { NOMES_EMAIL, pedeVaga } from '../dominio/emails';
import { dataCurta, encode, nomeEtapa, plural } from '../util';

const ROTA_CONTA = '/api/v1/email/conta';
const ROTA_LER = '/api/v1/email/ler';

type Resultado = { registrados: number; mudaram: number; aConfirmar: number; semVaga: number; pendentes: number; falhas: number; sensiveis: number };

// O que a leitura fez, em frases curtas.
export function resumoDaLeitura(r: Resultado): string {
  const partes: string[] = [];
  if (r.mudaram) partes.push(`${plural(r.mudaram, 'vaga mudou', 'vagas mudaram')} de etapa`);
  if (r.aConfirmar) partes.push(`${plural(r.aConfirmar, 'resposta', 'respostas')} para você confirmar`);
  if (r.semVaga) partes.push(`${plural(r.semVaga, 'resposta', 'respostas')} sem vaga`);
  const outras = r.registrados - r.mudaram - r.aConfirmar - r.semVaga;
  if (outras > 0) partes.push(`${plural(outras, 'resposta registrada', 'respostas registradas')} sem mudança`);
  let texto = partes.length ? `Pronto: ${partes.join(', ')}.` : 'Pronto. Nenhuma resposta nova.';
  if (r.sensiveis) texto += ` ${plural(r.sensiveis, 'e-mail de segurança ou de banco ficou', 'e-mails de segurança ou de banco ficaram')} de fora, sem ir para a IA.`;
  if (r.pendentes) texto += ' Ainda há e-mails para ler: clique de novo.';
  if (r.falhas) texto += ` A IA não conseguiu ler ${plural(r.falhas, 'e-mail', 'e-mails')}; a próxima leitura tenta de novo.`;
  return texto;
}

function explicar(e: unknown) {
  if (e instanceof ErroApi) {
    const motivo = e.dados?.motivo;
    if (motivo === 'sem-conta') return 'Conecte seu e-mail em Perfil e IA.';
    if (motivo === 'sem-ia') return 'Conecte uma IA em Perfil e IA para ler as respostas.';
    if (motivo === 'senha') return 'O e-mail recusou a senha de app. Conecte de novo em Perfil e IA.';
    if (motivo === 'lendo') return 'Já estou lendo o e-mail. Espere terminar.';
    if (e.status === 502) return 'Não consegui ler o e-mail agora. Confira a internet e tente de novo.';
    if (e.status === 403) return 'Só você pode pedir a leitura, por esta tela.';
  }
  return 'Não deu para ler o e-mail. Tente de novo.';
}

const rotuloConfirmar = (etapa: string) => etapa === 'encerrada' ? 'Encerrar a vaga' : `Mover para ${nomeEtapa(etapa)}`;
const linha = (e: any) => [e.de, e.assunto, e.resumo].filter(Boolean).join(' · ');

function EmailAConfirmar({ e, vaga }: { e: any; vaga: any }) {
  const app = useApp();
  const ir = (acao: string, msg: string) => app.agir(() => app.api(`/api/v1/emails/${encode(e.idMensagem)}/${acao}`, 'POST', {}), msg);
  return (
    <li className="j-row">
      <span className="j-company">{NOMES_EMAIL[e.tipo] ?? 'Email'}</span>
      <h3 className="j-email-titulo">{vaga ? `${vaga.empresa} — ${vaga.titulo}` : 'Vaga'}</h3>
      <p className="j-email-linha">{linha(e)}</p>
      <div className="j-email-acoes">
        <button className="btn primary" type="button" onClick={() => ir('confirmar', 'Vaga atualizada')}>{rotuloConfirmar(String(e.proposta?.etapa ?? ''))}</button>
        <button className="btn secondary" type="button" onClick={() => ir('descartar', 'E-mail tirado da lista')}>Não mudar a vaga</button>
      </div>
    </li>
  );
}

// Vaga que não está no app (em geral, candidatura feita fora dele): a pessoa confere a empresa e o cargo que a IA
// tirou do e-mail e cria a vaga, que entra no Quadro na etapa do e-mail; ou liga a uma vaga que já está no app; ou
// ignora.
const destino = (tipo: string) => tipo === 'oferta' ? 'Oferta' : 'Em processo';
const sugestao = (e: any, k: string) => typeof e?.sugestao?.[k] === 'string' ? e.sugestao[k] : '';

function EmailSemVaga({ e, opcoes }: { e: any; opcoes: any[] }) {
  const app = useApp();
  const [escolha, setEscolha] = useState('');
  const [empresa, setEmpresa] = useState(() => sugestao(e, 'empresa'));
  const [cargo, setCargo] = useState(() => sugestao(e, 'cargo'));
  const ir = (acao: string, corpo: object, msg: string) => app.agir(() => app.api(`/api/v1/emails/${encode(e.idMensagem)}/${acao}`, 'POST', corpo), msg);
  const id = encode(e.idMensagem);
  const podeCriar = empresa.trim() !== '' && cargo.trim() !== '';
  return (
    <li className="j-row">
      <span className="j-company">{NOMES_EMAIL[e.tipo] ?? 'Email'}</span>
      <h3 className="j-email-titulo">{e.assunto || '(sem assunto)'}</h3>
      <p className="j-email-linha">{[e.de, e.resumo].filter(Boolean).join(' · ')}</p>
      <form className="j-email-criar" onSubmit={ev => { ev.preventDefault(); if (podeCriar) void ir('criar-vaga', { empresa: empresa.trim(), cargo: cargo.trim() }, `Vaga criada no Quadro, em ${destino(e.tipo)}`); }}>
        <label htmlFor={`empresa-${id}`}>Empresa</label>
        <input id={`empresa-${id}`} type="text" maxLength={120} autoComplete="off" value={empresa} placeholder="Nome da empresa" onChange={ev => setEmpresa(ev.target.value)} />
        <label htmlFor={`cargo-${id}`}>Cargo</label>
        <input id={`cargo-${id}`} type="text" maxLength={120} autoComplete="off" value={cargo} placeholder="Título da vaga" onChange={ev => setCargo(ev.target.value)} />
        <button className="btn primary" type="submit" disabled={!podeCriar}>Criar vaga</button>
      </form>
      <div className="j-email-acoes">
        {opcoes.length > 0 && <>
          <select aria-label={`Vaga do e-mail "${e.assunto}"`} value={escolha} onChange={ev => setEscolha(ev.target.value)}>
            <option value="">Ou é de uma vaga que já está no app?</option>
            {opcoes.map(v => <option key={v.id} value={v.id}>{`${v.empresa} — ${v.titulo}`}</option>)}
          </select>
          <button className="btn secondary" type="button" disabled={!escolha} onClick={() => ir('ligar', { vagaId: escolha }, 'E-mail ligado à vaga')}>Ligar à vaga</button>
        </>}
        <button className="btn secondary" type="button" onClick={() => ir('descartar', {}, 'E-mail tirado da lista')}>Ignorar e-mail</button>
      </div>
    </li>
  );
}

export function RespostasEmail() {
  const app = useApp();
  const [conta, setConta] = useState<{ conectada: boolean; ultimaLeitura: string } | null>(null);
  const [lendo, setLendo] = useState(false);
  const [feito, setFeito] = useState('');
  const [erro, setErro] = useState('');

  useEffect(() => {
    app.api(ROTA_CONTA).then(r => setConta({ conectada: r?.conectada === true, ultimaLeitura: typeof r?.ultimaLeitura === 'string' ? r.ultimaLeitura : '' }))
      .catch(() => setConta(null));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const emails: any[] = app.estado.emails ?? [];
  const aConfirmar = emails.filter(e => e.status === 'a-confirmar');
  const semVaga = emails.filter(pedeVaga);
  const vagas: any[] = app.estado.vagas ?? [];
  const opcoes = vagas.filter(v => foiEnviada(v) || ['aplicar', 'bloqueada', 'em-processo', 'oferta'].includes(v.etapa))
    .sort((a, b) => String(a.empresa).localeCompare(String(b.empresa)));

  if (conta === null && !aConfirmar.length && !semVaga.length) return null;

  async function ler() {
    if (lendo) return;
    setLendo(true); setErro(''); setFeito('');
    try {
      const r = await app.api(ROTA_LER, 'POST', {});
      setFeito(resumoDaLeitura(r));
      if (r?.ultimaLeitura) setConta(c => c ? { ...c, ultimaLeitura: r.ultimaLeitura } : c);
      await app.recarregar();
    } catch (e) { setErro(explicar(e)); }
    finally { setLendo(false); }
  }

  const pendentes = aConfirmar.length + semVaga.length;
  return (
    <section className="j-list-panel j-email" aria-labelledby="respostas-email-titulo">
      <div className="j-panel-heading">
        <h2 id="respostas-email-titulo"><IconeUI nome="send" tamanho={17} />Respostas por e-mail{pendentes > 0 && <span className="j-count">{pendentes}</span>}</h2>
        {conta?.conectada && <button className="btn secondary" type="button" disabled={lendo} onClick={ler}>{lendo ? 'Lendo o e-mail…' : 'Ler respostas do e-mail'}</button>}
      </div>
      <div className="j-email-estado">
        {conta?.conectada
          ? <p>{lendo ? 'Lendo a caixa de entrada. Pode levar alguns minutos.' : conta.ultimaLeitura ? `Última leitura: ${dataCurta(conta.ultimaLeitura)}. O Applyer só lê: não marca, move, apaga nem envia e-mail.` : 'Na primeira vez, o Applyer lê os últimos 30 dias. Ele só lê: não marca, move, apaga nem envia e-mail.'}</p>
          : conta && <p>Conecte seu e-mail em <a href="#/perfil">Perfil e IA</a> para o Applyer ler as respostas das empresas e atualizar as vagas.</p>}
        {conta?.conectada && !lendo && <p>Cada e-mail é lido uma vez só: os que já passaram por aqui, até os ignorados, não voltam nas próximas leituras.</p>}
        {feito && <p className="j-email-feito" role="status">{feito}</p>}
        {erro && <p className="j-email-erro" role="alert">{erro}</p>}
      </div>
      {aConfirmar.length > 0 && <>
        <h3 className="j-email-grupo">Para confirmar</h3>
        <p className="j-email-ajuda">A empresa respondeu sobre uma vaga sua e isso mudaria a etapa dela. Nada muda até você confirmar. Se o e-mail não quer dizer isso, escolha "Não mudar a vaga": ele sai da lista e a vaga fica como está.</p>
        <ul className="j-list">{aConfirmar.map(e => <EmailAConfirmar key={e.idMensagem} e={e} vaga={vagas.find(v => v.id === e.vagaId)} />)}</ul>
      </>}
      {semVaga.length > 0 && <>
        <h3 className="j-email-grupo">Sem vaga</h3>
        <p className="j-email-ajuda">Convites, testes e ofertas de vagas que não estão no Applyer (em geral, candidaturas feitas fora dele). Confira a empresa e o cargo que tirei do e-mail e clique em "Criar vaga": ela entra no Quadro na etapa certa, com o e-mail junto. Se não interessa acompanhar (ou eu me enganei), clique em "Ignorar e-mail": ele sai da lista e nada é apagado do seu e-mail. Confirmações de candidatura e recusas de vagas que não estão no app não aparecem aqui.</p>
        <ul className="j-list">{semVaga.map(e => <EmailSemVaga key={e.idMensagem} e={e} opcoes={opcoes} />)}</ul>
      </>}
    </section>
  );
}
