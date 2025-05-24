// tailwind.config.js
import { defineConfig } from '@tailwindcss/vite';

export default defineConfig({
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      // You can add custom colors here, but keep defaults
    },
  },
  // plugins: [], // Remove if empty
});
