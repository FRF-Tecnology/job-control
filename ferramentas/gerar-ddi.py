# Gera app/frontend/src/dominio/ddi.ts (código de discagem de cada país) a partir do conjunto público mledoze/countries.
# Uso: curl -sSf https://raw.githubusercontent.com/mledoze/countries/master/countries.json -o countries.json
#      python3 -I ferramentas/gerar-ddi.py countries.json app/frontend/src/dominio/ddi.ts
import json, re, sys
src, dst = sys.argv[1], sys.argv[2]
FIXOS = {'SH': '+290', 'EH': '+212', 'VA': '+39', 'AX': '+358', 'SJ': '+47'}  # o conjunto lista mais de um sufixo ou o código de área junto
codigos = {}
for c in json.load(open(src)):
    iso, idd = c['cca2'], c.get('idd', {})
    raiz, suf = idd.get('root', ''), idd.get('suffixes', [])
    if iso in FIXOS:
        codigo = FIXOS[iso]
    elif not raiz:
        continue
    elif raiz in ('+1', '+7'):
        codigo = raiz  # América do Norte e Caribe (+1) e Rússia/Cazaquistão (+7): o resto é parte do número
    elif len(suf) == 1:
        codigo = raiz + suf[0]
    else:
        raise SystemExit(f'sem regra para {iso}: {raiz} {suf}')
    assert re.fullmatch(r'\+\d{1,4}', codigo), (iso, codigo)
    codigos[iso] = codigo
assert codigos['BR'] == '+55' and codigos['US'] == '+1' and codigos['PT'] == '+351' and codigos['GB'] == '+44'
linhas = [
    '// Código de discagem (DDI) de cada país, pelo código de duas letras. Gerado por ferramentas/gerar-ddi.py a partir',
    f'// do conjunto público mledoze/countries ({len(codigos)} países). Nada da pessoa.',
    'export const DDI: Record<string, string> = ' + json.dumps(dict(sorted(codigos.items())), separators=(',', ':')) + ';',
    '',
]
open(dst, 'w').write('\n'.join(linhas))
print(len(codigos))
