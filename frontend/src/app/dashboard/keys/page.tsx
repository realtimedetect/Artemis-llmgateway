'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { Trash2, Plus, Copy } from 'lucide-react';

type APIKey = {
  id: string;
  name: string;
  key_prefix: string;
  allowed_provider_ids?: string;
  allowed_models?: string;
  request_count?: number;
  total_tokens?: number;
  total_cost_usd?: number;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
  key?: string;
};

export default function KeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [name, setName] = useState('');
  const [allowedProviders, setAllowedProviders] = useState('');
  const [allowedModels, setAllowedModels] = useState('');
  const [newKey, setNewKey] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => { fetchKeys(); }, []);

  async function fetchKeys() {
    const { data } = await api.get('/api/keys');
    setKeys(data);
  }

  async function createKey() {
    if (!name.trim()) return;
    setLoading(true);
    try {
      const { data } = await api.post('/api/keys', {
        name,
        allowed_provider_ids: allowedProviders,
        allowed_models: allowedModels,
      });
      setNewKey(data.key);
      setName('');
      setAllowedProviders('');
      setAllowedModels('');
      fetchKeys();
    } finally {
      setLoading(false);
    }
  }

  async function deleteKey(id: string) {
    await api.delete(`/api/keys/${id}`);
    fetchKeys();
  }

  return (
    <div className="max-w-2xl space-y-6">
      <h2 className="text-xl font-bold text-slate-800">API Keys</h2>

      {newKey && (
        <div className="bg-green-50 border border-green-200 rounded-xl p-4">
          <p className="text-sm text-green-700 font-medium mb-2">
            Copy your new API key — it won&apos;t be shown again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 text-xs bg-white border border-green-200 rounded px-3 py-2 break-all">
              {newKey}
            </code>
            <button onClick={() => navigator.clipboard.writeText(newKey)} className="text-green-600">
              <Copy size={16} />
            </button>
          </div>
          <button
            className="mt-3 text-xs text-green-600 underline"
            onClick={() => setNewKey(null)}
          >
            Dismiss
          </button>
        </div>
      )}

      <div className="flex gap-2">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Key name…"
          className="flex-1 px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <input
          value={allowedProviders}
          onChange={(e) => setAllowedProviders(e.target.value)}
          placeholder="Allowed provider IDs (comma separated, optional)"
          className="flex-1 px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <input
          value={allowedModels}
          onChange={(e) => setAllowedModels(e.target.value)}
          placeholder="Allowed models (comma separated, optional)"
          className="flex-1 px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <button
          onClick={createKey}
          disabled={loading || !name.trim()}
          className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
        >
          <Plus size={16} /> Create
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-slate-100 divide-y divide-slate-100">
        {keys.length === 0 && (
          <p className="px-4 py-6 text-sm text-slate-400 text-center">No API keys yet.</p>
        )}
        {keys.map((k) => (
          <div key={k.id} className="flex items-center justify-between px-4 py-3 gap-4">
            <div>
              <p className="text-sm font-medium text-slate-800">{k.name}</p>
              <p className="text-xs text-slate-400 font-mono">{k.key_prefix}••••••••</p>
              <p className="text-xs text-slate-400 mt-1">Providers: {k.allowed_provider_ids || 'all'} | Models: {k.allowed_models || 'all'}</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="text-right">
                <p className="text-xs text-slate-500">Req: {(k.request_count ?? 0).toLocaleString()}</p>
                <p className="text-xs text-slate-500">Tokens: {(k.total_tokens ?? 0).toLocaleString()}</p>
                <p className="text-xs text-slate-500">Cost: ${(k.total_cost_usd ?? 0).toFixed(4)}</p>
              </div>
              <span className="text-xs text-slate-400">
                {new Date(k.created_at).toLocaleDateString()}
              </span>
              <button
                onClick={() => deleteKey(k.id)}
                className="text-red-400 hover:text-red-600 transition"
              >
                <Trash2 size={16} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
