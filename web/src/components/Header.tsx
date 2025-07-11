import React from 'react';

type ActiveTab = 'chat' | 'services' | 'history';
type ConnectionStatus = 'connected' | 'disconnected' | 'connecting';

interface HeaderProps {
  activeTab: ActiveTab;
  onTabChange: (tab: ActiveTab) => void;
  connectionStatus: ConnectionStatus;
  onRetryConnection: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  activeTab,
  onTabChange,
  connectionStatus,
  onRetryConnection,
}) => {
  const getStatusColor = () => {
    switch (connectionStatus) {
      case 'connected': return 'bg-green-500';
      case 'connecting': return 'bg-yellow-500 animate-pulse';
      case 'disconnected': return 'bg-red-500';
    }
  };

  const getStatusText = () => {
    switch (connectionStatus) {
      case 'connected': return 'Connected';
      case 'connecting': return 'Connecting...';
      case 'disconnected': return 'Disconnected';
    }
  };

  return (
    <header className="bg-gray-800 border-b border-gray-700">
      <div className="container mx-auto px-4 max-w-7xl">
        <div className="flex items-center justify-between h-16">
          {/* Logo and Title */}
          <div className="flex items-center space-x-4">
            <div className="flex items-center space-x-2">
              <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
                <span className="text-white font-bold text-sm">OA</span>
              </div>
              <h1 className="text-xl font-bold text-white">
                Observability AI
              </h1>
            </div>

            {/* Connection Status */}
            <div className="flex items-center space-x-2">
              <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} />
              <span className="text-sm text-gray-300">{getStatusText()}</span>
              {connectionStatus === 'disconnected' && (
                <button
                  onClick={onRetryConnection}
                  className="text-xs text-blue-400 hover:text-blue-300 underline"
                >
                  Retry
                </button>
              )}
            </div>
          </div>

          {/* Navigation Tabs */}
          <nav className="flex space-x-1">
            {[
              { id: 'chat' as const, label: 'Chat', icon: '💬' },
              { id: 'services' as const, label: 'Services', icon: '🔧' },
              { id: 'history' as const, label: 'History', icon: '📋' },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => onTabChange(tab.id)}
                className={`
                  px-4 py-2 rounded-lg text-sm font-medium transition-colors duration-200
                  flex items-center space-x-2
                  ${activeTab === tab.id
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-300 hover:text-white hover:bg-gray-700/50'
                  }
                `}
              >
                <span>{tab.icon}</span>
                <span>{tab.label}</span>
              </button>
            ))}
          </nav>

          {/* Actions */}
          <div className="flex items-center space-x-2">
            <button
              className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
              title="Settings"
            >
              ⚙️
            </button>
            <button
              className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
              title="Help"
            >
              ❓
            </button>
          </div>
        </div>
      </div>
    </header>
  );
};