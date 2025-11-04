// API Types for the Observability AI Web Interface

export interface QueryRequest {
    query: string;
    time_range?: string;
    context?: Record<string, string>;
    user_id?: string;
  }

  export interface QueryResponse {
    promql: string;
    explanation: string;
    confidence: number;
    suggestions?: string[];
    estimated_cost: number;
    cache_hit: boolean;
    processing_time: number; // in seconds
    metadata?: Record<string, any>;
  }

  export interface Service {
    id: string;
    name: string;
    namespace: string;
    labels: Record<string, string>;
    metric_names: string[];
    created_at: string;
    updated_at: string;
  }

  export interface Metric {
    id: string;
    name: string;
    type: 'counter' | 'gauge' | 'histogram' | 'summary';
    description: string;
    labels: Record<string, string>;
    service_id: string;
    created_at: string;
    updated_at: string;
  }

  export interface ChatMessage {
    id: string;
    type: 'user' | 'assistant' | 'system';
    content: string;
    promql?: string;
    confidence?: number;
    processing_time?: number;
    estimated_cost?: number;
    cache_hit?: boolean;
    timestamp: Date;
    error?: string;
  }

  export interface ApiError {
    error: string;
    message?: string;
    details?: any;
  }

  // WebSocket message types
  export interface WebSocketMessage {
    type: 'query' | 'response' | 'error' | 'status';
    data: any;
    id?: string;
  }

  // Query history
  export interface QueryHistoryItem {
    id: string;
    natural_query: string;
    promql: string;
    confidence: number;
    success: boolean;
    timestamp: Date;
    processing_time?: number;
  }

  // Authentication types
  export interface User {
    id: string;
    username: string;
    email: string;
    roles: string[];
    active: boolean;
  }

  export interface LoginRequest {
    username: string;
    password: string;
  }

  export interface LoginResponse {
    token: string;
    expires_at: string;
    user: User;
  }

  export interface AuthStatus {
    authenticated: boolean;
    user?: User;
  }