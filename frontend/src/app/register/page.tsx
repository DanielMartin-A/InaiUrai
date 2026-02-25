'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import type { Account } from '@/lib/types';

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [display_name, setDisplayName] = useState('');
  const [role, setRole] = useState<'requester' | 'provider'>('requester');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api<Account>('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, password, display_name, role }),
      });
      router.push('/login');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-sm space-y-6">
        <h1 className="text-2xl font-semibold text-center">Create account</h1>
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
          <div>
            <label htmlFor="display_name" className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              Display name
            </label>
            <input
              id="display_name"
              type="text"
              required
              value={display_name}
              onChange={(e) => setDisplayName(e.target.value)}
              className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100"
            />
          </div>
          <div>
            <label htmlFor="role" className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
              Role
            </label>
            <select
              id="role"
              value={role}
              onChange={(e) => setRole(e.target.value as 'requester' | 'provider')}
              className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 text-neutral-900 dark:text-neutral-100"
            >
              <option value="requester">Requester</option>
              <option value="provider">Provider</option>
            </select>
          </div>
          {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 px-4 bg-neutral-900 dark:bg-neutral-100 text-white dark:text-neutral-900 font-medium rounded-md hover:opacity-90 disabled:opacity-50"
          >
            {loading ? 'Creating…' : 'Register'}
          </button>
        </form>
        <p className="text-center text-sm text-neutral-600 dark:text-neutral-400">
          Already have an account?{' '}
          <Link href="/login" className="underline hover:no-underline">Log in</Link>
        </p>
      </div>
    </div>
  );
}
