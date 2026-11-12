// Perfil e regras (porte de web/telas/perfil.mjs). O formulário guarda as edições localmente até salvar.
import { useState } from 'react';
import { useApp, useTrava } from '../app/contexto';
import { JanelaApp } from './JanelaApp';
import { SeusDados } from './SeusDados';
import { SecaoConta } from './Conta';

// Campos simples do perfil: [chave, rótulo, tipo, dica]
const CAMPOS: [string, [string, string, string, string?][]][] = [
  ['Contato', [['nome', 'Nome completo', 'text'], ['email', 'E-mail', 'email'], ['telefone', 'Telefone', 'text'], ['cidade', 'Cidade', 'text'], ['linkedin', 'LinkedIn', 'url'], ['github', 'GitHub', 'url']]],
  ['Profissional', [['cargo', 'Como você se apresenta', 'text'], ['experienciaAnos', 'Anos de experiência', 'number'], ['ingles', 'Inglês', 'text', 'Básico, Conversacional, Avançado…'], ['formacao', 'Formação', 'text'], ['inicio', 'Disponibilidade para começar', 'text']]],
  ['Remuneração', [['pretensaoCLT', 'Pretensão CLT (R$/mês)', 'number'], ['pretensaoPJ', 'Pretensão PJ (R$/mês)', 'number'], ['pretensaoUSD', 'Pretensão internacional (US$/mês)', 'number', 'vagas contractor no exterior'], ['salarioAtual', 'Salário atual', 'text', 'deixe em branco se preferir']]],
];

const caminho = (nome: string) => `curriculos/${nome}`;

function perfilInicial(estado: any) {
  const perfil = structuredClone(estado.perfil ?? {});
  perfil.filtros ??= {};
  perfil.respostasPadrao ??= [];
  perfil.curriculos = { ...(perfil.curriculos ?? {}) };
  const pdfs: any[] = estado.curriculos ?? [];
  // Sem escolha válida: sugere o primeiro PDF para PT e nenhum para EN.
  for (const idioma of ['PT', 'EN']) {
    if (!pdfs.some(p => caminho(p.nome) === perfil.curriculos[idioma])) {
      if (idioma === 'PT' && pdfs[0]) perfil.curriculos.PT = caminho(pdfs[0].nome); else delete perfil.curriculos[idioma];
    }
  }
  return perfil;
}

export function TelaPerfil() {
  const app = useApp();
  const trava = useTrava();
  const [perfil, setPerfil] = useState<any>(() => perfilInicial(app.estado));
  const pdfs: any[] = app.estado.curriculos ?? [];
  const mudar = (fn: (p: any) => void) => setPerfil((p: any) => { const n = structuredClone(p); fn(n); return n; });

  const campo = ([k, rotulo, tipo, dica]: [string, string, string, string?]) => (
    <label key={k} className="campo"><span>{rotulo}</span>
      <input id={`perfil-${k}`} type={tipo} placeholder={dica ?? ''} value={perfil[k] ?? ''}
        onChange={ev => { const v = ev.target.value; mudar(p => { p[k] = tipo === 'number' ? (v === '' ? '' : Number(v)) : v; }); }} />
    </label>
  );
  const lista = (titulo: string, valores: string[] | undefined, aoMudar: (p: any, v: string[]) => void, dica: string) => (
    <label className="campo"><span>{titulo}</span>
      <textarea style={{ minHeight: '64px' }} placeholder={dica} defaultValue={(valores ?? []).join('\n')}
        onChange={ev => { const v = ev.target.value.split('\n').map(s => s.trim()).filter(Boolean); mudar(p => aoMudar(p, v)); }} />
    </label>
  );
  const escolhaCurriculo = (idioma: string, rotulo: string) => (
    <label className="campo"><span>{rotulo}</span>
      <select id={`perfil-cv-${idioma}`} value={perfil.curriculos[idioma] ?? ''} onChange={ev => { const v = ev.target.value; mudar(p => { if (v) p.curriculos[idioma] = v; else delete p.curriculos[idioma]; }); }}>
        {idioma === 'EN' && <option value="">Nenhum (usar o de português)</option>}
        {pdfs.map(p => <option key={p.nome} value={caminho(p.nome)}>{p.nome}</option>)}
      </select>
    </label>
  );
  const modo = (valor: string) => (
    <label style={{ display: 'flex', gap: '8px', alignItems: 'start' }}>
      <input type="radio" name="modo" value={valor} checked={(perfil.modo ?? 'revisar') === valor} onChange={() => mudar(p => { p.modo = valor; })} />
      <span>{valor === 'revisar'
        ? <span><b>Revisar antes</b> — só as vagas que você aprovou na Triagem vão para a Fila.</span>
        : <span><b>Aplicar direto</b> — as vagas do grupo A também vão para a Fila sem passar pela Triagem; B e C esperam por ela. O pacote de cada uma ainda precisa da sua aprovação.</span>}</span>
    </label>
  );

  return (<>
    <form className="formulario" onSubmit={async ev => {
      ev.preventDefault();
      if (trava.travado) return;
      if (await app.agir(() => app.api('/api/v1/perfil', 'PUT', perfil), 'Perfil salvo')) app.limpar();
    }}>
      <header className="cabecalho"><div><h1>Perfil e regras</h1><p>O Claude lê esta página antes de cada rodada. CPF, RG e senhas nunca ficam aqui.</p></div>
        <button className="botao" type="submit" {...trava()}>Salvar perfil</button></header>
      {CAMPOS.map(([grupo, campos]) => <fieldset key={grupo}><legend>{grupo}</legend><div className="duas">{campos.map(campo)}</div></fieldset>)}
      <fieldset><legend>Currículos</legend>
        {pdfs.length ? (
          <>
            <p className="pequeno mudo" style={{ margin: 0 }}>{`${pdfs.length === 1 ? 'Encontrei 1 PDF' : `Encontrei ${pdfs.length} PDFs`} na pasta de currículos. O Claude escolhe pelo idioma da vaga.`}</p>
            <div className="duas">{escolhaCurriculo('PT', 'Vagas em português')}{escolhaCurriculo('EN', 'Vagas em inglês')}</div>
          </>
        ) : <p className="aviso-cv"><strong>Nenhum currículo ainda. </strong>Copie seu PDF para a pasta <code>{app.estado.pastaCurriculos ?? 'de currículos do app'}</code>. Esta página atualiza sozinha quando ele aparecer.</p>}
      </fieldset>
      <fieldset><legend>Busca e filtros</legend>
        <div className="duas">
          {lista('Níveis aceitos', perfil.filtros.niveis, (p, v) => { p.filtros.niveis = v; }, 'Júnior\nPleno')}
          {lista('Modelo de trabalho', perfil.filtros.modelo, (p, v) => { p.filtros.modelo = v; }, 'Remoto')}
          {lista('Stack principal', perfil.stackPrincipal, (p, v) => { p.stackPrincipal = v; }, 'Java\nSpring Boot')}
          {lista('Stack secundária', perfil.stackSecundaria, (p, v) => { p.stackSecundaria = v; }, 'React\nNode.js')}
        </div>
        {lista('Pular quando', perfil.filtros.pularSe, (p, v) => { p.filtros.pularSe = v; }, 'Sênior\nGraduação concluída obrigatória')}
      </fieldset>
      <fieldset><legend>Como o Claude trabalha</legend>{modo('revisar')}{modo('direto')}
        <label className="campo" style={{ maxWidth: '260px' }}><span>Follow-up depois de quantos dias</span>
          <input id="perfil-dias" type="number" min={1} max={60} value={perfil.diasFollowUp ?? 7} onChange={ev => { const v = ev.target.value; mudar(p => { p.diasFollowUp = Number(v) || 7; }); }} />
        </label>
      </fieldset>
      <fieldset><legend>Respostas padrão dos formulários</legend>
        <p className="pequeno mudo" style={{ margin: 0 }}>Respostas honestas que o Claude repete nos formulários. Se algo não está aqui nem no currículo, ele responde "não" em vez de inventar.</p>
        <div className="respostas">
          {perfil.respostasPadrao.map((r: any, i: number) => (
            <div key={i} className="par">
              <input type="text" aria-label="Pergunta" value={r.pergunta ?? ''} onChange={ev => { const v = ev.target.value; mudar(p => { p.respostasPadrao[i].pergunta = v; }); }} />
              <input type="text" aria-label="Resposta" value={r.resposta ?? ''} onChange={ev => { const v = ev.target.value; mudar(p => { p.respostasPadrao[i].resposta = v; }); }} />
              <button className="botao-fantasma p" type="button" aria-label={`Remover ${r.pergunta}`} onClick={() => mudar(p => { p.respostasPadrao.splice(i, 1); })}>Remover</button>
            </div>
          ))}
        </div>
        <div><button className="botao-fantasma" type="button" onClick={() => mudar(p => { p.respostasPadrao.push({ pergunta: '', resposta: '' }); })}>Adicionar resposta</button></div>
      </fieldset>
      <div><button className="botao" type="submit" {...trava()}>Salvar perfil</button></div>
    </form>
    <SecaoConta />
    <JanelaApp />
    <SeusDados />
  </>);
}
