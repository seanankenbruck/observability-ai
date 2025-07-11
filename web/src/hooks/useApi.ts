import { useState } from 'react';
import { fetchPromQL } from '../utils/api';

export function useApi() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const queryPromQL = async (query: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchPromQL(query);
      return data;
    } catch (err: any) {
      setError(err.message);
      return null;
    } finally {
      setLoading(false);
    }
  };

  return { queryPromQL, loading, error };
}
