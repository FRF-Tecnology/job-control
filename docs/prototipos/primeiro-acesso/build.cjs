const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const arte = path.resolve(__dirname, '../../../app/frontend/src/arte/primeiro-acesso');
const ler = (pasta, nome) => fs.readFileSync(path.join(pasta, nome), 'utf8').replaceAll('\r\n', '\n');

function pdfFicticio() {
  const linhas = [
    'Perfil ficticio - demonstracao Applyer',
    'Desenvolvimento de software',
    'Tecnologias: Java, Spring Boot e React.',
    'Experiencia em desenvolvimento de aplicacoes.'
  ];
  const comandos = linhas.map((texto, i) => (i ? '0 -28 Td ' : '') + '(' + texto + ') Tj');
  const stream = 'BT /F1 14 Tf 64 770 Td ' + comandos.join('\n') + ' ET\n';
  const objetos = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Count 1 /Kids [3 0 R] >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
    '<< /Length ' + stream.length + ' >>\nstream\n' + stream + 'endstream'
  ];

  let pdf = '%PDF-1.4\n';
  const offsets = [];
  for (const [i, objeto] of objetos.entries()) {
    offsets.push(pdf.length);
    pdf += (i + 1) + ' 0 obj\n' + objeto + '\nendobj\n';
  }
  const inicioXref = pdf.length;
  const tamanho = objetos.length + 1;
  pdf += 'xref\n0 ' + tamanho + '\n0000000000 65535 f \n';
  for (const offset of offsets) {
    pdf += String(offset).padStart(10, '0') + ' 00000 n \n';
  }
  pdf += 'trailer\n<< /Size ' + tamanho + ' /Root 1 0 R >>\nstartxref\n' + inicioXref + '\n%%EOF\n';
  return Buffer.from(pdf, 'ascii').toString('base64');
}

const marca = ler(arte, 'applyer-inversa.svg')
  .replaceAll('#F7F8FF', 'var(--forte)')
  .replaceAll('#AFB6FF', 'var(--lilas)');
const codigo = ler(__dirname, 'matching.js') + '\n'
  + ler(__dirname, 'fluxo.js').replace('{{PDF_BASE64}}', pdfFicticio());
const hash = crypto.createHash('sha256').update(codigo).digest('base64');
const html = ler(__dirname, 'tela.html')
  .replace('{{MARCA}}', () => marca)
  .replace('{{PAISAGEM}}', () => ler(arte, 'paisagem.svg'))
  .replace('{{ESTILO}}', () => ler(__dirname, 'estilo.css'))
  .replace('{{SCRIPT_HASH}}', "'sha256-" + hash + "'")
  .replace('{{CODIGO}}', () => codigo);

const destino = process.argv[2]
  ? path.resolve(process.argv[2])
  : path.join(__dirname, 'Applyer-primeiro-acesso.html');
fs.writeFileSync(destino, html, 'utf8');
console.log(destino);
