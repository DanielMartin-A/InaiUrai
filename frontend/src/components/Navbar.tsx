'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuthStore } from '@/lib/store';
import { LayoutDashboard, Store, LogIn, LogOut } from 'lucide-react';

export function Navbar() {
  const pathname = usePathname();
  const token = useAuthStore((s) => s.token);
  const role = useAuthStore((s) => s.role);
  const logout = useAuthStore((s) => s.logout);

  return (
    <header className="border-b border-neutral-200 dark:border-neutral-800 bg-white dark:bg-neutral-950">
      <nav className="container max-w-6xl mx-auto flex items-center justify-between h-14 px-4">
        <Link href="/" className="font-semibold text-neutral-900 dark:text-white">
          inaiurai
        </Link>
        <div className="flex items-center gap-4">
          <Link
            href="/marketplace"
            className={`flex items-center gap-1.5 text-sm ${
              pathname === '/marketplace'
                ? 'text-neutral-900 dark:text-white font-medium'
                : 'text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white'
            }`}
          >
            <Store className="w-4 h-4" />
            Marketplace
          </Link>
          {token && (
            <Link
              href="/dashboard"
              className={`flex items-center gap-1.5 text-sm ${
                pathname === '/dashboard'
                  ? 'text-neutral-900 dark:text-white font-medium'
                  : 'text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white'
              }`}
            >
              <LayoutDashboard className="w-4 h-4" />
              Dashboard
            </Link>
          )}
          {token && role === 'provider' && (
            <Link
              href="/provider"
              className={`flex items-center gap-1.5 text-sm ${
                pathname === '/provider'
                  ? 'text-neutral-900 dark:text-white font-medium'
                  : 'text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white'
              }`}
            >
              Provider
            </Link>
          )}
          {token ? (
            <button
              type="button"
              onClick={() => logout()}
              className="flex items-center gap-1.5 text-sm text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white"
            >
              <LogOut className="w-4 h-4" />
              Logout
            </button>
          ) : (
            <Link
              href="/login"
              className="flex items-center gap-1.5 text-sm text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white"
            >
              <LogIn className="w-4 h-4" />
              Login
            </Link>
          )}
        </div>
      </nav>
    </header>
  );
}
