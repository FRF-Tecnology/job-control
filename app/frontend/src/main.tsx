import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
// Fonte Inter empacotada no app (funciona sem internet e não consulta servidor de fontes).
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '@fontsource/inter/700.css'
import '@fontsource/inter/800.css'
import './estilo/estilo.css'

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
