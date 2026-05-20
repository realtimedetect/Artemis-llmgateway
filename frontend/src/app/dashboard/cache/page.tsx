'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { CheckCircle2, RefreshCw, Database, AlertCircle } from 'lucide-react';

type CacheConfig = {
  enabled: boolean;
  semantic_enabled: boolean;
  semantic_threshold: number;
  semantic_max_candidates: number;
  semantic_embedding_model: string;
  redis_addr: string;
  redis_username: string;
  redis_password?: string;
  clear_password?: boolean;
  redis_db: number;
  default_ttl_seconds: number;
  key_prefix: string;
  has_password?: boolean;
};

const DEFAULT_FORM: CacheConfig = {
  enabled: false,
  semantic_enabled: false,
  semantic_threshold: 0.9,
  semantic_max_candidates: 30,
  semantic_embedding_model: 'text-embedding-3-small',
  redis_addr: 'localhost:6379',
  redis_username: '',
  redis_password: '',
  clear_password: false,
  redis_db: 0,
  default_ttl_seconds: 300,
  key_prefix: 'llm-gw',
  has_password: false,
};

export default function CachePage() {
  const [form, setForm] = useState<CacheConfig>({ ...DEFAULT_FORM });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    fetchConfig();
  }, []);

  async function fetchConfig() {
    setLoading(true);
    setError('');
    try {
      const { data } = await api.get('/api/cache/config');
      setForm({
        ...DEFAULT_FORM,
        ...data,
        redis_password: '',
        clear_password: false,
      });
    } catch {
      setError('Failed to load cache config.');
    } finally {
      setLoading(false);
    }
  }

  async function saveConfig() {
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await api.put('/api/cache/config', form);
      setSuccess('Cache settings saved successfully.');
      await fetchConfig();
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to save cache settings.');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <div className="text-sm text-slate-500">Loading cache settings...</div>;
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h2 className="text-xl font-bold text-slate-800">Cache</h2>
        <p className="text-sm text-slate-500 mt-1">
          Configure Redis connectivity for response caching. Chat completions are cached by request payload and model.
        </p>
      </div>

      <div className="bg-white border border-slate-200 rounded-xl p-6 space-y-5">
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

        <label className="flex items-center gap-3 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
            className="w-4 h-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500"
          />
          Enable Redis response cache
        </label>

        <label className="flex items-center gap-3 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={form.semantic_enabled}
            onChange={(e) => setForm((f) => ({ ...f, semantic_enabled: e.target.checked }))}
            className="w-4 h-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500"
          />
          Enable semantic cache lookup (embedding similarity)
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Redis Address</label>
            <input
              value={form.redis_addr}
              onChange={(e) => setForm((f) => ({ ...f, redis_addr: e.target.value }))}
              placeholder="localhost:6379"
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Redis DB</label>
            <input
              type="number"
              min={0}
              value={form.redis_db}
              onChange={(e) => setForm((f) => ({ ...f, redis_db: Number(e.target.value) || 0 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Redis Username</label>
            <input
              value={form.redis_username}
              onChange={(e) => setForm((f) => ({ ...f, redis_username: e.target.value }))}
              placeholder="optional"
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Redis Password</label>
            <input
              type="password"
              value={form.redis_password ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, redis_password: e.target.value, clear_password: false }))}
              placeholder={form.has_password ? 'Stored password is set; type to replace' : 'optional'}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
            {form.has_password && (
              <label className="mt-2 flex items-center gap-2 text-xs text-slate-500">
                <input
                  type="checkbox"
                  checked={Boolean(form.clear_password)}
                  onChange={(e) => setForm((f) => ({ ...f, clear_password: e.target.checked }))}
                  className="w-3.5 h-3.5"
                />
                Clear stored password on save
              </label>
            )}
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Default TTL (seconds)</label>
            <input
              type="number"
              min={1}
              max={86400}
              value={form.default_ttl_seconds}
              onChange={(e) => setForm((f) => ({ ...f, default_ttl_seconds: Number(e.target.value) || 1 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Cache Key Prefix</label>
            <input
              value={form.key_prefix}
              onChange={(e) => setForm((f) => ({ ...f, key_prefix: e.target.value }))}
              placeholder="llm-gw"
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Semantic Threshold</label>
            <input
              type="number"
              min={0.5}
              max={0.999}
              step={0.01}
              value={form.semantic_threshold}
              onChange={(e) => setForm((f) => ({ ...f, semantic_threshold: Number(e.target.value) || 0.9 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Semantic Candidate Scan</label>
            <input
              type="number"
              min={1}
              max={200}
              value={form.semantic_max_candidates}
              onChange={(e) => setForm((f) => ({ ...f, semantic_max_candidates: Number(e.target.value) || 30 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div className="sm:col-span-2">
            <label className="block text-xs font-medium text-slate-600 mb-1">Semantic Embedding Model</label>
            <input
              value={form.semantic_embedding_model}
              onChange={(e) => setForm((f) => ({ ...f, semantic_embedding_model: e.target.value }))}
              placeholder="text-embedding-3-small"
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>
        </div>

        <div className="bg-slate-50 border border-slate-200 rounded-lg px-4 py-3 text-xs text-slate-600 flex items-start gap-2">
          <Database size={15} className="mt-0.5 text-slate-500" />
          <p>
            Cache applies to non-stream chat completions. The key is derived from user, model, and full request payload.
            If semantic cache is enabled, the gateway also compares prompt embeddings and can serve a prior similar response.
          </p>
        </div>

        <div className="flex items-center gap-3 pt-1">
          <button
            onClick={saveConfig}
            disabled={saving}
            className="px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save Cache Config'}
          </button>
          <button
            onClick={fetchConfig}
            className="px-4 py-2.5 text-sm text-slate-600 hover:bg-slate-100 rounded-lg transition flex items-center gap-1.5"
          >
            <RefreshCw size={15} /> Refresh
          </button>
        </div>
      </div>
    </div>
  );
}
