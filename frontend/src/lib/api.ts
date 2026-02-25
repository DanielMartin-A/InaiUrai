import { useAuthStore } from './store';

function getApiBase(): string {
  if (typeof window !== 'undefined' && window.location.hostname === 'localhost') {
    return 'http://localhost:8080/api/v1';
  }
  return 'https://inaiurai-production.up.railway.app/api/v1';
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = typeof window !== 'undefined' ? useAuthStore.getState().token : null;
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${getApiBase()}${path}`, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  const contentType = res.headers.get('content-type');
  if (contentType?.includes('application/json')) {
    return res.json() as Promise<T>;
  }
  return undefined as T;
}
