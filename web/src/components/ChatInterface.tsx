import React, { useState } from 'react';
import MessageBubble from './MessageBubble';
import QueryInput from './QueryInput';

const initialMessages = [
  { id: 1, sender: 'user', text: 'Show me error rate for user-service' },
  { id: 2, sender: 'ai', text: 'PromQL: rate(user_service_errors_total[5m])' },
];

export default function ChatInterface() {
  const [messages, setMessages] = useState(initialMessages);

  const handleSend = (text: string) => {
    setMessages([...messages, { id: messages.length + 1, sender: 'user', text }]);
    // Here you would call your API and append the AI response
  };

  return (
    <div className="flex flex-col h-full bg-white rounded shadow p-4">
      <div className="flex-1 overflow-y-auto space-y-2 mb-4">
        {messages.map(msg => (
          <MessageBubble key={msg.id} sender={msg.sender} text={msg.text} />
        ))}
      </div>
      <QueryInput onSend={handleSend} />
    </div>
  );
}
