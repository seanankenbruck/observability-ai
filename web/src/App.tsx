import React from 'react';
import ChatInterface from './components/ChatInterface';
import ServiceExplorer from './components/ServiceExplorer';

export default function App() {
  return (
    <div className="min-h-screen flex flex-col items-center bg-gray-50">
      <header className="w-full bg-blue-600 text-white p-4 text-2xl font-bold text-center shadow">
        Observability AI
      </header>
      <main className="flex flex-1 w-full max-w-5xl mx-auto p-4 gap-8">
        <section className="flex-1">
          <ChatInterface />
        </section>
        <aside className="w-80 hidden md:block">
          <ServiceExplorer />
        </aside>
      </main>
    </div>
  );
}
