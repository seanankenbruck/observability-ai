export async function fetchPromQL(query: string) {
  // Replace with your backend endpoint
  const res = await fetch('/api/v1/query', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query }),
  });
  if (!res.ok) throw new Error('API error');
  return res.json();
}
