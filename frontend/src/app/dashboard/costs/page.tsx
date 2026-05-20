'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { Trash2, Plus, Pencil, X, Check, DollarSign, Info } from 'lucide-react';

type Provider = { id: string; name: string };

type CostRule = {
  id: string;
  user_id: string;
  provider_id: string;
  provider_name: string;
  model: string;
  input_cost_per_1m: number;
  output_cost_per_1m: number;
  currency: string;
  notes: string;
  created_at: string;
};

type UsageSummary = {
  total_requests: number;
  total_tokens: number;
  total_cost_usd: number;
  avg_latency_ms: number;
};

type CostGroup = {
  id: string;
  user_id: string;
  name: string;
  description: string;
  created_at?: string;
};

type APIKeyLite = {
  id: string;
  name: string;
  key_prefix: string;
  group_id?: string | null;
  group_name?: string;
};

type CostBreakdown = {
  period?: 'today' | '7d' | '30d';
  user_total: UsageSummary;
  groups: {
    group_id: string;
    group_name: string;
    requests: number;
    total_tokens: number;
    total_cost_usd: number;
  }[];
};

const EMPTY_FORM = {
  provider_id: '',
  model: '',
  input_cost_per_1m: 0,
  output_cost_per_1m: 0,
  currency: 'USD',
  notes: '',
};

type FormState = typeof EMPTY_FORM;

// Common presets to help users fill in pricing quickly.
const PRESETS: { label: string; model: string; input: number; output: number }[] = [
  { label: 'GPT-4o', model: 'gpt-4o', input: 2.5, output: 10 },
  { label: 'GPT-4o mini', model: 'gpt-4o-mini', input: 0.15, output: 0.6 },
  { label: 'GPT-4.1', model: 'gpt-4.1', input: 2, output: 8 },
  { label: 'GPT-4.1 mini', model: 'gpt-4.1-mini', input: 0.4, output: 1.6 },
  { label: 'o3', model: 'o3', input: 10, output: 40 },
  { label: 'o4-mini', model: 'o4-mini', input: 1.1, output: 4.4 },
  { label: 'Claude 3.5 Sonnet', model: 'claude-3-5-sonnet-20241022', input: 3, output: 15 },
  { label: 'Claude 3.5 Haiku', model: 'claude-3-5-haiku-20241022', input: 0.8, output: 4 },
  { label: 'Claude 3.7 Sonnet', model: 'claude-3-7-sonnet-20250219', input: 3, output: 15 },
  { label: 'Gemini 2.0 Flash', model: 'gemini-2.0-flash', input: 0.1, output: 0.4 },
  { label: 'Gemini 2.5 Pro', model: 'gemini-2.5-pro-preview-03-25', input: 1.25, output: 10 },
];

export default function CostsPage() {
  const [costs, setCosts] = useState<CostRule[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [groups, setGroups] = useState<CostGroup[]>([]);
  const [apiKeys, setAPIKeys] = useState<APIKeyLite[]>([]);
  const [breakdown, setBreakdown] = useState<CostBreakdown | null>(null);
  const [breakdownPeriod, setBreakdownPeriod] = useState<'today' | '7d' | '30d'>('30d');
  const [groupName, setGroupName] = useState('');
  const [groupDescription, setGroupDescription] = useState('');
  const [groupError, setGroupError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>({ ...EMPTY_FORM });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchCosts();
    fetchGroups();
    fetchAPIKeys();
    fetchBreakdown('30d');
    api.get('/api/providers').then((r) => setProviders(r.data));
    api.get('/api/usage').then((r) => setUsage(r.data));
  }, []);

  async function fetchCosts() {
    const { data } = await api.get('/api/costs');
    setCosts(data);
  }

  async function fetchGroups() {
    const { data } = await api.get('/api/cost-groups');
    setGroups(data);
  }

  async function fetchAPIKeys() {
    const { data } = await api.get('/api/keys');
    setAPIKeys(data);
  }

  async function fetchBreakdown(period: 'today' | '7d' | '30d' = breakdownPeriod) {
    const { data } = await api.get('/api/analytics/cost-breakdown', { params: { period } });
    setBreakdown(data);
    setBreakdownPeriod(period);
  }

  function openCreate() {
    setEditId(null);
    setForm({ ...EMPTY_FORM });
    setError('');
    setShowForm(true);
  }

  function openEdit(rule: CostRule) {
    setEditId(rule.id);
    setForm({
      provider_id: rule.provider_id,
      model: rule.model,
      input_cost_per_1m: rule.input_cost_per_1m,
      output_cost_per_1m: rule.output_cost_per_1m,
      currency: rule.currency,
      notes: rule.notes,
    });
    setError('');
    setShowForm(true);
  }

  function applyPreset(preset: (typeof PRESETS)[number]) {
    setForm((f) => ({
      ...f,
      model: preset.model,
      input_cost_per_1m: preset.input,
      output_cost_per_1m: preset.output,
      notes: `${preset.label} pricing`,
    }));
  }

  async function save() {
    if (!form.provider_id || !form.model) {
      setError('Provider and model are required.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      if (editId) {
        await api.put(`/api/costs/${editId}`, form);
      } else {
        await api.post('/api/costs', form);
      }
      setShowForm(false);
      fetchCosts();
      // Refresh usage summary to reflect updated cost totals.
      api.get('/api/usage').then((r) => setUsage(r.data));
      fetchBreakdown(breakdownPeriod);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to save cost rule.');
    } finally {
      setLoading(false);
    }
  }

  async function deleteCost(id: string) {
    if (!confirm('Delete this cost rule?')) return;
    await api.delete(`/api/costs/${id}`);
    fetchCosts();
  }

  async function createGroup() {
    if (!groupName.trim()) {
      setGroupError('Group name is required.');
      return;
    }
    setGroupError('');
    try {
      await api.post('/api/cost-groups', {
        name: groupName.trim(),
        description: groupDescription.trim(),
      });
      setGroupName('');
      setGroupDescription('');
      fetchGroups();
      fetchBreakdown(breakdownPeriod);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setGroupError(msg ?? 'Failed to create group.');
    }
  }

  async function deleteGroup(id: string) {
    if (!confirm('Delete this group? API keys will become ungrouped.')) return;
    await api.delete(`/api/cost-groups/${id}`);
    fetchGroups();
    fetchAPIKeys();
    fetchBreakdown(breakdownPeriod);
  }

  async function assignKeyGroup(keyId: string, nextGroupID: string) {
    await api.put(`/api/keys/${keyId}/group`, { group_id: nextGroupID });
    fetchAPIKeys();
    fetchBreakdown(breakdownPeriod);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-slate-800">Cost Settings</h2>
          <p className="text-sm text-slate-500 mt-0.5">
            Set token pricing per provider + model. The gateway calculates spend automatically after each request.
          </p>
        </div>
        <button
          onClick={openCreate}
          className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition"
        >
          <Plus size={16} /> Add Cost Rule
        </button>
      </div>

      {/* Total Cost Banner */}
      {usage !== null && (
        <div className="bg-white border border-slate-200 rounded-xl p-5 flex flex-wrap items-center gap-6">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-emerald-50 rounded-lg flex items-center justify-center shrink-0">
              <DollarSign size={20} className="text-emerald-600" />
            </div>
            <div>
              <p className="text-xs text-slate-500 uppercase tracking-wide">Estimated Total Spend</p>
              <p className="text-2xl font-bold text-slate-800">
                ${(usage.total_cost_usd ?? 0).toFixed(6)}
                <span className="ml-1 text-sm font-normal text-slate-400">USD</span>
              </p>
            </div>
          </div>
          <div className="flex gap-6 text-center ml-auto">
            <div>
              <p className="text-xs text-slate-400">Requests</p>
              <p className="text-lg font-semibold text-slate-700">{usage.total_requests.toLocaleString()}</p>
            </div>
            <div>
              <p className="text-xs text-slate-400">Total Tokens</p>
              <p className="text-lg font-semibold text-slate-700">{usage.total_tokens.toLocaleString()}</p>
            </div>
            <div>
              <p className="text-xs text-slate-400">Avg Latency</p>
              <p className="text-lg font-semibold text-slate-700">{Math.round(usage.avg_latency_ms)} ms</p>
            </div>
          </div>
        </div>
      )}

      {/* Cost Groups */}
      <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-4">
        <div>
          <h3 className="font-semibold text-slate-800">Spend Groups</h3>
          <p className="text-xs text-slate-500 mt-0.5">
            Create logical groups (team, project, environment) and assign API keys to track spend by group.
          </p>
        </div>

        {groupError && (
          <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-4 py-2">{groupError}</div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <input
            value={groupName}
            onChange={(e) => setGroupName(e.target.value)}
            placeholder="Group name (e.g. Team Alpha)"
            className="sm:col-span-1 px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <input
            value={groupDescription}
            onChange={(e) => setGroupDescription(e.target.value)}
            placeholder="Description (optional)"
            className="sm:col-span-1 px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
          <button
            onClick={createGroup}
            className="sm:col-span-1 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition"
          >
            Create Group
          </button>
        </div>

        <div className="overflow-x-auto border border-slate-100 rounded-lg">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
              <tr>
                <th className="text-left px-4 py-3">Group</th>
                <th className="text-left px-4 py-3">Description</th>
                <th className="text-left px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {groups.length === 0 && (
                <tr>
                  <td colSpan={3} className="px-4 py-8 text-center text-slate-400">No groups yet.</td>
                </tr>
              )}
              {groups.map((g) => (
                <tr key={g.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3 text-slate-800 font-medium">{g.name}</td>
                  <td className="px-4 py-3 text-slate-500 text-xs">{g.description || '—'}</td>
                  <td className="px-4 py-3">
                    <button onClick={() => deleteGroup(g.id)} className="text-red-500 hover:text-red-700">
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* API Key Group Assignment */}
      <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-4">
        <div>
          <h3 className="font-semibold text-slate-800">Assign API Keys To Groups</h3>
          <p className="text-xs text-slate-500 mt-0.5">
            New requests inherit the group from the API key used for authentication.
          </p>
        </div>
        <div className="overflow-x-auto border border-slate-100 rounded-lg">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
              <tr>
                <th className="text-left px-4 py-3">API Key</th>
                <th className="text-left px-4 py-3">Prefix</th>
                <th className="text-left px-4 py-3">Group</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {apiKeys.length === 0 && (
                <tr>
                  <td colSpan={3} className="px-4 py-8 text-center text-slate-400">No API keys available.</td>
                </tr>
              )}
              {apiKeys.map((k) => (
                <tr key={k.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3 text-slate-700">{k.name}</td>
                  <td className="px-4 py-3 text-slate-500 font-mono text-xs">{k.key_prefix}</td>
                  <td className="px-4 py-3">
                    <select
                      value={k.group_id ?? ''}
                      onChange={(e) => assignKeyGroup(k.id, e.target.value)}
                      className="min-w-[220px] px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    >
                      <option value="">Ungrouped</option>
                      {groups.map((g) => (
                        <option key={g.id} value={g.id}>{g.name}</option>
                      ))}
                    </select>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Group Level Tracking */}
      {breakdown !== null && (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-4">
          <div>
            <h3 className="font-semibold text-slate-800">Cost Tracking By Group</h3>
            <p className="text-xs text-slate-500 mt-0.5">
              User-level totals remain in the summary above. This table breaks down costs by group for easier chargeback.
            </p>
          </div>
          <div className="flex items-center gap-2">
            {[
              { label: 'Today', value: 'today' as const },
              { label: '7d', value: '7d' as const },
              { label: '30d', value: '30d' as const },
            ].map((p) => (
              <button
                key={p.value}
                onClick={() => fetchBreakdown(p.value)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${
                  breakdownPeriod === p.value
                    ? 'bg-brand-600 text-white border-brand-600'
                    : 'bg-white text-slate-600 border-slate-300 hover:border-brand-400'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
          <div className="overflow-x-auto border border-slate-100 rounded-lg">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
                <tr>
                  <th className="text-left px-4 py-3">Group</th>
                  <th className="text-left px-4 py-3">Requests</th>
                  <th className="text-left px-4 py-3">Tokens</th>
                  <th className="text-left px-4 py-3">Cost (USD)</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {breakdown.groups.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-slate-400">No request spend data yet.</td>
                  </tr>
                )}
                {breakdown.groups.map((g) => (
                  <tr key={g.group_id} className="hover:bg-slate-50">
                    <td className="px-4 py-3 text-slate-700 font-medium">{g.group_name}</td>
                    <td className="px-4 py-3 text-slate-600">{g.requests.toLocaleString()}</td>
                    <td className="px-4 py-3 text-slate-600">{g.total_tokens.toLocaleString()}</td>
                    <td className="px-4 py-3 text-slate-700 font-mono text-xs">${g.total_cost_usd.toFixed(6)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Create / Edit Form */}
      {showForm && (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-5">
          <div className="flex items-center justify-between">
            <h3 className="font-semibold text-slate-800">{editId ? 'Edit Cost Rule' : 'New Cost Rule'}</h3>
            <button onClick={() => setShowForm(false)} className="text-slate-400 hover:text-slate-600">
              <X size={18} />
            </button>
          </div>

          {/* Quick presets */}
          <div>
            <p className="text-xs font-medium text-slate-500 mb-2">Quick presets</p>
            <div className="flex flex-wrap gap-2">
              {PRESETS.map((p) => (
                <button
                  key={p.model}
                  onClick={() => applyPreset(p)}
                  className="px-2.5 py-1 text-xs bg-slate-100 hover:bg-brand-50 hover:text-brand-700 rounded-md transition"
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          {error && (
            <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-4 py-2">{error}</div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">Provider</label>
              <select
                value={form.provider_id}
                onChange={(e) => setForm((f) => ({ ...f, provider_id: e.target.value }))}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">Select provider…</option>
                {providers.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">Model</label>
              <input
                value={form.model}
                onChange={(e) => setForm((f) => ({ ...f, model: e.target.value }))}
                placeholder="e.g. gpt-4o"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Input cost <span className="text-slate-400 font-normal">($ per 1M prompt tokens)</span>
              </label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm">$</span>
                <input
                  type="number" min={0} step={0.01}
                  value={form.input_cost_per_1m}
                  onChange={(e) => setForm((f) => ({ ...f, input_cost_per_1m: parseFloat(e.target.value) || 0 }))}
                  className="w-full pl-7 pr-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Output cost <span className="text-slate-400 font-normal">($ per 1M completion tokens)</span>
              </label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-sm">$</span>
                <input
                  type="number" min={0} step={0.01}
                  value={form.output_cost_per_1m}
                  onChange={(e) => setForm((f) => ({ ...f, output_cost_per_1m: parseFloat(e.target.value) || 0 }))}
                  className="w-full pl-7 pr-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">Currency</label>
              <input
                value={form.currency}
                onChange={(e) => setForm((f) => ({ ...f, currency: e.target.value.toUpperCase() }))}
                placeholder="USD"
                maxLength={10}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Notes <span className="text-slate-400 font-normal">(optional)</span>
              </label>
              <input
                value={form.notes}
                onChange={(e) => setForm((f) => ({ ...f, notes: e.target.value }))}
                placeholder="e.g. as of March 2026"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
          </div>

          {/* Live cost preview */}
          {(form.input_cost_per_1m > 0 || form.output_cost_per_1m > 0) && (
            <div className="bg-slate-50 border border-slate-200 rounded-lg px-4 py-3 text-xs text-slate-600 flex gap-6">
              <span>
                1K input tokens ≈{' '}
                <strong>${((form.input_cost_per_1m / 1000)).toFixed(5)}</strong>
              </span>
              <span>
                1K output tokens ≈{' '}
                <strong>${((form.output_cost_per_1m / 1000)).toFixed(5)}</strong>
              </span>
            </div>
          )}

          <div className="flex items-center gap-3 pt-1">
            <button
              onClick={save}
              disabled={loading}
              className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
            >
              <Check size={16} /> {editId ? 'Update Rule' : 'Save Rule'}
            </button>
            <button
              onClick={() => setShowForm(false)}
              className="px-4 py-2.5 text-sm text-slate-600 hover:bg-slate-100 rounded-lg transition"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Cost Rules Table */}
      <div className="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
            <tr>
              {['Provider', 'Model', 'Input $/1M', 'Output $/1M', 'Currency', 'Notes', ''].map((h) => (
                <th key={h} className="text-left px-4 py-3">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {costs.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-slate-400 text-sm">
                  No cost rules configured. Click <strong>Add Cost Rule</strong> to start tracking spend.
                </td>
              </tr>
            )}
            {costs.map((rule) => (
              <tr key={rule.id} className="hover:bg-slate-50">
                <td className="px-4 py-3 text-slate-700 font-medium">{rule.provider_name || '—'}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-800">{rule.model}</td>
                <td className="px-4 py-3 text-slate-600 font-mono text-xs">${rule.input_cost_per_1m.toFixed(4)}</td>
                <td className="px-4 py-3 text-slate-600 font-mono text-xs">${rule.output_cost_per_1m.toFixed(4)}</td>
                <td className="px-4 py-3 text-slate-500 text-xs">{rule.currency}</td>
                <td className="px-4 py-3 text-slate-400 text-xs max-w-[160px] truncate">{rule.notes || '—'}</td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <button onClick={() => openEdit(rule)} className="text-slate-400 hover:text-slate-700 transition">
                      <Pencil size={15} />
                    </button>
                    <button onClick={() => deleteCost(rule.id)} className="text-red-400 hover:text-red-600 transition">
                      <Trash2 size={15} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* How it works */}
      <div className="bg-blue-50 border border-blue-100 rounded-xl px-5 py-4 text-sm text-blue-800 flex gap-3">
        <Info size={16} className="shrink-0 mt-0.5 text-blue-400" />
        <div className="space-y-1">
          <p className="font-medium">How cost tracking works</p>
          <p className="text-xs text-blue-700">
            After each chat request, the gateway reads the token usage from the provider response and multiplies
            by your configured rates. Cost is stored per-request and aggregated in the overview dashboard.
            Rules are matched by <strong>provider + exact model name</strong> — make sure the model field
            matches exactly what you send in requests (e.g. <code className="bg-blue-100 px-1 rounded">gpt-4o</code>).
          </p>
          <p className="text-xs text-blue-600 mt-1">
            All pricing is user-configurable. Check your provider&apos;s official page for current rates.
          </p>
        </div>
      </div>
    </div>
  );
}
