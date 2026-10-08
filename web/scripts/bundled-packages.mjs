// Prints the npm packages that end up in the built web interface, one per
// line, by building in memory and reading the bundle's module list. Used by
// scripts/third-party-licenses.sh; devDependencies that only run at build
// time, such as the Svelte compiler's parser, are not listed.
import { fileURLToPath } from 'node:url';
import { build } from 'vite';

const packages = new Set();
await build({
  configFile: fileURLToPath(new URL('../vite.config.ts', import.meta.url)),
  root: fileURLToPath(new URL('..', import.meta.url)),
  logLevel: 'silent',
  build: { write: false },
  plugins: [
    {
      name: 'list-bundled-packages',
      generateBundle(_, bundle) {
        for (const chunk of Object.values(bundle)) {
          if (chunk.type !== 'chunk') continue;
          for (const id of Object.keys(chunk.modules)) {
            const match = id.split('\\').join('/').match(/node_modules\/((@[^/]+\/)?[^/]+)/);
            if (match) packages.add(match[1]);
          }
        }
      },
    },
  ],
});
console.log([...packages].sort().join('\n'));
