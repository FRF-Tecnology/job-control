// "Seus dados neste computador" (jc-a2y.49), no fim do Perfil: sair das plataformas (só o login do navegador do app)
// e apagar tudo o que o app guarda. Rotas só da tela (x-so-tela), pelo api/openapi.yaml 0.4.10 (ResultadoApagar,
// ItemApagado, EnvioEmAndamento). A frase é sempre digitada pela pessoa; a tela diz antes o que não sai.
// - apagar: POST {confirmacao: FRASE}; o servidor confere a frase exata. 200 {apagado, faltou, completo, fechar}; com
//   fechar:true o servidor responde e fecha o app (na próxima abertura é usuário novo).
// - sair: POST {}; 200 com fechar:false; os dados ficam.
// - 409 {erro, estado:"enviando"}: candidatura em envio ou navegador em uso. Falha parcial não é erro: completo:false.
import { useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';

const ROTAS = { apagar: '/api/v1/dados/apagar', sair: '/api/v1/navegador/sair' };
export const FRASE = 'APAGAR MEUS DADOS';

// Lista fechada de itens do contrato, na ordem em que a tela mostra.
export const ITENS: Record<string, string> = {
  banco: 'As vagas, a fila de candidaturas, o perfil, o histórico, os limites e as configurações',
  curriculos: 'Os currículos copiados para a pasta do app',
  backups: 'As cópias diárias (backups)',
  navegador: 'O login nas plataformas (navegador do app)',
  chaves: 'As chaves de IA guardadas no cofre do sistema',
};

interface Resultado { apagado?: string[]; faltou?: string[]; completo?: boolean; fechar?: boolean }
type Feito = { tipo: 'saiu' } | { tipo: 'apagou'; fechar: boolean; faltou: string[] } | null;

// A frase é sempre digitada pela pessoa: a tela nunca preenche o campo (pedido do CyberSec no #54).

export function SeusDados() {
  const app = useApp();
  const [digitado, setDigitado] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState('');
  const [feito, setFeito] = useState<Feito>(null);
  const confere = digitado.trim() === FRASE;

  // Frases fixas: nenhum texto do servidor ou do sistema aparece aqui (nota do CyberSec no #57).
  const explicar = (e: unknown, acao: 'apagar' | 'sair') => {
    if (e instanceof ErroApi && e.status === 409) return acao === 'apagar'
      // No apagar, o servidor para o envio antes de conferir e só o solta quando o app abre de novo.
      ? 'Uma candidatura estava sendo enviada, então nada foi apagado. O envio fica parado até você abrir o app de novo. Feche e abra o app e repita.'
      : 'O app está enviando uma candidatura agora. Espere terminar e tente de novo.';
    if (e instanceof ErroApi && e.status === 403) return 'Só você pode fazer isso, por esta tela.';
    return 'Não deu para concluir. Tente de novo.';
  };

  async function sair() {
    const ok = await app.perguntar({
      titulo: 'Sair das plataformas?',
      texto: 'O app esquece o login no LinkedIn, na Gupy e na InHire. Suas vagas, o perfil e os currículos ficam. Para enviar de novo, você entra outra vez.',
      confirmar: 'Sair das plataformas',
    });
    if (!ok) return;
    setOcupado(true); setErro('');
    try {
      const r: Resultado = (await app.api(ROTAS.sair, 'POST', {})) ?? {};
      if (r.faltou?.includes('navegador')) setErro('Não deu para sair das plataformas: o navegador do app está em uso. Feche o app, abra de novo e repita.');
      else setFeito({ tipo: 'saiu' });
    }
    catch (e) { setErro(explicar(e, 'sair')); }
    finally { setOcupado(false); }
  }

  async function apagar(ev: React.FormEvent) {
    ev.preventDefault();
    if (!confere || ocupado) return;
    setOcupado(true); setErro('');
    try {
      const r: Resultado = (await app.api(ROTAS.apagar, 'POST', { confirmacao: FRASE })) ?? {};
      const faltou = (r.faltou ?? []).filter(i => i in ITENS);
      setDigitado('');
      setFeito({ tipo: 'apagou', fechar: !!r.fechar, faltou });
      // Com fechar, o servidor fecha o app; não adianta reler. Sem fechar, relê para mostrar o app vazio.
      if (!r.fechar) await app.recarregar();
    } catch (e) { setErro(explicar(e, 'apagar')); }
    finally { setOcupado(false); }
  }

  return (
    <section className="seus-dados" aria-labelledby="seus-dados-titulo">
      <h2 id="seus-dados-titulo">Seus dados neste computador</h2>
      <p className="mudo">Tudo fica só aqui. Nada disto vai para a nuvem.</p>
      {feito?.tipo === 'saiu' && <p className="feito" role="status">Pronto. O app saiu das plataformas.</p>}
      {feito?.tipo === 'apagou' && (
        <div role="status" className="feito-bloco">
          <p className={feito.faltou.length ? 'feito parcial' : 'feito'}>{(feito.faltou.length ? 'Parte dos dados foi apagada.' : 'Dados apagados.') + (feito.fechar ? ' O app vai fechar.' : '')}</p>
          {feito.faltou.length > 0 && (<>
            <p className="erro-resposta">Não deu para apagar tudo. Ficou:</p>
            <ul>{feito.faltou.map(i => <li key={i}>{ITENS[i]}</li>)}</ul>
            <p className="mudo">Abra o app de novo e repita, ou apague esses itens à mão.</p>
          </>)}
        </div>
      )}
      {erro && <p className="erro-resposta" role="alert">{erro}</p>}

      <div className="zona">
        <h3>Sair das plataformas</h3>
        <p className="mudo">Apaga só o login que o app guarda no LinkedIn, na Gupy e na InHire. Seus dados ficam.</p>
        <div><button className="botao-fantasma" type="button" disabled={ocupado} onClick={sair}>Sair das plataformas</button></div>
      </div>

      <form className="zona perigo" onSubmit={apagar}>
        <h3>Apagar meus dados</h3>
        <p className="mudo">Apaga de vez, sem volta, e fecha o app. Na próxima vez, ele abre como novo:</p>
        <ul>{Object.values(ITENS).map(t => <li key={t}>{t}</li>)}</ul>
        <p className="mudo">Não apaga: o login das ferramentas de IA (Claude, Codex, Gemini), os PDFs originais que você copiou, os dados da Central antiga (pasta dados/) e o cache da janela do app.</p>
        <label className="campo"><span>{`Para confirmar, digite ${FRASE}`}</span>
          <input type="text" autoComplete="off" spellCheck={false} value={digitado} onChange={e => setDigitado(e.target.value)} aria-describedby="seus-dados-dica" />
        </label>
        <small id="seus-dados-dica" className="mudo">{confere ? 'Confirmado. O botão está liberado.' : `O botão só libera depois de digitar ${FRASE}, em maiúsculas.`}</small>
        <div><button className="botao-perigo" type="submit" disabled={!confere || ocupado}>{ocupado ? 'Apagando…' : 'Apagar meus dados'}</button></div>
      </form>
    </section>
  );
}
