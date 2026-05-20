'use client';

import { FormEvent, useEffect, useState } from 'react';
import { Plus, Trash2, Users, BarChart3, Mail, X, Check, AlertCircle, Loader } from 'lucide-react';
import api from '@/lib/api';

type UserGroup = {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  created_at: string;
  member_count: number;
};

type GroupMember = {
  id: string;
  group_id: string;
  user_id: string;
  email: string;
  role: string;
  created_at: string;
};

type GroupAnalytics = {
  group_id: string;
  group_name: string;
  total_requests: number;
  total_tokens: number;
  total_cost_usd: number;
  avg_latency_ms: number;
  member_count: number;
  top_model: string;
  top_provider: string;
};

type MemberBreakdown = {
  user_id: string;
  email: string;
  total_requests: number;
  total_tokens: number;
  total_cost_usd: number;
};

export default function UserGroupsPage() {
  const [groups, setGroups] = useState<UserGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Create group form
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [groupName, setGroupName] = useState('');
  const [groupDescription, setGroupDescription] = useState('');
  const [creatingGroup, setCreatingGroup] = useState(false);

  // Selected group details
  const [selectedGroupID, setSelectedGroupID] = useState<string | null>(null);
  const [selectedMembers, setSelectedMembers] = useState<GroupMember[]>([]);
  const [selectedAnalytics, setSelectedAnalytics] = useState<GroupAnalytics | null>(null);
  const [selectedBreakdown, setSelectedBreakdown] = useState<MemberBreakdown[]>([]);
  const [analyticsPeriod, setAnalyticsPeriod] = useState<'today' | '7d' | '30d'>('30d');
  const [loadingAnalytics, setLoadingAnalytics] = useState(false);

  // Add member form
  const [showAddMemberForm, setShowAddMemberForm] = useState(false);
  const [memberEmail, setMemberEmail] = useState('');
  const [memberRole, setMemberRole] = useState('member');
  const [addingMember, setAddingMember] = useState(false);

  useEffect(() => {
    fetchGroups();
  }, []);

  async function fetchGroups() {
    setLoading(true);
    setError('');
    try {
      const { data } = await api.get<UserGroup[]>('/api/user-groups');
      setGroups(data ?? []);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to load groups');
    } finally {
      setLoading(false);
    }
  }

  async function createGroup(e: FormEvent) {
    e.preventDefault();
    if (!groupName.trim()) {
      setError('Group name is required');
      return;
    }

    setError('');
    setSuccess('');
    setCreatingGroup(true);

    try {
      await api.post('/api/user-groups', {
        name: groupName.trim(),
        description: groupDescription.trim(),
      });
      setGroupName('');
      setGroupDescription('');
      setShowCreateForm(false);
      setSuccess('Group created successfully!');
      await fetchGroups();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to create group');
    } finally {
      setCreatingGroup(false);
    }
  }

  async function deleteGroup(id: string) {
    if (!confirm('Are you sure you want to delete this group?')) return;

    setError('');
    try {
      await api.delete(`/api/user-groups/${id}`);
      if (selectedGroupID === id) {
        setSelectedGroupID(null);
        setSelectedMembers([]);
        setSelectedAnalytics(null);
      }
      setSuccess('Group deleted successfully');
      await fetchGroups();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to delete group');
    }
  }

  async function selectGroup(groupID: string) {
    setSelectedGroupID(groupID);
    setError('');
    setLoadingAnalytics(true);

    try {
      const [membersRes, analyticsRes, breakdownRes] = await Promise.all([
        api.get<GroupMember[]>(`/api/user-groups/${groupID}/members`),
        api.get<GroupAnalytics>(`/api/user-groups/${groupID}/analytics?period=30d`),
        api.get<MemberBreakdown[]>(`/api/user-groups/${groupID}/breakdown?period=30d`),
      ]);

      setSelectedMembers(membersRes.data ?? []);
      setSelectedAnalytics(analyticsRes.data ?? null);
      setSelectedBreakdown(breakdownRes.data ?? []);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to load group details');
    } finally {
      setLoadingAnalytics(false);
    }
  }

  async function updateAnalyticsPeriod(period: 'today' | '7d' | '30d') {
    if (!selectedGroupID) return;

    setAnalyticsPeriod(period);
    setLoadingAnalytics(true);

    try {
      const [analyticsRes, breakdownRes] = await Promise.all([
        api.get<GroupAnalytics>(`/api/user-groups/${selectedGroupID}/analytics?period=${period}`),
        api.get<MemberBreakdown[]>(`/api/user-groups/${selectedGroupID}/breakdown?period=${period}`),
      ]);

      setSelectedAnalytics(analyticsRes.data ?? null);
      setSelectedBreakdown(breakdownRes.data ?? []);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to load analytics');
    } finally {
      setLoadingAnalytics(false);
    }
  }

  async function addMember(e: FormEvent) {
    e.preventDefault();
    if (!selectedGroupID || !memberEmail.trim()) {
      setError('Email is required');
      return;
    }

    setError('');
    setSuccess('');
    setAddingMember(true);

    try {
      await api.post(`/api/user-groups/${selectedGroupID}/members`, {
        email: memberEmail.trim(),
        role: memberRole,
      });
      setMemberEmail('');
      setMemberRole('member');
      setShowAddMemberForm(false);
      setSuccess('Member added successfully!');
      await selectGroup(selectedGroupID);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to add member');
    } finally {
      setAddingMember(false);
    }
  }

  async function removeMember(memberID: string) {
    if (!selectedGroupID) return;
    if (!confirm('Remove this member from the group?')) return;

    setError('');
    try {
      await api.delete(`/api/user-groups/${selectedGroupID}/members/${memberID}`);
      setSuccess('Member removed successfully');
      await selectGroup(selectedGroupID);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg ?? 'Failed to remove member');
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-slate-800">User Groups & Teams</h2>
          <p className="text-sm text-slate-500 mt-1">
            Organize users into groups and track token usage by team, department, or project.
          </p>
        </div>
        <button
          onClick={() => setShowCreateForm(true)}
          className="flex items-center gap-2 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium rounded-lg transition"
        >
          <Plus size={16} /> New Group
        </button>
      </div>

      {/* Messages */}
      {error && (
        <div className="flex items-start gap-2 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
          <AlertCircle size={16} className="mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}
      {success && (
        <div className="flex items-start gap-2 p-3 bg-emerald-50 border border-emerald-200 rounded-lg text-sm text-emerald-700">
          <Check size={16} className="mt-0.5 flex-shrink-0" />
          <span>{success}</span>
        </div>
      )}

      {/* Create Group Form */}
      {showCreateForm && (
        <div className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="font-semibold text-slate-800">Create New Group</h3>
            <button
              onClick={() => setShowCreateForm(false)}
              className="text-slate-400 hover:text-slate-600"
            >
              <X size={20} />
            </button>
          </div>

          <form onSubmit={createGroup} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">
                Group Name
              </label>
              <input
                type="text"
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
                placeholder="e.g., Analytics Team, Production Env"
                className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">
                Description (optional)
              </label>
              <textarea
                value={groupDescription}
                onChange={(e) => setGroupDescription(e.target.value)}
                placeholder="Brief description of this group's purpose"
                rows={3}
                className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>

            <div className="flex gap-3">
              <button
                type="submit"
                disabled={creatingGroup}
                className="flex-1 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 disabled:bg-slate-400 text-white text-sm font-medium rounded-lg transition"
              >
                {creatingGroup ? 'Creating...' : 'Create Group'}
              </button>
              <button
                type="button"
                onClick={() => setShowCreateForm(false)}
                className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 text-slate-700 text-sm font-medium rounded-lg transition"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Groups List */}
        <div className="lg:col-span-1">
          <div className="bg-white rounded-xl border border-slate-200 overflow-hidden">
            <div className="p-4 border-b border-slate-100 bg-slate-50">
              <h3 className="font-semibold text-slate-800 text-sm">
                {loading ? 'Loading groups...' : `${groups.length} Groups`}
              </h3>
            </div>

            <div className="divide-y divide-slate-100 max-h-96 overflow-y-auto">
              {groups.length === 0 ? (
                <div className="p-4 text-center text-slate-500 text-sm">
                  No groups yet. Create one to get started.
                </div>
              ) : (
                groups.map((g) => (
                  <div
                    key={g.id}
                    className={`p-4 cursor-pointer transition ${
                      selectedGroupID === g.id
                        ? 'bg-brand-50 border-l-4 border-brand-600'
                        : 'hover:bg-slate-50'
                    }`}
                    onClick={() => selectGroup(g.id)}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <h4 className="font-medium text-slate-800 text-sm truncate">
                          {g.name}
                        </h4>
                        <p className="text-xs text-slate-500 mt-1 line-clamp-2">
                          {g.description || 'No description'}
                        </p>
                        <div className="flex items-center gap-1 text-xs text-slate-500 mt-2">
                          <Users size={14} />
                          {g.member_count} member{g.member_count !== 1 ? 's' : ''}
                        </div>
                      </div>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          deleteGroup(g.id);
                        }}
                        className="text-red-500 hover:text-red-700 flex-shrink-0"
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>

        {/* Group Details */}
        <div className="lg:col-span-2 space-y-6">
          {selectedGroupID ? (
            <>
              {/* Analytics Summary */}
              {selectedAnalytics && !loadingAnalytics && (
                <div className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="font-semibold text-slate-800 flex items-center gap-2">
                      <BarChart3 size={18} />
                      Group Analytics
                    </h3>
                    <div className="flex gap-2">
                      {[
                        { label: 'Today', value: 'today' as const },
                        { label: '7d', value: '7d' as const },
                        { label: '30d', value: '30d' as const },
                      ].map((p) => (
                        <button
                          key={p.value}
                          onClick={() => updateAnalyticsPeriod(p.value)}
                          className={`px-3 py-1 text-xs font-medium rounded transition ${
                            analyticsPeriod === p.value
                              ? 'bg-brand-600 text-white'
                              : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
                          }`}
                        >
                          {p.label}
                        </button>
                      ))}
                    </div>
                  </div>

                  {loadingAnalytics && (
                    <div className="flex items-center gap-2 text-slate-500 text-sm">
                      <Loader size={16} className="animate-spin" />
                      Loading...
                    </div>
                  )}

                  {!loadingAnalytics && (
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                      <div>
                        <p className="text-xs text-slate-500 uppercase tracking-wide">
                          Requests
                        </p>
                        <p className="text-lg font-semibold text-slate-800 mt-1">
                          {selectedAnalytics.total_requests.toLocaleString()}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500 uppercase tracking-wide">
                          Tokens
                        </p>
                        <p className="text-lg font-semibold text-slate-800 mt-1">
                          {(selectedAnalytics.total_tokens / 1_000_000).toFixed(2)}M
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500 uppercase tracking-wide">
                          Cost
                        </p>
                        <p className="text-lg font-semibold text-slate-800 mt-1">
                          ${selectedAnalytics.total_cost_usd.toFixed(4)}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-slate-500 uppercase tracking-wide">
                          Avg Latency
                        </p>
                        <p className="text-lg font-semibold text-slate-800 mt-1">
                          {Math.round(selectedAnalytics.avg_latency_ms)}ms
                        </p>
                      </div>
                    </div>
                  )}

                  {!loadingAnalytics && selectedAnalytics.top_model && (
                    <div className="pt-4 border-t border-slate-100 grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <p className="text-slate-500">Top Model</p>
                        <p className="font-medium text-slate-800 mt-1">
                          {selectedAnalytics.top_model}
                        </p>
                      </div>
                      <div>
                        <p className="text-slate-500">Top Provider</p>
                        <p className="font-medium text-slate-800 mt-1">
                          {selectedAnalytics.top_provider}
                        </p>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Members Breakdown */}
              {selectedBreakdown.length > 0 && !loadingAnalytics && (
                <div className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
                  <h3 className="font-semibold text-slate-800">Usage By Member</h3>

                  <div className="overflow-x-auto border border-slate-100 rounded-lg">
                    <table className="w-full text-sm">
                      <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
                        <tr>
                          <th className="text-left px-4 py-3">Email</th>
                          <th className="text-right px-4 py-3">Requests</th>
                          <th className="text-right px-4 py-3">Tokens</th>
                          <th className="text-right px-4 py-3">Cost</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100">
                        {selectedBreakdown.map((m) => (
                          <tr key={m.user_id} className="hover:bg-slate-50">
                            <td className="px-4 py-3 text-slate-700 font-medium">{m.email}</td>
                            <td className="px-4 py-3 text-right text-slate-600">
                              {m.total_requests.toLocaleString()}
                            </td>
                            <td className="px-4 py-3 text-right text-slate-600">
                              {(m.total_tokens / 1_000_000).toFixed(2)}M
                            </td>
                            <td className="px-4 py-3 text-right text-slate-600">
                              ${m.total_cost_usd.toFixed(4)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Members List */}
              <div className="bg-white rounded-xl border border-slate-200 p-6 space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-slate-800 flex items-center gap-2">
                    <Users size={18} />
                    Members ({selectedMembers.length})
                  </h3>
                  <button
                    onClick={() => setShowAddMemberForm(!showAddMemberForm)}
                    className="flex items-center gap-1 px-3 py-1.5 bg-brand-600 hover:bg-brand-700 text-white text-xs font-medium rounded transition"
                  >
                    <Plus size={14} /> Add Member
                  </button>
                </div>

                {/* Add Member Form */}
                {showAddMemberForm && (
                  <form onSubmit={addMember} className="space-y-3 p-3 bg-slate-50 rounded-lg border border-slate-200">
                    <div>
                      <label className="block text-xs font-medium text-slate-700 mb-1">
                        User Email
                      </label>
                      <input
                        type="email"
                        value={memberEmail}
                        onChange={(e) => setMemberEmail(e.target.value)}
                        placeholder="user@example.com"
                        className="w-full px-3 py-2 border border-slate-300 rounded text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                      />
                    </div>

                    <div>
                      <label className="block text-xs font-medium text-slate-700 mb-1">
                        Role
                      </label>
                      <select
                        value={memberRole}
                        onChange={(e) => setMemberRole(e.target.value)}
                        className="w-full px-3 py-2 border border-slate-300 rounded text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                      >
                        <option value="member">Member</option>
                        <option value="admin">Admin</option>
                      </select>
                    </div>

                    <div className="flex gap-2">
                      <button
                        type="submit"
                        disabled={addingMember}
                        className="flex-1 px-3 py-1.5 bg-brand-600 hover:bg-brand-700 disabled:bg-slate-400 text-white text-xs font-medium rounded transition"
                      >
                        {addingMember ? 'Adding...' : 'Add Member'}
                      </button>
                      <button
                        type="button"
                        onClick={() => setShowAddMemberForm(false)}
                        className="flex-1 px-3 py-1.5 bg-slate-200 hover:bg-slate-300 text-slate-700 text-xs font-medium rounded transition"
                      >
                        Cancel
                      </button>
                    </div>
                  </form>
                )}

                {/* Members Table */}
                <div className="overflow-x-auto border border-slate-100 rounded-lg">
                  <table className="w-full text-sm">
                    <thead className="bg-slate-50 text-slate-500 text-xs uppercase tracking-wide">
                      <tr>
                        <th className="text-left px-4 py-3">Email</th>
                        <th className="text-left px-4 py-3">Role</th>
                        <th className="text-left px-4 py-3">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {selectedMembers.length === 0 ? (
                        <tr>
                          <td colSpan={3} className="px-4 py-8 text-center text-slate-400 text-xs">
                            No members yet. Add one to get started.
                          </td>
                        </tr>
                      ) : (
                        selectedMembers.map((m) => (
                          <tr key={m.id} className="hover:bg-slate-50">
                            <td className="px-4 py-3 text-slate-700 font-medium flex items-center gap-2">
                              <Mail size={14} className="text-slate-400" />
                              {m.email}
                            </td>
                            <td className="px-4 py-3 text-slate-600 text-xs">
                              <span className="inline-block px-2 py-1 bg-slate-100 rounded">
                                {m.role}
                              </span>
                            </td>
                            <td className="px-4 py-3">
                              <button
                                onClick={() => removeMember(m.id)}
                                className="text-red-500 hover:text-red-700 font-medium text-xs"
                              >
                                Remove
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          ) : (
            <div className="bg-white rounded-xl border border-slate-200 p-12 text-center">
              <Users size={48} className="mx-auto text-slate-300 mb-4" />
              <h3 className="text-slate-800 font-semibold mb-2">Select a group</h3>
              <p className="text-slate-500 text-sm">
                Choose a group from the list to view members and analytics.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
