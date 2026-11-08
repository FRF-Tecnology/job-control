import { useEffect, useState } from 'react'

type Saude = { ok: boolean; app: string }

// Tela provisória da P0: confirma que a janela, as telas e a API local conversam.
// As telas de verdade (Hoje, Triagem, Quadro...) chegam na P3.
export default function App() {
  const [saude, setSaude] = useState<Saude | null>(null)
  const [erro, setErro] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/v1/saude')
      .then(r => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then(setSaude)
      .catch(e => setErro(String(e.message ?? e)))
  }, [])

  return (
    <main className="inicio">
      <img src="/icone.svg" alt="" width={56} height={56} />
      <h1>Central de Candidaturas</h1>
      <p className="mudo">A nova Central está sendo construída.</p>
      <p role="status" className={erro ? 'erro' : 'mudo'}>
        {erro ? `A API local não respondeu: ${erro}` : saude ? `Conectado · versão ${saude.app}` : 'Conectando…'}
      </p>
    </main>
  )
}
