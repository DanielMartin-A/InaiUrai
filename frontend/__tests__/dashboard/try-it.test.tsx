import { cleanup, render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useAuthStore } from '@/lib/store';
import TryItPage from '@/app/dashboard/try/page';

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/* ------------------------------------------------------------------ */
/*  Setup / Teardown                                                   */
/* ------------------------------------------------------------------ */

beforeEach(() => {
  useAuthStore.getState().login('test-token', 'requester');
  vi.useFakeTimers({ shouldAdvanceTime: true });
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  cleanup();
  useAuthStore.getState().logout();
});

/* ------------------------------------------------------------------ */
/*  1. Capability selector shows 3 cards                               */
/* ------------------------------------------------------------------ */

describe('Capability Selector', () => {
  it('renders Research, Summarize, and Data Extraction cards', () => {
    render(<TryItPage />);

    expect(screen.getByText('Research')).toBeInTheDocument();
    expect(screen.getByText('Summarize')).toBeInTheDocument();
    expect(screen.getByText('Data Extraction')).toBeInTheDocument();

    expect(screen.getByText('8 credits')).toBeInTheDocument();
    expect(screen.getByText('3 credits')).toBeInTheDocument();
    expect(screen.getByText('5 credits')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------ */
/*  2. Run task flow — click Summarize, see stepper                    */
/* ------------------------------------------------------------------ */

describe('Run Task Flow', () => {
  it('shows stepper after submitting a task', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    const taskId = '00000000-aaaa-bbbb-cccc-000000000001';

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: taskId, status: 'matching' }))
      .mockResolvedValue(jsonResponse({
        id: taskId,
        status: 'matching',
        output_status: '',
        output_payload: null,
        capability_required: 'Summarize',
        budget: 3,
        worker_agent_id: null,
        created_at: new Date().toISOString(),
      }));
    vi.stubGlobal('fetch', fetchMock);

    render(<TryItPage />);

    await user.click(screen.getByText('Summarize'));

    const runButton = screen.getByRole('button', { name: /Run Task/i });
    await user.click(runButton);

    await waitFor(() => {
      expect(screen.getByText('Validating')).toBeInTheDocument();
      expect(screen.getByText('Matching')).toBeInTheDocument();
      expect(screen.getByText('Processing')).toBeInTheDocument();
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/v1/tasks'),
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

/* ------------------------------------------------------------------ */
/*  3. Completed result — JSON and cost metadata displayed             */
/* ------------------------------------------------------------------ */

describe('Completed Result', () => {
  it('displays output JSON and cost metadata when task completes', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    const taskId = '00000000-aaaa-bbbb-cccc-000000000002';
    const outputPayload = {
      status: 'success',
      summary: 'AI is transforming software development.',
      word_count: 7,
      key_topics: ['AI', 'software'],
    };

    const completedTask = {
      id: taskId,
      status: 'completed',
      output_status: 'success',
      output_payload: outputPayload,
      capability_required: 'Summarize',
      budget: 3,
      worker_agent_id: '00000000-1111-2222-3333-444444444444',
      created_at: new Date().toISOString(),
    };

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: taskId, status: 'matching' }))
      .mockResolvedValue(jsonResponse(completedTask));
    vi.stubGlobal('fetch', fetchMock);

    render(<TryItPage />);

    await user.click(screen.getByText('Summarize'));
    await user.click(screen.getByRole('button', { name: /Run Task/i }));

    await act(async () => { vi.advanceTimersByTime(2000); });

    await waitFor(() => {
      expect(screen.getByText(/AI is transforming software development/)).toBeInTheDocument();
    });

    const creditElements = screen.getAllByText('3 credits');
    expect(creditElements.length).toBeGreaterThanOrEqual(1);

    expect(screen.getByText('Success')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------ */
/*  4. Error handling — error message in red                           */
/* ------------------------------------------------------------------ */

describe('Error Handling', () => {
  it('shows error message in red when task fails', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    const taskId = '00000000-aaaa-bbbb-cccc-000000000003';
    const errorPayload = {
      status: 'error',
      error: { code: 'TIMEOUT', message: 'The research request timed out.' },
    };

    const failedTask = {
      id: taskId,
      status: 'failed',
      output_status: 'error',
      output_payload: errorPayload,
      capability_required: 'Research',
      budget: 8,
      worker_agent_id: null,
      created_at: new Date().toISOString(),
    };

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ task_id: taskId, status: 'matching' }))
      .mockResolvedValue(jsonResponse(failedTask));
    vi.stubGlobal('fetch', fetchMock);

    render(<TryItPage />);

    await user.click(screen.getByRole('button', { name: /Run Task/i }));

    await act(async () => { vi.advanceTimersByTime(2000); });

    await waitFor(() => {
      expect(screen.getByText('Task Failed')).toBeInTheDocument();
    });

    expect(screen.getByText(/TIMEOUT/)).toBeInTheDocument();

    const errorContainer = screen.getByText('Task Failed').closest('div');
    expect(errorContainer?.className).toMatch(/red/);
  });
});
