'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { CreditLedgerEntry } from '@/lib/types';
import {
  ArrowDownRight,
  ArrowUpRight,
  Coins,
  CreditCard,
  ExternalLink,
  Loader2,
  Receipt,
  Wallet,
} from 'lucide-react';

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

const ENTRY_CONFIG: Record<string, { label: string; icon: typeof ArrowUpRight; color: string; sign: '+' | '-' | '' }> = {
  escrow_lock:    { label: 'Escrow Lock',    icon: ArrowDownRight, color: 'text-amber-600 dark:text-amber-400', sign: '-' },
  escrow_release: { label: 'Escrow Release', icon: ArrowUpRight,  color: 'text-green-600 dark:text-green-400',  sign: '+' },
  task_earning:   { label: 'Task Earning',   icon: ArrowUpRight,  color: 'text-green-600 dark:text-green-400',  sign: '+' },
  platform_fee:   { label: 'Platform Fee',   icon: ArrowDownRight, color: 'text-red-500',                       sign: '-' },
  refund:         { label: 'Refund',         icon: ArrowUpRight,  color: 'text-blue-600 dark:text-blue-400',    sign: '+' },
};

const CREDIT_PACKS = [
  { credits: 100,  price: '$5',   popular: false },
  { credits: 500,  price: '$20',  popular: true },
  { credits: 2000, price: '$70',  popular: false },
];

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function BillingPage() {
  const [balance, setBalance] = useState<number | null>(null);
  const [entries, setEntries] = useState<CreditLedgerEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    Promise.allSettled([
      api<{ credit_balance: number }>('/account/me').then((d) => setBalance(d.credit_balance)),
      api<CreditLedgerEntry[]>('/credit-ledger').then((d) => setEntries(d ?? [])),
    ])
      .catch(() => {})
      .finally(() => setLoading(false));

    // Fallback: read cached credit balance from localStorage
    try {
      const raw = localStorage.getItem('inaiurai-credits');
      if (raw) setBalance((prev) => prev ?? parseInt(raw, 10));
    } catch { /* noop */ }
  }, []);

  if (loading && balance === null) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-8">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-neutral-900 dark:text-white">Billing</h1>
        <p className="text-sm text-neutral-500 mt-0.5">Manage your credit balance and review transaction history.</p>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {/* Balance + Buy credits */}
      <div className="grid md:grid-cols-3 gap-6">
        {/* Big balance card */}
        <div className="md:col-span-1 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-6 flex flex-col items-center justify-center text-center">
          <div className="w-14 h-14 rounded-2xl bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center mb-4">
            <Wallet className="w-7 h-7 text-amber-600 dark:text-amber-400" />
          </div>
          <p className="text-xs text-neutral-500 uppercase tracking-wider font-medium mb-1">Credit Balance</p>
          <p className="text-4xl font-bold text-neutral-900 dark:text-white tabular-nums">
            {balance !== null ? balance.toLocaleString() : '—'}
          </p>
          <p className="text-sm text-neutral-400 mt-1">credits available</p>
        </div>

        {/* Buy credits */}
        <div className="md:col-span-2 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-6">
          <div className="flex items-center gap-2 mb-4">
            <CreditCard className="w-4 h-4 text-neutral-500" />
            <h2 className="font-semibold text-neutral-900 dark:text-white">Buy Credits</h2>
          </div>
          <div className="grid grid-cols-3 gap-3">
            {CREDIT_PACKS.map((pack) => (
              <button
                key={pack.credits}
                onClick={() => {
                  setError('Stripe Checkout integration coming soon.');
                }}
                className={`relative rounded-xl border p-4 text-center transition-all hover:border-blue-400 dark:hover:border-blue-600 ${
                  pack.popular
                    ? 'border-blue-500 bg-blue-50/50 dark:bg-blue-900/10 ring-1 ring-blue-500'
                    : 'border-neutral-200 dark:border-neutral-700 bg-white dark:bg-neutral-900'
                }`}
              >
                {pack.popular && (
                  <span className="absolute -top-2.5 left-1/2 -translate-x-1/2 text-[10px] px-2 py-0.5 rounded-full bg-blue-600 text-white font-medium">
                    Popular
                  </span>
                )}
                <div className="flex items-center justify-center gap-1 mb-1">
                  <Coins className="w-4 h-4 text-amber-500" />
                  <span className="text-lg font-bold text-neutral-900 dark:text-white">{pack.credits.toLocaleString()}</span>
                </div>
                <p className="text-sm text-neutral-500">credits</p>
                <p className="text-lg font-semibold text-neutral-900 dark:text-white mt-2">{pack.price}</p>
                <div className="flex items-center justify-center gap-1 mt-2 text-xs text-blue-600 dark:text-blue-400">
                  <ExternalLink className="w-3 h-3" />
                  Checkout
                </div>
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Transaction history */}
      <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 overflow-hidden">
        <div className="flex items-center gap-2 px-5 py-4 border-b border-neutral-100 dark:border-neutral-800">
          <Receipt className="w-4 h-4 text-neutral-500" />
          <h2 className="font-semibold text-neutral-900 dark:text-white">Transaction History</h2>
          <span className="ml-auto text-xs text-neutral-400">{entries.length} entries</span>
        </div>

        {entries.length === 0 ? (
          <div className="px-5 py-10 text-center text-sm text-neutral-400">
            No transactions yet. Use the Try It page to create your first task.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs text-neutral-500 uppercase tracking-wider border-b border-neutral-100 dark:border-neutral-800">
                  <th className="px-5 py-2.5 font-medium">Type</th>
                  <th className="px-5 py-2.5 font-medium">Task</th>
                  <th className="px-5 py-2.5 font-medium text-right">Amount</th>
                  <th className="px-5 py-2.5 font-medium text-right">Balance After</th>
                  <th className="px-5 py-2.5 font-medium text-right">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
                {entries.map((e) => {
                  const cfg = ENTRY_CONFIG[e.entry_type] ?? { label: e.entry_type, icon: ArrowUpRight, color: 'text-neutral-500', sign: '' };
                  const Icon = cfg.icon;
                  return (
                    <tr key={e.id} className="hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors">
                      <td className="px-5 py-3">
                        <div className="flex items-center gap-2">
                          <Icon className={`w-3.5 h-3.5 ${cfg.color}`} />
                          <span className="text-neutral-800 dark:text-neutral-200">{cfg.label}</span>
                        </div>
                      </td>
                      <td className="px-5 py-3 font-mono text-xs text-neutral-400 truncate max-w-[140px]">
                        {e.task_id ? e.task_id.slice(0, 8) + '…' : '—'}
                      </td>
                      <td className={`px-5 py-3 text-right font-mono font-medium ${
                        cfg.sign === '+' ? 'text-green-600 dark:text-green-400' : cfg.sign === '-' ? 'text-red-500' : 'text-neutral-600'
                      }`}>
                        {cfg.sign}{e.amount}
                      </td>
                      <td className="px-5 py-3 text-right font-mono text-neutral-500">
                        {e.balance_after !== null ? e.balance_after.toLocaleString() : '—'}
                      </td>
                      <td className="px-5 py-3 text-right text-neutral-400 whitespace-nowrap">
                        {new Date(e.created_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
