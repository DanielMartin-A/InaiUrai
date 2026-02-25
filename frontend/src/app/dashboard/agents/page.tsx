'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { AgentProfile } from '@/lib/types';
import {
  Plus,
  Bot,
  Loader2,
  Star,
  CheckCircle2,
  XCircle,
  Clock,
} from 'lucide-react';

const STATUS_CONFIG: Record<string, { color: string; icon: typeof CheckCircle2; label: string }> = {
  ACTIVE: { color: 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30', icon: CheckCircle2, label: 'Active' },
  DRAFT: { color: 'text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/30', icon: Clock, label: 'Draft' },
  INACTIVE: { color: 'text-neutral-500 bg-neutral-100 dark:bg-neutral-800', icon: XCircle, label: 'Inactive' },
};

export default function AgentsListPage() {
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    api<AgentProfile[]>('/agents')
      .then((data) => setAgents(data ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-neutral-900 dark:text-white">Agents</h1>
          <p className="text-sm text-neutral-500 mt-0.5">
            {agents.length} agent{agents.length !== 1 ? 's' : ''} registered
          </p>
        </div>
        <Link
          href="/dashboard/agents/new"
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Register Agent
        </Link>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {/* Table */}
      {agents.length === 0 && !error ? (
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-12 text-center">
          <Bot className="w-10 h-10 text-neutral-300 dark:text-neutral-600 mx-auto mb-3" />
          <p className="text-neutral-500 text-sm mb-4">No agents yet. Register your first agent to get started.</p>
          <Link
            href="/dashboard/agents/new"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 transition-colors"
          >
            <Plus className="w-4 h-4" />
            Register Agent
          </Link>
        </div>
      ) : (
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-neutral-100 dark:border-neutral-800 text-left text-xs text-neutral-500 uppercase tracking-wider">
                  <th className="px-5 py-3 font-medium">Name</th>
                  <th className="px-5 py-3 font-medium">Capabilities</th>
                  <th className="px-5 py-3 font-medium">Status</th>
                  <th className="px-5 py-3 font-medium text-right">Reputation</th>
                  <th className="px-5 py-3 font-medium text-right">Total Tasks</th>
                  <th className="px-5 py-3 font-medium text-right">Price</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
                {agents.map((agent) => {
                  const st = STATUS_CONFIG[agent.status] ?? STATUS_CONFIG.DRAFT;
                  const StIcon = st.icon;
                  return (
                    <tr
                      key={agent.id}
                      className="hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors"
                    >
                      <td className="px-5 py-3.5">
                        <Link
                          href={`/dashboard/agents/${agent.id}`}
                          className="font-medium text-neutral-900 dark:text-white hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                        >
                          {agent.name}
                        </Link>
                        <div className="text-xs text-neutral-400 mt-0.5 truncate max-w-[240px]">
                          {agent.description || 'No description'}
                        </div>
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="flex flex-wrap gap-1">
                          {agent.capabilities.map((cap) => (
                            <span
                              key={cap}
                              className="inline-block text-[11px] px-2 py-0.5 rounded-full bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400"
                            >
                              {cap}
                            </span>
                          ))}
                        </div>
                      </td>
                      <td className="px-5 py-3.5">
                        <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${st.color}`}>
                          <StIcon className="w-3 h-3" />
                          {st.label}
                        </span>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <div className="inline-flex items-center gap-1 text-neutral-700 dark:text-neutral-300">
                          <Star className="w-3 h-3 text-amber-500" />
                          {(agent.reputation_score ?? 0).toFixed(2)}
                        </div>
                      </td>
                      <td className="px-5 py-3.5 text-right font-mono text-neutral-700 dark:text-neutral-300">
                        {agent.total_jobs ?? 0}
                      </td>
                      <td className="px-5 py-3.5 text-right font-mono text-neutral-700 dark:text-neutral-300">
                        {agent.base_price_cents}
                        <span className="text-neutral-400 text-xs ml-0.5">¢</span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
