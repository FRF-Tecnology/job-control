import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { impressaoCandidatura } from '../app/impressao';
import { TelaListaDoDia, TelaQuadroCandidaturas } from './Jornada';

afterEach(cleanup);
const vaga = {id:'g-1', empresa:'Empresa de exemplo', titulo:'Analista', etapa:'aplicar', perfilBuscaId:'dados'};
function ambiente(vagas: unknown[], candidaturas: unknown[] = []) {
  const api = vi.fn(async (url: string, metodo = 'GET', _corpo?: unknown) => {
    if (metodo !== 'GET') return {};
    return url.endsWith('/limites') ? {gupy:{limite:10,usadas:0}} : candidaturas;
  });
  const app = {estado:{vagas,eventos:[],perfil:{cargo:'Engenharia'},perfilBuscaAtivo:'original',
    perfisBusca:[{id:'dados',nome:'Dados'},{id:'original',nome:'Engenharia'}]}, api,
    aviso:vi.fn(), recarregar:vi.fn(async () => {}), limpar:vi.fn(), perguntar:vi.fn(async () => 'ok'), navegar:vi.fn()
  } as unknown as App;
  return {app,api};
}
async function pacote() {
  const c = {id:'cand-1',vagaId:'g-1',plataforma:'gupy',url:'https://vaga.example/v',estado:'preparada',versao:3,
    criadaEm:'2026-11-10',atualizadaEm:'2026-11-10', conteudo:{curriculo:{arquivo:'dados.pdf'},idioma:'pt',contato:null,
      pretensao:null,respostas:[{pergunta:'Principal ferramenta?',resposta:'SQL',origem:'pessoa',perguntaId:'q1'}],mensagem:null}};
  return {...c,impressao:await impressaoCandidatura(c)};
}
it('mantém o perfil dono, revisa o pacote real e preserva as respostas ao editar a carta', async () => {
  const c = await pacote();
  const {app,api} = ambiente([vaga],[c]);
  render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  fireEvent.click(await screen.findByRole('button',{name:/Revisar pacote de Analista/}));
  const dialogo = screen.getByRole('dialog',{name:'Revisar pacote'});
  expect(within(dialogo).getByText('dados.pdf')).toBeTruthy();
  expect(within(dialogo).getByText('Empresa de exemplo · Dados')).toBeTruthy();
  fireEvent.change(within(dialogo).getByRole('textbox',{name:/Carta de apresentação/}),{target:{value:'Carta revisada'}});
  fireEvent.click(within(dialogo).getByRole('button',{name:'Salvar pacote'}));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/candidaturas/cand-1/conteudo','PUT',{
    conteudo:{...c.conteudo,mensagem:'Carta revisada'},versao:3}));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(api.mock.calls.filter(call => call[1] === 'POST')).toHaveLength(0);
});
it('aprovar exige confirmação e encaminha a impressão do pacote; devolver usa a operação atômica', async () => {
  const c = await pacote();
  const {app,api} = ambiente([vaga],[c]);
  render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  const botao = await screen.findByRole('button',{name:'Aprovar pacote'});
  await waitFor(() => expect((botao as HTMLButtonElement).disabled).toBe(false));
  fireEvent.click(botao);
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/candidaturas/cand-1/aprovar','POST',{impressao:c.impressao}));
  expect(app.perguntar).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole('button',{name:/Revisar pacote de Analista/}));
  fireEvent.click(screen.getByRole('button',{name:'Devolver ao radar'}));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/vagas/g-1/devolver-ao-radar','POST',{}));
});
it('mantém quatro colunas com candidaturas existentes de qualquer perfil e edita a etapa e a anotação juntas', async () => {
  const {app,api} = ambiente([{...vaga,etapa:'em-processo',subEtapa:'proposta',notas:'Avaliar contrato'},
    {...vaga,id:'outra',titulo:'Outra vaga',etapa:'encerrada',motivo:'Reprovada'}]);
  render(<Contexto.Provider value={app}><TelaQuadroCandidaturas /></Contexto.Provider>);
  for (const coluna of ['Enviada','Entrevista','Proposta','Conquistada']) expect(screen.getByRole('region',{name:coluna})).toBeTruthy();
  expect(screen.queryByText('Outra vaga')).toBeNull();
  fireEvent.click(within(screen.getByRole('region',{name:'Proposta'})).getByRole('button',{name:/Abrir candidatura de Analista/}));
  fireEvent.change(screen.getByLabelText('Etapa atual'),{target:{value:'Conquistada'}});
  fireEvent.change(screen.getByLabelText('Anotação'),{target:{value:'Assinei o contrato'}});
  fireEvent.click(screen.getByRole('button',{name:'Salvar alterações'}));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/vagas/g-1/acompanhamento','POST',{
    etapaAntes:'Proposta',notasAntes:'Avaliar contrato',etapa:'Conquistada',notas:'Assinei o contrato'}));
});
it('oferece um caminho até o Radar e a lista nos estados vazios', async () => {
  const {app} = ambiente([]);
  const comp = render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  expect(await screen.findByRole('heading',{name:'Sua lista está vazia'})).toBeTruthy();
  expect(screen.getByRole('link',{name:'Voltar ao radar'}).getAttribute('href')).toBe('#/radar');
  comp.rerender(<Contexto.Provider value={app}><TelaQuadroCandidaturas /></Contexto.Provider>);
  expect(screen.getByRole('link',{name:'Abrir lista do dia'}).getAttribute('href')).toBe('#/lista');
});

it('o resumo inicia o próximo pacote aprovado, abre o endereço da candidatura reservada e permite confirmar', async () => {
  const c = {...await pacote(),estado:'aprovada'};
  const {app,api} = ambiente([vaga],[c]);
  api.mockImplementation(async (url,metodo='GET') => {
    if (metodo === 'GET') return url.endsWith('/limites') ? {} : [c];
    if (url.endsWith('/iniciar-manual')) {
      const reservada = {...vaga,aplicacaoManual:true,candidaturaManualId:c.id,urlCandidaturaManual:c.url};
      app.estado.vagas = [reservada];
      c.estado = 'bloqueada';
      return reservada;
    }
    return {};
  });
  render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  fireEvent.click(await screen.findByRole('button',{name:'Aplicar na próxima vaga'}));
  const dialogo = await screen.findByRole('dialog',{name:'Candidatura no site da empresa'});
  expect(api).toHaveBeenCalledWith('/api/v1/vagas/g-1/iniciar-manual','POST',{});
  expect(within(dialogo).getByRole('link',{name:'Abrir candidatura no navegador'}).getAttribute('href')).toBe(c.url);
  expect(app.navegar).not.toHaveBeenCalled();
  fireEvent.click(within(dialogo).getByRole('button',{name:'Já enviei'}));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/vagas/g-1/confirmar-envio-manual','POST',{}));
});

it('ao retomar revalida a reserva no servidor e não oferece o endereço se ela mudou', async () => {
  const c = {...await pacote(),estado:'bloqueada'};
  const {app,api} = ambiente([{...vaga, aplicacaoManual:true, urlCandidaturaManual:c.url}], [c]);
  api.mockImplementation(async (url,metodo='GET') => {
    if (metodo === 'GET') return url.endsWith('/limites') ? {} : [c];
    throw new Error('Esta candidatura já saiu da lista.');
  });
  render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  const resumo = await screen.findByRole('complementary', {name:'Resumo do lote'});
  expect(screen.queryByRole('link', {name:/Abrir candidatura/})).toBeNull();
  fireEvent.click(within(resumo).getByRole('button',{name:'Retomar candidatura'}));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/vagas/g-1/iniciar-manual','POST',{}));
  await waitFor(() => expect(app.aviso).toHaveBeenCalledWith('Esta candidatura já saiu da lista.',true));
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.queryByRole('link', {name:/Abrir candidatura/})).toBeNull();
});

it('fecha o diálogo aberto quando uma recarga substitui a reserva pelo id de outro pacote', async () => {
  const c = {...await pacote(),estado:'bloqueada'};
  const reservada = {...vaga,aplicacaoManual:true,candidaturaManualId:c.id,urlCandidaturaManual:c.url};
  const {app,api} = ambiente([reservada],[c]);
  api.mockImplementation(async (url,metodo='GET') => metodo === 'GET' ? (url.endsWith('/limites') ? {} : [c]) : reservada);
  const tela = render(<Contexto.Provider value={app}><TelaListaDoDia /></Contexto.Provider>);
  const resumo = await screen.findByRole('complementary',{name:'Resumo do lote'});
  fireEvent.click(within(resumo).getByRole('button',{name:'Retomar candidatura'}));
  expect(await screen.findByRole('link',{name:'Abrir candidatura no navegador'})).toBeTruthy();
  const atualizado = {...app,estado:{...app.estado,vagas:[{...reservada,candidaturaManualId:'cand-2'}]}};
  tela.rerender(<Contexto.Provider value={atualizado}><TelaListaDoDia /></Contexto.Provider>);
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.queryByRole('link',{name:/Abrir candidatura/})).toBeNull();
});
