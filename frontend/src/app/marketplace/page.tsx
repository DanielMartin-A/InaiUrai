'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { AgentProfile } from '@/lib/types';

export default function MarketplacePage() {
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    api<AgentProfile[]>('/agents')
      .then(setAgents)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load agents'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-8 text-center text-neutral-500">Loading…</div>;
  if (error) return <div className="p-8 text-center text-red-600">{error}</div>;

  return (
    <div className="container max-w-6xl mx-auto p-6">
      <h1 className="text-2xl font-semibold mb-6">Marketplace</h1>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {agents.map((agent) => (
          <div
            key={agent.id}
            className="border border-neutral-200 dark:border-neutral-700 rounded-lg p-4 bg-white dark:bg-neutral-900 shadow-sm"
          >
            <h2 className="font-medium text-lg">{agent.name}</h2>
            <p className="text-sm text-neutral-600 dark:text-neutral-400 mt-1 line-clamp-2">{agent.description}</p>
            <p className="text-sm font-medium mt-2">${(agent.base_price_cents / 100).toFixed(2)}</p>
            <div className="flex flex-wrap gap-1 mt-2">
              {agent.capabilities.map((c) => (
                <span
                  key={c}
                  className="text-xs px-2 py-0.5 rounded bg-neutral-100 dark:bg-neutral-800 text-neutral-700 dark:text-neutral-300"
                >
                  {c}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
      {agents.length === 0 && <p className="text-neutral-500 text-center py-8">No agents available.</p>}
    </div>
  );
}
