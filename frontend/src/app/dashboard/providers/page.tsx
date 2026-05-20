'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { Trash2, Plus, ToggleLeft, ToggleRight } from 'lucide-react';

type Provider = {
  id: string;
  name: string;
  base_url: string;
  adapter: 'openai' | 'anthropic';
  api_version?: string;
  key_count?: number;
  enabled: boolean;
};

type ProviderHealth = {
  provider_id: string;
  provider_name: string;
  enabled: boolean;
  circuit_open: boolean;
  consecutive_failures: number;
  open_until?: string;
};

type ProviderKeyRuntimeStat = {
  key_id: string;
  selection_count: number;
  cooldown_until?: string;
  cooldown_remaining_seconds: number;
  available: boolean;
};

type ProviderKeyPoolStat = {
  provider_id: string;
  name: string;
  adapter: string;
  enabled: boolean;
  key_count: number;
  keys: ProviderKeyRuntimeStat[];
};

const EMPTY: Omit<Provider, 'id'> & { api_key: string } = {
  name: '',
  base_url: '',
  adapter: 'openai',
  api_version: '',
  api_key: '',
  enabled: true,
};

export default function ProvidersPage() {
  const singleLLMLicenseMsg = 'Only one LLM can be configured in this plan. To configure more than one LLM, get the license or contact pv@realtimedetect.com';

  const [providers, setProviders] = useState<Provider[]>([]);
  const [health, setHealth] = useState<Record<string, ProviderHealth>>({});
  const [keyPoolStats, setKeyPoolStats] = useState<Record<string, ProviderKeyPoolStat>>({});
  const [form, setForm] = useState({ ...EMPTY });
  const [extraKeysText, setExtraKeysText] = useState('');
  const [formError, setFormError] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshSeconds, setRefreshSeconds] = useState<3 | 4 | 5>(4);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchProviders();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => {
      fetchProviders();
    }, refreshSeconds * 1000);
    return () => {
      window.clearInterval(id);
    };
  }, [autoRefresh, refreshSeconds]);

  async function fetchProviders() {
    const [{ data }, { data: healthData }, statsRes] = await Promise.all([
      api.get('/api/providers'),
      api.get('/api/providers/health'),
      api.get('/api/admin/providers/key-pool-stats').catch(() => ({ data: [] })),
    ]);
    setProviders(data);
    setHealth(Object.fromEntries((healthData as ProviderHealth[]).map((h) => [h.provider_id, h])));
    setKeyPoolStats(Object.fromEntries((statsRes.data as ProviderKeyPoolStat[]).map((s) => [s.provider_id, s])));
  }

  async function createProvider() {
    if (!form.name || !form.base_url || !form.api_key) return;
    setFormError('');
    const apiKeys = extraKeysText
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line.length > 0);
    setLoading(true);
    try {
      await api.post('/api/providers', { ...form, api_keys: apiKeys });
      setForm({ ...EMPTY });
      setExtraKeysText('');
      fetchProviders();
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setFormError(msg ?? 'Failed to add provider.');
    } finally {
      setLoading(false);
    }
  }

  async function toggleProvider(p: Provider) {
    await api.put(`/api/providers/${p.id}`, { ...p, enabled: !p.enabled, api_key: '' });
    fetchProviders();
  }

  async function deleteProvider(id: string) {
    await api.delete(`/api/providers/${id}`);
    fetchProviders();
  }

  return (
    <div className="max-w-2xl space-y-6">
      <h2 className="text-xl font-bold text-slate-800">LLM Providers</h2>

      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100 space-y-3">
        <h3 className="text-sm font-medium text-slate-700">Add Provider</h3>
        {providers.length >= 1 && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
            {singleLLMLicenseMsg}
          </div>
        )}
        {formError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
            {formError}
          </div>
        )}
        <select
          value={form.adapter}
          onChange={(e) => setForm({ ...form, adapter: e.target.value as Provider['adapter'] })}
          className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          <option value="openai">OpenAI-compatible</option>
          <option value="anthropic">Anthropic</option>
        </select>
        {(['name', 'base_url', 'api_key'] as const).map((field) => (
          <input
            key={field}
            value={form[field]}
            onChange={(e) => setForm({ ...form, [field]: e.target.value })}
            placeholder={{
              name: 'Provider name (e.g. OpenAI, Groq, Anthropic)',
              base_url: form.adapter === 'anthropic' ? 'Base URL (e.g. https://api.anthropic.com/v1)' : 'Base URL (e.g. https://api.openai.com/v1)',
              api_key: 'API key',
            }[field]}
            type={field === 'api_key' ? 'password' : 'text'}
            className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        ))}
        {form.adapter === 'anthropic' && (
          <input
            value={form.api_version ?? ''}
            onChange={(e) => setForm({ ...form, api_version: e.target.value })}
            placeholder="Anthropic version header (optional, defaults to 2023-06-01)"
            className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        )}
        <textarea
          value={extraKeysText}
          onChange={(e) => setExtraKeysText(e.target.value)}
          placeholder="Additional API keys (optional), one per line"
          rows={4}
          className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <button
          onClick={createProvider}
          disabled={loading || providers.length >= 1}
          className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
        >
          <Plus size={16} /> Add Provider
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-slate-100 divide-y divide-slate-100">
        <div className="px-4 py-3 flex items-center justify-between bg-slate-50">
          <p className="text-xs font-medium text-slate-600">Key-Pool Live Telemetry</p>
          <div className="flex items-center gap-2">
            <label className="text-xs text-slate-600 inline-flex items-center gap-2">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                className="rounded border-slate-300"
              />
              Auto-refresh
            </label>
            <select
              value={refreshSeconds}
              onChange={(e) => setRefreshSeconds(Number(e.target.value) as 3 | 4 | 5)}
              disabled={!autoRefresh}
              className="text-xs border border-slate-300 rounded px-2 py-1 bg-white disabled:bg-slate-100 disabled:text-slate-400"
            >
              <option value={3}>3s</option>
              <option value={4}>4s</option>
              <option value={5}>5s</option>
            </select>
          </div>
        </div>
        {providers.length === 0 && (
          <p className="px-4 py-6 text-sm text-slate-400 text-center">No providers configured yet.</p>
        )}
        {providers.map((p) => (
          <div key={p.id} className="flex items-center justify-between px-4 py-4">
            <div>
              <p className="text-sm font-semibold text-slate-800">{p.name}</p>
              <p className="text-xs text-slate-400">{p.base_url}</p>
              <p className="text-xs text-slate-500 mt-1">adapter: {p.adapter}{p.api_version ? ` • version ${p.api_version}` : ''}</p>
              <p className="text-xs text-slate-500 mt-1">keys in pool: {p.key_count ?? 0}</p>
              {keyPoolStats[p.id]?.keys?.length ? (
                <div className="mt-2 overflow-x-auto">
                  <table className="text-xs text-slate-600 min-w-[460px] border border-slate-200 rounded-md">
                    <thead className="bg-slate-50 text-slate-500">
                      <tr>
                        <th className="px-2 py-1.5 text-left font-medium">Key</th>
                        <th className="px-2 py-1.5 text-left font-medium">Selections</th>
                        <th className="px-2 py-1.5 text-left font-medium">State</th>
                        <th className="px-2 py-1.5 text-left font-medium">Cooldown</th>
                      </tr>
                    </thead>
                    <tbody>
                      {keyPoolStats[p.id].keys.map((k) => (
                        <tr key={k.key_id} className="border-t border-slate-100">
                          <td className="px-2 py-1.5 font-mono text-[11px]">{k.key_id}</td>
                          <td className="px-2 py-1.5">{k.selection_count}</td>
                          <td className="px-2 py-1.5">
                            {k.available ? (
                              <span className="text-emerald-600">available</span>
                            ) : (
                              <span className="text-amber-600">cooling down</span>
                            )}
                          </td>
                          <td className="px-2 py-1.5">
                            {k.available
                              ? '-'
                              : `${k.cooldown_remaining_seconds}s${k.cooldown_until ? ` (until ${new Date(k.cooldown_until).toLocaleTimeString()})` : ''}`}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
              <p className="text-xs mt-1">
                {health[p.id]?.circuit_open ? (
                  <span className="text-red-600">circuit open until {health[p.id]?.open_until ? new Date(health[p.id].open_until as string).toLocaleTimeString() : 'n/a'}</span>
                ) : (
                  <span className="text-emerald-600">healthy</span>
                )}
                <span className="text-slate-400 ml-2">failures: {health[p.id]?.consecutive_failures ?? 0}</span>
              </p>
            </div>
            <div className="flex items-center gap-3">
              <button onClick={() => toggleProvider(p)} className={p.enabled ? 'text-brand-600' : 'text-slate-400'}>
                {p.enabled ? <ToggleRight size={22} /> : <ToggleLeft size={22} />}
              </button>
              <button onClick={() => deleteProvider(p.id)} className="text-red-400 hover:text-red-600">
                <Trash2 size={16} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
