export type AnalysisRecord = {
  id: string;
  relationship_id: string;
  created_at: string;
  period_start: string;
  period_end: string;
  message_count: number;
  action_count: number;
  event_count: number;
  model_name: string;
  object_key: string;
  object_uri: string;
  report_path: string;
  status: string;
};

export type JobEvent = {
  time: string;
  message: string;
};

export type AnalysisJob = {
  id: string;
  status: 'queued' | 'running' | 'completed' | 'failed';
  stage: string;
  relationship_id: string;
  file_name: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  llm_message_limit: number;
  llm_message_count: number;
  analysis_mode: string;
  processed_count: number;
  progress: number;
  preview_total: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  result_record_id?: string;
  error?: string;
  events: JobEvent[];
  result?: AnalysisRecord;
};

export type TimelineBucket = {
  id: string;
  granularity: string;
  start_time: string;
  end_time: string;
  message_count: number;
  participant_count: number;
  participant_messages: Record<string, number>;
  first_message_id: string;
  last_message_id: string;
  preview: string;
};

export type TimelineGranularity = 'year' | 'month' | 'week' | 'day' | 'hour' | '15m' | '5m';

export type TimelineCluster = {
  id: string;
  granularity: string;
  start_time: string;
  end_time: string;
  message_count: number;
  bucket_count: number;
  bucket_ids: string[];
  topic_hint: string;
  status: string;
};

export type BranchPreview = {
  granularity: string;
  start_time: string;
  end_time: string;
  message_count: number;
  bucket_ids: string[];
  cluster_id: string;
  topic_hint: string;
  status: string;
};

export type AnalysisBranch = {
  id: string;
  job_id: string;
  relationship_id: string;
  title: string;
  granularity: string;
  start_time: string;
  end_time: string;
  message_count: number;
  bucket_ids: string[];
  cluster_id: string;
  topic_hint: string;
  status: string;
  stage: string;
  progress: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  report_markdown?: string;
  model_name?: string;
  error?: string;
  created_at: string;
  updated_at: string;
};

export type WordCloudTerm = {
  term: string;
  count: number;
};

export type TopicSummary = {
  title: string;
  summary: string;
  topics: string[];
  key_events: string[];
  evidence_msg_ids: string[];
  confidence: number;
  uncertainty: string;
  model_name?: string;
};

export type AnalysisWorkItem = {
  id: string;
  job_id: string;
  kind: string;
  scope_type: string;
  scope_id: string;
  granularity: string;
  start_time: string;
  end_time: string;
  status: string;
  priority: number;
  progress: number;
  message_count: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  result?: WordCloudTerm[] | TopicSummary;
  error?: string;
  created_at: string;
  updated_at: string;
};

export type PreviewPage = {
  items: Array<{
    id: string;
    sender: string;
    time: string;
    type: string;
    content: string;
  }>;
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
};

export type Metrics = {
  metrics?: Record<string, unknown>;
};

export type AnalysisResult = {
  stored_object: {
    object_key: string;
    uri: string;
  };
  analysis: {
    relationship_id: string;
    messages: unknown[];
    actions: unknown[] | null;
    events: unknown[] | null;
    behavior_metrics: Metrics;
    period_report: {
      report_markdown: string;
      model_name: string;
    };
  };
  report_path: string;
  record: AnalysisRecord;
};

export type ReportResponse = {
  record: AnalysisRecord;
  report_markdown: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    ...init,
    headers: {
      ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...(init?.headers ?? {}),
    },
  });
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(data?.error ?? `Request failed with ${response.status}`);
  }
  return data as T;
}

export function login(username: string, password: string) {
  return request<{ ok: boolean; username: string }>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function register(username: string, password: string) {
  return request<{ ok: boolean; username: string }>('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function changePassword(oldPassword: string, newPassword: string) {
  return request<{ ok: boolean }>('/api/auth/password', {
    method: 'POST',
    body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
  });
}

export function logout() {
  return request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' });
}

export function getMe() {
  return request<{ authenticated: boolean; username: string }>('/api/auth/me');
}

export function createAnalysis(form: FormData) {
  return request<AnalysisResult>('/api/analyses', {
    method: 'POST',
    body: form,
  });
}

export function createAnalysisJob(form: FormData) {
  return request<AnalysisJob>('/api/jobs', {
    method: 'POST',
    body: form,
  });
}

export function getJob(id: string) {
  return request<AnalysisJob>(`/api/jobs/${encodeURIComponent(id)}`);
}

export function listJobs(relationshipID: string) {
  const params = new URLSearchParams();
  if (relationshipID.trim()) {
    params.set('relationship_id', relationshipID.trim());
  }
  const query = params.toString();
  return request<{ items: AnalysisJob[] }>(`/api/jobs${query ? `?${query}` : ''}`);
}

export function getJobPreview(id: string, page: number, pageSize: number) {
  return request<PreviewPage>(`/api/jobs/${encodeURIComponent(id)}/preview?page=${page}&page_size=${pageSize}`);
}

export function getJobTimeline(id: string, granularity: TimelineGranularity, range?: { start: string; end: string } | null) {
  const params = new URLSearchParams({ granularity });
  if (range) {
    params.set('start_time', range.start);
    params.set('end_time', range.end);
  }
  return request<{ granularity: string; items: TimelineBucket[]; clusters: TimelineCluster[] }>(
    `/api/jobs/${encodeURIComponent(id)}/timeline?${params.toString()}`,
  );
}

export function getTimelineBucketMessages(id: string, bucketID: string, page: number, pageSize: number) {
  return request<PreviewPage>(
    `/api/jobs/${encodeURIComponent(id)}/timeline/${encodeURIComponent(bucketID)}/messages?page=${page}&page_size=${pageSize}`,
  );
}

export function previewJobBranch(
  id: string,
  payload: { granularity: TimelineGranularity; start_time: string; end_time: string },
) {
  return request<BranchPreview>(`/api/jobs/${encodeURIComponent(id)}/branches/preview`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function createJobBranch(
  id: string,
  payload: { granularity: TimelineGranularity; start_time: string; end_time: string; title: string },
) {
  return request<AnalysisBranch>(`/api/jobs/${encodeURIComponent(id)}/branches`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function listJobBranches(id: string) {
  return request<{ items: AnalysisBranch[] }>(`/api/jobs/${encodeURIComponent(id)}/branches`);
}

export function runJobBranch(
  jobID: string,
  branchID: string,
  payload: { analysis_mode: 'quick' | 'full'; max_llm_messages: number },
) {
  return request<AnalysisBranch>(`/api/jobs/${encodeURIComponent(jobID)}/branches/${encodeURIComponent(branchID)}/run`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function listWorkItems(jobID: string, kind: string, granularity: string) {
  const params = new URLSearchParams({ kind, granularity });
  return request<{ items: AnalysisWorkItem[] }>(`/api/jobs/${encodeURIComponent(jobID)}/work-items?${params.toString()}`);
}

export function seedWorkItems(jobID: string, granularity: string, kind = 'word_cloud', range?: { start: string; end: string } | null) {
  const payload: { kind: string; granularity: string; start_time?: string; end_time?: string } = { kind, granularity };
  if (range) {
    payload.start_time = range.start;
    payload.end_time = range.end;
  }
  return request<{ items: AnalysisWorkItem[] }>(`/api/jobs/${encodeURIComponent(jobID)}/work-items/seed`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function createSummaryMergeWorkItem(
  jobID: string,
  payload: { granularity: string; start_time: string; end_time: string },
) {
  return request<AnalysisWorkItem>(`/api/jobs/${encodeURIComponent(jobID)}/work-items/merge`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function prioritizeWorkItem(jobID: string, itemID: string) {
  return request<AnalysisWorkItem>(`/api/jobs/${encodeURIComponent(jobID)}/work-items/${encodeURIComponent(itemID)}/prioritize`, {
    method: 'POST',
  });
}

export function listAnalyses(relationshipID: string) {
  const params = new URLSearchParams();
  if (relationshipID.trim()) {
    params.set('relationship_id', relationshipID.trim());
  }
  const query = params.toString();
  return request<{ items: AnalysisRecord[] }>(`/api/analyses${query ? `?${query}` : ''}`);
}

export function getReport(id: string) {
  return request<ReportResponse>(`/api/analyses/${encodeURIComponent(id)}/report`);
}
