import { FormEvent, useState } from 'react';
import { addAgent, removeAgent } from './api';
import type { AgentProfile, BootstrapResponse } from './types';

interface AddForm {
  id: string;
  display_name: string;
  command: string;
  args: string;
  use_pty: boolean;
}

const emptyForm: AddForm = {
  id: '',
  display_name: '',
  command: '',
  args: '',
  use_pty: false,
};

export function AgentsPanel(props: {
  agents: AgentProfile[];
  configPath?: string;
  onChange: (next: BootstrapResponse) => void;
  onError: (message: string) => void;
}) {
  const { agents, configPath, onChange, onError } = props;
  const [form, setForm] = useState<AddForm>(emptyForm);
  const [busy, setBusy] = useState(false);

  async function handleAdd(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    try {
      const next = await addAgent({
        id: form.id.trim(),
        display_name: form.display_name.trim(),
        command: form.command.trim(),
        args: splitArgs(form.args),
        aliases: [],
        use_pty: form.use_pty,
      });
      onChange(next);
      setForm(emptyForm);
      onError('');
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleRemove(id: string) {
    setBusy(true);
    try {
      const next = await removeAgent(id);
      onChange(next);
      onError('');
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const knownHint = isKnownAgent(form.id)
    ? `“${form.id}” is auto-configured — command arguments are filled in automatically.`
    : 'Custom agent — pass arguments below (use {prompt}, {cwd} placeholders).';

  return (
    <div className="agents-view">
      <section className="panel">
        <div className="panel-head">
          <div>
            <p className="section-kicker">Agents</p>
            <h2>Configured agents</h2>
          </div>
          <span className="panel-count">{agents.length}</span>
        </div>
        <div className="agent-list">
          {agents.length === 0 ? (
            <p className="empty-state">No agents configured yet. Add one on the right.</p>
          ) : (
            agents.map((agent) => (
              <div className="agent-row" key={agent.id}>
                <div className="agent-row-main">
                  <div className="agent-row-top">
                    <strong>{agent.display_name || agent.id}</strong>
                    <span className={`pill pill-${agent.availability === 'available' ? 'succeeded' : 'neutral'}`}>
                      {agent.availability}
                    </span>
                  </div>
                  <p className="agent-row-meta">
                    <span className="mono">{agent.id}</span>
                    <span className="mono">{agent.command}</span>
                  </p>
                </div>
                <button className="secondary-button" disabled={busy} onClick={() => handleRemove(agent.id)} type="button">
                  Remove
                </button>
              </div>
            ))
          )}
        </div>
        {configPath ? <p className="agent-config-path mono">{configPath}</p> : null}
      </section>

      <section className="panel panel-quiet">
        <div className="panel-head">
          <div>
            <p className="section-kicker">Add</p>
            <h2>Register agent</h2>
          </div>
        </div>
        <form className="run-form" onSubmit={handleAdd}>
          <label>
            ID
            <input
              onChange={(event) => setForm((current) => ({ ...current, id: event.target.value }))}
              placeholder="codex"
              type="text"
              value={form.id}
            />
          </label>
          <label>
            Display name
            <input
              onChange={(event) => setForm((current) => ({ ...current, display_name: event.target.value }))}
              placeholder="Codex"
              type="text"
              value={form.display_name}
            />
          </label>
          <label>
            Command (absolute path)
            <input
              onChange={(event) => setForm((current) => ({ ...current, command: event.target.value }))}
              placeholder="/opt/homebrew/bin/codex"
              type="text"
              value={form.command}
            />
          </label>
          <label>
            Arguments (comma separated, optional)
            <input
              onChange={(event) => setForm((current) => ({ ...current, args: event.target.value }))}
              placeholder="exec, --full-auto, {prompt}"
              type="text"
              value={form.args}
            />
          </label>
          <p className="form-hint">{knownHint}</p>
          <div className="form-foot">
            <label className="checkbox">
              <input
                checked={form.use_pty}
                onChange={(event) => setForm((current) => ({ ...current, use_pty: event.target.checked }))}
                type="checkbox"
              />
              Use PTY
            </label>
            <button
              className="primary-button"
              disabled={busy || !form.id.trim() || !form.command.trim()}
              type="submit"
            >
              {busy ? 'Saving…' : 'Add agent'}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}

function splitArgs(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function isKnownAgent(id: string) {
  return ['claude', 'codex', 'gemini', 'copilot'].includes(id.trim().toLowerCase());
}
