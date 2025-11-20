import { QueryRequest, QueryResponse, Service, Metric, ApiError } from '../types/api';

// Use relative path so it works with both Vite proxy (dev) and nginx proxy (production/docker)
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    const config: RequestInit = {
      headers,
      credentials: 'include', // Include cookies for session authentication
      ...options,
    };

    try {
      const response = await fetch(url, config);

      if (!response.ok) {
        const errorData: ApiError = await response.json().catch(() => ({
          error: `HTTP ${response.status}: ${response.statusText}`,
        }));

        // If error is a structured error object, throw it as-is
        if (typeof errorData.error === 'object' && errorData.error !== null) {
          const structuredError = new Error(errorData.error.message || 'Request failed');
          (structuredError as any).errorDetails = errorData.error;
          throw structuredError;
        }

        // Otherwise, throw a simple error
        throw new Error(errorData.error as string || `Request failed with status ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error) {
        throw error;
      }
      throw new Error('An unexpected error occurred');
    }
  }

  // Query processing
  async processQuery(request: QueryRequest): Promise<QueryResponse> {
    return this.request<QueryResponse>('/query', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }

  // Service management
  async getServices(): Promise<Service[]> {
    return this.request<Service[]>('/services');
  }

  async getService(serviceId: string): Promise<Service> {
    return this.request<Service>(`/services/${serviceId}`);
  }

  async searchServices(query: string): Promise<Service[]> {
    return this.request<Service[]>(`/services/search?q=${encodeURIComponent(query)}`);
  }

  // Metrics
  async getMetrics(serviceId: string): Promise<Metric[]> {
    return this.request<Metric[]>(`/services/${serviceId}/metrics`);
  }

  async getAllMetrics(): Promise<Metric[]> {
    return this.request<Metric[]>('/metrics');
  }

  // Health check
  async healthCheck(): Promise<{ status: string }> {
    return this.request<{ status: string }>('/health');
  }

  // Query suggestions (future feature)
  async getQuerySuggestions(partial: string): Promise<string[]> {
    return this.request<string[]>(`/suggestions?q=${encodeURIComponent(partial)}`);
  }
}

// Export singleton instance
export const apiClient = new ApiClient();

// Utility functions
export const formatProcessingTime = (timeInSeconds: number): string => {
  if (timeInSeconds < 1) {
    return `${Math.round(timeInSeconds * 1000)}ms`;
  }
  return `${timeInSeconds.toFixed(2)}s`;
};

export const formatConfidence = (confidence: number): string => {
  return `${Math.round(confidence * 100)}%`;
};

export const getConfidenceColor = (confidence: number): string => {
  if (confidence >= 0.8) return 'text-green-400';
  if (confidence >= 0.6) return 'text-yellow-400';
  return 'text-red-400';
};

export const getCostColor = (cost: number): string => {
  if (cost <= 2) return 'text-green-400';
  if (cost <= 5) return 'text-yellow-400';
  return 'text-red-400';
};

// Error handling utilities
export const isApiError = (error: any): error is ApiError => {
  return error && typeof error.error === 'string';
};

export const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  if (isApiError(error)) {
    if (typeof error.error === 'string') {
      return error.error;
    }
    return error.error.message;
  }
  return 'An unexpected error occurred';
};