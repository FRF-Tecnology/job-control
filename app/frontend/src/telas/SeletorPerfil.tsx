import { useEffect, useId, useRef, useState } from 'react';
import { useApp } from '../app/contexto';
import '../estilo/seletor-perfil.css';

const modelos = { ai: 'AI Engineer', java: 'Java Engineer', outro: '' };
type Modelo = keyof typeof modelos;

export function SeletorPerfil() {
  const { estado, api, recarregar, perguntar } = useApp();
  const ativo = estado.perfilBuscaAtivo ?? 'original';
  const perfis = estado.perfisBusca ?? [{ id: ativo, nome: estado.perfil?.cargo && estado.perfil.cargo !== estado.perfil.nome ? estado.perfil.cargo : 'Perfil principal' }];
  const [criando, abrir] = useState(false);
  const [modelo, definirModelo] = useState<Modelo>('ai');
  const [nome, definirNome] = useState(modelos.ai);
  const [ocupado, ocupar] = useState(false);
  const [erro, falhar] = useState('');
  const enviando = useRef(false);
  const dialogo = useRef<HTMLDialogElement>(null);
  const seletor = useRef<HTMLSelectElement>(null);
  const titulo = useId();
  useEffect(() => {
    const d = dialogo.current;
    if (!criando || !d || d.open) return;
    if (d.showModal) d.showModal(); else d.open = true;
  }, [criando]);

  async function confirmarTroca() {
    if (!document.querySelector('[data-perfil-alterado="true"]')) return true;
    return await perguntar({ titulo: 'Descartar alterações deste perfil?', texto: 'Você tem edições que ainda não foram salvas.', confirmar: 'Descartar alterações' }) === 'ok';
  }

  function cancelar() {
    if (enviando.current) return;
    dialogo.current?.close?.();
    abrir(false); falhar('');
  }
  async function salvar(caminho: string, dados: object) {
    if (enviando.current) return;
    enviando.current = true;
    ocupar(true); falhar('');
    try {
      await api(caminho, 'POST', { ...dados, perfilOrigem: ativo });
      await recarregar();
      dialogo.current?.close?.();
      abrir(false);
      seletor.current?.focus();
    } catch (e) { falhar(e instanceof Error ? e.message : 'Não foi possível salvar o perfil de busca.'); }
    finally { enviando.current = false; ocupar(false); }
  }
  return <div className="seletor-perfil">
    <label>Perfil de busca
      <select ref={seletor} value={ativo} disabled={ocupado} onChange={async e => {
        const destino = e.target.value;
        if (await confirmarTroca()) await salvar('/api/v1/perfis-busca/' + encodeURIComponent(destino) + '/ativar', {});
      }}>
        {perfis.map((p: { id: string; nome: string }) => <option key={p.id} value={p.id}>{p.nome}</option>)}
      </select>
    </label>
    <button type="button" className="criar-perfil" aria-haspopup="dialog" disabled={ocupado} onClick={async () => {
      if (!(await confirmarTroca())) return;
      definirModelo('ai'); definirNome(modelos.ai); falhar(''); abrir(true);
    }}><span aria-hidden="true">＋</span> Criar perfil</button>
    {erro && !criando && <p role="alert">{erro}</p>}
    {criando && <dialog ref={dialogo} className="dialogo-perfil" aria-labelledby={titulo} onCancel={e => { e.preventDefault(); cancelar(); }}>
      <form onSubmit={e => { e.preventDefault(); const valor = nome.trim();
        if (!valor) { falhar('Digite o nome do perfil.'); return; }
        void salvar('/api/v1/perfis-busca', { modelo, nome: valor });
      }}>
        <h2 id={titulo}>Criar perfil de busca</h2>
        <label>Cargo de interesse<select autoFocus value={modelo} disabled={ocupado} onChange={e => {
          const proximo = e.target.value as Modelo;
          if (!nome.trim() || nome === modelos[modelo]) definirNome(modelos[proximo]);
          definirModelo(proximo); falhar('');
        }}><option value="ai">AI Engineer</option><option value="java">Java Engineer</option><option value="outro">Outro cargo</option></select></label>
        <label>Nome do perfil<input required maxLength={120} value={nome} disabled={ocupado} onChange={e => { definirNome(e.target.value); falhar(''); }} /></label>
        <p>{modelo === 'outro' ? 'Use o cargo desejado como nome. Depois, ajuste os filtros.' : 'Você pode ajustar os filtros depois.'}</p>
        {erro && <p role="alert">{erro}</p>}
        <div><button className="botao-fantasma" type="button" disabled={ocupado} onClick={cancelar}>Cancelar</button>
          <button type="submit" className="botao" disabled={ocupado}>{ocupado ? 'Criando…' : 'Criar perfil'}</button></div>
      </form>
    </dialog>}
  </div>;
}
