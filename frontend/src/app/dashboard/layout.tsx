'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useAuthStore } from '@/lib/store';
import {
  LayoutDashboard,
  Bot,
  FlaskConical,
  KeyRound,
  CreditCard,
  Settings,
  ChevronLeft,
  Coins,
} from 'lucide-react';

const NAV_ITEMS = [
  { href: '/dashboard', label: 'Overview', icon: LayoutDashboard },
  { href: '/dashboard/agents', label: 'Agents', icon: Bot },
  { href: '/dashboard/try', label: 'Try It', icon: FlaskConical },
  { href: '/dashboard/keys', label: 'API Keys', icon: KeyRound },
  { href: '/dashboard/billing', label: 'Billing', icon: CreditCard },
  { href: '/dashboard/settings', label: 'Settings', icon: Settings },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const token = useAuthStore((s) => s.token);
  const logout = useAuthStore((s) => s.logout);
  const [collapsed, setCollapsed] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (mounted && !token) {
      router.push('/login');
    }
  }, [mounted, token, router]);

  if (!mounted || !token) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-57px)]">
      {/* Sidebar */}
      <aside
        className={`${
          collapsed ? 'w-16' : 'w-56'
        } shrink-0 border-r border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-950 flex flex-col transition-all duration-200`}
      >
        <div className="flex items-center justify-between px-3 py-3 border-b border-neutral-200 dark:border-neutral-800">
          {!collapsed && (
            <span className="text-sm font-semibold text-neutral-900 dark:text-white truncate">
              Dashboard
            </span>
          )}
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="p-1 rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 text-neutral-500"
          >
            <ChevronLeft
              className={`w-4 h-4 transition-transform ${collapsed ? 'rotate-180' : ''}`}
            />
          </button>
        </div>
        <nav className="flex-1 py-2 space-y-0.5 px-2">
          {NAV_ITEMS.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  active
                    ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 font-medium'
                    : 'text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 hover:text-neutral-900 dark:hover:text-white'
                }`}
                title={collapsed ? item.label : undefined}
              >
                <item.icon className="w-4 h-4 shrink-0" />
                {!collapsed && <span>{item.label}</span>}
              </Link>
            );
          })}
        </nav>
        <div className="p-2 border-t border-neutral-200 dark:border-neutral-800">
          <button
            onClick={() => {
              logout();
              router.push('/login');
            }}
            className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 hover:text-neutral-900 dark:hover:text-white transition-colors`}
          >
            {collapsed ? '←' : 'Log out'}
          </button>
        </div>
      </aside>

      {/* Main content area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Topbar */}
        <header className="h-12 shrink-0 border-b border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-950 flex items-center justify-between px-4">
          <span className="text-sm text-neutral-500 dark:text-neutral-400">
            {NAV_ITEMS.find((i) => i.href === pathname)?.label ?? 'Dashboard'}
          </span>
          <div className="flex items-center gap-3">
            <CreditBadge />
            <UserBadge />
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-6 bg-neutral-50 dark:bg-neutral-900/30">
          {children}
        </main>
      </div>
    </div>
  );
}

function CreditBadge() {
  const [balance, setBalance] = useState<number | null>(null);

  useEffect(() => {
    // Credit balance would come from an API call; use store or fetch.
    // For now, read from localStorage if available (set after login).
    try {
      const raw = localStorage.getItem('inaiurai-credits');
      if (raw) setBalance(parseInt(raw, 10));
    } catch { /* noop */ }
  }, []);

  return (
    <div className="flex items-center gap-1.5 text-sm text-neutral-600 dark:text-neutral-400 bg-neutral-100 dark:bg-neutral-800 px-2.5 py-1 rounded-full">
      <Coins className="w-3.5 h-3.5 text-amber-500" />
      <span className="font-medium">{balance !== null ? balance.toLocaleString() : '—'}</span>
      <span className="hidden sm:inline">credits</span>
    </div>
  );
}

function UserBadge() {
  const [name, setName] = useState('');

  useEffect(() => {
    try {
      const raw = localStorage.getItem('inaiurai-auth');
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed?.state?.role) {
          setName(parsed.state.role);
        }
      }
    } catch { /* noop */ }
  }, []);

  return (
    <div className="w-7 h-7 rounded-full bg-blue-600 flex items-center justify-center text-white text-xs font-semibold uppercase" title={name}>
      {name ? name[0] : '?'}
    </div>
  );
}
