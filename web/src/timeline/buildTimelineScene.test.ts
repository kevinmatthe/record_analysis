import { buildTimelineScene } from './buildTimelineScene';
import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket } from '../api';

const buckets: TimelineBucket[] = [
  bucket('bucket-1', '2026-01-01T00:00:00Z', '2026-01-01T01:00:00Z', 12),
  bucket('bucket-2', '2026-01-01T01:00:00Z', '2026-01-01T02:00:00Z', 20),
];

const summaryItem: AnalysisWorkItem = {
  id: 'summary-1',
  job_id: 'job-1',
  kind: 'topic_summary',
  scope_type: 'bucket',
  scope_id: 'bucket-1',
  granularity: 'hour',
  start_time: '2026-01-01T00:00:00Z',
  end_time: '2026-01-01T01:00:00Z',
  status: 'completed',
  priority: 0,
  progress: 100,
  message_count: 12,
  prompt_tokens: 64,
  completion_tokens: 64,
  total_tokens: 128,
  result: {
    title: '确认午饭安排',
    summary: '讨论午饭时间与地点。',
    topics: ['午饭'],
    key_events: ['确认 12 点出发'],
    evidence_msg_ids: ['msg-1'],
    confidence: 0.9,
    uncertainty: '',
  },
  error: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:05:00Z',
};

const wordCloudItem: AnalysisWorkItem = {
  ...summaryItem,
  id: 'word-1',
  kind: 'word_cloud',
  scope_id: 'bucket-2',
  start_time: '2026-01-01T01:00:00Z',
  end_time: '2026-01-01T02:00:00Z',
  result: [
    { term: '部署', count: 8 },
    { term: '告警', count: 5 },
  ],
};

const branchItem: AnalysisBranch = {
  id: 'branch-1',
  job_id: 'job-1',
  relationship_id: 'rel-1',
  granularity: 'hour',
  start_time: '2026-01-01T00:00:00Z',
  end_time: '2026-01-01T02:00:00Z',
  title: '午饭和部署沟通',
  topic_hint: '跨两个小时的沟通',
  message_count: 32,
  bucket_ids: ['bucket-1', 'bucket-2'],
  cluster_id: '',
  status: 'completed',
  stage: 'done',
  progress: 100,
  prompt_tokens: 128,
  completion_tokens: 128,
  report_markdown: '# 报告',
  error: '',
  total_tokens: 256,
  created_at: '2026-01-01T02:00:00Z',
  updated_at: '2026-01-01T02:10:00Z',
};

const scene = buildTimelineScene({
  buckets: [buckets[1], buckets[0]],
  summaryItems: [summaryItem],
  wordCloudItems: [wordCloudItem],
  branches: [branchItem],
});

assertDeepEqual(
  scene.buckets.map((node) => node.bucket.id),
  ['bucket-1', 'bucket-2'],
  'buckets should be sorted by start time',
);
assertEqual(scene.buckets[0].label, '确认午饭安排', 'bucket label should use short summary text');
assertEqual(scene.insights.length, 2, 'completed summary and word cloud should become insight nodes');
assertEqual(scene.insights[0].bucketID, 'bucket-1', 'summary insight should keep bucket id');
assertMatch(scene.insights[0].label, /^摘要 /, 'summary insight should be labeled as summary');
assertMatch(scene.insights[1].label, /部署/, 'word cloud insight should expose top terms');
assertEqual(scene.branches.length, 1, 'branch should become a branch node');
assertEqual(scene.branches[0].label, '午饭和部署沟通', 'branch node should use readable title');

console.log('buildTimelineScene tests passed');

function bucket(id: string, start: string, end: string, count: number): TimelineBucket {
  return {
    id,
    granularity: 'hour',
    start_time: start,
    end_time: end,
    message_count: count,
    participant_count: 2,
    participant_messages: { PERSON_A: count - 1, PERSON_B: 1 },
    first_message_id: `${id}-first`,
    last_message_id: `${id}-last`,
    preview: '',
    analysis_status: 'unseen',
    summary_status: 'unseen',
    word_cloud_status: 'unseen',
    summary_title: id === 'bucket-1' ? '确认午饭安排' : '',
    summary_topics: [],
  };
}

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

function assertDeepEqual(actual: unknown, expected: unknown, message: string) {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`${message}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

function assertMatch(actual: string, pattern: RegExp, message: string) {
  if (!pattern.test(actual)) {
    throw new Error(`${message}: ${actual} does not match ${pattern}`);
  }
}
