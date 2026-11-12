const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '../../..');
const appArt = path.join(root, 'app/frontend/src/arte');
const siteBrands = path.join(root, 'site/marcas');
const prototypeArt = path.join(__dirname, 'arte');
const read = file => fs.readFileSync(file, 'utf8');
const source = file => read(path.join(__dirname, file));
const svgUrl = file => 'data:image/svg+xml;base64,' + fs.readFileSync(file).toString('base64');

const assets = {
  logo: svgUrl(path.join(appArt, 'primeiro-acesso/applyer-inversa.svg')),
  radar: read(path.join(appArt, 'radar.svg')),
  landscape: svgUrl(path.join(appArt, 'primeiro-acesso/paisagem.svg')),
  companyLogos: { huggingface: svgUrl(path.join(appArt, 'huggingface.svg')) },
  chatgpt: svgUrl(path.join(siteBrands, 'chatgpt.svg')),
  claude: svgUrl(path.join(prototypeArt, 'claude.svg')),
  grok: svgUrl(path.join(siteBrands, 'grok.svg')),
  gemini: svgUrl(path.join(prototypeArt, 'gemini.svg'))
};

const scripts = ['art.js', 'data.js', 'tracks.js', 'shell.js', 'multi-ui.js', 'radar.js', 'journey.js', 'profile.js'];
const styles = ['base.css', 'radar.css', 'journey.css', 'profile.css', 'multi-ui.css', 'logos.css'];
const js = [
  'window.designAssets = ' + JSON.stringify(assets) + ';',
  ...scripts.map(source),
  'window.A.start();'
].join('\n');

const html = source('frame.html')
  .replace('__ICON__', () => svgUrl(path.join(prototypeArt, 'icone.svg')))
  .replace('/*__CSS__*/', () => styles.map(source).join('\n'))
  .replace('/*__JS__*/', () => js.replace(/<\/script/gi, '<\\/script'));

const output = process.argv[2]
  ? path.resolve(process.argv[2])
  : path.join(__dirname, 'Applyer-prototipo-navegavel.html');
fs.writeFileSync(output, html, 'utf8');
console.log(output);
