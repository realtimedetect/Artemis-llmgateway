'use client';

import { useEffect, useRef, useState } from 'react';
import api from '@/lib/api';
import ReactMarkdown from 'react-markdown';
import { Send, ToggleLeft, ToggleRight } from 'lucide-react';

type Message = { role: 'user' | 'assistant'; content: string };

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [model, setModel] = useState('gpt-4o-mini');
  const [stream, setStream] = useState(false);
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  async function sendMessage() {
    if (!input.trim() || loading) return;

    const userMsg: Message = { role: 'user', content: input.trim() };
    const history = [...messages, userMsg];
    setMessages(history);
    setInput('');
    setLoading(true);

    try {
      if (stream) {
        const raw = localStorage.getItem('auth-storage');
        const token: string | undefined = raw ? JSON.parse(raw)?.state?.token : undefined;
        const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL ?? ''}/api/chat/completions`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            model,
            messages: history,
            stream: true,
          }),
        });

        if (!res.ok || !res.body) {
          throw new Error('stream request failed');
        }

        // Start an empty assistant message and fill it as stream chunks arrive.
        setMessages([...history, { role: 'assistant', content: '' }]);
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let pending = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          pending += decoder.decode(value, { stream: true });
          const lines = pending.split('\n');
          pending = lines.pop() ?? '';

          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed.startsWith('data:')) continue;
            const data = trimmed.replace(/^data:\s*/, '');
            if (data === '[DONE]') continue;

            try {
              const parsed = JSON.parse(data);
              const delta: string =
                parsed?.choices?.[0]?.delta?.content ??
                parsed?.choices?.[0]?.message?.content ??
                '';
              if (!delta) continue;
              setMessages((prev) => {
                if (prev.length === 0) return prev;
                const copy = [...prev];
                const last = copy[copy.length - 1];
                if (last.role === 'assistant') {
                  copy[copy.length - 1] = { ...last, content: last.content + delta };
                }
                return copy;
              });
            } catch {
              // Ignore malformed non-JSON stream lines.
            }
          }
        }

        setMessages((prev) => {
          const copy = [...prev];
          const last = copy[copy.length - 1];
          if (last?.role === 'assistant' && !last.content.trim()) {
            copy[copy.length - 1] = { ...last, content: '(no response)' };
          }
          return copy;
        });
        return;
      }

      const { data } = await api.post('/api/chat/completions', {
        model,
        messages: history,
        stream: false,
      });
      const assistantContent: string =
        data?.choices?.[0]?.message?.content ?? '(no response)';
      setMessages([...history, { role: 'assistant', content: assistantContent }]);
    } catch {
      setMessages([...history, { role: 'assistant', content: '⚠️ Error reaching the gateway.' }]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col h-full max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-slate-800">Chat</h2>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setStream((s) => !s)}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border text-xs font-medium transition ${
              stream
                ? 'border-emerald-300 bg-emerald-50 text-emerald-700'
                : 'border-slate-300 bg-white text-slate-600'
            }`}
            title="Toggle stream=true in chat requests"
          >
            {stream ? <ToggleRight size={16} /> : <ToggleLeft size={16} />}
            Stream {stream ? 'ON' : 'OFF'}
          </button>
          <select
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className="text-sm border border-slate-300 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="gpt-4o-mini">gpt-4o-mini</option>
            <option value="gpt-4o">gpt-4o</option>
            <option value="claude-3-haiku-20240307">claude-3-haiku</option>
            <option value="claude-3-5-sonnet-20241022">claude-3.5-sonnet</option>
            <option value="gemini-1.5-flash">gemini-1.5-flash</option>
          </select>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto space-y-4 mb-4 pr-1">
        {messages.length === 0 && (
          <div className="text-center text-slate-400 mt-20 text-sm">
            Start a conversation with your LLM gateway.
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
            <div
              className={`max-w-[80%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
                msg.role === 'user'
                  ? 'bg-brand-600 text-white rounded-tr-sm'
                  : 'bg-white border border-slate-200 text-slate-800 rounded-tl-sm shadow-sm'
              }`}
            >
              <ReactMarkdown>{msg.content}</ReactMarkdown>
            </div>
          </div>
        ))}
        {loading && (
          <div className="flex justify-start">
            <div className="bg-white border border-slate-200 rounded-2xl rounded-tl-sm px-4 py-3 shadow-sm">
              <span className="inline-flex gap-1">
                {[0, 1, 2].map((d) => (
                  <span
                    key={d}
                    className="w-2 h-2 bg-slate-400 rounded-full animate-bounce"
                    style={{ animationDelay: `${d * 150}ms` }}
                  />
                ))}
              </span>
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      <div className="flex gap-2">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && sendMessage()}
          placeholder="Type a message…"
          className="flex-1 px-4 py-3 border border-slate-300 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        <button
          onClick={sendMessage}
          disabled={loading || !input.trim()}
          className="p-3 bg-brand-600 hover:bg-brand-700 text-white rounded-xl transition disabled:opacity-50"
        >
          <Send size={18} />
        </button>
      </div>
    </div>
  );
}
