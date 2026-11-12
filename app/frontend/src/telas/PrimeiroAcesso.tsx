import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { useRascunhoOnboarding, type PDFSalvo, type TentativaExterna } from '../app/rascunhoOnboarding';
import { Icone } from '../componentes';
import { NOMES_FOCO } from '../dominio/modelo';
import { aceitaTrabalhoLocal, cargoIgualAoNome, paises } from '../dominio/localDeTrabalho';
import { ConexaoOnboarding } from './ConexaoOnboarding';
import paisagem from '../arte/primeiro-acesso/paisagem.svg';
import marca from '../arte/primeiro-acesso/applyer.svg';
import marcaEscura from '../arte/primeiro-acesso/applyer-inversa.svg';
import arteCurriculo from '../../../../site/artes/curriculo-verificado.svg';
import '../estilo/primeiro-acesso.css';

const PASSOS = ['IA e currículo', 'Critérios de busca'];
const CLASSES_PROVEDOR: Record<string, string> = { Claude: 'anthropic', OpenAI: 'openai', Gemini: 'google', Grok: 'xai' };
const NIVEIS = ['Estágio', 'Júnior', 'Pleno', 'Sênior', 'Liderança'];
const MODELOS = ['Remoto', 'Híbrido', 'Presencial'];
const INGLES = ['Não falo inglês', 'Básico', 'Intermediário', 'Avançado', 'Fluente'];
const TAMANHO_PDF = 20 * 1024 * 1024;
const MOEDAS = { USD: 'Dólar americano', BRL: 'Real brasileiro', EUR: 'Euro' };
const NOMES_SUGESTAO = { cargo: 'Cargo', niveis: 'Nível', stackPrincipal: 'Competências principais', stackSecundaria: 'Competências complementares', ingles: 'Inglês' };
type Sugestao = { campo: keyof typeof NOMES_SUGESTAO; valor: string; evidencia: string };
const lista = (v: unknown): string[] => Array.isArray(v) ? v.filter((item): item is string => typeof item === 'string' && item.trim() !== '') : [];
const termos = (texto: string) => [...new Set(texto.split(/[,;\n]/).map(t => t.trim()).filter(Boolean))];

function perfilInicial(perfil: any = {}) {
  const faixa = perfil.faixaSalarial ?? { min: perfil.pretensaoUSD, moeda: 'USD' };
  return {
    cargo: perfil.cargo ?? '', niveis: lista(perfil.filtros?.niveis), modelos: lista(perfil.filtros?.modelo),
    paisTrabalho: String(perfil.paisTrabalho ?? ''), cidadeTrabalho: String(perfil.cidadeTrabalho ?? ''),
    foco: (Object.prototype.hasOwnProperty.call(NOMES_FOCO, perfil.foco) ? perfil.foco : '') as keyof typeof NOMES_FOCO | '',
    principal: lista(perfil.stackPrincipal).join(', '), secundaria: lista(perfil.stackSecundaria).join(', '),
    pular: lista(perfil.filtros?.pularSe).join(', '), ingles: perfil.ingles ?? '',
    moeda: (Object.prototype.hasOwnProperty.call(MOEDAS, faixa.moeda) ? faixa.moeda : 'USD') as keyof typeof MOEDAS,
    minimo: faixa.min == null ? '' : String(faixa.min), maximo: faixa.max == null ? '' : String(faixa.max),
  };
}

type Dados = ReturnType<typeof perfilInicial>;
function validar(dados: Dados) {
  const falha = (campo: string, texto: string) => ({ campo, texto });
  if (!dados.cargo.trim()) return falha('cargo', 'Informe um cargo ou área de interesse.');
  if (!dados.niveis.length) return falha('niveis', 'Selecione pelo menos um nível de experiência.');
  if (!termos(dados.principal).length) return falha('principal-stack', 'Informe pelo menos uma habilidade ou ferramenta principal.');
  if (!dados.ingles) return falha('ingles', 'Informe seu nível de inglês.');
  if (!dados.foco) return falha('mercado', 'Escolha o mercado em que deseja buscar vagas.');
  if (!dados.modelos.length) return falha('modelos', 'Selecione pelo menos um modelo de trabalho.');
  for (const campo of ['minimo', 'maximo'] as const) {
    if (dados[campo] && (!Number.isFinite(Number(dados[campo])) || Number(dados[campo]) <= 0)) return falha(campo, 'Use um valor positivo ou deixe o limite em branco.');
  }
  if (dados.minimo && dados.maximo && Number(dados.maximo) < Number(dados.minimo)) return falha('maximo', 'O máximo precisa ser maior ou igual ao mínimo.');
  return null;
}

function Opcoes({ nome, titulo, itens, ativos, mudar }: {
  nome: string; titulo: string; itens: string[]; ativos: string[]; mudar: (v: string[]) => void;
}) {
  return <fieldset><legend>{titulo}</legend><div className="pa-opcoes">
    {[...new Set([...itens, ...ativos])].map((v, i) => <label key={v}>
      <input id={i === 0 ? `pa-${nome}` : undefined} type="checkbox" checked={ativos.includes(v)}
        onChange={e => mudar(e.target.checked ? [...ativos, v] : ativos.filter(x => x !== v))} /><span>{v}</span>
    </label>)}
  </div></fieldset>;
}

export function lerArquivo(blob: Blob, base64 = false): Promise<string> {
  return new Promise((resolve, reject) => {
    const leitor = new FileReader();
    leitor.onerror = () => reject(new Error('Não foi possível ler esse arquivo. Escolha o PDF novamente.'));
    leitor.onload = () => {
      const resultado = String(leitor.result);
      resolve(base64 ? resultado.slice(resultado.indexOf(',') + 1) : resultado);
    };
    if (base64) leitor.readAsDataURL(blob); else leitor.readAsText(blob);
  });
}

function aplicarSugestoes(base: Dados, sugestoes: Sugestao[], editados: Set<keyof Dados>): Dados {
  const valores = (campo: Sugestao['campo']) => [...new Set(sugestoes.filter(s => s.campo === campo).map(s => s.valor))];
  const proximo = { ...base };
  const preencher = (campo: 'cargo' | 'principal' | 'secundaria' | 'ingles', valor: string) => {
    if (!editados.has(campo) && !base[campo]) proximo[campo] = valor;
  };
  preencher('cargo', valores('cargo')[0] ?? '');
  preencher('principal', valores('stackPrincipal').join(', '));
  preencher('secundaria', valores('stackSecundaria').join(', '));
  preencher('ingles', valores('ingles')[0] ?? '');
  if (!editados.has('niveis') && !base.niveis.length) proximo.niveis = valores('niveis');
  return proximo;
}

function Evidencias({ sugestoes }: { sugestoes: Sugestao[] }) {
  if (!sugestoes.length) return null;
  return <details className="pa-evidencias">
    <summary>Ver trechos do currículo</summary>
    <dl>{sugestoes.map(s => <div key={s.campo + s.valor}><dt>{NOMES_SUGESTAO[s.campo]} · {s.valor}</dt><dd>“{s.evidencia}”</dd></div>)}</dl>
  </details>;
}

export function PrimeiroAcesso({ alternarTema, concluir }: {
  alternarTema: () => void; concluir: (buscar: boolean) => void;
}) {
  const app = useApp();
  const [dados, setDados] = useState(() => perfilInicial(app.estado.perfil ?? {}));
  const [curriculo, setCurriculo] = useState<PDFSalvo | null>(null);
  const [nomeSelecionado, setNomeSelecionado] = useState('');
  const pdfParaRepetir = useRef<File | 'remover'>();
  const retomarCriterios = useRef(false);
  const [tentativaExterna, setTentativaExterna] = useState<TentativaExterna>('');
  const [sugestoes, setSugestoes] = useState<Sugestao[] | null>(null);
  const [etapa, setEtapa] = useState(1);
  const [provedor, setProvedor] = useState('');
  const verificacao=useRef<string>();
  const cabecalhosIA=():Record<string,string>=>verificacao.current ? {'X-Applyer-Onboarding':'1','X-Applyer-Verificacao':verificacao.current} : {'X-Applyer-Onboarding':'1'};
  const [erro, setErro] = useState('');
  const [lendo, setLendo] = useState(false);
  const [extraindo, setExtraindo] = useState(false);
  const [salvando, setSalvando] = useState(false);
  const [arrastando, setArrastando] = useState(false);
  const inputPDF = useRef<HTMLInputElement>(null);
  const titulo = useRef<HTMLHeadingElement>(null);
  const leitura = useRef(0);
  const enviando = useRef(false);
  const editados = useRef(new Set<keyof Dados>());
  const iniciais = useRef(dados);
  const rascunho = useRascunhoOnboarding(app.api, salvo => {
    leitura.current++; editados.current = new Set(salvo.editados as (keyof Dados)[]);
    pdfParaRepetir.current = undefined; setNomeSelecionado(''); setErro(''); setSugestoes(null);
    verificacao.current = undefined; setProvedor(''); setEtapa(1);
    setCurriculo(salvo.curriculo); setDados({ ...iniciais.current, ...salvo.dados });
    retomarCriterios.current = salvo.etapa === 2 && Boolean(salvo.curriculo);
    setTentativaExterna(salvo.tentativaExterna ?? '');
  });
  useEffect(() => { titulo.current?.focus(); }, [etapa]);
  useEffect(() => () => { leitura.current++; }, []);

  function campo<K extends keyof Dados>(chave: K, valor: Dados[K]) {
    editados.current.add(chave);
    rascunho.alterar({dados:{[chave]:valor},editados:[...editados.current]});
    setDados(antes => ({ ...antes, [chave]: valor })); setErro('');
  }
  async function selecionar(arquivos: ArrayLike<File> | null) {
    if (extraindo || salvando || lendo || rascunho.revisao.current == null || rascunho.situacao==='conflito') return;
    const versao = ++leitura.current;
    setArrastando(false); setErro(''); setLendo(false); setNomeSelecionado(''); pdfParaRepetir.current = undefined;
    if (!arquivos?.length) return;
    const pdf = arquivos[0];
    if (arquivos.length !== 1) { setErro('Escolha um currículo por vez.'); return; }
    if (!/\.pdf$/i.test(pdf.name)) { setErro('Escolha um arquivo com extensão .pdf.'); return; }
    if (pdf.size > TAMANHO_PDF) { setErro('O PDF deve ter até 20 MB.'); return; }
    setLendo(true); setNomeSelecionado(pdf.name);
    try {
      const [inicio, fim] = await Promise.all([lerArquivo(pdf.slice(0, 5)), lerArquivo(pdf.slice(-1024))]);
      if (leitura.current !== versao) return;
      if (inicio !== '%PDF-') setErro('Esse arquivo não tem o formato de um PDF.');
      else if (!fim.includes('%%EOF')) setErro('O PDF está incompleto. Exporte o currículo novamente e escolha o arquivo salvo.');
      else {
        const base64 = await lerArquivo(pdf, true);
        if (leitura.current !== versao) return;
        pdfParaRepetir.current = pdf;
        const salvo = await rascunho.documento('PUT', {nomeArquivo:pdf.name,curriculoBase64:base64});
        if (leitura.current !== versao) return;
        pdfParaRepetir.current = undefined;
        trocarCurriculo(salvo);
      }
    } catch (e) { if (leitura.current === versao && !pdfParaRepetir.current) setErro((e as Error).message); }
    finally { if (leitura.current === versao) {setLendo(false); setNomeSelecionado('');} }
  }
  function trocarCurriculo(novo: PDFSalvo | null) {
    setCurriculo(novo); setSugestoes(null);
    setDados(antes => {
      const limpo = { ...iniciais.current };
      for (const chave of editados.current) Object.assign(limpo, { [chave]: antes[chave] });
      return limpo;
    });
  }
  async function remover() {
    if (lendo || enviando.current) return;
    leitura.current++; setLendo(true); setErro(''); pdfParaRepetir.current='remover';
    try {
      await rascunho.documento('DELETE',{});
      trocarCurriculo(null); retomarCriterios.current=false; pdfParaRepetir.current=undefined;
    } catch { /* O rascunho mostra a falha e mantém o PDF atual. */ }
    finally {setLendo(false);}
  }
  async function irPara(etapa: number) {
    rascunho.alterar({etapa});
    try {await rascunho.gravar(); setErro(''); setEtapa(etapa);}
    catch { /* A pessoa pode repetir a gravação sem perder os campos. */ }
  }
  const podeRevisar = Boolean(curriculo && provedor && !lendo && !extraindo && rascunho.situacao!=='conflito' && rascunho.situacao!=='falha');
  async function preencherComIA() {
    if (!podeRevisar || !curriculo || enviando.current) return;
    setErro('');
    if (sugestoes !== null) { await irPara(2); return; }
    const versao = leitura.current;
    enviando.current = true; setExtraindo(true);
    try {
      await rascunho.gravar();
      const resposta = await app.api('/api/v1/onboarding/sugerir', 'POST', { arquivoId: curriculo.arquivoId, revisao: rascunho.revisao.current }, cabecalhosIA());
      if (leitura.current !== versao) return;
      const itens: Sugestao[] = resposta.sugestoes ?? [];
      setSugestoes(itens);
      const sugerido = aplicarSugestoes(dados, itens, editados.current);
      setDados(sugerido);
      rascunho.alterar({dados:sugerido,editados:[...editados.current]});
      await irPara(2);
    } catch (e) { if (leitura.current === versao) {
      if(e instanceof ErroApi && e.status===412){setProvedor('');verificacao.current=undefined;}
      if(e instanceof ErroApi && e.status===409) rascunho.avisarConflito();
      setErro((e as Error).message || 'A IA não conseguiu ler este PDF. Você pode tentar novamente ou preencher o perfil manualmente.');
    } }
    finally { enviando.current = false; setExtraindo(false); }
  }
  async function salvar(ev: FormEvent) {
    ev.preventDefault();
    if (enviando.current || !podeRevisar || !curriculo) return;
    const buscar = ((ev.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null)?.value === 'buscar';
    const problema = validar(dados);
    if (problema) {
      setErro(problema.texto);
      document.getElementById(`pa-${problema.campo}`)?.focus();
      return;
    }
    const perfil = {
      cargo: dados.cargo.trim(), stackPrincipal: termos(dados.principal), stackSecundaria: termos(dados.secundaria),
      ...(dados.paisTrabalho && {paisTrabalho:dados.paisTrabalho}),
      ...(dados.cidadeTrabalho.trim() && {cidadeTrabalho:dados.cidadeTrabalho.trim()}),
      filtros: { niveis: dados.niveis, modelo: dados.modelos, pularSe: termos(dados.pular) }, foco: dados.foco, ingles: dados.ingles,
      faixaSalarial: dados.minimo || dados.maximo ? {
        moeda: dados.moeda, ...(dados.minimo ? { min: Number(dados.minimo) } : {}), ...(dados.maximo ? { max: Number(dados.maximo) } : {}),
      } : null,
    };
    enviando.current = true; setSalvando(true); setErro('');
    try {
      await rascunho.gravar();
      await app.api('/api/v1/onboarding', 'POST', { perfil, arquivoId:curriculo.arquivoId, revisao:rascunho.revisao.current }, cabecalhosIA());
      app.limpar(); await concluir(buscar);
    } catch (e) {
      if(e instanceof ErroApi && e.status===412) {
        retomarCriterios.current=true; setEtapa(1); setProvedor(''); verificacao.current=undefined; setErro('A conexão de IA mudou. Verifique novamente; o currículo e os campos continuam aqui.');
      } else if (e instanceof ErroApi && e.status === 409) {
        rascunho.avisarConflito(); await app.recarregar();
      } else setErro((e as Error).message || 'Não foi possível salvar. Tente novamente.');
    } finally { enviando.current = false; setSalvando(false); }
  }
  const conexaoConfirmada = <span className={`pa-verificada ${CLASSES_PROVEDOR[provedor] ?? ''}`}>
    <span className="pa-logotipo" aria-hidden="true" />{provedor} conectado
  </span>;
  return <div className="primeiro-acesso">
    <a className="pular-link" href="#pa-principal" onClick={e => { e.preventDefault(); document.getElementById('pa-principal')?.focus(); }}>Ir para o conteúdo</a>
    <header className="pa-topo"><div className="pa-barra">
      <span className="pa-marca" role="img" aria-label="Applyer"><img className="pa-clara" src={marca} alt="" /><img className="pa-escura" src={marcaEscura} alt="" /></span>
      <span className="pa-titulo">Primeiro acesso</span><span className="pa-local">Dados locais</span>
      <button type="button" className="botao-icone" onClick={alternarTema} aria-label="Alternar tema claro/escuro"><Icone d="M12 3a9 9 0 1 0 9 9 7 7 0 0 1-9-9z" /></button>
    </div></header>
    <div className="pa-janela">
      <main id="pa-principal" tabIndex={-1}>
        <ol className="pa-andamento" aria-label={`Etapas da configuração. Etapa ${etapa} de ${PASSOS.length}`}>{PASSOS.map((nome, i) =>
          <li key={nome} aria-current={i + 1 === etapa ? 'step' : undefined} className={i + 1 < etapa ? 'concluida' : undefined}>
            <span aria-hidden="true">{i + 1 < etapa ? '✓' : i + 1}</span>{nome}
          </li>)}</ol>
        <div className="pa-rascunho" role="status">
          {rascunho.situacao === 'abrindo' && 'Abrindo rascunho…'}
          {rascunho.situacao === 'salvando' && 'Salvando…'}
          {rascunho.situacao === 'salvo' && (retomarCriterios.current ? 'Rascunho recuperado. Verifique a conexão para voltar aos seus critérios.' : 'Salvo neste computador')}
          {(rascunho.situacao === 'conflito' || rascunho.situacao === 'falha') && <>{rascunho.mensagem}{' '}
            <button type="button" className="pa-link" onClick={() => {
              if (rascunho.situacao === 'conflito') {void rascunho.carregar(); return;}
              const tentativa = pdfParaRepetir.current;
              if (tentativa === 'remover') void remover();
              else if (tentativa) void selecionar([tentativa]);
              else void rascunho.repetir();
            }}>{rascunho.situacao === 'conflito' ? 'Descartar edições locais e carregar a versão salva' : 'Tentar salvar novamente'}</button></>}
        </div>
        {etapa === 1 ? <div className="pa-passo">
          <section className="pa-painel-rolagem">
            <header className="pa-faixa"><div>
              <h1 ref={titulo} tabIndex={-1}>Sua IA e seu currículo</h1>
              <p className="pa-sub">Conecte seu provedor de IA e escolha o PDF que vai usar neste perfil.</p>
            </div><img src={paisagem} alt="" /></header>
            <div className="pa-cartoes">
              <section className="pa-cartao" aria-labelledby="pa-ia-titulo">
                <h2 id="pa-ia-titulo">Conexão de IA</h2>
                {rascunho.revisao.current == null ? <p className="pa-nota">A conexão estará disponível após abrir o rascunho local.</p> : provedor ? <div className="pa-conectada"><div role="status">{conexaoConfirmada}</div>
                  <button type="button" className="pa-link" disabled={extraindo} onClick={() => setProvedor('')}>Alterar conexão</button>
                </div> : <ConexaoOnboarding tentativaExterna={tentativaExterna} registrarTentativa={tipo=>{
                  setTentativaExterna(tipo);rascunho.alterar({tentativaExterna:tipo});void rascunho.gravar().catch(()=>{});
                }} aoConectar={(nome,id)=>{
                  verificacao.current=id; setProvedor(nome);
                  if(tentativaExterna){setTentativaExterna('');rascunho.alterar({tentativaExterna:''});}
                  if(retomarCriterios.current && curriculo){retomarCriterios.current=false;setEtapa(2);}
                }} />}
              </section>
              <section className="pa-cartao pa-cartao-pdf" aria-labelledby="pa-pdf-titulo">
                <h2 id="pa-pdf-titulo">Seu currículo</h2>
                <div className={`pa-drop${arrastando || curriculo ? ' destacado' : ''}`} onDragOver={e => { e.preventDefault(); setArrastando(true); }}
                  onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setArrastando(false); }}
                  onDrop={e => { e.preventDefault(); void selecionar(e.dataTransfer.files); }}>
                  <img className="pa-arte-cv" src={arteCurriculo} alt="" />
                  <div aria-live="polite"><b>{curriculo?.nomeExibicao ?? 'Arraste seu currículo para cá'}</b>
                    <p>{nomeSelecionado ? `Selecionado: ${nomeSelecionado} · Salvando…` : curriculo ? `Currículo salvo neste perfil · ${Math.max(1, Math.ceil(curriculo.tamanho / 1024))} KB` : 'PDF de até 20 MB'}</p></div>
                  <div className="pa-arquivo-acoes">
                    <button type="button" className="pa-secundario" disabled={extraindo || lendo || rascunho.revisao.current == null || rascunho.situacao==='conflito'} onClick={() => inputPDF.current?.click()}>{curriculo ? 'Trocar PDF' : 'Escolher PDF'}</button>
                    {curriculo && <button type="button" className="pa-link" disabled={extraindo || lendo || rascunho.situacao==='conflito'} onClick={() => void remover()}>Remover</button>}
                  </div>
                  <input type="file" ref={inputPDF} aria-label="Currículo em PDF" accept=".pdf,application/pdf" hidden onChange={e => { void selecionar(e.target.files); e.target.value = ''; }} />
                </div>
                <p className="pa-nota">O Applyer guarda uma cópia do PDF neste computador para você retomar o primeiro acesso.</p>
                {erro && <p className="pa-erro" role="alert">{erro}</p>}
              </section>
            </div>
          </section>
          <footer className="pa-acoes pa-avancar">
            {sugestoes === null && <p className="pa-nota">Ao preencher com IA, você compartilha o texto profissional do currículo com {provedor || 'seu provedor'} para receber sugestões.</p>}
            {sugestoes === null && <button type="button" className="pa-link pa-manual" disabled={!podeRevisar} onClick={() => void irPara(2)}>Preencher manualmente</button>}
            <button type="button" className="pa-primario" disabled={!podeRevisar} onClick={() => void preencherComIA()}>
              {extraindo ? 'Lendo currículo…' : sugestoes === null ? 'Preencher com IA' : 'Revisar critérios'} <span aria-hidden="true">→</span>
            </button>
          </footer>
        </div> : <form className="pa-passo" onSubmit={salvar} onBlur={rascunho.aoSairDoCampo} noValidate>
          <section className="pa-painel-rolagem">
            <h1 ref={titulo} tabIndex={-1}>Que vagas você procura?</h1>
            <p className="pa-sub">{sugestoes?.length ? 'Revise as sugestões do seu currículo e escolha onde quer trabalhar.' : sugestoes ? 'A IA não encontrou dados suficientes no currículo. Preencha os campos para orientar a busca.' : 'Informe o cargo, suas habilidades e as condições que você aceita.'}</p>
            <div className="pa-documentos">{conexaoConfirmada}<span><img src={arteCurriculo} alt="" />{curriculo?.nomeExibicao}</span></div>
            <Evidencias sugestoes={sugestoes ?? []} />
            <div className="pa-cartoes">
            <fieldset className="pa-grupo-campos" disabled={salvando}>
              <legend>Perfil profissional</legend>
              <div className="pa-campos">
                <label>Cargo ou área de interesse<input id="pa-cargo" maxLength={120} value={dados.cargo} onChange={e => campo('cargo', e.target.value)} /></label>
                {cargoIgualAoNome(dados.cargo, app.estado.perfil?.nome) && <div className="pa-nota">
                  <p>O cargo coincide com seu nome. Informe a função que deseja buscar.</p>
                  {sugestoes?.filter(s => s.campo === 'cargo' && !cargoIgualAoNome(s.valor, dados.cargo)).map(s =>
                    <button key={s.valor} type="button" className="pa-link" onClick={() => campo('cargo',s.valor)}>{s.valor}</button>)}
                </div>}
                <Opcoes nome="niveis" titulo="Níveis aceitos" itens={NIVEIS} ativos={dados.niveis} mudar={v => campo('niveis', v)} />
                <div className="pa-dupla">
                  <label>Competências principais<small>Habilidades ou ferramentas que usa na função</small><input id="pa-principal-stack" maxLength={350} value={dados.principal} onChange={e => campo('principal', e.target.value)} /></label>
                  <label>Competências complementares<small>Opcional</small><input maxLength={350} value={dados.secundaria} onChange={e => campo('secundaria', e.target.value)} /></label>
                </div>
                <label>Seu nível de inglês<select id="pa-ingles" value={dados.ingles} onChange={e => campo('ingles', e.target.value)}>
                  <option value="">Selecione um nível</option>{[...new Set([...INGLES, dados.ingles])].filter(Boolean).map(v => <option key={v}>{v}</option>)}</select></label>
              </div>
            </fieldset>
            <fieldset className="pa-grupo-campos" disabled={salvando}>
              <legend>Filtros de busca</legend>
              <div className="pa-campos">
                <fieldset><legend>Mercado de vagas</legend><div className="pa-opcoes">{(Object.entries(NOMES_FOCO) as [keyof typeof NOMES_FOCO, string][]).map(([key, nome], i) =>
                  <label key={key}><input id={i === 0 ? 'pa-mercado' : undefined} type="radio" name="mercado" checked={dados.foco === key} onChange={() => campo('foco', key)} /><span>{nome}</span></label>)}</div></fieldset>
                <Opcoes nome="modelos" titulo="Modelos de trabalho aceitos" itens={MODELOS} ativos={dados.modelos} mudar={v => campo('modelos', v)} />
                <label>País de trabalho<select value={dados.paisTrabalho} onChange={e => campo('paisTrabalho',e.target.value)}>
                  <option value="">Selecione de onde vai trabalhar</option>{paises.map(([codigo,nome]) => <option key={codigo} value={codigo}>{nome}</option>)}
                </select></label>
                {aceitaTrabalhoLocal(dados.modelos) && <label>Cidade de trabalho<input maxLength={120} value={dados.cidadeTrabalho}
                  onChange={e=>campo('cidadeTrabalho',e.target.value)} /></label>}
                <label>Pular quando<small>Termos que excluem uma vaga, separados por vírgula · opcional</small><input maxLength={350} value={dados.pular} placeholder="Ex.: plantão obrigatório" onChange={e => campo('pular', e.target.value)} /></label>
                <fieldset className="pa-salario"><legend>Faixa salarial mensal <small>Opcional</small></legend>
                  <div className="pa-limites">
                    <label>Moeda<select value={dados.moeda} onChange={e => campo('moeda', e.target.value as Dados['moeda'])}>{Object.entries(MOEDAS).map(([sigla, nome]) => <option key={sigla} value={sigla}>{sigla} · {nome}</option>)}</select></label>
                    <label>Mínimo<input id="pa-minimo" type="number" min=".01" step=".01" inputMode="decimal" placeholder="4.000" value={dados.minimo} onChange={e => campo('minimo', e.target.value)} /></label>
                    <label>Máximo<input id="pa-maximo" type="number" min=".01" step=".01" inputMode="decimal" placeholder="6.000" value={dados.maximo} onChange={e => campo('maximo', e.target.value)} /></label>
                  </div>
                </fieldset>
              </div>
            </fieldset>
            </div>
          </section>
          {erro && <p className="pa-erro" role="alert">{erro}</p>}
          <footer className="pa-acoes">
            <button type="button" className="pa-link" disabled={salvando} onClick={() => void irPara(1)}><span aria-hidden="true">← </span>Voltar</button>
            <div className="pa-salvar-acoes">
              <button type="submit" className="pa-secundario" disabled={salvando || !podeRevisar}>Concluir perfil</button>
              <button type="submit" value="buscar" className="pa-primario" disabled={salvando || !podeRevisar}>{salvando ? 'Salvando…' : 'Salvar e buscar vagas'}</button>
            </div>
          </footer>
        </form>}
      </main>
    </div>
  </div>;
}
