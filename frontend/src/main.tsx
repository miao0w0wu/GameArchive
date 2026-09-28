import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './style.css'

const container = document.getElementById('root')
if (!container) {
    throw new Error('未找到 #root 挂载节点')
}

createRoot(container).render(
    <React.StrictMode>
        <App />
    </React.StrictMode>,
)