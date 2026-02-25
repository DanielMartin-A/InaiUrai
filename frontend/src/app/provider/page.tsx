'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { useAuthStore } from '@/lib/store';
import type { AgentProfile } from '@/lib/types';

export default function ProviderPage() {
  const router = useRouter();
  const token = useAuthStore((s) => s.token);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [capabilities, setCapabilities] = useState('');
  const [base_price_cents, setBasePriceCents] = useState('');
  const [webhook_url, setWebhookUrl] = useState('');
  const [input_schema, setInputSchema] = useState('{}');
  const [output_schema, setOutputSchema] = useState('{}');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  if (typeof window !== 'undefined' && !token) {
    router.push('/login');
    return null;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      let input_schema_obj: Record<string, unknown> = {};
      let output_schema_obj: Record<string, unknown> = {};
      try {
        input_schema_obj = JSON.parse(input_schema || '{}');
        output_schema_obj = JSON.parse(output_schema || '{}');
      } catch {
        setError('input_schema and output_schema must be valid JSON');
        setLoading(false);
        return;
      }
      const price = parseInt(base_price_cents, 10);
      if (isNaN(price) || price < 0) {
        setError('base_price_cents must be a non-negative number');
        setLoading(false);
        return;
      }
      await api<AgentProfile>('/agents', {
        method: 'POST',
        body: JSON.stringify({
          name,
          description,
          capabilities: capabilities.split(',').map((s) => s.trim()).filter(Boolean),
          base_price_cents: price,
          webhook_url,
          input_schema: input_schema_obj,
          output_schema: output_schema_obj,
        }),
      });
      setName('');
      setDescription('');
      setCapabilities('');
      setBasePriceCents('');
      setWebhookUrl('');
      setInputSchema('{}');
      setOutputSchema('{}');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create agent');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="container max-w-xl mx-auto p-6">
      <h1 className="text-2xl font-semibold mb-6">Create agent profile</h1>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">Name</label>
          <input
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">Description</label>
          <textarea
            required
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900"
            rows={2}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
            Capabilities (comma-separated)
          </label>
          <input
            value={capabilities}
            onChange={(e) => setCapabilities(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
            Base price (cents)
          </label>
          <input
            type="number"
            min={0}
            required
            value={base_price_cents}
            onChange={(e) => setBasePriceCents(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">Webhook URL</label>
          <input
            type="url"
            required
            value={webhook_url}
            onChange={(e) => setWebhookUrl(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
            Input schema (JSON)
          </label>
          <textarea
            value={input_schema}
            onChange={(e) => setInputSchema(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 font-mono text-sm"
            rows={3}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-600 dark:text-neutral-400 mb-1">
            Output schema (JSON)
          </label>
          <textarea
            value={output_schema}
            onChange={(e) => setOutputSchema(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 dark:border-neutral-600 rounded-md bg-white dark:bg-neutral-900 font-mono text-sm"
            rows={3}
          />
        </div>
        {error && <p className="text-sm text-red-600">{error}</p>}
        <button
          type="submit"
          disabled={loading}
          className="py-2 px-4 bg-neutral-900 dark:bg-neutral-100 text-white dark:text-neutral-900 font-medium rounded-md disabled:opacity-50"
        >
          {loading ? 'Creating…' : 'Create agent'}
        </button>
      </form>
    </div>
  );
}
