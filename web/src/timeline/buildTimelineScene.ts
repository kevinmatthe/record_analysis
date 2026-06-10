import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket } from '../api';
import type { TimelineBranchNode, TimelineInsightNode, TimelineScene } from './types';

export function buildTimelineScene({
  buckets,
  wordCloudItems,
  summaryItems,
  branches,
}: {
  buckets: TimelineBucket[];
  wordCloudItems: AnalysisWorkItem[];
  summaryItems: AnalysisWorkItem[];
  branches: AnalysisBranch[];
}): TimelineScene {
  const sortedBuckets = [...buckets].sort((a, b) => Date.parse(a.start_time) - Date.parse(b.start_time));
  return {
    buckets: sortedBuckets.map((bucket) => ({
      bucket,
      x: Date.parse(bucket.start_time),
      y: 0,
      label: bucketSummaryLabel(bucket),
      labelLines: wrapShortText(bucketSummaryText(bucket), 8, 2),
    })),
    insights: buildInsightNodes(sortedBuckets, wordCloudItems, summaryItems),
    branches: buildBranchNodes(branches),
  };
}

function buildInsightNodes(
  buckets: TimelineBucket[],
  wordCloudItems: AnalysisWorkItem[],
  summaryItems: AnalysisWorkItem[],
): TimelineInsightNode[] {
  const bucketIDs = new Set(buckets.map((bucket) => bucket.id));
  const candidates: Array<{ time: number; label: string; item: AnalysisWorkItem }> = [];
  for (const item of summaryItems) {
    if (item.status !== 'completed') continue;
    const summary = topicSummaryResult(item);
    if (!summary) continue;
    const label = summary.title || summary.summary;
    if (!label) continue;
    candidates.push({ time: insightNodeTime(item), label: `摘要 ${shortNodeText(label, 18)}`, item });
  }
  for (const item of wordCloudItems) {
    if (item.status !== 'completed' || !Array.isArray(item.result)) continue;
    const terms = item.result
      .slice(0, 4)
      .map((term) => term.term)
      .filter(Boolean);
    if (terms.length === 0) continue;
    candidates.push({ time: insightNodeTime(item), label: terms.join(' / '), item });
  }

  const sortedCandidates = candidates.sort((a, b) => a.time - b.time);
  const maxNodes = 30;
  const step = Math.max(1, Math.ceil(sortedCandidates.length / maxNodes));
  const lanes = [0.58, -0.54, 0.86, -0.82];
  const minGap = timelineMinGap(sortedCandidates.map((candidate) => candidate.time), 0.035);
  const assignedLanes = assignLanes(sortedCandidates.map((candidate) => candidate.time), lanes, minGap);
  return sortedCandidates
    .map((candidate, index) => ({ ...candidate, lane: assignedLanes[index] ?? lanes[index % lanes.length] }))
    .filter((_, index) => index % step === 0)
    .map((candidate) => ({
      item: candidate.item,
      x: candidate.time,
      y: candidate.lane,
      label: candidate.label,
      bucketID: bucketIDs.has(candidate.item.scope_id) ? candidate.item.scope_id : '',
    }));
}

function buildBranchNodes(branches: AnalysisBranch[]): TimelineBranchNode[] {
  const lanes = [-0.24, -0.34, -0.16];
  const sliced = branches.slice(0, 40).map((branch) => {
    const start = Date.parse(branch.start_time);
    const end = Date.parse(branch.end_time);
    const midpoint = Number.isFinite(start) && Number.isFinite(end) ? start + (end - start) / 2 : start;
    return { branch, midpoint };
  });
  const sorted = sliced.sort((a, b) => a.midpoint - b.midpoint);
  const minGap = timelineMinGap(sorted.map((item) => item.midpoint), 0.045);
  const assignedLanes = assignLanes(sorted.map((item) => item.midpoint), lanes, minGap);
  return sorted.map(({ branch, midpoint }, index) => {
    const title = branch.title || branch.topic_hint || 'Branch';
    return {
      branch,
      x: midpoint,
      y: assignedLanes[index] ?? lanes[index % lanes.length],
      label: shortNodeText(title, 12),
    };
  });
}

function assignLanes(times: number[], lanes: number[], minGap: number) {
  const lastTimeByLane = new Map<number, number>();
  return times.map((time, index) => {
    if (!Number.isFinite(time)) return lanes[index % lanes.length];
    const available = lanes.find((lane) => {
      const lastTime = lastTimeByLane.get(lane);
      return lastTime === undefined || Math.abs(time - lastTime) >= minGap;
    });
    const lane = available ?? lanes[index % lanes.length];
    lastTimeByLane.set(lane, time);
    return lane;
  });
}

function timelineMinGap(times: number[], ratio: number) {
  const finiteTimes = times.filter(Number.isFinite).sort((a, b) => a - b);
  if (finiteTimes.length < 2) return 0;
  const span = finiteTimes[finiteTimes.length - 1] - finiteTimes[0];
  return Math.max(span * ratio, 30 * 60 * 1000);
}

function insightNodeTime(item: AnalysisWorkItem) {
  const start = Date.parse(item.start_time);
  const end = Date.parse(item.end_time);
  if (Number.isFinite(start) && Number.isFinite(end)) {
    return start + (end - start) / 2;
  }
  return Number.isFinite(start) ? start : Date.now();
}

function shortNodeText(value: string, maxLength: number) {
  const chars = Array.from(value.trim());
  return chars.length > maxLength ? `${chars.slice(0, maxLength).join('')}...` : value.trim();
}

function bucketSummaryLabel(bucket: TimelineBucket) {
  return wrapShortText(bucketSummaryText(bucket), 8, 2).join('\n');
}

function bucketSummaryText(bucket: TimelineBucket) {
  return bucket.summary_title || bucket.preview || `${bucket.message_count} 条消息`;
}

function wrapShortText(value: string, charsPerLine: number, maxLines: number) {
  const compact = value.replace(/\s+/g, ' ').trim();
  const chars = Array.from(compact);
  if (chars.length === 0) return ['暂无摘要'];
  const lines: string[] = [];
  for (let index = 0; index < chars.length && lines.length < maxLines; index += charsPerLine) {
    lines.push(chars.slice(index, index + charsPerLine).join(''));
  }
  if (chars.length > charsPerLine * maxLines && lines.length > 0) {
    lines[lines.length - 1] = `${Array.from(lines[lines.length - 1]).slice(0, Math.max(1, charsPerLine - 1)).join('')}…`;
  }
  return lines;
}

function topicSummaryResult(item: AnalysisWorkItem) {
  if (!item.result || Array.isArray(item.result)) {
    return null;
  }
  if (typeof item.result === 'object' && 'summary' in item.result) {
    return item.result;
  }
  return null;
}
