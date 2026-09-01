import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'
import { normalizePublicBasePath } from './src/app/base-path-core.js'

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, process.cwd(), '')
  const publicBasePath = normalizePublicBasePath(
    environment.VITE_PUBLIC_BASE_PATH ?? (mode === 'test' ? '/' : '/ause-discovery/'),
  )

  return {
    base: publicBasePath,
    plugins: [react()],
  }
})
