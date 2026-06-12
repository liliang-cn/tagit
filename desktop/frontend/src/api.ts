import {
  AddAgent,
  ArtifactGet,
  ArtifactList,
  Bootstrap,
  PlanApprove,
  PlanPreview,
  PlanReject,
  PlansInbox,
  PickWorkingDir,
  QueueCancel,
  QueueInspect,
  RemoveAgent,
  ResultShow,
  SessionHistory,
  SessionInspect,
  SetWorkingDir,
  Snapshot,
  StopJobStream,
  StreamJob,
  SubmitRun,
} from '../wailsjs/go/main/App';
import { EventsOff, EventsOn } from '../wailsjs/runtime';
import type {
  AgentMutateRequest,
  ArtifactEnvelope,
  BootstrapResponse,
  JobEventPayload,
  PlanApplyResponse,
  PlanInboxEntry,
  PlanPreviewRequest,
  QueueInspectResponse,
  QueueRequest,
  ResultShowResponse,
  RunSubmitRequest,
  SessionInspectResponse,
  SessionRecord,
  SnapshotResponse,
  SubmitResponse,
} from './types';

export async function bootstrapApp() {
  return (await Bootstrap()) as BootstrapResponse;
}

export async function snapshotApp() {
  return (await Snapshot()) as SnapshotResponse;
}

export async function pickWorkingDir() {
  return (await PickWorkingDir()) as string;
}

export async function setWorkingDir(dir: string) {
  return (await SetWorkingDir(dir)) as BootstrapResponse;
}

export async function submitRun(payload: RunSubmitRequest) {
  return (await SubmitRun(payload)) as SubmitResponse;
}

export async function inspectJob(jobID: string) {
  return (await QueueInspect(jobID)) as QueueInspectResponse;
}

export async function inspectSession(sessionID: string) {
  return (await SessionInspect(sessionID)) as SessionInspectResponse;
}

export async function resultShow(sessionID: string) {
  return (await ResultShow(sessionID)) as ResultShowResponse;
}

export async function cancelJob(jobID: string) {
  return (await QueueCancel(jobID)) as QueueRequest;
}

export async function listPlans(sessionID: string) {
  return (await PlansInbox(sessionID)) as PlanInboxEntry[];
}

export async function approvePlan(artifactID: string) {
  await PlanApprove(artifactID);
}

export async function rejectPlan(artifactID: string) {
  await PlanReject(artifactID);
}

export async function previewPlan(payload: PlanPreviewRequest) {
  return (await PlanPreview(payload)) as PlanApplyResponse;
}

export async function addAgent(payload: AgentMutateRequest) {
  return (await AddAgent(payload)) as BootstrapResponse;
}

export async function removeAgent(id: string) {
  return (await RemoveAgent(id)) as BootstrapResponse;
}

export async function sessionHistory() {
  return (await SessionHistory()) as SessionRecord[];
}

export async function artifactList(sessionID: string) {
  return (await ArtifactList(sessionID)) as ArtifactEnvelope[];
}

export async function artifactGet(artifactID: string) {
  return (await ArtifactGet(artifactID)) as ArtifactEnvelope;
}

export async function startJobStream(jobID: string) {
  await StreamJob(jobID);
}

export async function stopJobStream(jobID: string) {
  await StopJobStream(jobID);
}

export function onJobEvent(handler: (payload: JobEventPayload) => void) {
  return EventsOn('job:event', (payload: JobEventPayload) => handler(payload));
}

export function offJobEvents() {
  EventsOff('job:event');
  EventsOff('job:stream-done');
}
