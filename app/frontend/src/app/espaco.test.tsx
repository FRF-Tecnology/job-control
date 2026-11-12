import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within, fireEvent } from '@testing-library/react';
import App from '../App';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); location.hash = ''; });
it('mostra os quatro destinos do desenho e deixa a navegação avançada recuperável', async () => {
  vi.stubGlobal('matchMedia', () => ({matches:false}));
  vi.stubGlobal('scrollTo', () => {});
  vi.stubGlobal('fetch', async (url:string) => {
    const dados=url.endsWith('/estado') ? {perfil:{nome:'Pessoa de teste'},vagas:[],rodadas:[],hoje:'2026-11-12',versao:'1',onboardingConcluido:true} : {};
    return new Response(JSON.stringify(dados),{headers:{'content-type':'application/json'}});
  });
  render(<App />);
  await screen.findByText('Espaço pessoal');
  const menu=screen.getByRole('navigation',{name:'Navegação principal'});
  expect(within(menu).getAllByRole('link').map(a=>a.getAttribute('href'))).toEqual(['#/radar','#/lista','#/quadro']);
  expect(within(screen.getByRole('navigation',{name:'Preferências'})).getByRole('link',{name:'Perfil e IA'})).toBeTruthy();
  expect(within(menu).queryByText('Triagem')).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'Ferramentas'}));
  expect((await screen.findByRole('link',{name:'Triagem'})).getAttribute('href')).toBe('#/triagem');
});
