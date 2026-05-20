'use client';

import { useEffect, useState } from 'react';
import api from '@/lib/api';
import { Plus, FlaskConical, CheckCircle2 } from 'lucide-react';

type PromptTemplate = {
  id: string;
  name: string;
  description: string;
  active_version_id?: string;
};

type PromptVersion = {
  id: string;
  version: number;
  content: string;
  test_input?: string;
  test_output?: string;
  test_status: number;
  is_active: boolean;
  created_at: string;
};

export default function PromptsPage() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([]);
  const [selectedTemplateID, setSelectedTemplateID] = useState('');
  const [versions, setVersions] = useState<PromptVersion[]>([]);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [content, setContent] = useState('');

  const [newVersionContent, setNewVersionContent] = useState('');
  const [setActive, setSetActive] = useState(true);

  const [testInput, setTestInput] = useState('');
  const [testModel, setTestModel] = useState('gpt-4o-mini');
  const [testOutput, setTestOutput] = useState('');
  const [message, setMessage] = useState('');

  useEffect(() => {
    loadTemplates();
  }, []);

  useEffect(() => {
    if (selectedTemplateID) {
      loadVersions(selectedTemplateID);
    }
  }, [selectedTemplateID]);

  async function loadTemplates() {
    const { data } = await api.get('/api/prompts/templates');
    setTemplates(data);
    if (!selectedTemplateID && data.length > 0) {
      setSelectedTemplateID(data[0].id);
    }
  }

  async function loadVersions(templateID: string) {
    const { data } = await api.get(`/api/prompts/templates/${templateID}/versions`);
    setVersions(data);
  }

  async function createTemplate() {
    setMessage('');
    await api.post('/api/prompts/templates', { name, description, content });
    setName('');
    setDescription('');
    setContent('');
    setMessage('Template created.');
    await loadTemplates();
  }

  async function createVersion() {
    if (!selectedTemplateID) return;
    setMessage('');
    await api.post(`/api/prompts/templates/${selectedTemplateID}/versions`, {
      content: newVersionContent,
      set_active: setActive,
    });
    setNewVersionContent('');
    setMessage('New version created.');
    await loadVersions(selectedTemplateID);
    await loadTemplates();
  }

  async function activateVersion(versionID: string) {
    if (!selectedTemplateID) return;
    await api.put(`/api/prompts/templates/${selectedTemplateID}/active`, { version_id: versionID });
    setMessage('Active version updated.');
    await loadVersions(selectedTemplateID);
    await loadTemplates();
  }

  async function testPrompt() {
    if (!selectedTemplateID) return;
    const activeVersion = versions.find((v) => v.is_active);
    const { data } = await api.post('/api/prompts/test', {
      template_id: selectedTemplateID,
      version_id: activeVersion?.id,
      input: testInput,
      model: testModel,
    });
    setTestOutput(data.output ?? '');
    setMessage(`Test complete (status ${data.status}).`);
    await loadVersions(selectedTemplateID);
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-bold text-slate-800">Prompt Management</h2>
        <p className="text-sm text-slate-500 mt-1">Create prompt templates, publish versions, and test centrally.</p>
      </div>

      {message && (
        <div className="text-sm text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-lg px-3 py-2 flex items-center gap-2">
          <CheckCircle2 size={15} />
          {message}
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-5">
        <div className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
          <h3 className="font-semibold text-slate-700">Create Template</h3>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Template name" className="w-full px-3 py-2 border rounded-lg text-sm" />
          <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Description" className="w-full px-3 py-2 border rounded-lg text-sm" />
          <textarea value={content} onChange={(e) => setContent(e.target.value)} placeholder="Initial prompt content" rows={6} className="w-full px-3 py-2 border rounded-lg text-sm" />
          <button onClick={createTemplate} className="px-4 py-2 bg-brand-600 text-white rounded-lg text-sm flex items-center gap-1.5">
            <Plus size={15} /> Create Template
          </button>
        </div>

        <div className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
          <h3 className="font-semibold text-slate-700">Template Versions</h3>
          <select value={selectedTemplateID} onChange={(e) => setSelectedTemplateID(e.target.value)} className="w-full px-3 py-2 border rounded-lg text-sm">
            <option value="">Select template…</option>
            {templates.map((t) => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </select>

          <textarea value={newVersionContent} onChange={(e) => setNewVersionContent(e.target.value)} placeholder="New version content" rows={5} className="w-full px-3 py-2 border rounded-lg text-sm" />
          <label className="text-sm text-slate-700 flex items-center gap-2">
            <input type="checkbox" checked={setActive} onChange={(e) => setSetActive(e.target.checked)} /> Set as active version
          </label>
          <button onClick={createVersion} className="px-4 py-2 bg-slate-800 text-white rounded-lg text-sm">Publish Version</button>

          <div className="space-y-2 max-h-52 overflow-auto pr-1">
            {versions.map((v) => (
              <div key={v.id} className="border rounded-lg p-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-medium">v{v.version} {v.is_active ? '(active)' : ''}</span>
                  {!v.is_active && (
                    <button onClick={() => activateVersion(v.id)} className="text-xs px-2 py-1 rounded bg-brand-50 text-brand-700">Activate</button>
                  )}
                </div>
                <pre className="mt-2 text-xs text-slate-600 whitespace-pre-wrap">{v.content}</pre>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="bg-white border border-slate-200 rounded-xl p-5 space-y-3">
        <h3 className="font-semibold text-slate-700">Test Active Prompt</h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <input value={testModel} onChange={(e) => setTestModel(e.target.value)} placeholder="Model" className="w-full px-3 py-2 border rounded-lg text-sm" />
          <input value={testInput} onChange={(e) => setTestInput(e.target.value)} placeholder="Test input" className="w-full px-3 py-2 border rounded-lg text-sm" />
        </div>
        <button onClick={testPrompt} className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm flex items-center gap-1.5">
          <FlaskConical size={15} /> Run Test
        </button>
        <textarea value={testOutput} readOnly rows={8} className="w-full px-3 py-2 border rounded-lg text-sm font-mono bg-slate-50" />
      </div>
    </div>
  );
}
