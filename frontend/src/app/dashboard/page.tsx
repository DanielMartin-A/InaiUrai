'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import {
  ListChecks,
  Coins,
  TrendingUp,
  Bot,
} from 'lucide-react';

interface TaskRow {
  id: string;
  capability_required: string;
  status: string;
  budget: number;
  created_at: string;
}

interface StatsData {
  tasksToday: number;
  credits: number;
  successRate: number;
  activeAgents: number;
}

const STATUS_COLORS: Record<string, string> = {
  completed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
  in_progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  matching: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  dispatched: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/30 dark:text-indigo-400',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  created: 'bg-neutral-100 text-neutral-700 dark:bg-neutral-800 dark:text-neutral-300',
};

const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

export default function DashboardOverview() {
  const [tasks, setTasks] = useState<TaskRow[]>([]);
  const [stats, setStats] = useState<StatsData>({
    tasksToday: 0,
    credits: 1000,
    successRate: 0,
    activeAgents: 3,
  });
  const [spending, setSpending] = useState<number[]>([12, 24, 18, 32, 28, 8, 0]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const taskList = await api<TaskRow[]>('/tasks').catch(() => [] as TaskRow[]);
        setTasks(taskList);

        const today = new Date().toISOString().slice(0, 10);
        const todayTasks = taskList.filter((t) => t.created_at?.startsWith(today));
        const completed = taskList.filter((t) => t.status === 'completed').length;
        const rate = taskList.length > 0 ? Math.round((completed / taskList.length) * 100) : 0;

        setStats({
          tasksToday: todayTasks.length,
          credits: 1000,
          successRate: rate,
          activeAgents: 3,
        });

        // Build daily spending from task budgets (last 7 days).
        const days: number[] = [];
        for (let i = 6; i >= 0; i--) {
          const d = new Date();
          d.setDate(d.getDate() - i);
          const key = d.toISOString().slice(0, 10);
          const sum = taskList
            .filter((t) => t.created_at?.startsWith(key))
            .reduce((acc, t) => acc + (t.budget || 0), 0);
          days.push(sum);
        }
        setSpending(days);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  const STAT_CARDS = [
    { label: 'Tasks Today', value: stats.tasksToday, icon: ListChecks, color: 'text-blue-600 dark:text-blue-400', bg: 'bg-blue-100 dark:bg-blue-900/30' },
    { label: 'Credits Remaining', value: stats.credits.toLocaleString(), icon: Coins, color: 'text-amber-600 dark:text-amber-400', bg: 'bg-amber-100 dark:bg-amber-900/30' },
    { label: 'Success Rate', value: `${stats.successRate}%`, icon: TrendingUp, color: 'text-green-600 dark:text-green-400', bg: 'bg-green-100 dark:bg-green-900/30' },
    { label: 'Active Agents', value: stats.activeAgents, icon: Bot, color: 'text-purple-600 dark:text-purple-400', bg: 'bg-purple-100 dark:bg-purple-900/30' },
  ];

  const maxSpend = Math.max(...spending, 1);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {STAT_CARDS.map((card) => (
          <div
            key={card.label}
            className="p-5 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900"
          >
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-neutral-500 dark:text-neutral-400">{card.label}</span>
              <div className={`w-8 h-8 rounded-lg ${card.bg} flex items-center justify-center`}>
                <card.icon className={`w-4 h-4 ${card.color}`} />
              </div>
            </div>
            <span className="text-2xl font-bold text-neutral-900 dark:text-white">{card.value}</span>
          </div>
        ))}
      </div>

      <div className="grid lg:grid-cols-3 gap-6">
        {/* Recent Activity */}
        <div className="lg:col-span-2 rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900">
          <div className="px-5 py-4 border-b border-neutral-200 dark:border-neutral-800">
            <h3 className="font-semibold text-neutral-900 dark:text-white">Recent Activity</h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-neutral-100 dark:border-neutral-800 text-neutral-500 dark:text-neutral-400">
                  <th className="text-left px-5 py-3 font-medium">Task</th>
                  <th className="text-left px-5 py-3 font-medium">Capability</th>
                  <th className="text-left px-5 py-3 font-medium">Status</th>
                  <th className="text-right px-5 py-3 font-medium">Budget</th>
                </tr>
              </thead>
              <tbody>
                {tasks.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-5 py-8 text-center text-neutral-400">
                      No tasks yet. Create one from the Try It page.
                    </td>
                  </tr>
                ) : (
                  tasks.slice(0, 10).map((task) => (
                    <tr
                      key={task.id}
                      className="border-b border-neutral-50 dark:border-neutral-800/50 hover:bg-neutral-50 dark:hover:bg-neutral-800/30"
                    >
                      <td className="px-5 py-3 font-mono text-xs text-neutral-600 dark:text-neutral-400">
                        {task.id.slice(0, 8)}…
                      </td>
                      <td className="px-5 py-3 text-neutral-900 dark:text-white">
                        {task.capability_required}
                      </td>
                      <td className="px-5 py-3">
                        <span
                          className={`inline-block text-xs px-2 py-0.5 rounded-full font-medium ${
                            STATUS_COLORS[task.status] ?? STATUS_COLORS.created
                          }`}
                        >
                          {task.status}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-right text-neutral-900 dark:text-white tabular-nums">
                        {task.budget} cr
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Daily Spending Chart */}
        <div className="rounded-xl border border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-900 flex flex-col">
          <div className="px-5 py-4 border-b border-neutral-200 dark:border-neutral-800">
            <h3 className="font-semibold text-neutral-900 dark:text-white">Daily Spending</h3>
            <p className="text-xs text-neutral-400 mt-0.5">Last 7 days (credits)</p>
          </div>
          <div className="flex-1 flex items-end gap-2 px-5 pb-5 pt-4 min-h-[180px]">
            {spending.map((val, i) => (
              <div key={i} className="flex-1 flex flex-col items-center gap-1">
                <span className="text-xs text-neutral-400 tabular-nums">
                  {val > 0 ? val : ''}
                </span>
                <div
                  className="w-full rounded-t bg-blue-500 dark:bg-blue-600 transition-all"
                  style={{
                    height: `${Math.max((val / maxSpend) * 120, val > 0 ? 4 : 0)}px`,
                  }}
                />
                <span className="text-xs text-neutral-400">{DAYS[i]}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
