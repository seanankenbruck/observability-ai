import React, { useState } from 'react';

export default function QueryInput({ onSend }: { onSend: (text: string) => void }) {
  const [value, setValue] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (value.trim()) {
      onSend(value.trim());
      setValue('');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <input
        className="flex-1 border rounded px-3 py-2 focus:outline-none focus:ring"
        type="text"
        placeholder="Type your query..."
        value={value}
        onChange={e => setValue(e.target.value)}
      />
      <button type="submit" className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700">
        Send
      </button>
    </form>
  );
}
