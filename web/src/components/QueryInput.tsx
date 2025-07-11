import React, { useState, useRef, useEffect } from 'react';
import { Service } from '../types/api';

interface QueryInputProps {
  onSendMessage: (message: string) => void;
  disabled: boolean;
  services: Service[];
}

export const QueryInput: React.FC<QueryInputProps> = ({
  onSendMessage,
  disabled,
  services,
}) => {
  const [input, setInput] = useState('');
  const [showSuggestions, setShowSuggestions] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`;
    }
  }, [input]);

  // Close suggestions when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setShowSuggestions(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim() && !disabled) {
      onSendMessage(input.trim());
      setInput('');
      setShowSuggestions(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value);
    setShowSuggestions(e.target.value.length > 2);
  };

  // Generate smart suggestions based on available services
  const getSuggestions = () => {
    const query = input.toLowerCase();
    const suggestions: string[] = [];

    // Service-specific suggestions
    (services || []).forEach(service => {
      if (service.name.toLowerCase().includes(query) || query.includes(service.name.toLowerCase())) {
        suggestions.push(`Show error rate for ${service.name}`);
        suggestions.push(`What's the latency for ${service.name}?`);
        suggestions.push(`Requests per second for ${service.name}`);
        suggestions.push(`Database connections for ${service.name}`);
      }
    });

    // General query patterns
    const patterns = [
      'Show error rate for',
      'What is the 95th percentile latency for',
      'Requests per second for',
      'Memory usage for',
      'CPU usage for',
      'Database connections for',
      'Queue length for',
      'Response time for',
      'Throughput of',
      'Availability of',
    ];

    patterns.forEach(pattern => {
      if (pattern.toLowerCase().includes(query) || query.includes(pattern.toLowerCase())) {
        suggestions.push(pattern + ' [service-name]');
      }
    });

    // Remove duplicates and limit results
    return [...new Set(suggestions)].slice(0, 6);
  };

  const suggestions = showSuggestions ? getSuggestions() : [];

  const handleSuggestionClick = (suggestion: string) => {
    if (suggestion.includes('[service-name]') && services && services.length > 0) {
      // Replace placeholder with first available service
      const completeSuggestion = suggestion.replace('[service-name]', services[0].name);
      setInput(completeSuggestion);
    } else {
      setInput(suggestion);
    }
    setShowSuggestions(false);
    textareaRef.current?.focus();
  };

  return (
    <div ref={containerRef} className="relative">
      {/* Suggestions Dropdown */}
      {showSuggestions && suggestions.length > 0 && (
        <div className="absolute bottom-full mb-2 w-full bg-gray-800 border border-gray-600 rounded-lg shadow-lg z-10 max-h-48 overflow-y-auto">
          <div className="p-2">
            <div className="text-xs text-gray-400 mb-2 px-2">Suggestions:</div>
            {suggestions.map((suggestion, index) => (
              <button
                key={index}
                onClick={() => handleSuggestionClick(suggestion)}
                className="w-full text-left px-2 py-2 text-sm text-gray-300 hover:bg-gray-700 rounded transition-colors"
              >
                💡 {suggestion}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Input Form */}
      <form onSubmit={handleSubmit} className="relative">
        <div className="flex items-end space-x-3">
          {/* Textarea */}
          <div className="flex-1 relative">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              onFocus={() => setShowSuggestions(input.length > 2)}
              placeholder="Ask about your metrics... (e.g., 'Show error rate for user-service')"
              disabled={disabled}
              className="w-full resize-none bg-gray-800 border border-gray-600 rounded-lg px-4 py-3 pr-12 text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed min-h-[52px] max-h-32"
              rows={1}
            />

            {/* Character count (optional) */}
            {input.length > 100 && (
              <div className="absolute bottom-1 right-12 text-xs text-gray-500">
                {input.length}
              </div>
            )}
          </div>

          {/* Send Button */}
          <button
            type="submit"
            disabled={!input.trim() || disabled}
            className="px-4 py-3 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded-lg transition-colors flex items-center justify-center min-w-[52px]"
          >
            {disabled ? (
              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
            ) : (
              <span className="text-lg">→</span>
            )}
          </button>
        </div>

        {/* Quick Actions */}
        <div className="flex items-center justify-between mt-2">
          <div className="flex items-center space-x-2 text-xs text-gray-400">
            <span>Press Enter to send, Shift+Enter for new line</span>
          </div>

          {services && services.length > 0 && (
            <div className="flex items-center space-x-1">
              <span className="text-xs text-gray-400">Services available:</span>
              <span className="text-xs text-blue-400">
                {services.slice(0, 2).map(s => s.name).join(', ')}
                {services.length > 2 && ` +${services.length - 2}`}
              </span>
            </div>
          )}
        </div>
      </form>
    </div>
  );
};