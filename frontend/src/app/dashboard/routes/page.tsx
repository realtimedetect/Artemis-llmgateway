'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { Trash2, Plus, Pencil, ToggleLeft, ToggleRight, X, Check } from 'lucide-react';

type Provider = { id: string; name: string; enabled: boolean };

type Route = {
  id: string;
  name: string;
  slug: string;
  description: string;
  provider_id: string;
  provider_name: string;
  model: string;
  system_prompt: string;
  temperature: number;
  max_tokens: number;
  stream_passthrough: boolean;
  prompt_version_id: string;
  enforce_json_schema: boolean;
  output_json_schema: string;
  failover_provider_ids: string;
  allowed_models: string;
  enabled: boolean;
  created_at: string;
};

type RoutingConfig = {
  smart_enabled: boolean;
  cost_weight: number;
  performance_weight: number;
  complexity_threshold: number;
};

const DEFAULT_ROUTING_CONFIG: RoutingConfig = {
  smart_enabled: false,
  cost_weight: 0.7,
  performance_weight: 0.3,
  complexity_threshold: 1200,
};

const EMPTY_FORM = {
  name: '',
  slug: '',
  description: '',
  provider_id: '',
  model: '',
  system_prompt: '',
  temperature: 1,
  max_tokens: 0,
  stream_passthrough: true,
  prompt_version_id: '',
  enforce_json_schema: false,
  output_json_schema: '',
  failover_provider_ids: '',
  allowed_models: '',
  enabled: true,
};

type FormState = typeof EMPTY_FORM;

export default function RoutesPage() {
  const [routes, setRoutes] = useState<Route[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>({ ...EMPTY_FORM });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [routingConfig, setRoutingConfig] = useState<RoutingConfig>({ ...DEFAULT_ROUTING_CONFIG });
  const [routingSaving, setRoutingSaving] = useState(false);
  const [routingMessage, setRoutingMessage] = useState('');

  useEffect(() => {
    fetchRoutes();
    api.get('/api/providers').then((r) => setProviders(r.data.filter((p: Provider) => p.enabled)));
    api.get('/api/routing/config').then((r) => setRoutingConfig({ ...DEFAULT_ROUTING_CONFIG, ...r.data }));
  }, []);

  async function saveRoutingConfig() {
    setRoutingSaving(true);
    setRoutingMessage('');
    try {
      await api.put('/api/routing/config', routingConfig);
      setRoutingMessage('Smart routing settings saved.');
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setRoutingMessage(msg ?? 'Failed to save smart routing settings.');
    } finally {
      setRoutingSaving(false);
    }
  }

  async function fetchRoutes() {
    const { data } = await api.get('/api/routes');
    setRoutes(data);
  }

  function openCreate() {
    setEditId(null);
    setForm({ ...EMPTY_FORM });
    setError('');
    setShowForm(true);
  }

  function openEdit(rt: Route) {
    setEditId(rt.id);
    setForm({
      name: rt.name,
      slug: rt.slug,
      description: rt.description,
      provider_id: rt.provider_id,
      model: rt.model,
      system_prompt: rt.system_prompt,
      temperature: rt.temperature,
      max_tokens: rt.max_tokens,
      stream_passthrough: rt.stream_passthrough,
      prompt_version_id: rt.prompt_version_id ?? '',
      enforce_json_schema: rt.enforce_json_schema ?? false,
      output_json_schema: rt.output_json_schema ?? '',
      failover_provider_ids: rt.failover_provider_ids ?? '',
      allowed_models: rt.allowed_models ?? '',
      enabled: rt.enabled,
    });
    setError('');
    setShowForm(true);
  }

  async function saveRoute() {
    if (!form.name || !form.slug || !form.provider_id || !form.model) {
      setError('Name, slug, provider and model are required.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      if (editId) {
        await api.put(`/api/routes/${editId}`, form);
      } else {
        await api.post('/api/routes', form);
      }
      setShowForm(false);
      fetchRoutes();
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to save route.');
    } finally {
      setLoading(false);
    }
  }

  async function toggleRoute(rt: Route) {
    await api.put(`/api/routes/${rt.id}`, { ...rt, enabled: !rt.enabled });
    fetchRoutes();
  }

  async function deleteRoute(id: string) {
    if (!confirm('Delete this route?')) return;
    await api.delete(`/api/routes/${id}`);
    fetchRoutes();
  }

  // Auto-generate slug from name
  function handleNameChange(name: string) {
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    setForm((f) => ({ ...f, name, slug: editId ? f.slug : slug }));
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-slate-800">LLM Routes</h2>
          <p className="text-sm text-slate-500 mt-0.5">
            Map a slug to a provider + model. Use the slug as the <code className="bg-slate-100 px-1 rounded text-xs">model</code> field in chat requests.
          </p>
        </div>
        <button
          onClick={openCreate}
          className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition"
        >
          <Plus size={16} /> New Route
        </button>
      </div>

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="font-semibold text-slate-800">Smart Routing</h3>
            <p className="text-xs text-slate-500 mt-0.5">
              Reorders candidate providers by cost and recent latency for non-route direct model calls.
            </p>
          </div>
          <label className="flex items-center gap-2 text-sm text-slate-700">
            <input
              type="checkbox"
              checked={routingConfig.smart_enabled}
              onChange={(e) => setRoutingConfig((c) => ({ ...c, smart_enabled: e.target.checked }))}
              className="h-4 w-4 accent-brand-600"
            />
            Enable
          </label>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Cost Weight</label>
            <input
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={routingConfig.cost_weight}
              onChange={(e) => setRoutingConfig((c) => ({ ...c, cost_weight: Number(e.target.value) || 0 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Performance Weight</label>
            <input
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={routingConfig.performance_weight}
              onChange={(e) => setRoutingConfig((c) => ({ ...c, performance_weight: Number(e.target.value) || 0 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-600 mb-1">Complexity Threshold (tokens)</label>
            <input
              type="number"
              min={200}
              max={20000}
              value={routingConfig.complexity_threshold}
              onChange={(e) => setRoutingConfig((c) => ({ ...c, complexity_threshold: Number(e.target.value) || 1200 }))}
              className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={saveRoutingConfig}
            disabled={routingSaving}
            className="px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
          >
            {routingSaving ? 'Saving...' : 'Save Smart Routing'}
          </button>
          {routingMessage && <p className="text-xs text-slate-600">{routingMessage}</p>}
        </div>
      </div>

      {/* ── Form Panel ── */}
      {showForm && (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-6 space-y-4">
          <div className="flex items-center justify-between mb-1">
            <h3 className="font-semibold text-slate-800">{editId ? 'Edit Route' : 'New Route'}</h3>
            <button onClick={() => setShowForm(false)} className="text-slate-400 hover:text-slate-600">
              <X size={18} />
            </button>
          </div>

          {error && (
            <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-4 py-2">
              {error}
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">Route Name</label>
              <input
                value={form.name}
                onChange={(e) => handleNameChange(e.target.value)}
                placeholder="e.g. Fast Chat"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Slug <span className="text-slate-400 font-normal">(used as model name)</span>
              </label>
              <input
                value={form.slug}
                onChange={(e) => setForm((f) => ({ ...f, slug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '') }))}
                placeholder="e.g. fast-chat"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">Provider</label>
              <select
                value={form.provider_id}
                onChange={(e) => setForm((f) => ({ ...f, provider_id: e.target.value }))}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">Select a provider…</option>
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
                placeholder="e.g. gpt-4o-mini"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">Description</label>
              <input
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                placeholder="Optional description"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">
                System Prompt <span className="text-slate-400 font-normal">(prepended to every request using this route)</span>
              </label>
              <textarea
                value={form.system_prompt}
                onChange={(e) => setForm((f) => ({ ...f, system_prompt: e.target.value }))}
                placeholder="You are a helpful assistant…"
                rows={3}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Temperature <span className="text-slate-400">(0 – 2, default 1)</span>
              </label>
              <input
                type="number"
                min={0}
                max={2}
                step={0.1}
                value={form.temperature}
                onChange={(e) => setForm((f) => ({ ...f, temperature: parseFloat(e.target.value) || 1 }))}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Max Tokens <span className="text-slate-400">(0 = provider default)</span>
              </label>
              <input
                type="number"
                min={0}
                value={form.max_tokens}
                onChange={(e) => setForm((f) => ({ ...f, max_tokens: parseInt(e.target.value) || 0 }))}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="flex items-center justify-between border border-slate-300 rounded-lg px-3 py-2.5">
                <span>
                  <span className="block text-xs font-medium text-slate-600">Streaming Pass-through</span>
                  <span className="block text-xs text-slate-400 mt-0.5">If enabled and request has stream=true, gateway forwards provider SSE stream directly.</span>
                </span>
                <input
                  type="checkbox"
                  checked={form.stream_passthrough}
                  onChange={(e) => setForm((f) => ({ ...f, stream_passthrough: e.target.checked }))}
                  className="h-4 w-4 accent-brand-600"
                />
              </label>
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Prompt Version ID <span className="text-slate-400 font-normal">(optional, from Prompts page)</span>
              </label>
              <input
                value={form.prompt_version_id}
                onChange={(e) => setForm((f) => ({ ...f, prompt_version_id: e.target.value }))}
                placeholder="prompt-version-id"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="flex items-center justify-between border border-slate-300 rounded-lg px-3 py-2.5">
                <span>
                  <span className="block text-xs font-medium text-slate-600">Structured Output Enforcing</span>
                  <span className="block text-xs text-slate-400 mt-0.5">Validate assistant JSON output against schema below.</span>
                </span>
                <input
                  type="checkbox"
                  checked={form.enforce_json_schema}
                  onChange={(e) => setForm((f) => ({ ...f, enforce_json_schema: e.target.checked }))}
                  className="h-4 w-4 accent-brand-600"
                />
              </label>
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">Output JSON Schema</label>
              <textarea
                value={form.output_json_schema}
                onChange={(e) => setForm((f) => ({ ...f, output_json_schema: e.target.value }))}
                placeholder='{"type":"object","required":["answer"],"properties":{"answer":{"type":"string"}}}'
                rows={5}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Failover Provider IDs <span className="text-slate-400 font-normal">(comma-separated, optional)</span>
              </label>
              <input
                value={form.failover_provider_ids}
                onChange={(e) => setForm((f) => ({ ...f, failover_provider_ids: e.target.value }))}
                placeholder="provider-id-1,provider-id-2"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-600 mb-1">
                Allowed Models <span className="text-slate-400 font-normal">(comma-separated, optional)</span>
              </label>
              <input
                value={form.allowed_models}
                onChange={(e) => setForm((f) => ({ ...f, allowed_models: e.target.value }))}
                placeholder="gpt-4o-mini,gpt-4o"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
          </div>

          <div className="flex items-center gap-3 pt-2">
            <button
              onClick={saveRoute}
              disabled={loading}
              className="flex items-center gap-1.5 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-50"
            >
              <Check size={16} /> {editId ? 'Update Route' : 'Create Route'}
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

      {/* ── Routes Table ── */}
      <div className="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
            <tr>
              {['Name / Slug', 'Provider', 'Model', 'Temp', 'Streaming', 'Policy', 'System Prompt', 'Status', ''].map((h) => (
                <th key={h} className="text-left px-4 py-3">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {routes.length === 0 && (
              <tr>
                <td colSpan={9} className="px-4 py-8 text-center text-slate-400 text-sm">
                  No routes yet. Click <strong>New Route</strong> to get started.
                </td>
              </tr>
            )}
            {routes.map((rt) => (
              <tr key={rt.id} className="hover:bg-slate-50">
                <td className="px-4 py-3">
                  <p className="font-medium text-slate-800">{rt.name}</p>
                  <code className="text-xs text-brand-600 bg-brand-50 px-1.5 py-0.5 rounded">{rt.slug}</code>
                </td>
                <td className="px-4 py-3 text-slate-600">{rt.provider_name || <span className="text-slate-300">—</span>}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-700">{rt.model}</td>
                <td className="px-4 py-3 text-slate-500">{rt.temperature}</td>
                <td className="px-4 py-3 text-xs">
                  {rt.stream_passthrough ? (
                    <span className="inline-flex items-center px-2 py-0.5 rounded bg-emerald-50 text-emerald-700">on</span>
                  ) : (
                    <span className="inline-flex items-center px-2 py-0.5 rounded bg-slate-100 text-slate-500">off</span>
                  )}
                </td>
                <td className="px-4 py-3 text-xs text-slate-500">
                  <div>failover: {rt.failover_provider_ids || 'all enabled'}</div>
                  <div>models: {rt.allowed_models || 'all'}</div>
                </td>
                <td className="px-4 py-3 max-w-[180px] truncate text-slate-400 text-xs">
                  {rt.system_prompt || <span className="italic">none</span>}
                </td>
                <td className="px-4 py-3">
                  <button onClick={() => toggleRoute(rt)} title={rt.enabled ? 'Disable' : 'Enable'}>
                    {rt.enabled
                      ? <span className="flex items-center gap-1 text-brand-600"><ToggleRight size={20}/><span className="text-xs">on</span></span>
                      : <span className="flex items-center gap-1 text-slate-400"><ToggleLeft size={20}/><span className="text-xs">off</span></span>}
                  </button>
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <button onClick={() => openEdit(rt)} className="text-slate-400 hover:text-slate-700 transition">
                      <Pencil size={15} />
                    </button>
                    <button onClick={() => deleteRoute(rt.id)} className="text-red-400 hover:text-red-600 transition">
                      <Trash2 size={15} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* ── Usage hint ── */}
      {routes.length > 0 && (
        <div className="bg-slate-50 border border-slate-200 rounded-xl px-5 py-4 text-sm text-slate-600">
          <p className="font-medium text-slate-700 mb-1">How to use a route</p>
          <p>Send chat requests with the route slug as the <code className="bg-white border border-slate-200 px-1 rounded text-xs">model</code> value:</p>
          <pre className="mt-2 bg-white border border-slate-200 rounded-lg p-3 text-xs overflow-x-auto">{`POST /api/chat/completions
{
  "model": "${routes[0]?.slug ?? 'your-slug'}",
  "messages": [{ "role": "user", "content": "Hello!" }]
}`}</pre>
        </div>
      )}
    </div>
  );
}
