import React, { useState, useEffect } from 'react';
import { Service, Metric } from '../types/api';
import { apiClient } from '../utils/api';

interface ServiceExplorerProps {
  services: Service[];
  onRefresh: () => void;
  onServiceSelect: (service: Service) => void;
}

export const ServiceExplorer: React.FC<ServiceExplorerProps> = ({
  services,
  onRefresh,
  onServiceSelect,
}) => {
  const [selectedService, setSelectedService] = useState<Service | null>(null);
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [isLoadingMetrics, setIsLoadingMetrics] = useState(false);

  // Load metrics when service is selected
  useEffect(() => {
    if (selectedService) {
      loadMetrics(selectedService.id);
    }
  }, [selectedService]);

  const loadMetrics = async (serviceId: string) => {
    setIsLoadingMetrics(true);
    try {
      const metricsData = await apiClient.getMetrics(serviceId);
      setMetrics(metricsData);
    } catch (error) {
      console.error('Failed to load metrics:', error);
      setMetrics([]);
    } finally {
      setIsLoadingMetrics(false);
    }
  };

  const filteredServices = services.filter(service =>
    service.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    service.namespace.toLowerCase().includes(searchTerm.toLowerCase()) ||
    Object.values(service.labels).some(label =>
      label.toLowerCase().includes(searchTerm.toLowerCase())
    )
  );

  const getServiceIcon = (service: Service) => {
    const team = service.labels.team || '';
    switch (team.toLowerCase()) {
      case 'backend': return '🔧';
      case 'frontend': return '🎨';
      case 'data': return '📊';
      case 'platform': return '⚙️';
      case 'payments': return '💳';
      default: return '🔍';
    }
  };

  const getMetricIcon = (type: string) => {
    switch (type) {
      case 'counter': return '📈';
      case 'gauge': return '📏';
      case 'histogram': return '📊';
      case 'summary': return '📋';
      default: return '📊';
    }
  };

  return (
    <div className="h-full flex bg-gray-900">
      {/* Services List */}
      <div className="w-1/2 border-r border-gray-700 flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold text-white">Services</h2>
            <button
              onClick={onRefresh}
              className="px-3 py-1.5 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
            >
              🔄 Refresh
            </button>
          </div>

          {/* Search */}
          <div className="relative">
            <input
              type="text"
              placeholder="Search services..."
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

        {/* Services List */}
        <div className="flex-1 overflow-y-auto p-4 space-y-2">
          {filteredServices.length === 0 ? (
            <div className="text-center py-8 text-gray-400">
              {searchTerm ? 'No services match your search' : 'No services found'}
            </div>
          ) : (
            filteredServices.map((service) => (
              <div
                key={service.id}
                onClick={() => setSelectedService(service)}
                className={`p-3 rounded-lg border cursor-pointer transition-colors ${
                  selectedService?.id === service.id
                    ? 'bg-blue-600/20 border-blue-500'
                    : 'bg-gray-800 border-gray-600 hover:bg-gray-700 hover:border-gray-500'
                }`}
              >
                <div className="flex items-center space-x-3">
                  <span className="text-lg">{getServiceIcon(service)}</span>
                  <div className="flex-1">
                    <div className="flex items-center space-x-2">
                      <h3 className="font-medium text-white">{service.name}</h3>
                      <span className="text-xs px-2 py-1 bg-gray-700 text-gray-300 rounded">
                        {service.namespace}
                      </span>
                    </div>
                    <div className="flex items-center space-x-4 mt-1">
                      {service.labels.team && (
                        <span className="text-xs text-blue-400">
                          Team: {service.labels.team}
                        </span>
                      )}
                      <span className="text-xs text-gray-400">
                        {service.metric_names.length} metrics
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Service Details */}
      <div className="w-1/2 flex flex-col">
        {selectedService ? (
          <>
            {/* Service Header */}
            <div className="p-4 border-b border-gray-700">
              <div className="flex items-center space-x-3 mb-4">
                <span className="text-2xl">{getServiceIcon(selectedService)}</span>
                <div>
                  <h2 className="text-lg font-semibold text-white">
                    {selectedService.name}
                  </h2>
                  <p className="text-sm text-gray-400">
                    {selectedService.namespace} namespace
                  </p>
                </div>
              </div>

              {/* Labels */}
              {Object.keys(selectedService.labels).length > 0 && (
                <div className="mb-4">
                  <h3 className="text-sm font-medium text-gray-400 mb-2">Labels:</h3>
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(selectedService.labels).map(([key, value]) => (
                      <span
                        key={key}
                        className="px-2 py-1 bg-gray-700 text-gray-300 rounded text-xs"
                      >
                        {key}: {value}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Quick Actions */}
              <div className="flex space-x-2">
                <button
                  onClick={() => onServiceSelect(selectedService)}
                  className="px-3 py-1.5 text-sm bg-green-600 hover:bg-green-700 text-white rounded-lg transition-colors"
                >
                  💬 Ask about this service
                </button>
              </div>
            </div>

            {/* Metrics */}
            <div className="flex-1 overflow-y-auto p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="font-medium text-white">Metrics</h3>
                {isLoadingMetrics && (
                  <div className="text-sm text-gray-400">Loading...</div>
                )}
              </div>

              {metrics.length === 0 && !isLoadingMetrics ? (
                <div className="text-center py-8 text-gray-400">
                  No metrics found for this service
                </div>
              ) : (
                <div className="space-y-3">
                  {metrics.map((metric) => (
                    <div
                      key={metric.id}
                      className="p-3 bg-gray-800 border border-gray-600 rounded-lg hover:bg-gray-700 transition-colors"
                    >
                      <div className="flex items-center space-x-3">
                        <span className="text-lg">{getMetricIcon(metric.type)}</span>
                        <div className="flex-1">
                          <div className="flex items-center space-x-2">
                            <h4 className="font-mono text-sm text-white">
                              {metric.name}
                            </h4>
                            <span className="text-xs px-2 py-1 bg-gray-700 text-gray-300 rounded">
                              {metric.type}
                            </span>
                          </div>
                          {metric.description && (
                            <p className="text-xs text-gray-400 mt-1">
                              {metric.description}
                            </p>
                          )}
                          {Object.keys(metric.labels).length > 0 && (
                            <div className="flex flex-wrap gap-1 mt-2">
                              {Object.entries(metric.labels).map(([key, value]) => (
                                <span
                                  key={key}
                                  className="text-xs px-1 py-0.5 bg-gray-700 text-gray-400 rounded"
                                >
                                  {key}
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-gray-400">
            <div className="text-center">
              <div className="text-4xl mb-4">🔍</div>
              <h3 className="text-lg font-medium mb-2">Select a Service</h3>
              <p className="text-sm">
                Choose a service from the list to view its metrics and details
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};