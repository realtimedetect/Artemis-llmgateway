'use client';

import { useEffect, useMemo, useState } from 'react';
import api from '@/lib/api';
import { Activity, TimerReset, AlertTriangle, RefreshCw } from 'lucide-react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts';

type ObservabilitySummary = {
  total_requests: number;
  error_requests: number;
  error_rate: number;
  throughput_rps: number;
  avg_latency_ms: number;
  p95_latency_ms: number;
  avg_ttft_ms: number;
  p95_ttft_ms: number;
};

type ObservabilityPoint = {
  minute: string;
  requests: number;
  errors: number;
  error_rate: number;
  avg_latency_ms: number;
  avg_ttft_ms: number;
  throughput_rps: number;
};

type ObservabilityPayload = {
  period: '15m' | '1h' | '24h';
  summary: ObservabilitySummary;
  timeseries: ObservabilityPoint[];
};

const EMPTY_PAYLOAD: ObservabilityPayload = {
  period: '1h',
  summary: {
    total_requests: 0,
    error_requests: 0,
    error_rate: 0,
    throughput_rps: 0,
    avg_latency_ms: 0,
    p95_latency_ms: 0,
    avg_ttft_ms: 0,
    p95_ttft_ms: 0,
  },
  timeseries: [],
};

export default function ObservabilityPage() {
  const [period, setPeriod] = useState<'15m' | '1h' | '24h'>('1h');
  const [payload, setPayload] = useState<ObservabilityPayload>(EMPTY_PAYLOAD);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);

  const chartData = useMemo(() => {
    return payload.timeseries.map((p) => ({
      ...p,
      label: formatMinute(p.minute),
      error_rate_pct: p.error_rate * 100,
    }));
  }, [payload.timeseries]);

  useEffect(() => {
    fetchMetrics(period, true);
  }, [period]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => {
      fetchMetrics(period, false);
    }, 5000);
    return () => window.clearInterval(id);
  }, [autoRefresh, period]);

  async function fetchMetrics(nextPeriod: '15m' | '1h' | '24h', showLoading: boolean) {
    if (showLoading) setLoading(true);
    setRefreshing(true);
    setError('');
    try {
      const { data } = await api.get('/api/analytics/observability', { params: { period: nextPeriod } });
      setPayload(data);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to load observability metrics.');
    } finally {
      setRefreshing(false);
      setLoading(false);
    }
  }

  function metricCard(title: string, value: string, hint: string, icon: React.ReactNode) {
    return (
      <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-xs text-slate-400 uppercase tracking-wide mb-1">{title}</p>
            <p className="text-2xl font-bold text-slate-800">{value}</p>
            <p className="text-xs text-slate-500 mt-1">{hint}</p>
          </div>
          <div className="text-slate-400">{icon}</div>
        </div>
      </div>
    );
  }

  if (loading) {
    return <div className="text-sm text-slate-500">Loading observability metrics...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-xl font-bold text-slate-800">Observability</h2>
          <p className="text-sm text-slate-500 mt-0.5">Live monitoring for TTFT, error rate, and request throughput.</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={period}
            onChange={(e) => setPeriod(e.target.value as '15m' | '1h' | '24h')}
            className="text-sm border border-slate-300 rounded-lg px-3 py-2"
          >
            <option value="15m">Last 15 min</option>
            <option value="1h">Last 1 hour</option>
            <option value="24h">Last 24 hours</option>
          </select>
          <label className="flex items-center gap-2 px-3 py-2 text-sm border border-slate-200 rounded-lg bg-white">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="h-4 w-4"
            />
            Auto-refresh (5s)
          </label>
          <button
            onClick={() => fetchMetrics(period, false)}
            className="px-3 py-2 text-sm rounded-lg border border-slate-300 bg-white hover:bg-slate-50 flex items-center gap-1.5"
          >
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} /> Refresh
          </button>
        </div>
      </div>

      {error && (
        <div className="text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">{error}</div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {metricCard('Avg TTFT', `${Math.round(payload.summary.avg_ttft_ms)} ms`, `p95 ${Math.round(payload.summary.p95_ttft_ms)} ms`, <TimerReset size={18} />)}
        {metricCard('Error Rate', `${(payload.summary.error_rate * 100).toFixed(2)}%`, `${payload.summary.error_requests}/${payload.summary.total_requests} requests`, <AlertTriangle size={18} />)}
        {metricCard('Throughput', `${payload.summary.throughput_rps.toFixed(2)} rps`, `${payload.summary.total_requests} requests in window`, <Activity size={18} />)}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
          <h3 className="text-sm font-medium text-slate-600 mb-3">TTFT Trend</h3>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="label" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Line type="monotone" dataKey="avg_ttft_ms" stroke="#0ea5e9" strokeWidth={2.5} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
          <h3 className="text-sm font-medium text-slate-600 mb-3">Error Rate Trend</h3>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="label" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Line type="monotone" dataKey="error_rate_pct" stroke="#ef4444" strokeWidth={2.5} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-xl p-5 shadow-sm border border-slate-100">
          <h3 className="text-sm font-medium text-slate-600 mb-3">Throughput Trend</h3>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="label" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Line type="monotone" dataKey="throughput_rps" stroke="#16a34a" strokeWidth={2.5} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}

function formatMinute(value: string): string {
  const parsed = new Date(value.replace(' ', 'T') + 'Z');
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
