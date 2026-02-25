'use client';

import Link from 'next/link';
import { useParams, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { api } from '@/lib/api';
import { AgentProfile } from '@/lib/types';
import {
  ArrowLeft,
  Bot,
  CheckCircle2,
  Clock,
  Edit3,
  Loader2,
  Plus,
  Save,
  Star,
  TrendingUp,
  X,
  XCircle,
  Zap,
} from 'lucide-react';

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface EditFormValues {
  base_price_cents: number;
  capabilities: { value: string }[];
  webhook_url: string;
  description: string;
}

const AVAILABLE_CAPABILITIES = [
  'research',
  'summarize',
  'data extraction',
  'translation',
  'code review',
  'classification',
];

const STATUS_CONFIG: Record<string, { color: string; icon: typeof CheckCircle2; label: string }> = {
  ACTIVE: { color: 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30', icon: CheckCircle2, label: 'Active' },
  DRAFT: { color: 'text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/30', icon: Clock, label: 'Draft' },
  INACTIVE: { color: 'text-neutral-500 bg-neutral-100 dark:bg-neutral-800', icon: XCircle, label: 'Inactive' },
};

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function AgentDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const agentId = params.id;

  const [agent, setAgent] = useState<AgentProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState(false);
  const [saveError, setSaveError] = useState('');

  useEffect(() => {
    api<AgentProfile[]>('/agents')
      .then((list) => {
        const found = (list ?? []).find((a) => a.id === agentId);
        if (found) {
          setAgent(found);
        } else {
          setError('Agent not found');
        }
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [agentId]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />
      </div>
    );
  }

  if (error || !agent) {
    return (
      <div className="max-w-2xl mx-auto space-y-4">
        <Link href="/dashboard/agents" className="inline-flex items-center gap-1.5 text-sm text-neutral-500 hover:text-neutral-900 dark:hover:text-white transition-colors">
          <ArrowLeft className="w-4 h-4" /> Back to Agents
        </Link>
        <div className="p-6 rounded-xl border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-sm text-center">
          {error || 'Agent not found'}
        </div>
      </div>
    );
  }

  const st = STATUS_CONFIG[agent.status] ?? STATUS_CONFIG.DRAFT;
  const StIcon = st.icon;
  const reputationPct = Math.min((agent.reputation_score ?? 0) * 100, 100);
  const successRate = agent.total_jobs > 0 ? Math.round(reputationPct) : 0;
  const uptime = agent.status === 'ACTIVE' ? 99.9 : 0;

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      {/* Back link */}
      <Link
        href="/dashboard/agents"
        className="inline-flex items-center gap-1.5 text-sm text-neutral-500 hover:text-neutral-900 dark:hover:text-white transition-colors"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Agents
      </Link>

      {/* Header */}
      <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-6">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-blue-100 dark:bg-blue-900/40 flex items-center justify-center shrink-0">
              <Bot className="w-6 h-6 text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-neutral-900 dark:text-white">{agent.name}</h1>
              <p className="text-sm text-neutral-500 mt-0.5">{agent.description || 'No description'}</p>
              <div className="flex flex-wrap items-center gap-2 mt-3">
                <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${st.color}`}>
                  <StIcon className="w-3 h-3" />
                  {st.label}
                </span>
                {agent.capabilities.map((cap) => (
                  <span
                    key={cap}
                    className="text-[11px] px-2 py-0.5 rounded-full bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400"
                  >
                    {cap}
                  </span>
                ))}
              </div>
            </div>
          </div>
          {!editing && (
            <button
              onClick={() => setEditing(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-neutral-200 dark:border-neutral-700 text-sm text-neutral-600 dark:text-neutral-400 hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors"
            >
              <Edit3 className="w-3.5 h-3.5" />
              Edit
            </button>
          )}
        </div>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Uptime" value={`${uptime}%`} icon={Zap} color="text-green-500" />
        <StatCard label="Success Rate" value={`${successRate}%`} icon={TrendingUp} color="text-blue-500" />
        <StatCard label="Reputation" value={(agent.reputation_score ?? 0).toFixed(2)} icon={Star} color="text-amber-500" />
        <StatCard label="Total Tasks" value={String(agent.total_jobs ?? 0)} icon={CheckCircle2} color="text-violet-500" />
      </div>

      {/* Details / Edit form */}
      {editing ? (
        <EditPanel
          agent={agent}
          onSaved={(updated) => {
            setAgent(updated);
            setEditing(false);
          }}
          onCancel={() => setEditing(false)}
          saveError={saveError}
          setSaveError={setSaveError}
        />
      ) : (
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 space-y-4">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Details</h2>
          <DetailRow label="ID" value={agent.id} mono />
          <DetailRow label="Endpoint" value={agent.webhook_url || '—'} mono />
          <DetailRow label="Price" value={`${agent.base_price_cents}¢ per task`} />
          <DetailRow label="Input Schema">
            <pre className="text-xs font-mono bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-lg p-3 mt-1 overflow-x-auto text-neutral-700 dark:text-neutral-300">
              {JSON.stringify(agent.input_schema, null, 2)}
            </pre>
          </DetailRow>
          <DetailRow label="Output Schema">
            <pre className="text-xs font-mono bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-lg p-3 mt-1 overflow-x-auto text-neutral-700 dark:text-neutral-300">
              {JSON.stringify(agent.output_schema, null, 2)}
            </pre>
          </DetailRow>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function StatCard({
  label,
  value,
  icon: Icon,
  color,
}: {
  label: string;
  value: string;
  icon: typeof Zap;
  color: string;
}) {
  return (
    <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-4">
      <div className="flex items-center gap-2 mb-2">
        <Icon className={`w-4 h-4 ${color}`} />
        <span className="text-xs text-neutral-500">{label}</span>
      </div>
      <div className="text-xl font-bold text-neutral-900 dark:text-white">{value}</div>
    </div>
  );
}

function DetailRow({
  label,
  value,
  mono,
  children,
}: {
  label: string;
  value?: string;
  mono?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <div>
      <span className="text-xs text-neutral-500 block">{label}</span>
      {children ?? (
        <span className={`text-sm text-neutral-800 dark:text-neutral-200 ${mono ? 'font-mono' : ''}`}>
          {value}
        </span>
      )}
    </div>
  );
}

function EditPanel({
  agent,
  onSaved,
  onCancel,
  saveError,
  setSaveError,
}: {
  agent: AgentProfile;
  onSaved: (a: AgentProfile) => void;
  onCancel: () => void;
  saveError: string;
  setSaveError: (s: string) => void;
}) {
  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isSubmitting },
    watch,
  } = useForm<EditFormValues>({
    defaultValues: {
      base_price_cents: agent.base_price_cents,
      capabilities: agent.capabilities.map((c) => ({ value: c })),
      webhook_url: agent.webhook_url || '',
      description: agent.description,
    },
  });

  const { fields, append, remove } = useFieldArray({ control, name: 'capabilities' });
  const selectedCaps = watch('capabilities');
  const usedCaps = new Set(selectedCaps.map((c) => c.value));

  async function onSubmit(data: EditFormValues) {
    setSaveError('');
    try {
      const updated = await api<AgentProfile>('/agents', {
        method: 'POST',
        body: JSON.stringify({
          name: agent.name,
          description: data.description,
          capabilities: data.capabilities.map((c) => c.value),
          base_price_cents: Number(data.base_price_cents),
          webhook_url: data.webhook_url,
          input_schema: agent.input_schema,
          output_schema: agent.output_schema,
        }),
      });
      onSaved(updated);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save');
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
      <div className="rounded-xl border border-blue-200 dark:border-blue-800 bg-blue-50/50 dark:bg-blue-900/10 p-5 space-y-5">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Edit Agent</h2>
          <button
            type="button"
            onClick={onCancel}
            className="text-sm text-neutral-500 hover:text-neutral-900 dark:hover:text-white transition-colors"
          >
            Cancel
          </button>
        </div>

        {saveError && (
          <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
            {saveError}
          </div>
        )}

        {/* Description */}
        <div>
          <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
            Description
          </label>
          <textarea
            rows={2}
            {...register('description')}
            className="input-base"
          />
        </div>

        {/* Endpoint */}
        <div>
          <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
            Endpoint URL
          </label>
          <input
            {...register('webhook_url', {
              required: 'Endpoint URL is required',
              pattern: { value: /^https?:\/\/.+/, message: 'Must be a valid URL' },
            })}
            className="input-base"
          />
          {errors.webhook_url && <p className="text-xs text-red-600 mt-1">{errors.webhook_url.message}</p>}
        </div>

        {/* Price */}
        <div>
          <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
            Price per Task (cents)
          </label>
          <input
            type="number"
            min={0}
            {...register('base_price_cents', { valueAsNumber: true, min: 0 })}
            className="input-base w-36"
          />
        </div>

        {/* Capabilities */}
        <div>
          <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
            Capabilities
          </label>
          <div className="space-y-2">
            {fields.map((field, idx) => (
              <div key={field.id} className="flex items-center gap-2">
                <select
                  {...register(`capabilities.${idx}.value` as const)}
                  className="input-base flex-1"
                >
                  {AVAILABLE_CAPABILITIES.map((cap) => (
                    <option key={cap} value={cap} disabled={usedCaps.has(cap) && selectedCaps[idx]?.value !== cap}>
                      {cap}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={() => fields.length > 1 && remove(idx)}
                  className="p-2 text-neutral-400 hover:text-red-500 transition-colors"
                  disabled={fields.length <= 1}
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            ))}
            {fields.length < AVAILABLE_CAPABILITIES.length && (
              <button
                type="button"
                onClick={() => {
                  const next = AVAILABLE_CAPABILITIES.find((c) => !usedCaps.has(c));
                  if (next) append({ value: next });
                }}
                className="inline-flex items-center gap-1 text-sm text-blue-600 dark:text-blue-400 hover:underline"
              >
                <Plus className="w-3.5 h-3.5" />
                Add capability
              </button>
            )}
          </div>
        </div>
      </div>

      <button
        type="submit"
        disabled={isSubmitting}
        className="flex items-center justify-center gap-2 w-full py-3 rounded-xl bg-blue-600 text-white font-semibold text-sm hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {isSubmitting ? (
          <>
            <Loader2 className="w-4 h-4 animate-spin" />
            Saving…
          </>
        ) : (
          <>
            <Save className="w-4 h-4" />
            Save Changes
          </>
        )}
      </button>
    </form>
  );
}
