import { useEffect, useRef, useState } from 'react';
import { offJobEvents, onJobEvent, startJobStream, stopJobStream } from './api';
import type { EventRecord, JobEventPayload } from './types';

const MAX_EVENTS = 500;

export function LiveLog(props: { jobID: string }) {
  const { jobID } = props;
  const [records, setRecords] = useState<EventRecord[]>([]);
  const [streaming, setStreaming] = useState(false);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    setRecords([]);
    if (!jobID) {
      return;
    }
    let active = true;
    const unsubscribe = onJobEvent((payload: JobEventPayload) => {
      if (!active || !payload || payload.job_id !== jobID || !payload.record) {
        return;
      }
      setRecords((current) => {
        const next = [...current, payload.record];
        return next.length > MAX_EVENTS ? next.slice(next.length - MAX_EVENTS) : next;
      });
    });
    startJobStream(jobID)
      .then(() => {
        if (active) {
          setStreaming(true);
        }
      })
      .catch(() => {
        if (active) {
          setStreaming(false);
        }
      });
    return () => {
      active = false;
      setStreaming(false);
      unsubscribe();
      offJobEvents();
      stopJobStream(jobID).catch(() => undefined);
    };
  }, [jobID]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [records]);

  if (!jobID) {
    return <p className="empty-state">No run selected.</p>;
  }

  return (
    <div className="live-log">
      <div className="live-log-head">
        <span className={`stream-dot ${streaming ? 'stream-dot-on' : ''}`} />
        <span>{streaming ? 'Streaming live events' : 'Stream idle'}</span>
        <span className="live-log-count">{records.length}</span>
      </div>
      <div className="live-log-body">
        {records.length === 0 ? (
          <p className="empty-state">Waiting for events…</p>
        ) : (
          records.map((record, index) => (
            <div className="log-line" key={`${record.id || 'evt'}-${index}`}>
              <span className="log-time">{formatTime(record.occurred_at)}</span>
              <span className={`log-type log-type-${actorClass(record.actor_type)}`}>{record.type}</span>
              <span className="log-detail">{describeEvent(record)}</span>
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

function actorClass(actor?: string) {
  return (actor || 'system').toLowerCase().replace(/[^a-z]/g, '') || 'system';
}

function formatTime(value?: string) {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  return date.toLocaleTimeString();
}

function describeEvent(record: EventRecord) {
  const payload = record.payload || {};
  const preview =
    pickString(payload, ['preview', 'output', 'message', 'summary', 'text', 'detail']) ||
    record.reason_code ||
    '';
  return preview.length > 240 ? `${preview.slice(0, 239)}…` : preview;
}

function pickString(payload: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
  }
  return '';
}
