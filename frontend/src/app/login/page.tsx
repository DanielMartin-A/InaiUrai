'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { useAuthStore, type Role } from '@/lib/store';
import { decodeJwtPayload } from '@/lib/jwt';
import type { LoginResponse } from '@/lib/types';

export default function LoginPage() {
  const router = useRouter();
  const login = useAuthStore((s) => s.login);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = await api<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      const payload = decodeJwtPayload(data.token);
      const role = (payload.role as Role) || 'requester';
      login(data.token, role);
      if (role === 'provider') {
        router.push('/provider');
      } else {
        router.push('/dashboard');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-sm space-y-6">
        <h1 className="text-2xl font-semibold text-center">Log in</h1>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              Email
            </label>
            <input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100"
            />
          </div>
          <div>
            <label htmlFor="password" className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100"
            />
          </div>
          {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 px-4 bg-neutral-900 dark:bg-neutral-100 text-white dark:text-neutral-900 font-medium rounded-md hover:opacity-90 disabled:opacity-50"
          >
            {loading ? 'Signing in…' : 'Log in'}
          </button>
        </form>
        <p className="text-center text-sm text-neutral-600 dark:text-neutral-400">
          No account?{' '}
          <Link href="/register" className="underline hover:no-underline">Register</Link>
        </p>
      </div>
    </div>
  );
}
