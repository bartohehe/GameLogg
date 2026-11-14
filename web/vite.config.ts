/// <reference types="vitest/config" />
import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Vite czyści dist przed buildem, a binarka Go wbudowuje dist przez go:embed,
// który wymaga co najmniej jednego pliku. Odtwarzamy .gitkeep po każdym buildzie,
// żeby `go build` działał także na świeżym klonie bez zbudowanego frontendu.
function keepGitkeep(): Plugin {
  return {
    name: 'gamelogg-keep-gitkeep',
    apply: 'build',
    closeBundle() {
      writeFileSync(resolve(import.meta.dirname, 'dist/.gitkeep'), '')
    },
  }
}

export default defineConfig({
  plugins: [react(), keepGitkeep()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.{ts,tsx}'],
    passWithNoTests: true,
  },
})
