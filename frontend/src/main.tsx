import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

const applicationElement = document.getElementById('root')

if (!applicationElement) {
  throw new Error('Application root element is missing')
}

createRoot(applicationElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
