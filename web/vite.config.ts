import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 5174 karena 5173 sering sudah dipakai project lain.
    port: 5174,
    strictPort: true,
  },
});
