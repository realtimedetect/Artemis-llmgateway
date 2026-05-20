'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ShieldAlert, UserPlus, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';
import api from '@/lib/api';
import { useAuthStore } from '@/store/authStore';

type Plan = {
  id: string;
  name: string;
  monthly_token_limit?: number | null;
  description?: string;
};

type AdminUser = {
  id: string;
  email: string;
  role: string;
  plan_id: string;
  created_at: string;
};

type LicenseStatus = {
  user_id: string;
  plan_id: string;
  monthly_used_tokens: number;
  monthly_token_limit: number | null;
  remaining_tokens: number | null;
  next_reset_at: string;
};

export default function ManageUsersPage() {
  const router = useRouter();
  const { user } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [savingUserId, setSavingUserId] = useState('');
  const [activatingLicense, setActivatingLicense] = useState(false);
  const [licenseKey, setLicenseKey] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [plans, setPlans] = useState<Plan[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [licenseStatus, setLicenseStatus] = useState<LicenseStatus | null>(null);

  const isAdmin = (user?.role ?? '').toLowerCase() === 'admin';

  useEffect(() => {
    if (user && !isAdmin) {
      router.replace('/dashboard');
    }
  }, [user, isAdmin, router]);

  useEffect(() => {
    if (!isAdmin) return;
    void loadPlanData();
  }, [isAdmin]);

  async function loadPlanData() {
    setLoadingUsers(true);
    setError('');
    try {
      const [plansRes, usersRes, licenseRes] = await Promise.all([
        api.get<Plan[]>('/api/admin/plans'),
        api.get<AdminUser[]>('/api/admin/users'),
        api.get<LicenseStatus>('/api/admin/license/status'),
      ]);
      setPlans(plansRes.data ?? []);
      setUsers(usersRes.data ?? []);
      setLicenseStatus(licenseRes.data ?? null);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to load users/plans');
    } finally {
      setLoadingUsers(false);
    }
  }

  async function createUser(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');
    setLoading(true);
    try {
      await api.post('/api/users', { email, password });
      setSuccess(`Access granted to ${email}. New user has view-only role.`);
      setEmail('');
      setPassword('');
      await loadPlanData();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to create user');
    } finally {
      setLoading(false);
    }
  }

  async function updateUserPlan(targetUserId: string, planId: string) {
    setError('');
    setSuccess('');
    setSavingUserId(targetUserId);
    try {
      await api.put(`/api/admin/users/${targetUserId}/plan`, { plan_id: planId });
      setUsers((prev) => prev.map((u) => (u.id === targetUserId ? { ...u, plan_id: planId } : u)));
      setSuccess('User plan updated successfully.');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to update user plan');
    } finally {
      setSavingUserId('');
    }
  }

  async function activateProfessionalLicense(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');
    setActivatingLicense(true);
    try {
      await api.post('/api/admin/license/activate', { license_key: licenseKey });
      setLicenseKey('');
      setSuccess('Professional license activated. You can now assign Professional plan.');
      await loadPlanData();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to activate license');
    } finally {
      setActivatingLicense(false);
    }
  }

  function planLabel(plan: Plan) {
    if (plan.monthly_token_limit == null) return `${plan.name} (Unlimited)`;
    return `${plan.name} (${(plan.monthly_token_limit / 1_000_000).toFixed(0)}M / month)`;
  }

  function formatTokens(value: number | null | undefined) {
    if (value == null) return 'Unlimited';
    return new Intl.NumberFormat().format(value);
  }

  const currentAdminPlan = (licenseStatus?.plan_id ?? users.find((u) => u.id === user?.id)?.plan_id ?? 'basic').toLowerCase();
  const isCurrentAdminProfessional = currentAdminPlan === 'professional';

  if (user && !isAdmin) {
    return (
      <div className="max-w-3xl bg-white border border-slate-200 rounded-xl p-6">
        <div className="flex items-start gap-3 text-amber-700">
          <ShieldAlert size={18} className="mt-0.5" />
          <div>
            <h2 className="text-base font-semibold">Admin Access Required</h2>
            <p className="text-sm text-slate-600 mt-1">Only admin users can manage user access.</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-2xl space-y-6">
      <div>
        <h2 className="text-xl font-bold text-slate-800">Manage Users</h2>
        <p className="text-sm text-slate-500 mt-1">
          Admins can create new users and provide gateway access. New users are created with view-only role.
        </p>
      </div>

      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100 space-y-4">
        {error && (
          <div className="flex items-start gap-2 text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
            <AlertCircle size={16} className="mt-0.5" />
            <span>{error}</span>
          </div>
        )}
        {success && (
          <div className="flex items-start gap-2 text-sm text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg px-3 py-2">
            <CheckCircle2 size={16} className="mt-0.5" />
            <span>{success}</span>
          </div>
        )}

        <form onSubmit={createUser} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">User Email</label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              placeholder="new.user@example.com"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Temporary Password</label>
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              placeholder="Minimum 8 characters"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-60"
          >
            <UserPlus size={16} />
            {loading ? 'Creating user...' : 'Grant Access'}
          </button>
        </form>
      </div>

      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100 space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-base font-semibold text-slate-800">License Upgrade</h3>
          <span
            className={isCurrentAdminProfessional
              ? 'inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-emerald-100 text-emerald-700 border border-emerald-200'
              : 'inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-amber-100 text-amber-800 border border-amber-200'}
          >
            {isCurrentAdminProfessional ? 'Professional' : 'Basic'}
          </span>
        </div>
        <div>
          <p className="text-xs text-slate-500 mt-1">
            Upload your Professional license key to unlock Professional plan assignment.
          </p>
          {licenseStatus && (
            <div className="mt-2 text-xs text-slate-600 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2">
              <div>Monthly used: <span className="font-medium text-slate-800">{formatTokens(licenseStatus.monthly_used_tokens)}</span> tokens</div>
              <div>
                Remaining: <span className="font-medium text-slate-800">{isCurrentAdminProfessional ? 'Unlimited' : `${formatTokens(licenseStatus.remaining_tokens)} tokens`}</span>
              </div>
            </div>
          )}
        </div>

        <form onSubmit={activateProfessionalLicense} className="flex flex-col sm:flex-row gap-3">
          <input
            type="password"
            required
            value={licenseKey}
            onChange={(e) => setLicenseKey(e.target.value)}
            className="flex-1 px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            placeholder="Enter Professional license key"
          />
          <button
            type="submit"
            disabled={activatingLicense || !licenseKey.trim()}
            className="px-4 py-2.5 bg-slate-900 hover:bg-slate-800 text-white text-sm font-medium rounded-lg transition disabled:opacity-60"
          >
            {activatingLicense ? 'Activating...' : 'Activate Professional'}
          </button>
        </form>
      </div>

      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-semibold text-slate-800">User Plans</h3>
            <p className="text-xs text-slate-500 mt-1">Assign Basic (5M/month) or Professional (unlimited) plans.</p>
          </div>
          <button
            type="button"
            onClick={() => void loadPlanData()}
            className="inline-flex items-center gap-2 px-3 py-2 text-xs border border-slate-300 rounded-lg hover:bg-slate-50"
            disabled={loadingUsers}
          >
            <RefreshCw size={14} className={loadingUsers ? 'animate-spin' : ''} />
            Refresh
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 border-b border-slate-200">
                <th className="py-2 pr-3">Email</th>
                <th className="py-2 pr-3">Role</th>
                <th className="py-2 pr-3">Plan</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-slate-100">
                  <td className="py-2 pr-3 text-slate-700">{u.email}</td>
                  <td className="py-2 pr-3">
                    <span className="px-2 py-0.5 rounded-full text-xs bg-slate-100 text-slate-700">{u.role}</span>
                  </td>
                  <td className="py-2 pr-3">
                    <select
                      value={u.plan_id || 'basic'}
                      onChange={(e) => void updateUserPlan(u.id, e.target.value)}
                      disabled={savingUserId === u.id || loadingUsers}
                      className="px-3 py-1.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    >
                      {plans.map((p) => (
                        <option key={p.id} value={p.id}>
                          {planLabel(p)}
                        </option>
                      ))}
                    </select>
                  </td>
                </tr>
              ))}
              {!loadingUsers && users.length === 0 && (
                <tr>
                  <td colSpan={3} className="py-4 text-center text-slate-500">No users found.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
