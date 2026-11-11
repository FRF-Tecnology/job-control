import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { Icone } from '../componentes';
import { NOMES_FOCO } from '../dominio/modelo';
import { TelaConectarIA } from './ConectarIA';
import paisagem from '../arte/primeiro-acesso/paisagem.svg';
import marca from '../arte/primeiro-acesso/applyer.svg';
import marcaEscura from '../arte/primeiro-acesso/applyer-inversa.svg';
import '../estilo/primeiro-acesso.css';

const PASSOS = [
  ['Currículo', 'Seu documento de referência'],
  ['Vagas e stack', 'Cargo, nível e habilidades'],
  ['Condições', 'Mercado, idioma e valor'],
  ['Triagem', 'Revisão e primeira busca'],
];
const NIVEIS = ['Estágio', 'Júnior', 'Pleno', 'Sênior', 'Liderança'];
const MODELOS = ['Remoto', 'Híbrido', 'Presencial'];
const INGLES = ['Não falo inglês', 'Básico', 'Intermediário', 'Avançado', 'Fluente'];
const TAMANHO_PDF = 20 * 1024 * 1024;
const lista = (v: unknown): string[] => Array.isArray(v) ? v.filter((item): item is string => typeof item === 'string' && item.trim() !== '') : [];
const termos = (texto: string) => [...new Set(texto.split(/[,;\n]/).map(t => t.trim()).filter(Boolean))];

function perfilInicial(perfil: any = {}) {
  return {
    cargo: perfil.cargo ?? '', niveis: lista(perfil.filtros?.niveis), modelos: lista(perfil.filtros?.modelo),
    foco: (Object.prototype.hasOwnProperty.call(NOMES_FOCO, perfil.foco) ? perfil.foco : '') as keyof typeof NOMES_FOCO | '',
    principal: lista(perfil.stackPrincipal).join(', '), secundaria: lista(perfil.stackSecundaria).join(', '),
    pular: lista(perfil.filtros?.pularSe).join(', '), ingles: perfil.ingles ?? '',
    usd: perfil.pretensaoUSD == null ? '' : String(perfil.pretensaoUSD),
  };
}

type Dados = ReturnType<typeof perfilInicial>;
type Problema = { etapa: number; campo: string; texto: string };

function validar(etapa: number, dados: Dados, arquivo: File | null): Problema | null {
  const falha = (campo: string, texto: string) => ({ etapa, campo, texto });
  if (etapa === 0 && !arquivo) return falha('arquivo', 'Escolha seu currículo em PDF para continuar.');
  if (etapa === 1) {
    if (!dados.cargo.trim()) return falha('cargo', 'Informe um cargo ou área de interesse.');
    if (!dados.niveis.length) return falha('niveis', 'Selecione pelo menos um nível de experiência.');
    if (!termos(dados.principal).length) return falha('principal-stack', 'Informe pelo menos uma habilidade ou ferramenta principal.');
  }
  if (etapa === 2) {
    if (!dados.foco) return falha('mercado', 'Escolha o mercado em que deseja buscar vagas.');
    if (!dados.modelos.length) return falha('modelos', 'Selecione pelo menos um modelo de trabalho.');
    if (!dados.ingles) return falha('ingles', 'Informe seu nível de inglês.');
    if (dados.foco !== 'nacional' && dados.usd && (!Number.isFinite(Number(dados.usd)) || Number(dados.usd) <= 0)) {
      return falha('usd', 'Informe um valor positivo em dólares ou deixe o campo em branco.');
    }
  }
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

function lerTrecho(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const leitor = new FileReader();
    leitor.onerror = () => reject(new Error('Não foi possível ler esse arquivo. Escolha o PDF novamente.'));
    leitor.onload = () => resolve(String(leitor.result));
    leitor.readAsText(blob);
  });
}

export function PrimeiroAcesso({ iaPronta, alternarTema, concluir }: {
  iaPronta: boolean; alternarTema: () => void; concluir: (buscar: boolean) => void;
}) {
  const app = useApp();
  const [dados, setDados] = useState(() => perfilInicial(app.estado.perfil ?? {}));
  const [arquivo, setArquivo] = useState<File | null>(null);
  const [etapa, setEtapa] = useState(0);
  const [liberada, setLiberada] = useState(0);
  const [erro, setErro] = useState('');
  const [lendo, setLendo] = useState(false);
  const [salvando, setSalvando] = useState(false);
  const [arrastando, setArrastando] = useState(false);
  const inputPDF = useRef<HTMLInputElement>(null);
  const titulo = useRef<HTMLHeadingElement>(null);
  const leitura = useRef(0);
  const enviando = useRef(false);
  const naIA = app.rota === 'ia';
  useEffect(() => { if (!naIA) titulo.current?.focus(); }, [etapa, naIA]);
  useEffect(() => () => { leitura.current++; }, []);

  function campo<K extends keyof Dados>(chave: K, valor: Dados[K]) {
    setDados(antes => ({ ...antes, [chave]: valor })); setErro('');
  }
  async function selecionar(arquivos: FileList | null) {
    const versao = ++leitura.current;
    setArrastando(false); setErro(''); setLendo(false);
    if (!arquivos?.length) return;
    const pdf = arquivos[0];
    if (arquivos.length !== 1) { setErro('Escolha um currículo por vez.'); return; }
    if (!/\.pdf$/i.test(pdf.name)) { setErro('Escolha um arquivo com extensão .pdf.'); return; }
    if (pdf.size > TAMANHO_PDF) { setErro('O PDF deve ter até 20 MB.'); return; }
    setLendo(true);
    try {
      const [inicio, fim] = await Promise.all([lerTrecho(pdf.slice(0, 5)), lerTrecho(pdf.slice(-1024))]);
      if (leitura.current !== versao) return;
      if (inicio !== '%PDF-') setErro('Esse arquivo não tem o formato de um PDF.');
      else if (!fim.includes('%%EOF')) setErro('O PDF está incompleto. Exporte o currículo novamente e escolha o arquivo salvo.');
      else setArquivo(pdf);
    } catch (e) { if (leitura.current === versao) setErro((e as Error).message); }
    finally { if (leitura.current === versao) setLendo(false); }
  }
  function ir(destino: number) {
    for (let i = 0; i < destino; i++) {
      const problema = validar(i, dados, arquivo);
      if (problema) {
        setEtapa(i); setErro(problema.texto);
        requestAnimationFrame(() => document.getElementById(`pa-${problema.campo}`)?.focus());
        return false;
      }
    }
    setErro(''); setEtapa(destino); setLiberada(n => Math.max(n, destino)); return true;
  }
  async function continuar(ev: FormEvent) {
    ev.preventDefault();
    if (enviando.current || lendo) return;
    if (etapa < 3) { ir(etapa + 1); return; }
    if (!ir(3) || !arquivo) return;
    const perfil = {
      cargo: dados.cargo.trim(), stackPrincipal: termos(dados.principal), stackSecundaria: termos(dados.secundaria),
      filtros: { niveis: dados.niveis, modelo: dados.modelos, pularSe: termos(dados.pular) }, foco: dados.foco, ingles: dados.ingles,
      ...(dados.foco !== 'nacional' && dados.usd ? { pretensaoUSD: Number(dados.usd) } : {}),
    };
    const corpo = new FormData();
    corpo.set('perfil', JSON.stringify(perfil)); corpo.set('curriculo', arquivo);
    enviando.current = true; setSalvando(true);
    try {
      await app.api('/api/v1/onboarding', 'POST', corpo, { 'X-Applyer-Onboarding': '1' });
      app.limpar(); concluir(iaPronta);
    } catch (e) {
      if (e instanceof ErroApi && e.status === 409) {
        app.aviso('O perfil já foi salvo em outra janela.'); await app.recarregar();
      } else setErro((e as Error).message || 'Não foi possível salvar. Tente novamente.');
    } finally { enviando.current = false; setSalvando(false); }
  }
  const resumo = [
    ['Currículo', arquivo?.name], ['Cargo ou área', dados.cargo], ['Níveis', dados.niveis.join(', ')],
    ['Stack principal', termos(dados.principal).join(', ')], ['Stack secundária', termos(dados.secundaria).join(', ') || 'Não informada'],
    ['Pular quando', termos(dados.pular).join(', ') || 'Sem exclusões adicionais'],
    ['Mercado', dados.foco ? NOMES_FOCO[dados.foco] : ''], ['Modelos de trabalho', dados.modelos.join(', ')], ['Inglês', dados.ingles],
    ...(dados.foco !== 'nacional' ? [['Pretensão internacional', dados.usd ? `US$ ${Number(dados.usd).toLocaleString('pt-BR')} por mês` : 'A combinar']] : []),
  ];
  return <div className="primeiro-acesso">
    <a className="pular-link" href="#pa-principal" onClick={e => { e.preventDefault(); document.getElementById('pa-principal')?.focus(); }}>Ir para o conteúdo</a>
    <header className="pa-topo"><div className="pa-barra">
      <span className="pa-marca" role="img" aria-label="Applyer"><img className="pa-clara" src={marca} alt="" /><img className="pa-escura" src={marcaEscura} alt="" /></span>
      <span className="pa-titulo">Primeiro acesso</span><span className="pa-local">Dados locais</span>
      <button type="button" className="botao-icone" onClick={alternarTema} aria-label="Alternar tema claro/escuro"><Icone d="M12 3a9 9 0 1 0 9 9 7 7 0 0 1-9-9z" /></button>
    </div></header>
    <div className="pa-janela">
      <aside className="pa-rail"><div><h2>Perfil de busca</h2><nav aria-label="Etapas da configuração">{PASSOS.map(([nome, sub], i) =>
        <button type="button" key={nome} disabled={i > liberada || salvando || lendo} aria-current={i === etapa && !naIA ? 'step' : undefined}
          onClick={() => { if (naIA) app.navegar('hoje'); ir(i); }}>
          <span className="pa-numero" aria-hidden="true">{i < etapa ? '✓' : i + 1}</span><span><b>{nome}</b><small>{sub}</small></span>
        </button>)}</nav></div><img src={paisagem} alt="" className="pa-arte" /></aside>
      <main id="pa-principal" tabIndex={-1}>
        {naIA ? <><button className="pa-link" type="button" onClick={() => app.navegar('hoje')}>← Voltar ao primeiro acesso</button><TelaConectarIA /></> :
          <form onSubmit={continuar}>
            <div className="pa-andamento"><span>Etapa {etapa + 1} de 4</span><div aria-hidden="true">{PASSOS.map(([nome], i) => <i key={nome} className={i <= etapa ? 'ativa' : ''} />)}</div></div>
            {etapa === 0 && <section>
              <h1 ref={titulo} tabIndex={-1}>Comece pelo <em>seu currículo.</em></h1>
              <p className="pa-sub">Anexe o PDF que você usa para se candidatar. Em seguida, escolha que tipo de vaga quer encontrar.</p>
              <div className={`pa-drop${arrastando || arquivo ? ' destacado' : ''}`} onDragOver={e => { e.preventDefault(); setArrastando(true); }}
                onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setArrastando(false); }}
                onDrop={e => { e.preventDefault(); void selecionar(e.dataTransfer.files); }}>
                <div className="pa-papel" aria-hidden="true"><i /><b>PDF</b></div>
                <div aria-live="polite"><h2>{arquivo?.name ?? 'Arraste seu currículo para cá'}</h2><p>{lendo ? 'Conferindo o arquivo…' : arquivo ? `PDF selecionado · ${Math.max(1, Math.ceil(arquivo.size / 1024))} KB` : 'PDF de até 20 MB'}</p></div>
                <div className="pa-arquivo-acoes"><button id="pa-arquivo" type="button" className="pa-secundario" onClick={() => inputPDF.current?.click()}>{arquivo ? 'Trocar PDF' : 'Escolher PDF'}</button>
                  {arquivo && <button type="button" className="pa-link" onClick={() => { leitura.current++; setArquivo(null); setLendo(false); setErro(''); setLiberada(0); }}>Remover</button>}</div>
                <input type="file" ref={inputPDF} aria-label="Currículo em PDF" accept=".pdf,application/pdf" hidden onChange={e => { void selecionar(e.target.files); e.target.value = ''; }} />
              </div>
              <p className="pa-nota">Ao concluir, o Applyer salva uma cópia do PDF na sua pasta local de currículos.</p>
            </section>}
            {etapa === 1 && <section>
              <h1 ref={titulo} tabIndex={-1}>Quais vagas <em>interessam?</em></h1><p className="pa-sub">Escolha o cargo, suas habilidades e os critérios que devem deixar uma vaga de fora.</p>
              <div className="pa-campos">
                <label>Cargo ou área de interesse<input id="pa-cargo" maxLength={120} value={dados.cargo} placeholder="Ex.: desenvolvimento de software" onChange={e => campo('cargo', e.target.value)} /></label>
                <Opcoes nome="niveis" titulo="Níveis aceitos" itens={NIVEIS} ativos={dados.niveis} mudar={v => campo('niveis', v)} />
                <div className="pa-dupla">
                  <label>Stack principal<small>Ferramentas ou habilidades centrais</small><input id="pa-principal-stack" maxLength={350} value={dados.principal} placeholder="Ex.: Java, Spring Boot" onChange={e => campo('principal', e.target.value)} /></label>
                  <label>Stack secundária<small>Complementares · opcional</small><input maxLength={350} value={dados.secundaria} placeholder="Ex.: React, SQL" onChange={e => campo('secundaria', e.target.value)} /></label>
                </div>
                <label>Pular quando<small>Separe por vírgula os termos que excluem uma vaga.</small><input maxLength={350} value={dados.pular} placeholder="Ex.: plantão obrigatório" onChange={e => campo('pular', e.target.value)} /></label>
              </div></section>}
            {etapa === 2 && <section>
              <h1 ref={titulo} tabIndex={-1}>Em quais <em>condições?</em></h1><p className="pa-sub">Defina onde quer trabalhar e os requisitos que a triagem deve comparar.</p>
              <div className="pa-campos"><fieldset><legend>Mercado de vagas</legend><div className="pa-opcoes">{(Object.entries(NOMES_FOCO) as [keyof typeof NOMES_FOCO, string][]).map(([key, nome], i) =>
                <label key={key}><input id={i === 0 ? 'pa-mercado' : undefined} type="radio" name="mercado" checked={dados.foco === key} onChange={() => campo('foco', key)} /><span>{nome}</span></label>)}</div></fieldset>
                <Opcoes nome="modelos" titulo="Modelos de trabalho aceitos" itens={MODELOS} ativos={dados.modelos} mudar={v => campo('modelos', v)} />
                <div className="pa-dupla"><label>Seu nível de inglês<select id="pa-ingles" value={dados.ingles} onChange={e => campo('ingles', e.target.value)}>
                  <option value="">Selecione um nível</option>{[...new Set([...INGLES, dados.ingles])].filter(Boolean).map(v => <option key={v}>{v}</option>)}</select></label>
                  {dados.foco !== 'nacional' && <label>Pretensão internacional<small>US$ por mês · opcional</small><input id="pa-usd" type="number" min=".01" step=".01" inputMode="decimal" value={dados.usd} placeholder="Ex.: 3500" onChange={e => campo('usd', e.target.value)} /></label>}
                </div>
              </div></section>}
            {etapa === 3 && <section>
              <h1 ref={titulo} tabIndex={-1}>Revise seu <em>perfil de busca.</em></h1>
              <p className="pa-sub">Confira o currículo e os critérios que o Applyer vai usar na sua primeira busca.</p>
              <dl className="pa-resumo">{resumo.map(([nome, valor]) => <div key={nome}><dt>{nome}</dt><dd>{valor}</dd></div>)}</dl>
              <p className="pa-nota">{iaPronta ? 'A busca começa depois que seu perfil for salvo.' : 'Salve seu perfil e conecte uma IA para iniciar a busca.'}</p>
              {!iaPronta && <button type="button" className="pa-link" disabled={salvando} onClick={() => app.navegar('ia')}>Conectar IA agora</button>}
            </section>}
            {erro && <p className="pa-erro" role="alert">{erro}</p>}
            <footer className="pa-acoes">{etapa > 0 && <button type="button" className="pa-link" disabled={salvando} onClick={() => ir(etapa - 1)}>← Voltar</button>}
              <button type="submit" className="pa-primario" disabled={lendo || salvando}>
                {salvando ? 'Salvando…' : etapa < 3 ? 'Continuar' : iaPronta ? 'Salvar e buscar vagas' : 'Salvar e conectar IA'} <span aria-hidden="true">→</span>
              </button></footer>
          </form>}
      </main>
    </div>
  </div>;
}
