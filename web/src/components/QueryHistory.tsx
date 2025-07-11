import React, { useState } from 'react';
import { ChatMessage } from '../types/api';
import { formatProcessingTime, formatConfidence, getConfidenceColor } from '../utils/api';

interface QueryHistoryProps {
  messages: ChatMessage[];
  onReplayQuery: (query: string) => void;
}

export const QueryHistory: React.FC<QueryHistoryProps> = ({
  messages,
  onReplayQuery,
}) => {
  const [filter, setFilter] = useState<'all' | 'successful' | 'failed'>('all');
  const [searchTerm, setSearchTerm] = useState('');

  // Filter to only user messages that have responses
  const queryPairs = messages.reduce((pairs: Array<{user: ChatMessage, assistant?: ChatMessage}>, message, index) => {
    if (message.type === 'user') {
      const nextMessage = messages[index + 1];
      pairs.push({
        user: message,
        assistant: nextMessage?.type === 'assistant' ? nextMessage : undefined
      });
    }
    return pairs;
  }, []);

  // Apply filters
  const filteredPairs = queryPairs.filter(pair => {
    // Filter by success/failure
    if (filter === 'successful' && (pair.assistant?.error || !pair.assistant?.promql)) return false;
    if (filter === 'failed' && !pair.assistant?.error && pair.assistant?.promql) return false;

    // Filter by search term
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      return pair.user.content.toLowerCase().includes(searchLower) ||
             pair.assistant?.promql?.toLowerCase().includes(searchLower) ||
             false;
    }

    return true;
  });

  const getStatusIcon = (pair: {user: ChatMessage, assistant?: ChatMessage}) => {
    if (!pair.assistant) return '⏳';
    if (pair.assistant.error) return '❌';
    if (pair.assistant.promql) return '✅';
    return '⚠️';
  };

  const getStatusText = (pair: {user: ChatMessage, assistant?: ChatMessage}) => {
    if (!pair.assistant) return 'Processing';
    if (pair.assistant.error) return 'Failed';
    if (pair.assistant.promql) return 'Success';
    return 'Partial';
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  };

  return (
    <div className="h-full flex flex-col bg-gray-900">
      {/* Header */}
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">Query History</h2>
          <div className="text-sm text-gray-400">
            {filteredPairs.length} of {queryPairs.length} queries
          </div>
        </div>

        {/* Filters and Search */}
        <div className="flex items-center space-x-4">
          {/* Status Filter */}
          <div className="flex space-x-1 bg-gray-800 rounded-lg p-1">
            {[
              { id: 'all' as const, label: 'All', icon: '📋' },
              { id: 'successful' as const, label: 'Success', icon: '✅' },
              { id: 'failed' as const, label: 'Failed', icon: '❌' },
            ].map((filterOption) => (
              <button
                key={filterOption.id}
                onClick={() => setFilter(filterOption.id)}
                className={`px-3 py-1.5 text-sm rounded-md transition-colors flex items-center space-x-1 ${
                  filter === filterOption.id
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-400 hover:text-white hover:bg-gray-700'
                }`}
              >
                <span>{filterOption.icon}</span>
                <span>{filterOption.label}</span>
              </button>
            ))}
          </div>

          {/* Search */}
          <div className="flex-1 max-w-md relative">
            <input
              type="text"
              placeholder="Search queries or PromQL..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            {searchTerm && (
              <button
                onClick={() => setSearchTerm('')}
                className="absolute right-2 top-2 text-gray-400 hover:text-white"
              >
                ✕
              </button>
            )}
          </div>
        </div>
      </div>

      {/* History List */}
      <div className="flex-1 overflow-y-auto p-4">
        {filteredPairs.length === 0 ? (
          <div className="text-center py-12 text-gray-400">
            <div className="text-4xl mb-4">📝</div>
            <h3 className="text-lg font-medium mb-2">
              {queryPairs.length === 0 ? 'No queries yet' : 'No queries match your filters'}
            </h3>
            <p className="text-sm">
              {queryPairs.length === 0
                ? 'Start asking questions in the chat to see your query history here'
                : 'Try adjusting your filters or search term'
              }
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {filteredPairs.map((pair, index) => (
              <div
                key={pair.user.id}
                className="bg-gray-800 border border-gray-600 rounded-lg p-4 hover:bg-gray-750 transition-colors"
              >
                {/* Query Header */}
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center space-x-3">
                    <span className="text-lg">{getStatusIcon(pair)}</span>
                    <div>
                      <div className="text-sm text-gray-400">
                        {pair.user.timestamp.toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-500">
                        Status: {getStatusText(pair)}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => onReplayQuery(pair.user.content)}
                      className="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors"
                    >
                      🔄 Replay
                    </button>
                  </div>
                </div>

                {/* User Query */}
                <div className="mb-3">
                  <div className="text-xs text-gray-400 mb-1">QUERY:</div>
                  <div className="bg-gray-900 rounded p-2 text-gray-100 text-sm">
                    {pair.user.content}
                  </div>
                </div>

                {/* Assistant Response */}
                {pair.assistant && (
                  <div>
                    {pair.assistant.promql && (
                      <div className="mb-3">
                        <div className="flex items-center justify-between mb-1">
                          <div className="text-xs text-gray-400">PROMQL:</div>
                          <button
                            onClick={() => copyToClipboard(pair.assistant!.promql!)}
                            className="text-xs text-gray-400 hover:text-white transition-colors"
                          >
                            📋 Copy
                          </button>
                        </div>
                        <div className="bg-black/30 rounded p-2 font-mono text-sm text-green-400 overflow-x-auto">
                          {pair.assistant.promql}
                        </div>
                      </div>
                    )}

                    {/* Metadata */}
                    <div className="flex flex-wrap items-center gap-4 text-xs text-gray-400">
                      {pair.assistant.confidence !== undefined && (
                        <div className="flex items-center space-x-1">
                          <span>Confidence:</span>
                          <span className={getConfidenceColor(pair.assistant.confidence)}>
                            {formatConfidence(pair.assistant.confidence)}
                          </span>
                        </div>
                      )}

                      {pair.assistant.processing_time !== undefined && (
                        <div className="flex items-center space-x-1">
                          <span>Time:</span>
                          <span className="text-blue-400">
                            {formatProcessingTime(pair.assistant.processing_time)}
                          </span>
                        </div>
                      )}

                      {pair.assistant.cache_hit && (
                        <div className="flex items-center space-x-1">
                          <span>⚡</span>
                          <span className="text-green-400">Cached</span>
                        </div>
                      )}

                      {pair.assistant.estimated_cost !== undefined && (
                        <div className="flex items-center space-x-1">
                          <span>Cost:</span>
                          <span className="text-yellow-400">{pair.assistant.estimated_cost}</span>
                        </div>
                      )}
                    </div>

                    {/* Error Display */}
                    {pair.assistant.error && (
                      <div className="mt-2 p-2 bg-red-900/30 border border-red-700 rounded text-red-300 text-sm">
                        <span className="font-medium">Error:</span> {pair.assistant.error}
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Footer Stats */}
      {queryPairs.length > 0 && (
        <div className="border-t border-gray-700 p-4">
          <div className="flex items-center justify-between text-sm text-gray-400">
            <div className="flex items-center space-x-4">
              <span>
                Success Rate: {Math.round((queryPairs.filter(p => p.assistant?.promql && !p.assistant?.error).length / queryPairs.length) * 100)}%
              </span>
              <span>
                Avg Time: {queryPairs.filter(p => p.assistant?.processing_time).length > 0
                  ? formatProcessingTime(
                      queryPairs
                        .filter(p => p.assistant?.processing_time)
                        .reduce((sum, p) => sum + (p.assistant?.processing_time || 0), 0) /
                      queryPairs.filter(p => p.assistant?.processing_time).length
                    )
                  : 'N/A'
                }
              </span>
            </div>
            <div>
              Total Queries: {queryPairs.length}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};