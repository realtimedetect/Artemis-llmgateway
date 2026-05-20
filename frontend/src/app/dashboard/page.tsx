'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, LineChart, Line, CartesianGrid, ComposedChart, Legend } from 'recharts';

type Usage = {
  total_requests: number;
  total_tokens: number;
  total_cost_usd: number;
  avg_latency_ms: number;
};

type Request = {
  id: string;
  model: string;
  api_key_id?: string;
  total_tokens: number;
  latency_ms: number;
  status: number;
  cost_usd: number;
  created_at: string;
};

type ProviderHealth = {
  provider_id: string;
  provider_name: string;
  enabled: boolean;
  circuit_open: boolean;
  consecutive_failures: number;
  open_until?: string;
};

type APIKeyTimeSeriesRow = {
  day: string;
  key_id: string;
  key_name: string;
  key_prefix: string;
  requests: number;
  total_tokens: number;
  total_cost_usd: number;
};

type APIKeyCostRow = {
  key_id: string;
  key_name: string;
  key_prefix: string;
  total_cost_usd: number;
};

type APIKeyTopTokensRow = {
  key_id: string;
  key_name: string;
  key_prefix: string;
  total_tokens: number;
  requests: number;
};

type APIKeyAnalytics = {
  timeseries: APIKeyTimeSeriesRow[];
  cost_by_key: APIKeyCostRow[];
  top_keys_by_tokens: APIKeyTopTokensRow[];
};

const DEMO_API_KEY_TIMESERIES: APIKeyTimeSeriesRow[] = [
  { day: '2026-03-09', key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', requests: 34, total_tokens: 21400, total_cost_usd: 3.12 },
  { day: '2026-03-10', key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', requests: 41, total_tokens: 26600, total_cost_usd: 3.86 },
  { day: '2026-03-11', key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', requests: 29, total_tokens: 18900, total_cost_usd: 2.74 },
  { day: '2026-03-12', key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', requests: 47, total_tokens: 30100, total_cost_usd: 4.31 },
  { day: '2026-03-13', key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', requests: 52, total_tokens: 33800, total_cost_usd: 4.92 },
];

const DEMO_API_KEY_COST_BY_KEY: APIKeyCostRow[] = [
  { key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', total_cost_usd: 18.95 },
  { key_id: 'demo-key-2', key_name: 'Batch Jobs', key_prefix: 'gw_demo2', total_cost_usd: 12.44 },
  { key_id: 'demo-key-3', key_name: 'Support Bot', key_prefix: 'gw_demo3', total_cost_usd: 9.18 },
];

const DEMO_API_KEY_TOP_TOKENS: APIKeyTopTokensRow[] = [
  { key_id: 'demo-key-1', key_name: 'Web App', key_prefix: 'gw_demo1', total_tokens: 130800, requests: 203 },
  { key_id: 'demo-key-2', key_name: 'Batch Jobs', key_prefix: 'gw_demo2', total_tokens: 92500, requests: 118 },
  { key_id: 'demo-key-3', key_name: 'Support Bot', key_prefix: 'gw_demo3', total_tokens: 67300, requests: 89 },
];

type DemoCostPoint = {
  day: string;
  requests: number;
  tokens: number;
  cost: number;
};

const DEMO_COST_SERIES: DemoCostPoint[] = [
  { day: 'Mon', requests: 92, tokens: 54000, cost: 8.2 },
  { day: 'Tue', requests: 134, tokens: 79000, cost: 11.6 },
  { day: 'Wed', requests: 121, tokens: 72000, cost: 10.4 },
  { day: 'Thu', requests: 168, tokens: 101000, cost: 14.9 },
  { day: 'Fri', requests: 187, tokens: 116000, cost: 17.3 },
  { day: 'Sat', requests: 149, tokens: 93000, cost: 13.5 },
  { day: 'Sun', requests: 173, tokens: 109000, cost: 16.1 },
];

export default function DashboardPage() {
  const [usage, setUsage] = useState<Usage | null>(null);
  const [requests, setRequests] = useState<Request[]>([]);
  const [providerHealth, setProviderHealth] = useState<ProviderHealth[]>([]);
  const [apiKeyAnalytics, setApiKeyAnalytics] = useState<APIKeyAnalytics | null>(null);
  const [selectedKeyID, setSelectedKeyID] = useState<string>('');

  useEffect(() => {
    api.get('/api/usage').then((r) => setUsage(r.data));
    api.get('/api/requests').then((r) => setRequests(r.data));
    api.get('/api/providers/health').then((r) => setProviderHealth(r.data));
    api.get('/api/analytics/api-keys').then((r) => setApiKeyAnalytics(r.data));
  }, []);

  useEffect(() => {
    if (!apiKeyAnalytics || selectedKeyID) return;
    const first = apiKeyAnalytics.cost_by_key[0]?.key_id ?? '';
    if (first) setSelectedKeyID(first);
  }, [apiKeyAnalytics, selectedKeyID]);

  const hasLiveAPIKeyAnalytics = Boolean(
    apiKeyAnalytics &&
      ((apiKeyAnalytics.timeseries?.length ?? 0) > 0 ||
        (apiKeyAnalytics.cost_by_key?.length ?? 0) > 0 ||
        (apiKeyAnalytics.top_keys_by_tokens?.length ?? 0) > 0),
  );

  const effectiveAPIKeyAnalytics: APIKeyAnalytics = hasLiveAPIKeyAnalytics && apiKeyAnalytics
    ? apiKeyAnalytics
    : {
        timeseries: DEMO_API_KEY_TIMESERIES,
        cost_by_key: DEMO_API_KEY_COST_BY_KEY,
        top_keys_by_tokens: DEMO_API_KEY_TOP_TOKENS,
      };

  const effectiveSelectedKeyID = selectedKeyID || effectiveAPIKeyAnalytics.cost_by_key[0]?.key_id || '';

  const selectedKeySeries = (effectiveAPIKeyAnalytics.timeseries ?? [])
    .filter((row) => row.key_id === effectiveSelectedKeyID)
    .map((row) => ({ day: row.day, tokens: row.total_tokens, cost: row.total_cost_usd, requests: row.requests }));

  const stat = (label: string, value: string | number) => (
    <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
      <p className="text-xs text-slate-400 uppercase tracking-wide mb-1">{label}</p>
      <p className="text-2xl font-bold text-slate-800">{value}</p>
    </div>
  );

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-slate-800">Overview</h2>

      {usage && (
        <div className="grid grid-cols-2 xl:grid-cols-4 gap-4">
          {stat('Total Requests', usage.total_requests.toLocaleString())}
          {stat('Total Tokens', usage.total_tokens.toLocaleString())}
          {stat('Avg Latency', `${Math.round(usage.avg_latency_ms)} ms`)}
          {stat('Est. Cost', `$${usage.total_cost_usd.toFixed(4)}`)}
        </div>
      )}

      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
        <div className="flex items-start justify-between gap-3 mb-4">
          <div>
            <h3 className="text-sm font-medium text-slate-700">Demo Cost and Usage Trend</h3>
            <p className="text-xs text-slate-500 mt-1">
              Sample data for product demos. Shows request volume and estimated spend together.
            </p>
          </div>
          <span className="text-[11px] px-2 py-1 rounded bg-amber-100 text-amber-700">demo data</span>
        </div>
        <ResponsiveContainer width="100%" height={250}>
          <ComposedChart data={DEMO_COST_SERIES}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="day" tick={{ fontSize: 10 }} />
            <YAxis yAxisId="left" tick={{ fontSize: 10 }} />
            <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 10 }} />
            <Tooltip />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            <Bar yAxisId="left" name="Requests" dataKey="requests" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            <Line yAxisId="right" name="Cost (USD)" type="monotone" dataKey="cost" stroke="#16a34a" strokeWidth={2.5} dot />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {requests.length > 0 && (
        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
          <h3 className="text-sm font-medium text-slate-600 mb-4">Token usage (last 20)</h3>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={requests.slice(0, 20).reverse()}>
              <XAxis dataKey="model" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Bar dataKey="total_tokens" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {providerHealth.length > 0 && (
        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
          <h3 className="text-sm font-medium text-slate-600 mb-3">Provider Health & Failover</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {providerHealth.map((p) => (
              <div key={p.provider_id} className="rounded-lg border border-slate-200 px-3 py-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-slate-700">{p.provider_name}</span>
                  {p.circuit_open ? (
                    <span className="text-xs px-2 py-0.5 rounded bg-red-100 text-red-700">circuit open</span>
                  ) : (
                    <span className="text-xs px-2 py-0.5 rounded bg-emerald-100 text-emerald-700">healthy</span>
                  )}
                </div>
                <p className="text-xs text-slate-500 mt-1">
                  failures: {p.consecutive_failures} {p.open_until ? `| open until ${new Date(p.open_until).toLocaleTimeString()}` : ''}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      {effectiveAPIKeyAnalytics && (
        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100 space-y-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-medium text-slate-600">API Key Analytics</h3>
              {!hasLiveAPIKeyAnalytics && (
                <span className="text-[11px] px-2 py-1 rounded bg-amber-100 text-amber-700">demo data</span>
              )}
            </div>
            <select
              value={effectiveSelectedKeyID}
              onChange={(e) => setSelectedKeyID(e.target.value)}
              className="text-xs border border-slate-300 rounded-lg px-2 py-1.5"
            >
              {(effectiveAPIKeyAnalytics.cost_by_key ?? []).map((k) => (
                <option key={k.key_id} value={k.key_id}>{k.key_name} ({k.key_prefix})</option>
              ))}
            </select>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
            <div className="xl:col-span-2 bg-slate-50 rounded-lg border border-slate-200 p-3">
              <p className="text-xs text-slate-500 mb-2">Per key over time (tokens & cost)</p>
              <ResponsiveContainer width="100%" height={220}>
                <LineChart data={selectedKeySeries}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="day" tick={{ fontSize: 10 }} />
                  <YAxis yAxisId="left" tick={{ fontSize: 10 }} />
                  <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 10 }} />
                  <Tooltip />
                  <Line yAxisId="left" type="monotone" dataKey="tokens" stroke="#2563eb" strokeWidth={2} dot={false} />
                  <Line yAxisId="right" type="monotone" dataKey="cost" stroke="#16a34a" strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            </div>

            <div className="bg-slate-50 rounded-lg border border-slate-200 p-3">
              <p className="text-xs text-slate-500 mb-2">Cost by key</p>
              <ResponsiveContainer width="100%" height={220}>
                <BarChart data={(effectiveAPIKeyAnalytics.cost_by_key ?? []).slice(0, 10)}>
                  <XAxis dataKey="key_prefix" tick={{ fontSize: 10 }} />
                  <YAxis tick={{ fontSize: 10 }} />
                  <Tooltip />
                  <Bar dataKey="total_cost_usd" fill="#16a34a" radius={[3, 3, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="bg-slate-50 rounded-lg border border-slate-200 p-3">
            <p className="text-xs text-slate-500 mb-2">Top keys by token usage</p>
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={effectiveAPIKeyAnalytics.top_keys_by_tokens ?? []}>
                <XAxis dataKey="key_prefix" tick={{ fontSize: 10 }} />
                <YAxis tick={{ fontSize: 10 }} />
                <Tooltip />
                <Bar dataKey="total_tokens" fill="#7c3aed" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
            <tr>
              {['Model', 'Tokens', 'Cost', 'Auth', 'Latency', 'Status', 'Time'].map((h) => (
                <th key={h} className="text-left px-4 py-3">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {requests.map((req) => (
              <tr key={req.id} className="hover:bg-slate-50">
                <td className="px-4 py-3 font-mono text-xs">{req.model}</td>
                <td className="px-4 py-3">{req.total_tokens}</td>
                <td className="px-4 py-3">${(req.cost_usd ?? 0).toFixed(4)}</td>
                <td className="px-4 py-3 text-xs text-slate-500">{req.api_key_id ? 'api-key' : 'jwt'}</td>
                <td className="px-4 py-3">{req.latency_ms} ms</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                    req.status === 200 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                  }`}>{req.status}</span>
                </td>
                <td className="px-4 py-3 text-slate-400 text-xs">
                  {new Date(req.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
