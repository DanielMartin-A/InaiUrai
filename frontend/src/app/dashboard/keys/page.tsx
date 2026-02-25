'use client';

import { useCallback, useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { APIKey } from '@/lib/types';
import {
  Copy,
  Check,
  Eye,
  EyeOff,
  KeyRound,
  Loader2,
  Plus,
  ShieldAlert,
  Trash2,
} from 'lucide-react';

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  /* The freshly-created raw key — shown once then gone forever. */
  const [newRawKey, setNewRawKey] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);

  const fetchKeys = useCallback(async () => {
    try {
      const data = await api<APIKey[]>('/api-keys');
      setKeys(data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load API keys');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  async function handleCreate() {
    setCreating(true);
    setNewRawKey(null);
    setError('');
    try {
      const created = await api<APIKey>('/api-keys', { method: 'POST' });
      if (created.raw_key) setNewRawKey(created.raw_key);
      await fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create key');
    } finally {
      setCreating(false);
    }
  }

  async function handleRevoke(id: string) {
    setRevoking(id);
    setError('');
    try {
      await api(`/api-keys/${id}`, { method: 'DELETE' });
      setKeys((prev) => prev.filter((k) => k.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke key');
    } finally {
      setRevoking(null);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-neutral-900 dark:text-white">API Keys</h1>
          <p className="text-sm text-neutral-500 mt-0.5">
            Manage keys used to authenticate requests to the INAIURAI API.
          </p>
        </div>
        <button
          onClick={handleCreate}
          disabled={creating}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {creating ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
          Create New Key
        </button>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {/* Newly-created raw key banner */}
      {newRawKey && (
        <NewKeyBanner rawKey={newRawKey} onDismiss={() => setNewRawKey(null)} />
      )}

      {/* Keys list */}
      {keys.length === 0 ? (
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-12 text-center">
          <KeyRound className="w-10 h-10 text-neutral-300 dark:text-neutral-600 mx-auto mb-3" />
          <p className="text-neutral-500 text-sm">No API keys yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {keys.map((k) => (
            <div
              key={k.id}
              className="flex items-center justify-between gap-4 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 px-5 py-4"
            >
              <div className="flex items-center gap-3 min-w-0">
                <div className={`w-9 h-9 rounded-lg flex items-center justify-center shrink-0 ${
                  k.is_active
                    ? 'bg-green-100 dark:bg-green-900/30'
                    : 'bg-neutral-100 dark:bg-neutral-800'
                }`}>
                  <KeyRound className={`w-4 h-4 ${k.is_active ? 'text-green-600 dark:text-green-400' : 'text-neutral-400'}`} />
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm text-neutral-900 dark:text-white">
                      {k.key_prefix}••••••••••••
                    </span>
                    {k.is_active ? (
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 font-medium">
                        Active
                      </span>
                    ) : (
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-neutral-100 dark:bg-neutral-800 text-neutral-500 font-medium">
                        Revoked
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-neutral-400 mt-0.5 font-mono truncate">ID: {k.id}</p>
                </div>
              </div>

              {k.is_active && (
                <button
                  onClick={() => handleRevoke(k.id)}
                  disabled={revoking === k.id}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm text-red-600 dark:text-red-400 border border-red-200 dark:border-red-800 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50 transition-colors shrink-0"
                >
                  {revoking === k.id ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Trash2 className="w-3.5 h-3.5" />
                  )}
                  Revoke
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Security notice */}
      <div className="flex items-start gap-3 p-4 rounded-xl bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-800 text-sm">
        <ShieldAlert className="w-5 h-5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5" />
        <div>
          <p className="font-medium text-amber-800 dark:text-amber-200">Keep your keys safe</p>
          <p className="text-amber-700 dark:text-amber-300 mt-0.5">
            API keys grant full access to your account. Never share them publicly or commit them to source control.
            If a key is compromised, revoke it immediately and create a new one.
          </p>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  New Key Banner — shown once after creation                         */
/* ------------------------------------------------------------------ */

function NewKeyBanner({ rawKey, onDismiss }: { rawKey: string; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false);
  const [visible, setVisible] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(rawKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* clipboard not available */ }
  }

  return (
    <div className="rounded-xl border-2 border-green-300 dark:border-green-700 bg-green-50 dark:bg-green-900/20 p-5">
      <div className="flex items-center justify-between mb-2">
        <h3 className="font-semibold text-green-800 dark:text-green-200 text-sm">New API Key Created</h3>
        <button onClick={onDismiss} className="text-xs text-green-600 dark:text-green-400 hover:underline">
          Dismiss
        </button>
      </div>
      <p className="text-xs text-green-700 dark:text-green-300 mb-3">
        Copy this key now. For security, it will not be shown again.
      </p>
      <div className="flex items-center gap-2">
        <div className="flex-1 bg-white dark:bg-neutral-950 border border-green-200 dark:border-green-800 rounded-lg px-3 py-2 font-mono text-sm text-neutral-900 dark:text-neutral-100 truncate">
          {visible ? rawKey : rawKey.slice(0, 8) + '•'.repeat(Math.max(rawKey.length - 8, 12))}
        </div>
        <button
          onClick={() => setVisible(!visible)}
          className="p-2 rounded-lg border border-neutral-200 dark:border-neutral-700 hover:bg-neutral-50 dark:hover:bg-neutral-800 text-neutral-600 dark:text-neutral-400 transition-colors"
          title={visible ? 'Hide' : 'Reveal'}
        >
          {visible ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
        </button>
        <button
          onClick={handleCopy}
          className="p-2 rounded-lg border border-neutral-200 dark:border-neutral-700 hover:bg-neutral-50 dark:hover:bg-neutral-800 text-neutral-600 dark:text-neutral-400 transition-colors"
          title="Copy"
        >
          {copied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
        </button>
      </div>
    </div>
  );
}
