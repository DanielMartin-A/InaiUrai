'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useAuthStore } from '@/lib/store';
import {
  Search,
  FileText,
  Database,
  Play,
  Loader2,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Coins,
} from 'lucide-react';

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface TaskResponse {
  task_id: string;
  status: string;
}

interface TaskPoll {
  id: string;
  status: string;
  output_status: string;
  output_payload: Record<string, unknown> | null;
  capability_required: string;
  budget: number;
  worker_agent_id: string | null;
  created_at: string;
}

interface CapabilityDef {
  key: string;
  label: string;
  icon: typeof Search;
  price: number;
  description: string;
}

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

const API_ORIGIN =
  typeof window !== 'undefined' && window.location.hostname === 'localhost'
    ? 'http://localhost:8080'
    : 'https://inaiurai-production.up.railway.app';

const CAPABILITIES: CapabilityDef[] = [
  { key: 'Research', label: 'Research', icon: Search, price: 8, description: 'Deep research with cited sources' },
  { key: 'Summarize', label: 'Summarize', icon: FileText, price: 3, description: 'Condense text into summaries' },
  { key: 'Data Extraction', label: 'Data Extraction', icon: Database, price: 5, description: 'Extract structured fields' },
];

const STEPPER_STATES = ['created', 'matching', 'dispatched', 'in_progress', 'completed'] as const;

const STEPPER_LABELS: Record<string, string> = {
  created: 'Validating',
  matching: 'Matching',
  dispatched: 'Dispatching',
  in_progress: 'Processing',
  completed: 'Done',
  failed: 'Failed',
};

const POLL_INTERVAL = 1500;

/* ------------------------------------------------------------------ */
/*  API helper (targets /v1/ routes, not /api/v1/)                     */
/* ------------------------------------------------------------------ */

async function v1<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_ORIGIN}/v1${path}`, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function TryItPage() {
  const [capability, setCapability] = useState<CapabilityDef>(CAPABILITIES[0]);

  // Research
  const [query, setQuery] = useState('Latest trends in autonomous AI agents');
  const [depth, setDepth] = useState('standard');
  const [maxSources, setMaxSources] = useState(5);

  // Summarize
  const [text, setText] = useState('');
  const [format, setFormat] = useState('paragraph');
  const [focus, setFocus] = useState('');

  // Extraction
  const [extractText, setExtractText] = useState('');
  const [fields, setFields] = useState([{ name: 'email', type: 'string', description: 'email address' }]);

  // Task lifecycle
  const [submitting, setSubmitting] = useState(false);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<TaskPoll | null>(null);
  const [error, setError] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  useEffect(() => () => stopPolling(), [stopPolling]);

  function buildPayload(): { input_payload: Record<string, unknown>; budget: number } {
    switch (capability.key) {
      case 'Research':
        return {
          input_payload: { query, depth, max_sources: maxSources },
          budget: capability.price,
        };
      case 'Summarize':
        return {
          input_payload: { text, format, focus: focus || undefined },
          budget: capability.price,
        };
      case 'Data Extraction':
        return {
          input_payload: { text: extractText, fields },
          budget: capability.price,
        };
      default:
        return { input_payload: {}, budget: capability.price };
    }
  }

  async function handleRun() {
    setError('');
    setTask(null);
    setTaskId(null);
    stopPolling();
    setSubmitting(true);

    try {
      const { input_payload, budget } = buildPayload();

      // We need a requester_agent_id. Fetch agent from middleware context —
      // the API middleware sets account+agent from key. We include a placeholder
      // that the backend handler reads from the authenticated agent if missing.
      const resp = await v1<TaskResponse>('/tasks', {
        method: 'POST',
        body: JSON.stringify({
          requester_agent_id: '00000000-0000-0000-0000-000000000000',
          capability_required: capability.key,
          input_payload,
          budget,
          routing_preference: 'auto',
        }),
      });

      setTaskId(resp.task_id);

      // Start polling
      pollRef.current = setInterval(async () => {
        try {
          const t = await v1<TaskPoll>(`/tasks/${resp.task_id}`);
          setTask(t);
          if (t.status === 'completed' || t.status === 'failed') {
            stopPolling();
          }
        } catch (pollErr) {
          // ignore transient poll errors
        }
      }, POLL_INTERVAL);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create task');
    } finally {
      setSubmitting(false);
    }
  }

  function addField() {
    setFields([...fields, { name: '', type: 'string', description: '' }]);
  }

  function updateField(idx: number, key: string, val: string) {
    setFields(fields.map((f, i) => (i === idx ? { ...f, [key]: val } : f)));
  }

  function removeField(idx: number) {
    if (fields.length > 1) setFields(fields.filter((_, i) => i !== idx));
  }

  function resetOutput() {
    setTask(null);
    setTaskId(null);
    setError('');
    stopPolling();
  }

  const currentStep = task?.status ?? (taskId ? 'created' : null);
  const isTerminal = currentStep === 'completed' || currentStep === 'failed';

  return (
    <div className="grid lg:grid-cols-2 gap-6 h-full">
      {/* =================== LEFT PANEL =================== */}
      <div className="flex flex-col gap-6 overflow-y-auto">
        {/* Capability Selector */}
        <div>
          <h2 className="text-lg font-semibold text-neutral-900 dark:text-white mb-3">Capability</h2>
          <div className="grid grid-cols-3 gap-3">
            {CAPABILITIES.map((cap) => {
              const active = capability.key === cap.key;
              return (
                <button
                  key={cap.key}
                  onClick={() => { setCapability(cap); resetOutput(); }}
                  className={`p-4 rounded-xl border text-left transition-all ${
                    active
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-500'
                      : 'border-neutral-200 dark:border-neutral-700 bg-white dark:bg-neutral-900 hover:border-neutral-300 dark:hover:border-neutral-600'
                  }`}
                >
                  <div className={`w-8 h-8 rounded-lg flex items-center justify-center mb-2 ${
                    active ? 'bg-blue-100 dark:bg-blue-900/40' : 'bg-neutral-100 dark:bg-neutral-800'
                  }`}>
                    <cap.icon className={`w-4 h-4 ${active ? 'text-blue-600 dark:text-blue-400' : 'text-neutral-500'}`} />
                  </div>
                  <div className="font-medium text-sm text-neutral-900 dark:text-white">{cap.label}</div>
                  <div className="text-xs text-neutral-500 mt-0.5">{cap.description}</div>
                  <div className="flex items-center gap-1 mt-2 text-xs font-medium text-amber-600 dark:text-amber-400">
                    <Coins className="w-3 h-3" />
                    {cap.price} credits
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Dynamic Form */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5">
          <h3 className="font-semibold text-neutral-900 dark:text-white mb-4">Input</h3>

          {capability.key === 'Research' && (
            <div className="space-y-4">
              <Field label="Query">
                <textarea
                  rows={3}
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="input-base"
                  placeholder="What do you want to research?"
                />
              </Field>
              <Field label="Depth">
                <div className="flex gap-2">
                  {(['quick', 'standard', 'deep'] as const).map((d) => (
                    <button
                      key={d}
                      onClick={() => setDepth(d)}
                      className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors ${
                        depth === d
                          ? 'bg-blue-600 text-white'
                          : 'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400 hover:bg-neutral-200 dark:hover:bg-neutral-700'
                      }`}
                    >
                      {d}
                    </button>
                  ))}
                </div>
              </Field>
              <Field label="Max Sources">
                <input
                  type="number"
                  min={1}
                  max={20}
                  value={maxSources}
                  onChange={(e) => setMaxSources(parseInt(e.target.value, 10) || 5)}
                  className="input-base w-24"
                />
              </Field>
            </div>
          )}

          {capability.key === 'Summarize' && (
            <div className="space-y-4">
              <Field label="Text">
                <textarea
                  rows={6}
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                  className="input-base"
                  placeholder="Paste the text you want summarized…"
                />
              </Field>
              <Field label="Format">
                <div className="flex gap-2">
                  {(['paragraph', 'bullets'] as const).map((f) => (
                    <button
                      key={f}
                      onClick={() => setFormat(f)}
                      className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors ${
                        format === f
                          ? 'bg-blue-600 text-white'
                          : 'bg-neutral-100 dark:bg-neutral-800 text-neutral-600 dark:text-neutral-400 hover:bg-neutral-200 dark:hover:bg-neutral-700'
                      }`}
                    >
                      {f}
                    </button>
                  ))}
                </div>
              </Field>
              <Field label="Focus (optional)">
                <input
                  value={focus}
                  onChange={(e) => setFocus(e.target.value)}
                  className="input-base"
                  placeholder="e.g. key takeaways, technical details"
                />
              </Field>
            </div>
          )}

          {capability.key === 'Data Extraction' && (
            <div className="space-y-4">
              <Field label="Text">
                <textarea
                  rows={5}
                  value={extractText}
                  onChange={(e) => setExtractText(e.target.value)}
                  className="input-base"
                  placeholder="Paste text to extract data from…"
                />
              </Field>
              <Field label="Fields to Extract">
                <div className="space-y-2">
                  {fields.map((f, i) => (
                    <div key={i} className="flex gap-2 items-start">
                      <input
                        value={f.name}
                        onChange={(e) => updateField(i, 'name', e.target.value)}
                        className="input-base flex-1"
                        placeholder="name"
                      />
                      <select
                        value={f.type}
                        onChange={(e) => updateField(i, 'type', e.target.value)}
                        className="input-base w-28"
                      >
                        <option value="string">string</option>
                        <option value="number">number</option>
                        <option value="boolean">boolean</option>
                        <option value="date">date</option>
                      </select>
                      <input
                        value={f.description}
                        onChange={(e) => updateField(i, 'description', e.target.value)}
                        className="input-base flex-1"
                        placeholder="description"
                      />
                      <button
                        onClick={() => removeField(i)}
                        className="p-2 text-neutral-400 hover:text-red-500"
                        title="Remove"
                      >
                        ×
                      </button>
                    </div>
                  ))}
                  <button
                    onClick={addField}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    + Add field
                  </button>
                </div>
              </Field>
            </div>
          )}
        </div>

        {/* Run Button */}
        <button
          onClick={handleRun}
          disabled={submitting || (taskId !== null && !isTerminal)}
          className="flex items-center justify-center gap-2 w-full py-3 rounded-xl bg-blue-600 text-white font-semibold text-sm hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {submitting ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Submitting…
            </>
          ) : (
            <>
              <Play className="w-4 h-4" />
              Run Task — {capability.price} credits
            </>
          )}
        </button>

        {error && (
          <div className="flex items-start gap-2 p-3 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 text-sm">
            <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
            {error}
          </div>
        )}
      </div>

      {/* =================== RIGHT PANEL =================== */}
      <div className="flex flex-col gap-6 overflow-y-auto">
        {/* Stepper */}
        {taskId && (
          <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5">
            <h3 className="font-semibold text-neutral-900 dark:text-white mb-4">Progress</h3>
            <Stepper current={currentStep} failed={currentStep === 'failed'} />
          </div>
        )}

        {/* Output */}
        <div className="flex-1 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-5 flex flex-col">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-neutral-900 dark:text-white">Output</h3>
            {task?.output_status && (
              <OutputBadge status={task.output_status} />
            )}
          </div>

          {!taskId && (
            <div className="flex-1 flex items-center justify-center text-neutral-400 text-sm">
              Run a task to see results here.
            </div>
          )}

          {taskId && !isTerminal && (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <Loader2 className="w-8 h-8 text-blue-500 animate-spin mx-auto mb-3" />
                <p className="text-sm text-neutral-500">{STEPPER_LABELS[currentStep ?? 'created'] ?? 'Processing'}…</p>
              </div>
            </div>
          )}

          {task?.status === 'failed' && task.output_status === 'error' && (
            <div className="flex-1">
              <div className="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800">
                <div className="flex items-center gap-2 text-red-700 dark:text-red-300 font-medium mb-2">
                  <XCircle className="w-4 h-4" />
                  Task Failed
                </div>
                {task.output_payload && (
                  <pre className="text-sm text-red-600 dark:text-red-400 font-mono whitespace-pre-wrap overflow-x-auto">
                    {JSON.stringify(task.output_payload, null, 2)}
                  </pre>
                )}
              </div>
            </div>
          )}

          {task?.status === 'completed' && (
            <div className="flex-1 overflow-y-auto">
              <pre className="text-sm font-mono text-neutral-800 dark:text-neutral-200 whitespace-pre-wrap bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-lg p-4 overflow-x-auto">
                {JSON.stringify(task.output_payload, null, 2)}
              </pre>
            </div>
          )}

          {task?.status === 'failed' && task.output_status !== 'error' && (
            <div className="flex-1 overflow-y-auto">
              <pre className="text-sm font-mono text-neutral-800 dark:text-neutral-200 whitespace-pre-wrap bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-lg p-4 overflow-x-auto">
                {JSON.stringify(task.output_payload, null, 2)}
              </pre>
            </div>
          )}
        </div>

        {/* Task Meta */}
        {task && (
          <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 p-4 text-xs text-neutral-500 dark:text-neutral-400 grid grid-cols-2 gap-2">
            <div>Task ID</div>
            <div className="font-mono truncate">{task.id}</div>
            <div>Worker</div>
            <div className="font-mono truncate">{task.worker_agent_id ?? '—'}</div>
            <div>Budget</div>
            <div>{task.budget} credits</div>
            <div>Created</div>
            <div>{new Date(task.created_at).toLocaleTimeString()}</div>
          </div>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300 mb-1.5 block">{label}</span>
      {children}
    </label>
  );
}

function Stepper({ current, failed }: { current: string | null; failed: boolean }) {
  const idx = STEPPER_STATES.indexOf(current as typeof STEPPER_STATES[number]);

  return (
    <div className="flex items-center gap-1">
      {STEPPER_STATES.map((step, i) => {
        const isCurrent = step === current;
        const isDone = idx > i;
        const isFailed = failed && isCurrent;

        let dotClass = 'bg-neutral-200 dark:bg-neutral-700';
        let textClass = 'text-neutral-400';
        if (isDone) {
          dotClass = 'bg-green-500';
          textClass = 'text-green-600 dark:text-green-400';
        } else if (isFailed) {
          dotClass = 'bg-red-500';
          textClass = 'text-red-600 dark:text-red-400';
        } else if (isCurrent) {
          dotClass = 'bg-blue-500 animate-pulse';
          textClass = 'text-blue-600 dark:text-blue-400 font-medium';
        }

        return (
          <div key={step} className="flex-1 flex flex-col items-center gap-1.5">
            <div className="flex items-center w-full">
              {i > 0 && (
                <div className={`flex-1 h-0.5 ${isDone ? 'bg-green-500' : 'bg-neutral-200 dark:bg-neutral-700'}`} />
              )}
              <div className={`w-3 h-3 rounded-full shrink-0 ${dotClass} flex items-center justify-center`}>
                {isDone && <CheckCircle2 className="w-3 h-3 text-white" />}
              </div>
              {i < STEPPER_STATES.length - 1 && (
                <div className={`flex-1 h-0.5 ${idx > i ? 'bg-green-500' : 'bg-neutral-200 dark:bg-neutral-700'}`} />
              )}
            </div>
            <span className={`text-[10px] leading-tight ${textClass}`}>
              {failed && isCurrent ? 'Failed' : STEPPER_LABELS[step]}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function OutputBadge({ status }: { status: string }) {
  if (status === 'error') {
    return (
      <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 font-medium">
        <XCircle className="w-3 h-3" />
        Error
      </span>
    );
  }
  if (status === 'partial') {
    return (
      <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 font-medium">
        <AlertTriangle className="w-3 h-3" />
        Partial Result
      </span>
    );
  }
  if (status === 'success') {
    return (
      <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 font-medium">
        <CheckCircle2 className="w-3 h-3" />
        Success
      </span>
    );
  }
  return null;
}
