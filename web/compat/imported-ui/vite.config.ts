import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';
export default defineConfig({
    root: fileURLToPath(new URL('.', import.meta.url)), publicDir: false, server: {
        host: '127.0.0.1', port: 4181, strictPort: true, fs: { strict: true, allow: [fileURLToPath(new URL('.', import.meta.url)), fileURLToPath(new URL('../../node_modules', import.meta.url))] }
    }, build: { outDir: 'dist', emptyOutDir: true }
});
