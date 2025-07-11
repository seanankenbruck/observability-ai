import React, { useRef, useEffect } from 'react';
import { ChatMessage, Service } from '../types/api';
import { MessageBubble } from './MessageBubble';
import { QueryInput } from './QueryInput';

interface ChatInterfaceProps {
  messages: ChatMessage[];
  onSendMessage: (message: string) => void;
  onClearChat: () => void;
  isLoading: boolean;
  services: Service[];
}

export const ChatInterface: React.FC<ChatInterfaceProps> = ({
  messages,
  onSendMessage,
  onClearChat,
  isLoading,
  services,
}) => {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const exampleQueries = [
    'Show error rate for user-service',
    'What is the 95th percentile latency for payment-service?',
    'Requests per second for notification-service',
    'Database connections for all services',
    'Memory usage over the last hour',
  ];

  return (
    <div className="h-full flex flex-col bg-gray-900">
      {/* Chat Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-700">
        <div>
          <h2 className="text-lg font-semibold text-white">Natural Language to PromQL</h2>
          <p className="text-sm text-gray-400">
            Ask questions about your metrics in plain English
          </p>
        </div>
        <button
          onClick={onClearChat}
          className="px-3 py-1.5 text-sm bg-gray-700 hover:bg-gray-600 text-gray-300 hover:text-white rounded-lg transition-colors"
        >
          Clear Chat
        </button>
      </div>

      {/* Messages Area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 1 && (
          <div className="text-center py-8">
            <div className="mb-6">
              <h3 className="text-lg font-medium text-white mb-2">
                Try asking something like:
              </h3>
              <div className="grid gap-2 max-w-2xl mx-auto">
                {exampleQueries.map((query, index) => (
                  <button
                    key={index}
                    onClick={() => onSendMessage(query)}
                    className="p-3 text-left bg-gray-800 hover:bg-gray-700 border border-gray-600 hover:border-gray-500 rounded-lg transition-colors text-gray-300 hover:text-white"
                  >
                    💬 {query}
                  </button>
                ))}
              </div>
            </div>

            {services.length > 0 && (
              <div className="text-sm text-gray-400">
                <p>Available services: {services.slice(0, 3).map(s => s.name).join(', ')}
                  {services.length > 3 && ` and ${services.length - 3} more`}
                </p>
              </div>
            )}
          </div>
        )}

        {messages.map((message) => (
          <MessageBubble key={message.id} message={message} />
        ))}

        {isLoading && (
          <div className="flex justify-start">
            <div className="bg-gray-800 rounded-lg p-4 max-w-xs">
              <div className="flex items-center space-x-2">
                <div className="flex space-x-1">
                  <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                  <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                  <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                </div>
                <span className="text-sm text-gray-400">Generating PromQL...</span>
              </div>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input Area */}
      <div className="border-t border-gray-700 p-4">
        <QueryInput
          onSendMessage={onSendMessage}
          disabled={isLoading}
          services={services}
        />
      </div>
    </div>
  );
};