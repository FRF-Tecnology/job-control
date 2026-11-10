// "Janela do app", no Perfil (api/openapi.yaml 0.15.0, rotas /app/janela). O app abre num tamanho padrão e lembra o
// que a pessoa deixar; isso é do app e não passa por aqui. Esta seção tem as duas escolhas da pessoa:
// - "Continuar aberto em segundo plano": PUT {segundoPlano}; ligado, fechar a janela só a esconde e o app ganha um
//   ícone perto do relógio (Windows) ou na barra de menus (Mac). 409 {motivo: sem-bandeja}: o sistema não deixou
//   criar o ícone e nada mudou.
// - "Voltar ao tamanho padrão": POST /app/janela/padrao {}.
// Só o app de PC tem janela: com disponivel false (central-dev), rota ausente ou erro, a seção não aparece.
// Frases fixas: nenhum texto do servidor vai para a tela.
import { useEffect, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';

const ROTA = '/api/v1/app/janela';
const noMac = () => typeof navigator !== 'undefined' && /Mac/i.test(navigator.userAgent);

export function JanelaApp() {
  const app = useApp();
  const [segundoPlano, setSegundoPlano] = useState<boolean | null>(null); // null: sem janela para ajustar
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState('');
  const [feito, setFeito] = useState('');
  const lugar = noMac() ? 'na barra de menus' : 'perto do relógio';

  useEffect(() => {
    app.api(ROTA).then(r => setSegundoPlano(r?.disponivel === true ? r.segundoPlano === true : null)).catch(() => setSegundoPlano(null));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (segundoPlano === null) return null;

  const explicar = (e: unknown) => {
    if (e instanceof ErroApi && e.status === 409) return `O sistema não deixou pôr o ícone do Applyer ${lugar}. O app continua fechando junto com a janela.`;
    if (e instanceof ErroApi && e.status === 403) return 'Só você pode mudar isso, por esta tela.';
    return 'Não deu para mudar. Tente de novo.';
  };

  async function mudar(ligar: boolean) {
    if (ocupado) return;
    setOcupado(true); setErro(''); setFeito('');
    try {
      const r = await app.api(ROTA, 'PUT', { segundoPlano: ligar });
      const ficou = r?.segundoPlano === true;
      setSegundoPlano(ficou);
      setFeito(ficou ? 'Pronto. O app continua aberto quando você fechar a janela.' : 'Pronto. Fechar a janela volta a fechar o app.');
    } catch (e) { setErro(explicar(e)); }
    finally { setOcupado(false); }
  }

  async function padrao() {
    if (ocupado) return;
    setOcupado(true); setErro(''); setFeito('');
    try {
      await app.api(`${ROTA}/padrao`, 'POST', {});
      setFeito('Pronto. A janela voltou ao tamanho padrão.');
    } catch (e) { setErro(explicar(e)); }
    finally { setOcupado(false); }
  }

  return (
    <section className="janela-app" aria-labelledby="janela-app-titulo">
      <h2 id="janela-app-titulo">Janela do app</h2>
      <p className="mudo">O app abre do tamanho que você deixou da última vez.</p>
      {feito && <p className="feito" role="status">{feito}</p>}
      {erro && <p className="erro-resposta" role="alert">{erro}</p>}

      <div className="zona">
        <h3>Segundo plano</h3>
        <label className="marcar">
          <input type="checkbox" checked={segundoPlano} disabled={ocupado} onChange={e => mudar(e.target.checked)} aria-describedby="janela-app-dica" />
          <span>Continuar aberto em segundo plano</span>
        </label>
        <p className="mudo" id="janela-app-dica">Com isto ligado, fechar a janela não fecha o app: ele fica aberto, com o ícone do Applyer {lugar}. Clique no ícone para abrir a janela de novo. Para fechar de verdade, use Sair no menu do ícone. Com a janela fechada, o app faz só o que já fazia com ela aberta.</p>
      </div>

      <div className="zona">
        <h3>Tamanho da janela</h3>
        <p className="mudo">Arraste a borda da janela para mudar o tamanho. O app lembra.</p>
        <div><button className="botao-fantasma" type="button" disabled={ocupado} onClick={padrao}>Voltar ao tamanho padrão</button></div>
      </div>
    </section>
  );
}
