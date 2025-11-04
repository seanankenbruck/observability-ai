import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import { AuthProvider } from './contexts/AuthContext';
import './index.css';

console.log('Starting React app...');

const container = document.getElementById('root');
console.log('Container found:', container);

if (container) {
  const root = createRoot(container);
  console.log('Root created, rendering App...');
  root.render(
    <AuthProvider>
      <App />
    </AuthProvider>
  );
  console.log('App rendered successfully');
} else {
  console.error('Root element not found!');
}