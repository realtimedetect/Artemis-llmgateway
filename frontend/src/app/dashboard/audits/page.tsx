'use client';

import { Fragment, useEffect, useMemo, useState } from 'react';
import api from '@/lib/api';
import { ChevronDown, ChevronRight, RefreshCw } from 'lucide-react';

type AuditLog = {
  id: string;
  request_id: string;
  endpoint: string;
  direction: string;
  route_slug: string;
  model: string;
  http_status: number;
  latency_ms: number;
  success: boolean;
  error: string;
  payload: string;
  created_at: string;
  api_key_id?: string;
  provider_id?: string;
};

type Filters = {
  request_id: string;
  endpoint: string;
  direction: string;
  status: string;
  from: string;
  to: string;
};

const DEFAULT_FILTERS: Filters = {
  request_id: '',
  endpoint: '',
  direction: '',
  status: '',
  from: '',
  to: '',
};

export default function AuditsPage() {
  const [rows, setRows] = useState<AuditLog[]>([]);
  const [filters, setFilters] = useState<Filters>({ ...DEFAULT_FILTERS });
  const [loading, setLoading] = useState(false);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const hasFilters = useMemo(
    () => Object.values(filters).some((v) => v !== ''),
    [filters],
  );

  useEffect(() => {
    fetchAudits();
  }, []);

  async function fetchAudits(customFilters?: Filters) {
    setLoading(true);
    try {
      const active = customFilters ?? filters;
      const params: Record<string, string | number> = { limit: 200 };
      if (active.request_id) params.request_id = active.request_id;
      if (active.endpoint) params.endpoint = active.endpoint;
      if (active.direction) params.direction = active.direction;
      if (active.status) params.status = active.status;
      if (active.from) params.from = active.from;
      if (active.to) params.to = active.to;

      const { data } = await api.get('/api/audits', { params });
      setRows(data);
    } finally {
      setLoading(false);
    }
  }

  function applyFilters() {
    fetchAudits(filters);
  }

  function resetFilters() {
    const cleared = { ...DEFAULT_FILTERS };
    setFilters(cleared);
    fetchAudits(cleared);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-slate-800">Audit Logs</h2>
          <p className="text-sm text-slate-500 mt-0.5">
            Track gateway-to-LLM and LLM-to-gateway traffic with full payload traces.
          </p>
        </div>
        <button
          onClick={() => fetchAudits()}
          className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-white border border-slate-300 hover:bg-slate-50 rounded-lg text-sm text-slate-700"
        >
          <RefreshCw size={15} /> Refresh
        </button>
      </div>

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-4 space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-3">
          <input
            value={filters.request_id}
            onChange={(e) => setFilters((f) => ({ ...f, request_id: e.target.value }))}
            placeholder="Request ID"
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          />

          <select
            value={filters.endpoint}
            onChange={(e) => setFilters((f) => ({ ...f, endpoint: e.target.value }))}
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          >
            <option value="">All endpoints</option>
            <option value="/chat/completions">/chat/completions</option>
            <option value="/embeddings">/embeddings</option>
          </select>

          <select
            value={filters.direction}
            onChange={(e) => setFilters((f) => ({ ...f, direction: e.target.value }))}
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          >
            <option value="">All directions</option>
            <option value="gateway_to_llm">gateway_to_llm</option>
            <option value="llm_to_gateway">llm_to_gateway</option>
          </select>

          <input
            value={filters.status}
            onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value.replace(/[^0-9]/g, '') }))}
            placeholder="HTTP status"
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          />

          <input
            type="date"
            value={filters.from}
            onChange={(e) => setFilters((f) => ({ ...f, from: e.target.value }))}
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          />

          <input
            type="date"
            value={filters.to}
            onChange={(e) => setFilters((f) => ({ ...f, to: e.target.value }))}
            className="px-3 py-2 border border-slate-300 rounded-lg text-sm"
          />
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={applyFilters}
            className="px-3.5 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-lg text-sm"
          >
            Apply Filters
          </button>
          <button
            onClick={resetFilters}
            disabled={!hasFilters}
            className="px-3.5 py-2 border border-slate-300 hover:bg-slate-50 disabled:opacity-50 rounded-lg text-sm"
          >
            Clear
          </button>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
            <tr>
              {['', 'Time', 'Request ID', 'Endpoint', 'Direction', 'Model', 'Status', 'Latency', 'Success'].map((h) => (
                <th key={h} className="text-left px-4 py-3">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={9} className="px-4 py-8 text-center text-slate-400 text-sm">
                  No audit logs found for current filters.
                </td>
              </tr>
            )}
            {rows.map((item) => {
              const expanded = expandedId === item.id;
              return (
                <Fragment key={item.id}>
                  <tr className="hover:bg-slate-50">
                    <td className="px-4 py-3">
                      <button
                        onClick={() => setExpandedId(expanded ? null : item.id)}
                        className="text-slate-400 hover:text-slate-700"
                      >
                        {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                      </button>
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-500">{new Date(item.created_at).toLocaleString()}</td>
                    <td className="px-4 py-3 font-mono text-xs">{item.request_id}</td>
                    <td className="px-4 py-3 font-mono text-xs">{item.endpoint}</td>
                    <td className="px-4 py-3 text-xs">{item.direction}</td>
                    <td className="px-4 py-3 font-mono text-xs">{item.model || '-'}</td>
                    <td className="px-4 py-3 text-xs">{item.http_status || '-'}</td>
                    <td className="px-4 py-3 text-xs">{item.latency_ms} ms</td>
                    <td className="px-4 py-3 text-xs">
                      {item.success ? (
                        <span className="px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700">yes</span>
                      ) : (
                        <span className="px-2 py-0.5 rounded-full bg-red-100 text-red-700">no</span>
                      )}
                    </td>
                  </tr>
                  {expanded && (
                    <tr>
                      <td colSpan={9} className="px-4 pb-4 pt-1 bg-slate-50">
                        <div className="space-y-2">
                          {item.error && (
                            <div className="text-xs text-red-700 bg-red-50 border border-red-200 rounded px-3 py-2">
                              {item.error}
                            </div>
                          )}
                          <div className="text-xs text-slate-600">
                            <span className="font-medium">Route:</span> {item.route_slug || '-'}
                            <span className="mx-3 text-slate-300">|</span>
                            <span className="font-medium">Provider:</span> {item.provider_id || '-'}
                            <span className="mx-3 text-slate-300">|</span>
                            <span className="font-medium">API Key:</span> {item.api_key_id || '-'}
                          </div>
                          <pre className="text-xs bg-white border border-slate-200 rounded-lg p-3 overflow-x-auto max-h-[320px] overflow-y-auto whitespace-pre-wrap break-all">
                            {item.payload || '(no payload)'}
                          </pre>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
