'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/authStore';
import { LayoutDashboard, MessageSquare, Key, Server, Route, DollarSign, ScrollText, Database, Users, Activity, FileStack, Bot, LogOut, UserCheck } from 'lucide-react';
import { clsx } from 'clsx';

const navItems = [
  { href: '/dashboard', label: 'Overview', icon: LayoutDashboard },
  { href: '/dashboard/chat', label: 'Chat', icon: MessageSquare },
  { href: '/dashboard/agent', label: 'Agentic AI', icon: Bot },
  { href: '/dashboard/keys', label: 'API Keys', icon: Key },
  { href: '/dashboard/providers', label: 'Providers', icon: Server },
  { href: '/dashboard/routes', label: 'Routes', icon: Route },
  { href: '/dashboard/prompts', label: 'Prompts', icon: FileStack },
  { href: '/dashboard/costs', label: 'Cost Settings', icon: DollarSign },
  { href: '/dashboard/groups', label: 'Groups & Teams', icon: UserCheck },
  { href: '/dashboard/observability', label: 'Observability', icon: Activity },
  { href: '/dashboard/cache', label: 'Cache', icon: Database },
  { href: '/dashboard/audits', label: 'Audit Logs', icon: ScrollText },
  { href: '/dashboard/users', label: 'Manage Users', icon: Users, adminOnly: true },
];

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuthStore();
  const isAdmin = (user?.role ?? '').toLowerCase() === 'admin';

  function handleLogout() {
    logout();
    router.replace('/login');
  }

  return (
    <aside className="w-56 flex flex-col bg-white border-r border-slate-200 py-6 px-3 shrink-0">
      <div className="px-3 mb-8">
        <h1 className="text-lg font-bold text-brand-700">LLM Gateway</h1>
        <p className="text-xs text-slate-400 truncate">{user?.email}</p>
      </div>

      <nav className="flex-1 space-y-1">
        {navItems
          .filter((item) => !item.adminOnly || isAdmin)
          .map(({ href, label, icon: Icon }) => {
          const active = pathname === href;
          return (
            <Link
              key={href}
              href={href}
              className={clsx(
                'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition',
                active
                  ? 'bg-brand-50 text-brand-700'
                  : 'text-slate-600 hover:bg-slate-100 hover:text-slate-800',
              )}
            >
              <Icon size={17} />
              {label}
            </Link>
          );
          })}
      </nav>

      <button
        onClick={handleLogout}
        className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-800 transition mt-2"
      >
        <LogOut size={17} />
        Sign out
      </button>
    </aside>
  );
}
