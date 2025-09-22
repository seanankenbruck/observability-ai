import React, { useState, useEffect } from 'react';
import { ChatInterface } from './components/ChatInterface';
import { ServiceExplorer } from './components/ServiceExplorer';
import { QueryHistory } from './components/QueryHistory';
import { Header } from './components/Header';
import { ChatMessage, Service } from './types/api';
import { apiClient } from './utils/api';

type ActiveTab = 'chat' | 'services' | 'history';

function App() {
  const [activeTab, setActiveTab] = useState<ActiveTab>('chat');
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: '1',
      type: 'system',
      content: 'Welcome to Observability AI! Ask me about your metrics in natural language, and I\'ll convert it to PromQL.',
      timestamp: new Date(),
    },
  ]);
  const [services, setServices] = useState<Service[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isServicesLoading, setIsServicesLoading] = useState(true);
  const [connectionStatus, setConnectionStatus] = useState<'connected' | 'disconnected' | 'connecting'>('disconnected');

  // Load services on mount
  useEffect(() => {
    loadServices();
    checkConnection();
  }, []);

  const loadServices = async () => {
    setIsServicesLoading(true);
    try {
      const servicesData = await apiClient.getServices();
      setServices(servicesData || []); // Ensure it's always an array
    } catch (error) {
      console.error('Failed to load services:', error);
      setServices([]); // Set to empty array on error
      // Add error message to chat
      const errorMessage: ChatMessage = {
        id: Date.now().toString(),
        type: 'system',
        content: 'Warning: Could not load services from the backend. Some features may be limited.',
        timestamp: new Date(),
        error: error instanceof Error ? error.message : 'Unknown error',
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setIsServicesLoading(false);
    }
  };

  const checkConnection = async () => {
    setConnectionStatus('connecting');
    try {
      await apiClient.healthCheck();
      setConnectionStatus('connected');
    } catch (error) {
      setConnectionStatus('disconnected');
      console.error('Backend connection failed:', error);
    }
  };

  const handleSendMessage = async (content: string) => {
    // Add user message
    const userMessage: ChatMessage = {
      id: Date.now().toString(),
      type: 'user',
      content,
      timestamp: new Date(),
    };
    setMessages(prev => [...prev, userMessage]);
    setIsLoading(true);

    try {
      // Process query
      const response = await apiClient.processQuery({
        query: content,
        user_id: 'web-user', // TODO: Implement proper user management
      });

      // Add assistant response
      const assistantMessage: ChatMessage = {
        id: (Date.now() + 1).toString(),
        type: 'assistant',
        content: response.explanation || 'Here\'s your PromQL query:',
        promql: response.promql,
        confidence: response.confidence,
        processing_time: response.processing_time,
        estimated_cost: response.estimated_cost,
        cache_hit: response.cache_hit,
        timestamp: new Date(),
      };
      setMessages(prev => [...prev, assistantMessage]);
    } catch (error) {
      // Add error message
      const errorMessage: ChatMessage = {
        id: (Date.now() + 1).toString(),
        type: 'assistant',
        content: 'Sorry, I encountered an error processing your query.',
        timestamp: new Date(),
        error: error instanceof Error ? error.message : 'Unknown error',
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleClearChat = () => {
    setMessages([
      {
        id: '1',
        type: 'system',
        content: 'Chat cleared. How can I help you with your observability queries?',
        timestamp: new Date(),
      },
    ]);
  };

  const handleRetryConnection = () => {
    checkConnection();
  };

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      <Header
        activeTab={activeTab}
        onTabChange={setActiveTab}
        connectionStatus={connectionStatus}
        onRetryConnection={handleRetryConnection}
      />

      <main className="container mx-auto px-4 py-6 max-w-7xl">
        <div className="h-[calc(100vh-140px)]">
          {activeTab === 'chat' && (
            <ChatInterface
              messages={messages}
              onSendMessage={handleSendMessage}
              onClearChat={handleClearChat}
              isLoading={isLoading}
              services={services}
              isServicesLoading={isServicesLoading}
            />
          )}

          {activeTab === 'services' && (
            <ServiceExplorer
              services={services}
              onRefresh={loadServices}
              isLoading={isServicesLoading}
              onServiceSelect={(service) => {
                // Switch to chat and insert service query
                setActiveTab('chat');
                // You could auto-populate the input here
              }}
            />
          )}

          {activeTab === 'history' && (
            <QueryHistory
              messages={messages.filter(m => m.type !== 'system')}
              onReplayQuery={(query) => {
                setActiveTab('chat');
                handleSendMessage(query);
              }}
            />
          )}
        </div>
      </main>
    </div>
  );
}

export default App;