// Preparar o pacote sozinho (jc-a2y.6.12, pedido do Vini): quando uma vaga vai para Aplicar (ou é do grupo A no modo
// direto) e ainda não tem candidatura ativa, o app cria a candidatura com o pacote do perfil e da pasta de
// currículos, o mesmo do botão "Preparar candidatura". Ela nasce "preparada": nada é aprovado nem enviado; a pessoa
// revisa e aprova na Fila. Sem PDF na pasta, não prepara (a vaga segue em "Prontas para preparar", com o aviso).
import { conteudoParaEnviar, pacoteInicial } from './pacote';
import { ErroApi } from './contexto';

type Vaga = { id: string; etapa?: string; aderencia?: string; triagemDecisao?: string; idioma?: string; contrato?: string; mercado?: string };
type Cand = { vagaId: string; estado: string };

// As mesmas regras da seção "Prontas para preparar" da Fila.
export function grupoADireto(v: Vaga): boolean {
  return v.etapa === 'descoberta' && v.aderencia === 'A' && (!v.triagemDecisao || v.triagemDecisao === 'aplicar');
}
export function vagasProntas(vagas: Vaga[], candidaturas: Cand[], perfil: any): Vaga[] {
  const ativas = new Set(candidaturas.filter(c => c.estado !== 'cancelada').map(c => c.vagaId));
  return vagas.filter(v => !ativas.has(v.id)
    && (v.etapa === 'aplicar' || (perfil?.modo === 'direto' && grupoADireto(v))));
}

export type ResultadoPreparo = { preparadas: number; semPdf: number };

// Prepara as prontas ainda não tentadas nesta abertura do app. "tentadas" evita repetir uma vaga que deu erro a cada
// leitura; vaga sem PDF não conta como tentada, para ser preparada quando o PDF aparecer.
export async function prepararProntas(
  api: (caminho: string, metodo?: string, corpo?: unknown) => Promise<any>,
  dados: { vagas: Vaga[]; perfil: any; curriculos: { nome: string }[] },
  candidaturas: Cand[], tentadas: Set<string>,
): Promise<ResultadoPreparo> {
  const r: ResultadoPreparo = { preparadas: 0, semPdf: 0 };
  for (const v of vagasProntas(dados.vagas, candidaturas, dados.perfil)) {
    if (tentadas.has(v.id)) continue;
    const pacote = pacoteInicial({ idioma: v.idioma, contrato: v.contrato, mercado: v.mercado }, dados.perfil, dados.curriculos);
    if (!pacote.curriculo.arquivo) { r.semPdf++; continue; }
    tentadas.add(v.id);
    try {
      await api('/api/v1/candidaturas', 'POST', { vagaId: v.id, conteudo: conteudoParaEnviar(pacote) });
      r.preparadas++;
    } catch (e) {
      // 409 com "existente": outra aba ou a própria pessoa preparou antes. Outros erros: a vaga fica para o botão.
      if (!(e instanceof ErroApi && e.status === 409)) continue;
    }
  }
  return r;
}
