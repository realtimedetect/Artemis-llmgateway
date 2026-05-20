'use client';

import { FormEvent, useState } from 'react';
import { Sparkles, AlertCircle, CheckCircle2, Wrench, ChevronDown, ChevronRight } from 'lucide-react';
import api from '@/lib/api';

type AgentToolEvent = {
  tool: string;
  args?: Record<string, unknown>;
  result: string;
};

type AgentStep = {
  index: number;
  title: string;
  objective: string;
  output: string;
  tool_calls?: AgentToolEvent[];
};

type AgentRunResponse = {
  model: string;
  input: string;
  use_tools: boolean;
  total_steps: number;
  steps: AgentStep[];
  final_answer: string;
};

const TOOL_LABELS: Record<string, string> = {
  query_usage_analytics: 'Usage Analytics',
  list_routes:           'List Routes',
  list_providers:        'List Providers',
};

function ToolCallCard({ event }: { event: AgentToolEvent }) {
  const [open, setOpen] = useState(false);
  let parsed: unknown = null;
  try { parsed = JSON.parse(event.result); } catch { /* raw string */ }
  return (
    <div className="border border-violet-200 rounded-md bg-violet-50 text-xs">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 w-full px-2.5 py-1.5 text-left text-violet-700 font-medium"
      >
        <Wrench size={11} />
        <span>{TOOL_LABELS[event.tool] ?? event.tool}</span>
        {open ? <ChevronDown size={11} className="ml-auto" /> : <ChevronRight size={11} className="ml-auto" />}
      </button>
      {open && (
        <div className="px-2.5 pb-2 space-y-1">
          <pre className="bg-white border border-violet-100 rounded p-1.5 text-[11px] overflow-auto max-h-40 text-slate-700 whitespace-pre-wrap">
            {parsed !== null ? JSON.stringify(parsed, null, 2) : event.result}
          </pre>
        </div>
      )}
    </div>
  );
}

export default function AgenticPage() {
  const [model, setModel] = useState('gpt-4o-mini');
  const [input, setInput] = useState('Create a migration checklist for moving a REST API to this gateway.');
  const [instructions, setInstructions] = useState('Prefer concise, implementation-focused output.');
  const [maxSteps, setMaxSteps] = useState(3);
  const [useTools, setUseTools] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<AgentRunResponse | null>(null);

  async function runAgent(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');
    setResult(null);
    try {
      const res = await api.post<AgentRunResponse>('/api/agent/run', {
        model,
        input,
        instructions,
        max_steps: maxSteps,
        use_tools: useTools,
      });
      setResult(res.data);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to run agent workflow');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-bold text-slate-800">Agentic AI</h2>
        <p className="text-sm text-slate-500 mt-1">
          Multi-step planner/executor through the gateway. Uses your configured providers, policies, and quota limits.
        </p>
      </div>

      <div className="bg-white rounded-xl p-5 border border-slate-100 shadow-sm">
        <form onSubmit={runAgent} className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Model or Route Slug</label>
              <input
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full px-3 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Max Steps</label>
              <input
                type="number"
                min={1}
                max={8}
                value={maxSteps}
                onChange={(e) => setMaxSteps(Number(e.target.value))}
                className="w-full px-3 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Task</label>
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              rows={4}
              required
              className="w-full px-3 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Agent Instructions (Optional)</label>
            <textarea
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
              rows={2}
              className="w-full px-3 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          {/* Gateway tools toggle */}
          <div className="flex items-start gap-3 p-3 bg-violet-50 border border-violet-200 rounded-lg">
            <input
              id="use-tools"
              type="checkbox"
              checked={useTools}
              onChange={(e) => setUseTools(e.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-violet-300 accent-violet-600 cursor-pointer"
            />
            <label htmlFor="use-tools" className="cursor-pointer">
              <span className="text-sm font-medium text-violet-800 flex items-center gap-1.5">
                <Wrench size={13} /> Enable Gateway Tools
              </span>
              <p className="text-xs text-violet-600 mt-0.5">
                Lets the agent call <code className="bg-violet-100 px-1 rounded">query_usage_analytics</code>,{' '}
                <code className="bg-violet-100 px-1 rounded">list_routes</code>, and{' '}
                <code className="bg-violet-100 px-1 rounded">list_providers</code> in real time. Requires a provider that supports function calling (e.g. OpenAI GPT-4o).
              </p>
            </label>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition disabled:opacity-60"
          >
            <Sparkles size={16} />
            {loading ? 'Running agent...' : 'Run Agent Workflow'}
          </button>
        </form>
      </div>

      {error && (
        <div className="flex items-start gap-2 text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
          <AlertCircle size={16} className="mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {result && (
        <div className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg px-3 py-2">
            <CheckCircle2 size={16} className="mt-0.5" />
            <span>
              Completed {result.total_steps} step{result.total_steps !== 1 ? 's' : ''} using {result.model}.
              {result.use_tools && (
                <span className="ml-2 inline-flex items-center gap-1 text-violet-700 bg-violet-100 px-1.5 py-0.5 rounded text-xs font-medium">
                  <Wrench size={10} /> tools enabled
                </span>
              )}
            </span>
          </div>

          <div className="bg-white rounded-xl p-5 border border-slate-100 shadow-sm space-y-3">
            <h3 className="font-semibold text-slate-800">Execution Steps</h3>
            {result.steps.map((step) => (
              <div key={step.index} className="border border-slate-200 rounded-lg p-3 space-y-2">
                <div className="text-sm font-medium text-slate-800">{step.index}. {step.title}</div>
                <div className="text-xs text-slate-500">{step.objective}</div>

                {/* Tool call events */}
                {step.tool_calls && step.tool_calls.length > 0 && (
                  <div className="space-y-1.5">
                    <div className="text-xs font-medium text-violet-700 flex items-center gap-1">
                      <Wrench size={11} /> Tool calls ({step.tool_calls.length})
                    </div>
                    {step.tool_calls.map((tc, idx) => (
                      <ToolCallCard key={idx} event={tc} />
                    ))}
                  </div>
                )}

                <div className="text-sm text-slate-700 whitespace-pre-wrap">{step.output}</div>
              </div>
            ))}
          </div>

          <div className="bg-white rounded-xl p-5 border border-slate-100 shadow-sm">
            <h3 className="font-semibold text-slate-800 mb-2">Final Answer</h3>
            <p className="text-sm text-slate-700 whitespace-pre-wrap">{result.final_answer}</p>
          </div>
        </div>
      )}
    </div>
  );
}
