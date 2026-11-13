import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ancoraDoCartaoMs, rolarAteOCartao } from './Fila';

// ResizeObserver de mentira: o teste avisa quando a página "cresceu" (os painéis de cima chegaram).
let avisar: () => void = () => {};
let desligado = false;
class ObservadorFalso {
  constructor(cb: () => void) { avisar = cb; }
  observe() {}
  disconnect() { desligado = true; }
}

let rolagens = 0;
beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal('ResizeObserver', ObservadorFalso);
  vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => { f(0); return 0; });
  desligado = false; rolagens = 0;
  const cartao = document.createElement('article');
  cartao.dataset.testid = 'cand-1';
  cartao.scrollIntoView = () => { rolagens++; };
  document.body.append(cartao);
});
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); document.body.innerHTML = ''; });

it('rola até o cartão e volta a rolar quando os painéis de cima chegam', () => {
  rolarAteOCartao('cand-1');
  expect(rolagens).toBe(1);
  avisar(); avisar();
  expect(rolagens).toBe(3);
});

it('para de segurar o cartão no primeiro toque da pessoa', () => {
  rolarAteOCartao('cand-1');
  window.dispatchEvent(new Event('wheel'));
  expect(desligado).toBe(true);
});

it('para de segurar o cartão depois do prazo', () => {
  rolarAteOCartao('cand-1');
  vi.advanceTimersByTime(ancoraDoCartaoMs - 1);
  expect(desligado).toBe(false);
  vi.advanceTimersByTime(1);
  expect(desligado).toBe(true);
});
