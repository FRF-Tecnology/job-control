# Gera app/frontend/src/dominio/municipios.ts (estados e municípios do Brasil) a partir da API de localidades do IBGE.
# Uso: curl -sSf "https://servicodados.ibge.gov.br/api/v1/localidades/municipios?view=nivelado" -o municipios.json
#      python3 -I ferramentas/gerar-municipios.py municipios.json app/frontend/src/dominio/municipios.ts
import json, sys, collections
src, dst = sys.argv[1], sys.argv[2]
d = json.load(open(src))
est = {}
cid = collections.defaultdict(list)
for m in d:
    est[m['UF-sigla']] = m['UF-nome']
    cid[m['UF-sigla']].append(m['municipio-nome'])
import locale
def chave(s):
    import unicodedata
    return unicodedata.normalize('NFD', s).encode('ascii','ignore').decode().lower()
estados = sorted(est.items(), key=lambda kv: chave(kv[1]))
for s in cid: cid[s] = sorted(set(cid[s]), key=chave)
assert len(estados) == 27, len(estados)
for nomes in cid.values():
    for n in nomes: assert '|' not in n and '\n' not in n
linhas = [
 "// Estados e municípios do Brasil, da API de localidades do IBGE (servicodados.ibge.gov.br, municípios, view=nivelado).",
 f"// Gerado por ferramentas/gerar-municipios.py: {sum(len(v) for v in cid.values())} municípios. Dado público, sem nada da pessoa.",
 "// Fica num módulo à parte, carregado só pela tela do perfil (import dinâmico).",
 "export const ESTADOS: [sigla: string, nome: string][] = " + json.dumps([[s, n] for s, n in estados], ensure_ascii=False) + ";",
 "// Por estado, os nomes separados por \"|\" (ocupa menos que uma lista).",
 "const CIDADES: Record<string, string> = {",
]
for s, _ in estados:
    linhas.append(f"  {s}: {json.dumps('|'.join(cid[s]), ensure_ascii=False)},")
linhas += ["};", "export const cidadesDoEstado = (uf: string): string[] => CIDADES[uf]?.split('|') ?? [];", ""]
open(dst, 'w').write('\n'.join(linhas))
