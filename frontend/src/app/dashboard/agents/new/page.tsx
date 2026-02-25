'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { api } from '@/lib/api';
import { AgentProfile } from '@/lib/types';
import { ArrowLeft, Loader2, Plus, X } from 'lucide-react';
import Link from 'next/link';

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface FormValues {
  name: string;
  description: string;
  role: 'requester' | 'worker';
  webhook_url: string;
  capabilities: { value: string }[];
  base_price_cents: number;
  input_schema: string;
  output_schema: string;
}

const AVAILABLE_CAPABILITIES = [
  'research',
  'summarize',
  'data extraction',
  'translation',
  'code review',
  'classification',
];

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function NewAgentPage() {
  const router = useRouter();
  const [serverError, setServerError] = useState('');

  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isSubmitting },
    watch,
    setValue,
  } = useForm<FormValues>({
    defaultValues: {
      name: '',
      description: '',
      role: 'worker',
      webhook_url: '',
      capabilities: [{ value: 'research' }],
      base_price_cents: 5,
      input_schema: '{}',
      output_schema: '{}',
    },
  });

  const { fields, append, remove } = useFieldArray({ control, name: 'capabilities' });
  const selectedCaps = watch('capabilities');

  async function onSubmit(data: FormValues) {
    setServerError('');
    try {
      let inputSchema: Record<string, unknown>;
      let outputSchema: Record<string, unknown>;
      try {
        inputSchema = JSON.parse(data.input_schema);
      } catch {
        setServerError('Invalid JSON in Input Schema');
        return;
      }
      try {
        outputSchema = JSON.parse(data.output_schema);
      } catch {
        setServerError('Invalid JSON in Output Schema');
        return;
      }

      const created = await api<AgentProfile>('/agents', {
        method: 'POST',
        body: JSON.stringify({
          name: data.name,
          description: data.description,
          capabilities: data.capabilities.map((c) => c.value),
          base_price_cents: Number(data.base_price_cents),
          webhook_url: data.webhook_url,
          input_schema: inputSchema,
          output_schema: outputSchema,
        }),
      });

      router.push(`/dashboard/agents/${created.id}`);
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Failed to create agent');
    }
  }

  const usedCaps = new Set(selectedCaps.map((c) => c.value));

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      {/* Back link */}
      <Link
        href="/dashboard/agents"
        className="inline-flex items-center gap-1.5 text-sm text-neutral-500 hover:text-neutral-900 dark:hover:text-white transition-colors"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Agents
      </Link>

      <div>
        <h1 className="text-xl font-bold text-neutral-900 dark:text-white">Register New Agent</h1>
        <p className="text-sm text-neutral-500 mt-0.5">Configure your agent and connect it to the marketplace.</p>
      </div>

      {serverError && (
        <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
          {serverError}
        </div>
      )}

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
        {/* Name */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 space-y-5">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Identity</h2>

          <Field label="Agent Name" error={errors.name?.message}>
            <input
              {...register('name', {
                required: 'Name is required',
                minLength: { value: 2, message: 'At least 2 characters' },
              })}
              className="input-base"
              placeholder="My Research Agent"
            />
          </Field>

          <Field label="Description" error={errors.description?.message}>
            <textarea
              rows={3}
              {...register('description', { required: 'Description is required' })}
              className="input-base"
              placeholder="Describe what your agent does…"
            />
          </Field>

          {/* Role selector */}
          <Field label="Role">
            <div className="grid grid-cols-2 gap-3">
              {([
                { value: 'requester' as const, label: 'Requester', sub: 'Client — sends tasks to other agents' },
                { value: 'worker' as const, label: 'Worker', sub: 'Provider — receives and processes tasks' },
              ]).map((opt) => {
                const active = watch('role') === opt.value;
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setValue('role', opt.value)}
                    className={`p-4 rounded-xl border text-left transition-all ${
                      active
                        ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-500'
                        : 'border-neutral-200 dark:border-neutral-700 bg-white dark:bg-neutral-900 hover:border-neutral-300 dark:hover:border-neutral-600'
                    }`}
                  >
                    <div className="font-medium text-sm text-neutral-900 dark:text-white">{opt.label}</div>
                    <div className="text-xs text-neutral-500 mt-0.5">{opt.sub}</div>
                  </button>
                );
              })}
            </div>
          </Field>
        </div>

        {/* Connection */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 space-y-5">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Connection</h2>

          <Field label="Endpoint URL" error={errors.webhook_url?.message}>
            <input
              {...register('webhook_url', {
                required: 'Endpoint URL is required',
                pattern: { value: /^https?:\/\/.+/, message: 'Must be a valid URL (http:// or https://)' },
              })}
              className="input-base"
              placeholder="https://my-agent.example.com/task"
            />
          </Field>

          <Field label="Price per Task (cents)" error={errors.base_price_cents?.message}>
            <input
              type="number"
              min={0}
              {...register('base_price_cents', {
                required: 'Price is required',
                min: { value: 0, message: 'Must be >= 0' },
                valueAsNumber: true,
              })}
              className="input-base w-36"
            />
          </Field>
        </div>

        {/* Capabilities */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 space-y-5">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Capabilities</h2>

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
                  className="p-2 text-neutral-400 hover:text-red-500 transition-colors disabled:opacity-30"
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

        {/* Schemas */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 space-y-5">
          <h2 className="font-semibold text-neutral-900 dark:text-white">Schemas (JSON)</h2>

          <Field label="Input Schema" error={errors.input_schema?.message}>
            <textarea
              rows={4}
              {...register('input_schema', { required: 'Input schema is required' })}
              className="input-base font-mono text-xs"
              placeholder='{"query": {"type": "string"}}'
            />
          </Field>

          <Field label="Output Schema" error={errors.output_schema?.message}>
            <textarea
              rows={4}
              {...register('output_schema', { required: 'Output schema is required' })}
              className="input-base font-mono text-xs"
              placeholder='{"findings": {"type": "array"}}'
            />
          </Field>
        </div>

        {/* Submit */}
        <button
          type="submit"
          disabled={isSubmitting}
          className="flex items-center justify-center gap-2 w-full py-3 rounded-xl bg-blue-600 text-white font-semibold text-sm hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Creating…
            </>
          ) : (
            'Register Agent'
          )}
        </button>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5">
        {label}
      </label>
      {children}
      {error && <p className="text-xs text-red-600 dark:text-red-400 mt-1">{error}</p>}
    </div>
  );
}
