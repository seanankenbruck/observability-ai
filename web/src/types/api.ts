export interface PromQLResponse {
  promql: string;
  confidence: number;
  cacheHit: boolean;
  estimatedCost: number;
  explanation: string;
}
