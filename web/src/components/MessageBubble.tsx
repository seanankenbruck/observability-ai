import React, { useState } from 'react';
import { ChatMessage, ErrorDetails } from '../types/api';
import { formatProcessingTime, formatConfidence, getConfidenceColor, getCostColor } from '../utils/api';

interface MessageBubbleProps {
  message: ChatMessage;
}

export const MessageBubble: React.FC<MessageBubbleProps> = ({ message }) => {
  const [copied, setCopied] = useState(false);

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      console.error('Failed to copy:', error);
    }
  };

  const formatTimestamp = (timestamp: Date) => {
    return timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  if (message.type === 'system') {
    return (
      <div className="flex justify-center">
        <div className="bg-gray-800 border border-gray-600 rounded-lg p-3 max-w-md text-center">
          <p className="text-sm text-gray-300">{message.content}</p>
          {message.error && (
            <p className="text-xs text-red-400 mt-1">
              Error: {typeof message.error === 'string' ? message.error : (message.error as ErrorDetails).message}
            </p>
          )}
        </div>
      </div>
    );
  }

  const isUser = message.type === 'user';
  const bubbleClass = isUser
    ? 'bg-blue-600 text-white ml-auto'
    : 'bg-gray-800 text-gray-100 mr-auto';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`rounded-lg p-4 max-w-3xl ${bubbleClass}`}>
        {/* Message Content */}
        <div className="mb-2">
          <p className="text-sm leading-relaxed">{message.content}</p>
        </div>

        {/* PromQL Query Display */}
        {message.promql && (
          <div className="mt-3 bg-gray-900 rounded-lg p-3 border border-gray-600">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-gray-400 uppercase tracking-wide">
                PromQL Query
              </span>
              <button
                onClick={() => copyToClipboard(message.promql!)}
                className="flex items-center space-x-1 text-xs text-gray-400 hover:text-white transition-colors"
              >
                {copied ? (
                  <>
                    <span>✓</span>
                    <span>Copied!</span>
                  </>
                ) : (
                  <>
                    <span>📋</span>
                    <span>Copy</span>
                  </>
                )}
              </button>
            </div>
            <div className="bg-black/30 rounded p-2 font-mono text-sm text-green-400 overflow-x-auto">
              {message.promql}
            </div>
          </div>
        )}

        {/* Metadata */}
        {(message.confidence !== undefined || message.processing_time !== undefined) && (
          <div className="mt-3 flex flex-wrap items-center gap-3 text-xs">
            {message.confidence !== undefined && (
              <div className="flex items-center space-x-1">
                <span className="text-gray-400">Confidence:</span>
                <span className={getConfidenceColor(message.confidence)}>
                  {formatConfidence(message.confidence)}
                </span>
              </div>
            )}

            {message.processing_time !== undefined && (
              <div className="flex items-center space-x-1">
                <span className="text-gray-400">Time:</span>
                <span className="text-blue-400">
                  {formatProcessingTime(message.processing_time)}
                </span>
              </div>
            )}

            {message.estimated_cost !== undefined && (
              <div className="flex items-center space-x-1">
                <span className="text-gray-400">Cost:</span>
                <span className={getCostColor(message.estimated_cost)}>
                  {message.estimated_cost}
                </span>
              </div>
            )}

            {message.cache_hit && (
              <div className="flex items-center space-x-1">
                <span className="text-gray-400">⚡</span>
                <span className="text-green-400">Cached</span>
              </div>
            )}
          </div>
        )}

        {/* Error Display */}
        {message.error && (
          <div className="mt-3 p-3 bg-red-900/30 border border-red-700 rounded">
            {typeof message.error === 'string' ? (
              // Simple string error
              <div className="text-red-300 text-sm">
                <span className="font-medium">Error:</span> {message.error}
              </div>
            ) : (
              // Structured error with details
              <div className="space-y-2">
                <div className="flex items-start space-x-2">
                  <span className="text-red-400 text-lg flex-shrink-0">⚠️</span>
                  <div className="flex-1">
                    <div className="font-semibold text-red-300 text-sm mb-1">
                      {(message.error as ErrorDetails).code?.replace(/_/g, ' ') || 'Error'}
                    </div>
                    <div className="text-red-200 text-sm">
                      {(message.error as ErrorDetails).message}
                    </div>
                  </div>
                </div>

                {(message.error as ErrorDetails).details && (
                  <div className="pl-7 text-red-300/80 text-xs">
                    {(message.error as ErrorDetails).details}
                  </div>
                )}

                {(message.error as ErrorDetails).suggestion && (
                  <div className="pl-7 mt-2 p-2 bg-yellow-900/20 border border-yellow-700/50 rounded">
                    <div className="flex items-start space-x-2">
                      <span className="text-yellow-400 flex-shrink-0">💡</span>
                      <div className="text-yellow-200 text-xs">
                        <span className="font-medium">Suggestion:</span> {(message.error as ErrorDetails).suggestion}
                      </div>
                    </div>
                  </div>
                )}

                {(message.error as ErrorDetails).metadata?.retryable && (
                  <div className="pl-7 text-xs text-gray-400 italic">
                    This operation can be retried
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Timestamp */}
        <div className="mt-2 text-xs text-gray-500">
          {formatTimestamp(message.timestamp)}
        </div>
      </div>
    </div>
  );
};