import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { Icone } from '../componentes';
import { NOMES_FOCO } from '../dominio/modelo';
import { ConexaoOnboarding } from './ConexaoOnboarding';
import paisagem from '../arte/primeiro-acesso/paisagem.svg';
import marca from '../arte/primeiro-acesso/applyer.svg';
import marcaEscura from '../arte/primeiro-acesso/applyer-inversa.svg';
import '../estilo/primeiro-acesso.css';

const PASSOS = [
  ['Sua IA', 'Conexão com o provedor'],
  ['Currículo', 'Seu documento de referência'],
  ['Vagas e stack', 'Cargo, nível e habilidades'],
  ['Condições', 'Mercado, idioma e valor'],
  ['Triagem', 'Revisão e primeira busca'],
];
const NIVEIS = ['Estágio', 'Júnior', 'Pleno', 'Sênior', 'Liderança'];
const MODELOS = ['Remoto', 'Híbrido', 'Presencial'];
const INGLES = ['Não falo inglês', 'Básico', 'Intermediário', 'Avançado', 'Fluente'];
const TAMANHO_PDF = 20 * 1024 * 1024;
const MOEDAS = { USD: 'Dólar americano', BRL: 'Real brasileiro', EUR: 'Euro' };
const NOMES_SUGESTAO = { cargo: 'Cargo', niveis: 'Nível', stackPrincipal: 'Stack principal', stackSecundaria: 'Stack secundária', ingles: 'Inglês' };
type Sugestao = { campo: keyof typeof NOMES_SUGESTAO; valor: string; evidencia: string };
type Curriculo = { arquivo: File; base64: string };
const lista = (v: unknown): string[] => Array.isArray(v) ? v.filter((item): item is string => typeof item === 'string' && item.trim() !== '') : [];
const termos = (texto: string) => [...new Set(texto.split(/[,;\n]/).map(t => t.trim()).filter(Boolean))];

function perfilInicial(perfil: any = {}) {
  const faixa = perfil.faixaSalarial ?? { min: perfil.pretensaoUSD, moeda: 'USD' };
  return {
    cargo: perfil.cargo ?? '', niveis: lista(perfil.filtros?.niveis), modelos: lista(perfil.filtros?.modelo),
    foco: (Object.prototype.hasOwnProperty.call(NOMES_FOCO, perfil.foco) ? perfil.foco : '') as keyof typeof NOMES_FOCO | '',
    principal: lista(perfil.stackPrincipal).join(', '), secundaria: lista(perfil.stackSecundaria).join(', '),
    pular: lista(perfil.filtros?.pularSe).join(', '), ingles: perfil.ingles ?? '',
    moeda: (Object.prototype.hasOwnProperty.call(MOEDAS, faixa.moeda) ? faixa.moeda : 'USD') as keyof typeof MOEDAS,
    minimo: faixa.min == null ? '' : String(faixa.min), maximo: faixa.max == null ? '' : String(faixa.max),
  };
}

type Dados = ReturnType<typeof perfilInicial>;
type Problema = { etapa: number; campo: string; texto: string };

function validar(etapa: number, dados: Dados, curriculo: Curriculo | null): Problema | null {
  const falha = (campo: string, texto: string) => ({ etapa, campo, texto });
  if (etapa === 1 && !curriculo) return falha('arquivo', 'Escolha seu currículo em PDF para continuar.');
  if (etapa === 2) {
    if (!dados.cargo.trim()) return falha('cargo', 'Informe um cargo ou área de interesse.');
    if (!dados.niveis.length) return falha('niveis', 'Selecione pelo menos um nível de experiência.');
    if (!termos(dados.principal).length) return falha('principal-stack', 'Informe pelo menos uma habilidade ou ferramenta principal.');
  }
  if (etapa === 3) {
    if (!dados.foco) return falha('mercado', 'Escolha o mercado em que deseja buscar vagas.');
    if (!dados.modelos.length) return falha('modelos', 'Selecione pelo menos um modelo de trabalho.');
    if (!dados.ingles) return falha('ingles', 'Informe seu nível de inglês.');
    for (const campo of ['minimo', 'maximo'] as const) {
      if (dados[campo] && (!Number.isFinite(Number(dados[campo])) || Number(dados[campo]) <= 0)) return falha(campo, 'Use um valor positivo ou deixe o limite em branco.');
    }
    if (dados.minimo && dados.maximo && Number(dados.maximo) < Number(dados.minimo)) return falha('maximo', 'O máximo precisa ser maior ou igual ao mínimo.');
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

function lerArquivo(blob: Blob, base64 = false): Promise<string> {
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
  const [curriculo, setCurriculo] = useState<Curriculo | null>(null);
  const arquivo = curriculo?.arquivo;
  const [sugestoes, setSugestoes] = useState<Sugestao[] | null>(null);
  const [etapa, setEtapa] = useState(0);
  const [liberada, setLiberada] = useState(0);
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
  useEffect(() => { titulo.current?.focus(); }, [etapa]);
  useEffect(() => () => { leitura.current++; }, []);

  function campo<K extends keyof Dados>(chave: K, valor: Dados[K]) {
    editados.current.add(chave);
    setDados(antes => ({ ...antes, [chave]: valor })); setErro('');
  }
  async function selecionar(arquivos: FileList | null) {
    if (extraindo || salvando) return;
    const versao = ++leitura.current;
    setArrastando(false); setErro(''); setLendo(false);
    if (!arquivos?.length) return;
    const pdf = arquivos[0];
    if (arquivos.length !== 1) { setErro('Escolha um currículo por vez.'); return; }
    if (!/\.pdf$/i.test(pdf.name)) { setErro('Escolha um arquivo com extensão .pdf.'); return; }
    if (pdf.size > TAMANHO_PDF) { setErro('O PDF deve ter até 20 MB.'); return; }
    setLendo(true);
    try {
      const [inicio, fim] = await Promise.all([lerArquivo(pdf.slice(0, 5)), lerArquivo(pdf.slice(-1024))]);
      if (leitura.current !== versao) return;
      if (inicio !== '%PDF-') setErro('Esse arquivo não tem o formato de um PDF.');
      else if (!fim.includes('%%EOF')) setErro('O PDF está incompleto. Exporte o currículo novamente e escolha o arquivo salvo.');
      else {
        const base64 = await lerArquivo(pdf, true);
        if (leitura.current !== versao) return;
        trocarCurriculo({ arquivo: pdf, base64 });
      }
    } catch (e) { if (leitura.current === versao) setErro((e as Error).message); }
    finally { if (leitura.current === versao) setLendo(false); }
  }
  function trocarCurriculo(novo: Curriculo | null) {
    setCurriculo(novo); setSugestoes(null); setLiberada(1);
    setDados(antes => {
      const limpo = { ...iniciais.current };
      for (const chave of editados.current) Object.assign(limpo, { [chave]: antes[chave] });
      return limpo;
    });
  }
  function ir(destino: number) {
    for (let i = 0; i < destino; i++) {
      const problema = validar(i, dados, curriculo);
      if (problema) {
        setEtapa(i); setErro(problema.texto);
        requestAnimationFrame(() => document.getElementById(`pa-${problema.campo}`)?.focus());
        return false;
      }
    }
    setErro(''); setEtapa(destino); setLiberada(n => Math.max(n, destino)); return true;
  }
  async function preencherComIA() {
    if (!curriculo) { ir(2); return; }
    if (sugestoes !== null) { ir(2); return; }
    const versao = leitura.current;
    enviando.current = true; setExtraindo(true); setErro('');
    try {
      const resposta = await app.api('/api/v1/onboarding/sugerir', 'POST', { curriculoBase64: curriculo.base64 }, { 'X-Applyer-Onboarding': '1' });
      if (leitura.current !== versao) return;
      const itens: Sugestao[] = resposta.sugestoes ?? [];
      setSugestoes(itens);
      setDados(antes => aplicarSugestoes(antes, itens, editados.current));
      ir(2);
    } catch (e) { if (leitura.current === versao) setErro((e as Error).message || 'A IA não conseguiu ler este PDF. Você pode tentar novamente ou preencher o perfil manualmente.'); }
    finally { enviando.current = false; setExtraindo(false); }
  }
  async function continuar(ev: FormEvent) {
    ev.preventDefault();
    if (enviando.current || lendo) return;
    if (etapa === 1) { await preencherComIA(); return; }
    if (etapa < 4) { ir(etapa + 1); return; }
    if (!ir(4) || !curriculo) return;
    const perfil = {
      cargo: dados.cargo.trim(), stackPrincipal: termos(dados.principal), stackSecundaria: termos(dados.secundaria),
      filtros: { niveis: dados.niveis, modelo: dados.modelos, pularSe: termos(dados.pular) }, foco: dados.foco, ingles: dados.ingles,
      ...(dados.minimo || dados.maximo ? { faixaSalarial: {
        moeda: dados.moeda, ...(dados.minimo ? { min: Number(dados.minimo) } : {}), ...(dados.maximo ? { max: Number(dados.maximo) } : {}),
      } } : {}),
    };
    enviando.current = true; setSalvando(true);
    try {
      await app.api('/api/v1/onboarding', 'POST', { perfil, curriculoBase64: curriculo.base64 }, { 'X-Applyer-Onboarding': '1' });
      app.limpar(); concluir(true);
    } catch (e) {
      if (e instanceof ErroApi && e.status === 409) {
        app.aviso('O perfil já foi salvo em outra janela.'); await app.recarregar();
      } else setErro((e as Error).message || 'Não foi possível salvar. Tente novamente.');
    } finally { enviando.current = false; setSalvando(false); }
  }
  const dinheiro = (valor: string) => Number(valor).toLocaleString('pt-BR', { style: 'currency', currency: dados.moeda, maximumFractionDigits: 2, minimumFractionDigits: 0 });
  const salario = dados.minimo && dados.maximo ? `${dinheiro(dados.minimo)} a ${dinheiro(dados.maximo)}`
    : dados.minimo ? `A partir de ${dinheiro(dados.minimo)}` : dados.maximo ? `Até ${dinheiro(dados.maximo)}` : '';
  const resumo = [
    ['Currículo', arquivo?.name], ['Cargo ou área', dados.cargo], ['Níveis', dados.niveis.join(', ')],
    ['Stack principal', termos(dados.principal).join(', ')], ['Stack secundária', termos(dados.secundaria).join(', ') || 'Não informada'],
    ['Pular quando', termos(dados.pular).join(', ') || 'Sem exclusões adicionais'],
    ['Mercado', dados.foco ? NOMES_FOCO[dados.foco] : ''], ['Modelos de trabalho', dados.modelos.join(', ')], ['Inglês', dados.ingles],
    ['Faixa salarial', salario ? `${salario} por mês (${dados.moeda})` : 'A combinar'],
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
        <button type="button" key={nome} disabled={i > liberada || (etapa === 0 && i > 0) || extraindo || salvando || lendo} aria-current={i === etapa ? 'step' : undefined}
          onClick={() => ir(i)}>
          <span className="pa-numero" aria-hidden="true">{i < etapa ? '✓' : i + 1}</span><span><b>{nome}</b><small>{sub}</small></span>
        </button>)}</nav></div><figure className="pa-ilhas"><img src={paisagem} alt="" className="pa-arte" />
          <figcaption>E se a vaga dos seus sonhos estiver <strong>a um clique?</strong></figcaption>
        </figure></aside>
      <main id="pa-principal" tabIndex={-1}>
        <div className="pa-andamento"><span>Etapa {etapa + 1} de {PASSOS.length}</span><div aria-hidden="true">{PASSOS.map(([nome], i) => <i key={nome} className={i <= etapa ? 'ativa' : ''} />)}</div></div>
        {etapa === 0 ? <section className="pa-etapa-ia">
          <h1 ref={titulo} tabIndex={-1}>Conecte <em>sua IA.</em></h1>
          <p className="pa-sub">Escolha quem vai ler seu currículo e comparar seu perfil com as vagas.</p>
          <ConexaoOnboarding aoConectar={() => ir(1)} />
        </section> :
          <form onSubmit={continuar}>
            {etapa === 1 && <section>
              <h1 ref={titulo} tabIndex={-1}>Anexe <em>seu currículo.</em></h1>
              <p className="pa-sub">Use o PDF com sua experiência mais recente. A IA pode sugerir cargo, nível, habilidades e inglês para você revisar.</p>
              <div className={`pa-drop${arrastando || arquivo ? ' destacado' : ''}`} onDragOver={e => { e.preventDefault(); setArrastando(true); }}
                onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setArrastando(false); }}
                onDrop={e => { e.preventDefault(); void selecionar(e.dataTransfer.files); }}>
                <div className="pa-papel" aria-hidden="true"><i /><b>PDF</b></div>
                <div aria-live="polite"><h2>{arquivo?.name ?? 'Arraste seu currículo para cá'}</h2><p>{lendo ? 'Conferindo o arquivo…' : arquivo ? `PDF selecionado · ${Math.max(1, Math.ceil(arquivo.size / 1024))} KB` : 'PDF de até 20 MB'}</p></div>
                <div className="pa-arquivo-acoes"><button id="pa-arquivo" type="button" className="pa-secundario" disabled={extraindo} onClick={() => inputPDF.current?.click()}>{arquivo ? 'Trocar PDF' : 'Escolher PDF'}</button>
                  {arquivo && <button type="button" className="pa-link" disabled={extraindo} onClick={() => { leitura.current++; trocarCurriculo(null); setLendo(false); setErro(''); }}>Remover</button>}</div>
                <input type="file" ref={inputPDF} aria-label="Currículo em PDF" accept=".pdf,application/pdf" hidden onChange={e => { void selecionar(e.target.files); e.target.value = ''; }} />
              </div>
              <p className="pa-nota">Ao escolher “Preencher com IA”, o Applyer compartilha o texto profissional do currículo com o provedor conectado. O PDF é salvo no seu computador ao concluir.</p>
              {extraindo && <p className="pa-leitura" role="status"><span aria-hidden="true" />Lendo seu currículo…</p>}
            </section>}
            {etapa === 2 && <section>
              <h1 ref={titulo} tabIndex={-1}>Quais vagas <em>interessam?</em></h1><p className="pa-sub">{sugestoes?.length ? 'Confira as sugestões do seu currículo, ajuste os níveis que você aceita e escolha o que a busca deve pular.' : sugestoes ? 'A IA não encontrou dados suficientes neste currículo. Preencha os campos para orientar sua busca.' : 'Escolha o cargo, suas habilidades e os critérios que devem deixar uma vaga de fora.'}</p>
              <Evidencias sugestoes={(sugestoes ?? []).filter(s => s.campo !== 'ingles')} />
              <div className="pa-campos">
                <label>Cargo ou área de interesse<input id="pa-cargo" maxLength={120} value={dados.cargo} placeholder="Ex.: desenvolvimento de software" onChange={e => campo('cargo', e.target.value)} /></label>
                <Opcoes nome="niveis" titulo="Níveis aceitos" itens={NIVEIS} ativos={dados.niveis} mudar={v => campo('niveis', v)} />
                <div className="pa-dupla">
                  <label>Stack principal<small>Ferramentas ou habilidades centrais</small><input id="pa-principal-stack" maxLength={350} value={dados.principal} placeholder="Ex.: Java, Spring Boot" onChange={e => campo('principal', e.target.value)} /></label>
                  <label>Stack secundária<small>Complementares · opcional</small><input maxLength={350} value={dados.secundaria} placeholder="Ex.: React, SQL" onChange={e => campo('secundaria', e.target.value)} /></label>
                </div>
                <label>Pular quando<small>Separe por vírgula os termos que excluem uma vaga.</small><input maxLength={350} value={dados.pular} placeholder="Ex.: plantão obrigatório" onChange={e => campo('pular', e.target.value)} /></label>
              </div></section>}
            {etapa === 3 && <section>
              <h1 ref={titulo} tabIndex={-1}>Em quais <em>condições?</em></h1><p className="pa-sub">Defina onde quer trabalhar e os requisitos que a triagem deve comparar.</p>
              <div className="pa-campos"><fieldset><legend>Mercado de vagas</legend><div className="pa-opcoes">{(Object.entries(NOMES_FOCO) as [keyof typeof NOMES_FOCO, string][]).map(([key, nome], i) =>
                <label key={key}><input id={i === 0 ? 'pa-mercado' : undefined} type="radio" name="mercado" checked={dados.foco === key} onChange={() => campo('foco', key)} /><span>{nome}</span></label>)}</div></fieldset>
                <Opcoes nome="modelos" titulo="Modelos de trabalho aceitos" itens={MODELOS} ativos={dados.modelos} mudar={v => campo('modelos', v)} />
                <label>Seu nível de inglês<select id="pa-ingles" value={dados.ingles} onChange={e => campo('ingles', e.target.value)}>
                  <option value="">Selecione um nível</option>{[...new Set([...INGLES, dados.ingles])].filter(Boolean).map(v => <option key={v}>{v}</option>)}</select></label>
                <Evidencias sugestoes={(sugestoes ?? []).filter(s => s.campo === 'ingles')} />
                <fieldset className="pa-salario"><legend>Faixa salarial mensal <small>Opcional</small></legend>
                  <div className="pa-limites">
                    <label>Moeda<select value={dados.moeda} onChange={e => campo('moeda', e.target.value as Dados['moeda'])}>{Object.entries(MOEDAS).map(([sigla, nome]) => <option key={sigla} value={sigla}>{sigla} · {nome}</option>)}</select></label>
                    <label>Mínimo<input id="pa-minimo" type="number" min=".01" step=".01" inputMode="decimal" placeholder="4.000" value={dados.minimo} onChange={e => campo('minimo', e.target.value)} /></label>
                    <label>Máximo<input id="pa-maximo" type="number" min=".01" step=".01" inputMode="decimal" placeholder="6.000" value={dados.maximo} onChange={e => campo('maximo', e.target.value)} /></label>
                  </div>
                </fieldset>
              </div></section>}
            {etapa === 4 && <section>
              <h1 ref={titulo} tabIndex={-1}>Revise seu <em>perfil de busca.</em></h1>
              <p className="pa-sub">Confira o currículo e os critérios que o Applyer vai usar na sua primeira busca.</p>
              <dl className="pa-resumo">{resumo.map(([nome, valor]) => <div key={nome}><dt>{nome}</dt><dd>{valor}</dd></div>)}</dl>
              <p className="pa-nota">Ao salvar, sua IA avalia as vagas e ordena os resultados em A, B e C.</p>
            </section>}
            {erro && <p className="pa-erro" role="alert">{erro}</p>}
            <footer className="pa-acoes"><button type="button" className="pa-link" disabled={salvando || extraindo} onClick={() => ir(etapa - 1)}>← Voltar</button>
              {etapa === 1 && sugestoes === null && arquivo && <button type="button" className="pa-link pa-manual" disabled={lendo || extraindo} onClick={() => ir(2)}>Preencher manualmente</button>}
              <button type="submit" className="pa-primario" disabled={lendo || extraindo || salvando}>
                {salvando ? 'Salvando…' : extraindo ? 'Lendo currículo…' : etapa === 1 && sugestoes === null ? 'Preencher com IA' : etapa === 4 ? 'Salvar e buscar vagas' : 'Continuar'} <span aria-hidden="true">→</span>
              </button></footer>
          </form>}
      </main>
    </div>
  </div>;
}
