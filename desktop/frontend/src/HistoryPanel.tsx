import { useEffect, useState } from 'react';
import { artifactList, sessionHistory } from './api';
import type { ArtifactEnvelope, SessionRecord } from './types';

export function HistoryPanel(props: { onError: (message: string) => void }) {
  const { onError } = props;
  const [sessions, setSessions] = useState<SessionRecord[]>([]);
  const [selected, setSelected] = useState<SessionRecord | null>(null);
  const [artifacts, setArtifacts] = useState<ArtifactEnvelope[]>([]);
  const [activeArtifact, setActiveArtifact] = useState<ArtifactEnvelope | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let active = true;
    sessionHistory()
      .then((records) => {
        if (active) {
          setSessions(records);
        }
      })
      .catch((err) => onError(err instanceof Error ? err.message : String(err)));
    return () => {
      active = false;
    };
  }, [onError]);

  useEffect(() => {
    if (!selected) {
      setArtifacts([]);
      setActiveArtifact(null);
      return;
    }
    let active = true;
    setLoading(true);
    artifactList(selected.id)
      .then((items) => {
        if (!active) {
          return;
        }
        setArtifacts(items);
        setActiveArtifact(items[0] ?? null);
      })
      .catch((err) => onError(err instanceof Error ? err.message : String(err)))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [selected, onError]);

  return (
    <div className="history-view">
      <section className="panel queue-panel">
        <div className="panel-head">
          <div>
            <p className="section-kicker">History</p>
            <h2>Sessions</h2>
          </div>
          <span className="panel-count">{sessions.length}</span>
        </div>
        <div className="job-list">
          {sessions.length === 0 ? (
            <p className="empty-state">No sessions recorded yet.</p>
          ) : (
            sessions.map((session) => (
              <button
                className={`job-row ${selected?.id === session.id ? 'job-row-active' : ''}`}
                key={session.id}
                onClick={() => setSelected(session)}
                type="button"
              >
                <div className="job-row-top">
                  <strong>{trim(session.prompt, 46) || session.id}</strong>
                  <span className={`pill pill-${session.status}`}>{session.status}</span>
                </div>
                <div className="job-row-bottom">
                  <span>{session.starter || 'no-agent'}</span>
                  <span>{formatDate(session.updated_at)}</span>
                </div>
              </button>
            ))
          )}
        </div>
      </section>

      <section className="panel inspector">
        <div className="inspector-head">
          <div>
            <p className="section-kicker">Artifacts</p>
            <h2>{selected ? trim(selected.prompt, 40) || selected.id : 'No session selected'}</h2>
          </div>
        </div>
        {!selected ? (
          <div className="empty-view">
            <p>Select a session on the left to browse its artifacts and diffs.</p>
          </div>
        ) : loading ? (
          <p className="empty-state">Loading artifacts…</p>
        ) : (
          <div className="artifact-layout">
            <div className="artifact-list">
              {artifacts.length === 0 ? (
                <p className="empty-state">No artifacts for this session.</p>
              ) : (
                artifacts.map((artifact) => (
                  <button
                    className={`artifact-chip ${activeArtifact?.id === artifact.id ? 'artifact-chip-active' : ''}`}
                    key={artifact.id}
                    onClick={() => setActiveArtifact(artifact)}
                    type="button"
                  >
                    <strong>{artifact.kind || 'artifact'}</strong>
                    <span className="mono">{trim(artifact.id, 24)}</span>
                  </button>
                ))
              )}
            </div>
            <div className="artifact-detail">
              {activeArtifact ? <ArtifactView artifact={activeArtifact} /> : <p className="empty-state">Pick an artifact.</p>}
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

function ArtifactView(props: { artifact: ArtifactEnvelope }) {
  const diff = extractDiff(props.artifact.payload);
  return (
    <div className="content-stack">
      <div className="hero-meta">
        <span>kind {props.artifact.kind || 'artifact'}</span>
        <span className="mono">{props.artifact.id}</span>
        {props.artifact.payload_schema ? <span>{props.artifact.payload_schema}</span> : null}
      </div>
      {diff ? (
        <div className="text-block">
          <h4>Diff</h4>
          <pre className="diff-block">{diff.split('\n').map((line, index) => (
            <span className={diffLineClass(line)} key={index}>{line + '\n'}</span>
          ))}</pre>
        </div>
      ) : null}
      <div className="text-block">
        <h4>Payload</h4>
        <pre>{JSON.stringify(props.artifact.payload ?? {}, null, 2)}</pre>
      </div>
    </div>
  );
}

function diffLineClass(line: string) {
  if (line.startsWith('+') && !line.startsWith('+++')) {
    return 'diff-add';
  }
  if (line.startsWith('-') && !line.startsWith('---')) {
    return 'diff-del';
  }
  if (line.startsWith('@@')) {
    return 'diff-hunk';
  }
  return 'diff-ctx';
}

function extractDiff(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }
  for (const value of Object.values(payload as Record<string, unknown>)) {
    if (typeof value === 'string' && (value.includes('diff --git') || /\n@@ /.test(value))) {
      return value;
    }
  }
  return null;
}

function trim(value: string, max: number) {
  if (!value) {
    return '';
  }
  return value.length <= max ? value : `${value.slice(0, max - 1)}…`;
}

function formatDate(value?: string) {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString();
}
