'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import {
  AlertTriangle,
  Loader2,
  Power,
  Save,
  Shield,
  Sliders,
} from 'lucide-react';

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function SettingsPage() {
  const [maxPerTask, setMaxPerTask] = useState('');
  const [maxPerDay, setMaxPerDay] = useState('');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const [killSwitchActive, setKillSwitchActive] = useState(false);
  const [killingAll, setKillingAll] = useState(false);
  const [killConfirm, setKillConfirm] = useState(false);

  useEffect(() => {
    api<{ global_max_per_task: number | null; global_max_per_day: number | null }>('/account/me')
      .then((d) => {
        setMaxPerTask(d.global_max_per_task != null ? String(d.global_max_per_task) : '');
        setMaxPerDay(d.global_max_per_day != null ? String(d.global_max_per_day) : '');
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  async function handleSave() {
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      await api('/account/settings', {
        method: 'PATCH',
        body: JSON.stringify({
          global_max_per_task: maxPerTask ? Number(maxPerTask) : null,
          global_max_per_day: maxPerDay ? Number(maxPerDay) : null,
        }),
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  }

  async function handleKillSwitch() {
    if (!killConfirm) {
      setKillConfirm(true);
      return;
    }
    setKillingAll(true);
    setError('');
    try {
      await api('/agents/kill-all', { method: 'POST' });
      setKillSwitchActive(true);
      setKillConfirm(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to pause agents');
    } finally {
      setKillingAll(false);
    }
  }

  async function handleResume() {
    setKillingAll(true);
    setError('');
    try {
      await api('/agents/resume-all', { method: 'POST' });
      setKillSwitchActive(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resume agents');
    } finally {
      setKillingAll(false);
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
    <div className="max-w-2xl mx-auto space-y-8">
      <div>
        <h1 className="text-xl font-bold text-neutral-900 dark:text-white">Settings</h1>
        <p className="text-sm text-neutral-500 mt-0.5">Configure spending controls and safety switches.</p>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {/* ---- Spending Controls ---- */}
      <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-6 space-y-6">
        <div className="flex items-center gap-2">
          <Sliders className="w-4 h-4 text-neutral-500" />
          <h2 className="font-semibold text-neutral-900 dark:text-white">Spending Controls</h2>
        </div>

        <div className="grid md:grid-cols-2 gap-5">
          {/* Max per task */}
          <div>
            <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
              Max credits per task
            </label>
            <input
              type="number"
              min={0}
              value={maxPerTask}
              onChange={(e) => setMaxPerTask(e.target.value)}
              className="input-base"
              placeholder="No limit"
            />
            <p className="text-xs text-neutral-400 mt-1">
              Reject any task whose budget exceeds this amount. Leave empty for no limit.
            </p>
          </div>

          {/* Max per day */}
          <div>
            <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
              Max credits per day
            </label>
            <input
              type="number"
              min={0}
              value={maxPerDay}
              onChange={(e) => setMaxPerDay(e.target.value)}
              className="input-base"
              placeholder="No limit"
            />
            <p className="text-xs text-neutral-400 mt-1">
              Pause spending after this daily total is reached. Leave empty for no limit.
            </p>
          </div>
        </div>

        <button
          onClick={handleSave}
          disabled={saving}
          className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {saving ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : saved ? (
            <Shield className="w-4 h-4" />
          ) : (
            <Save className="w-4 h-4" />
          )}
          {saved ? 'Saved!' : 'Save Controls'}
        </button>
      </div>

      {/* ---- Kill Switch ---- */}
      <div className={`rounded-xl border p-6 space-y-4 transition-colors ${
        killSwitchActive
          ? 'border-red-300 dark:border-red-700 bg-red-50 dark:bg-red-900/10'
          : 'border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900'
      }`}>
        <div className="flex items-center gap-2">
          <Power className={`w-4 h-4 ${killSwitchActive ? 'text-red-500' : 'text-neutral-500'}`} />
          <h2 className="font-semibold text-neutral-900 dark:text-white">Emergency Kill Switch</h2>
        </div>

        <p className="text-sm text-neutral-600 dark:text-neutral-400">
          Immediately pause <strong>all</strong> your agents. In-progress tasks will finish but no new tasks will be dispatched.
        </p>

        {killSwitchActive ? (
          <div className="space-y-3">
            <div className="flex items-start gap-2 p-3 rounded-lg bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 text-sm">
              <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
              All agents are currently <strong>paused</strong>. No new tasks will be accepted.
            </div>
            <button
              onClick={handleResume}
              disabled={killingAll}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-green-600 text-white text-sm font-medium hover:bg-green-700 disabled:opacity-50 transition-colors"
            >
              {killingAll ? <Loader2 className="w-4 h-4 animate-spin" /> : <Power className="w-4 h-4" />}
              Resume All Agents
            </button>
          </div>
        ) : (
          <div>
            {killConfirm && (
              <div className="flex items-start gap-2 p-3 mb-3 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300 text-sm">
                <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
                Are you sure? This will set all agents to <strong>offline</strong> immediately.
              </div>
            )}
            <div className="flex items-center gap-3">
              <button
                onClick={handleKillSwitch}
                disabled={killingAll}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-red-600 text-white text-sm font-medium hover:bg-red-700 disabled:opacity-50 transition-colors"
              >
                {killingAll ? <Loader2 className="w-4 h-4 animate-spin" /> : <Power className="w-4 h-4" />}
                {killConfirm ? 'Confirm — Pause All' : 'Pause All Agents'}
              </button>
              {killConfirm && (
                <button
                  onClick={() => setKillConfirm(false)}
                  className="text-sm text-neutral-500 hover:text-neutral-900 dark:hover:text-white transition-colors"
                >
                  Cancel
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
