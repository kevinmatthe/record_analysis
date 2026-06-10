import { useEffect, useMemo, useRef, useState } from 'react';
import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket, TimelineGranularity } from '../api';
import { buildTimelineScene } from './buildTimelineScene';

type HitTarget =
  | { kind: 'bucket'; bucket: TimelineBucket; x: number; y: number; radius: number }
  | { kind: 'insight'; item: AnalysisWorkItem; x: number; y: number; width: number; height: number }
  | { kind: 'branch'; branch: AnalysisBranch; x: number; y: number; width: number; height: number };

export function CanvasTimelineRenderer({
  buckets,
  granularity,
  selectedBucketID,
  selectedBranchID,
  selectedInsightID,
  selectedRange,
  bucketPeak,
  wordCloudItems,
  summaryItems,
  branches,
  onSelectBucket,
  onSelectRange,
  onSelectBranch,
  onSelectInsight,
}: {
  buckets: TimelineBucket[];
  granularity: TimelineGranularity;
  selectedBucketID: string;
  selectedBranchID: string;
  selectedInsightID: string;
  selectedRange: { start: string; end: string } | null;
  bucketPeak: number;
  wordCloudItems: AnalysisWorkItem[];
  summaryItems: AnalysisWorkItem[];
  branches: AnalysisBranch[];
  onSelectBucket: (bucket: TimelineBucket) => void;
  onSelectRange: (start: string, end: string, firstBucket: TimelineBucket) => void;
  onSelectBranch: (branch: AnalysisBranch) => void;
  onSelectInsight: (item: AnalysisWorkItem) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const hitTargetsRef = useRef<HitTarget[]>([]);
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const [dragEnd, setDragEnd] = useState<{ x: number; y: number } | null>(null);
  const [viewport, setViewport] = useState<[number, number] | null>(null);
  const [hoverTarget, setHoverTarget] = useState<HitTarget | null>(null);
  const scene = useMemo(() => buildTimelineScene({ buckets, wordCloudItems, summaryItems, branches }), [buckets, wordCloudItems, summaryItems, branches]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const context = canvas.getContext('2d');
    if (!context) return;
    const resizeObserver = new ResizeObserver(() => drawCanvas());
    resizeObserver.observe(canvas);
    drawCanvas();
    return () => resizeObserver.disconnect();

    function drawCanvas() {
      if (!canvas || !context) return;
      const rect = canvas.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.floor(rect.width * dpr));
      canvas.height = Math.max(1, Math.floor(rect.height * dpr));
      context.setTransform(1, 0, 0, 1, 0, 0);
      context.scale(dpr, dpr);
      drawTimeline(context, rect.width, rect.height);
    }
  }, [scene, granularity, selectedBucketID, selectedBranchID, selectedInsightID, selectedRange, bucketPeak, dragEnd, viewport]);

  useEffect(() => {
    setViewport(null);
  }, [buckets, granularity]);

  const onPointerDown = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const point = { x: event.clientX - rect.left, y: event.clientY - rect.top };
    dragStartRef.current = point;
    setDragEnd(dragStartRef.current);
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const onPointerMove = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const next = { x: event.clientX - rect.left, y: event.clientY - rect.top };
    if (dragStartRef.current === null) return;
    if (!isRangeSelectGesture(event)) {
      panViewport(dragStartRef.current.x - next.x, event.currentTarget);
      dragStartRef.current = next;
      setDragEnd(null);
      return;
    }
    setDragEnd(next);
  };

  const onPointerHover = (event: React.PointerEvent<HTMLCanvasElement>) => {
    if (dragStartRef.current !== null) return;
    const rect = event.currentTarget.getBoundingClientRect();
    setHoverTarget(findHitTarget(event.clientX - rect.left, event.clientY - rect.top, hitTargetsRef.current));
  };

  const onPointerUp = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const startX = dragStartRef.current;
    dragStartRef.current = null;
    setDragEnd(null);
    if (startX === null) return;
    const delta = Math.abs(x - startX.x);
    if (delta < 6) {
      const target = findHitTarget(x, event.clientY - rect.top, hitTargetsRef.current);
      if (target?.kind === 'bucket') onSelectBucket(target.bucket);
      if (target?.kind === 'insight') onSelectInsight(target.item);
      if (target?.kind === 'branch') onSelectBranch(target.branch);
      return;
    }
    if (!isRangeSelectGesture(event)) {
      return;
    }
    const range = pixelRangeToBuckets(Math.min(startX.x, x), Math.max(startX.x, x), hitTargetsRef.current);
    if (range) {
      onSelectRange(range.first.start_time, range.last.end_time, range.first);
    }
  };

  const onPointerCancel = () => {
    dragStartRef.current = null;
    setDragEnd(null);
    setHoverTarget(null);
  };

  const onWheel = (event: React.WheelEvent<HTMLCanvasElement>) => {
    event.preventDefault();
    if (event.shiftKey) {
      panViewport(event.deltaY || event.deltaX, event.currentTarget);
      return;
    }
    zoomViewport(event.deltaY > 0 ? 1.18 : 0.82, event.clientX - event.currentTarget.getBoundingClientRect().left, event.currentTarget);
  };

  function drawTimeline(context: CanvasRenderingContext2D, width: number, height: number) {
    context.clearRect(0, 0, width, height);
    hitTargetsRef.current = [];
    const bucketNodes = scene.buckets;
    const chartBuckets = bucketNodes.map((node) => node.bucket);
    if (chartBuckets.length === 0) {
      context.fillStyle = '#7a888a';
      context.font = '14px sans-serif';
      context.textAlign = 'center';
      context.fillText('暂无时间桶', width / 2, height / 2);
      return;
    }

    const fullExtent = timelineExtent(chartBuckets, granularity);
    const extent = viewport ?? fullExtent;
    const xForTime = (value: number) => {
      const [min, max] = extent;
      return 34 + ((value - min) / Math.max(1, max - min)) * Math.max(1, width - 68);
    };
    const axisY = height * 0.54;
    const peak = Math.max(bucketPeak, ...chartBuckets.map((bucket) => bucket.message_count), 1);

    context.strokeStyle = 'rgba(13, 64, 71, 0.22)';
    context.lineWidth = 4;
    context.beginPath();
    context.moveTo(28, axisY);
    context.lineTo(width - 28, axisY);
    context.stroke();

    context.font = '12px sans-serif';
    context.textAlign = 'center';
    context.textBaseline = 'top';
    for (const tick of buildAxisTicks(extent[0], extent[1], granularity)) {
      const x = xForTime(tick);
      context.strokeStyle = 'rgba(13, 64, 71, 0.22)';
      context.lineWidth = 1;
      context.beginPath();
      context.moveTo(x, axisY + 24);
      context.lineTo(x, axisY + 38);
      context.stroke();
      context.fillStyle = '#657577';
      context.fillText(formatTimelineAxisLabel(tick, granularity), x, axisY + 43);
    }

    context.fillStyle = 'rgba(255, 255, 255, 0.78)';
    context.strokeStyle = 'rgba(19, 93, 102, 0.16)';
    roundRect(context, 34, 58, 168, 28, 7);
    context.fill();
    context.stroke();
    context.fillStyle = '#657577';
    context.font = '12px sans-serif';
    context.textAlign = 'left';
    context.textBaseline = 'middle';
    context.fillText('拖拽平移 · Shift 拖拽框选 · 滚轮缩放', 44, 72, 148);

    for (const bucketNode of bucketNodes) {
      const bucket = bucketNode.bucket;
      const center = xForTime(bucketCenterTime(bucket));
      const start = xForTime(Date.parse(bucket.start_time));
      const end = xForTime(Date.parse(bucket.end_time));
      const selected = bucket.id === selectedBucketID || bucketInRange(bucket, selectedRange);
      const half = bucketThickness(bucket.message_count, peak, selected) / 2;
      const y = axisY + (selected ? -10 : 0);
      hitTargetsRef.current.push({
        kind: 'bucket',
        bucket,
        x: center,
        y,
        radius: Math.max(24, Math.abs(end - start) / 2 + 8, half + 12),
      });
      context.fillStyle = bucketStatusColor(bucket.analysis_status, selected);
      context.shadowColor = selected ? 'rgba(13, 64, 71, 0.34)' : 'rgba(13, 64, 71, 0.18)';
      context.shadowBlur = selected ? 22 : 10;
      roundRect(context, start, y - half, Math.max(6, end - start), half * 2, half);
      context.fill();
      context.shadowBlur = 0;

      drawBucketSummaryLabel(context, bucketNode.labelLines, center, y - half - 12, selected);
    }

    for (const node of scene.insights) {
      const x = xForTime(node.x);
      const y = axisY - node.y * 86;
      const active = node.item.id === selectedInsightID;
      const width = Math.min(150, Math.max(68, node.label.length * 8 + 18));
      drawLabel(context, node.label, x, y, width, active ? '#ffffff' : 'rgba(255, 255, 255, 0.9)', active ? '#135d66' : 'rgba(19, 93, 102, 0.2)', '#12343a');
      hitTargetsRef.current.push({ kind: 'insight', item: node.item, x, y, width, height: 28 });
    }

    for (const node of scene.branches) {
      const x = xForTime(node.x);
      const y = axisY - node.y * 125;
      const active = node.branch.id === selectedBranchID;
      const width = Math.min(124, Math.max(54, node.label.length * 11 + 24));
      context.strokeStyle = 'rgba(15, 79, 87, 0.24)';
      context.lineWidth = 1;
      context.setLineDash([3, 4]);
      context.beginPath();
      context.moveTo(x, axisY);
      context.lineTo(x, y);
      context.stroke();
      context.setLineDash([]);
      drawLabel(context, node.label, x, y, width, branchStatusColor(node.branch.status, active), '#ffffff', '#ffffff');
      hitTargetsRef.current.push({ kind: 'branch', branch: node.branch, x, y, width, height: 26 });
    }

    if (dragStartRef.current !== null && dragEnd !== null) {
      const left = Math.min(dragStartRef.current.x, dragEnd.x);
      const right = Math.max(dragStartRef.current.x, dragEnd.x);
      context.fillStyle = 'rgba(19, 93, 102, 0.035)';
      context.strokeStyle = '#135d66';
      context.setLineDash([6, 5]);
      context.lineWidth = 2;
      context.fillRect(left, 54, right - left, height - 112);
      context.strokeRect(left, 54, right - left, height - 112);
      context.setLineDash([]);
    }
  }

  function zoomViewport(factor: number, anchorX: number, canvas: HTMLCanvasElement) {
    const chartBuckets = scene.buckets.map((node) => node.bucket);
    if (chartBuckets.length === 0) return;
    const full = timelineExtent(chartBuckets, granularity);
    const current = viewport ?? full;
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.max(0, Math.min(1, (anchorX - 34) / Math.max(1, rect.width - 68)));
    const anchorTime = current[0] + (current[1] - current[0]) * ratio;
    const nextSpan = clamp((current[1] - current[0]) * factor, granularityDurationMs('5m'), full[1] - full[0]);
    setViewport(clampExtent(anchorTime - nextSpan * ratio, anchorTime + nextSpan * (1 - ratio), full));
  }

  function panViewport(deltaX: number, canvas: HTMLCanvasElement) {
    const chartBuckets = scene.buckets.map((node) => node.bucket);
    if (chartBuckets.length === 0) return;
    const full = timelineExtent(chartBuckets, granularity);
    const current = viewport ?? full;
    const rect = canvas.getBoundingClientRect();
    const deltaTime = (deltaX / Math.max(1, rect.width - 68)) * (current[1] - current[0]);
    setViewport(clampExtent(current[0] + deltaTime, current[1] + deltaTime, full));
  }

  return (
    <div className="timelineChartFrame canvasTimelineFrame">
      <canvas
        className="timelineChart canvasTimeline"
        ref={canvasRef}
        aria-label="聊天记录时间轴"
        onPointerDown={onPointerDown}
        onPointerMove={(event) => {
          onPointerHover(event);
          onPointerMove(event);
        }}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        onPointerLeave={onPointerCancel}
        onWheel={onWheel}
      />
      {hoverTarget && <CanvasTimelineTooltip target={hoverTarget} />}
      {viewport && (
        <button className="canvasViewportReset" onClick={() => setViewport(null)}>
          重置视图
        </button>
      )}
    </div>
  );
}

function CanvasTimelineTooltip({ target }: { target: HitTarget }) {
  const style = {
    left: `${target.x}px`,
    top: `${target.y}px`,
  };
  if (target.kind === 'bucket') {
    return (
      <div className="canvasTimelineTooltip" style={style}>
        <strong>{formatBucketWindow(target.bucket)}</strong>
        <span>{target.bucket.message_count} 条消息 · {target.bucket.participant_count} 人</span>
        <span>{target.bucket.total_tokens || 0} tokens</span>
      </div>
    );
  }
  if (target.kind === 'branch') {
    return (
      <div className="canvasTimelineTooltip" style={style}>
        <strong>{target.branch.title}</strong>
        <span>{target.branch.message_count} 条消息 · {target.branch.status}</span>
        <span>{target.branch.total_tokens || 0} tokens</span>
      </div>
    );
  }
  return (
    <div className="canvasTimelineTooltip" style={style}>
      <strong>{workItemTitle(target.item)}</strong>
      <span>{workItemKindText(target.item.kind)} · {target.item.status}</span>
      <span>{target.item.total_tokens || 0} tokens</span>
    </div>
  );
}

function isRangeSelectGesture(event: React.PointerEvent<HTMLCanvasElement>) {
  return event.shiftKey || event.metaKey || event.ctrlKey;
}

function findHitTarget(x: number, y: number, targets: HitTarget[]) {
  for (const target of [...targets].reverse()) {
    if (target.kind === 'bucket') {
      if (Math.abs(target.x - x) <= target.radius && Math.abs(target.y - y) <= Math.max(30, target.radius)) return target;
    } else if (Math.abs(target.x - x) <= target.width / 2 && Math.abs(target.y - y) <= target.height / 2) {
      return target;
    }
  }
  return null;
}

function pixelRangeToBuckets(left: number, right: number, targets: HitTarget[]) {
  const buckets = targets
    .filter((target): target is Extract<HitTarget, { kind: 'bucket' }> => target.kind === 'bucket' && target.x >= left && target.x <= right)
    .sort((a, b) => Date.parse(a.bucket.start_time) - Date.parse(b.bucket.start_time));
  if (buckets.length === 0) return null;
  return { first: buckets[0].bucket, last: buckets[buckets.length - 1].bucket };
}

function roundRect(context: CanvasRenderingContext2D, x: number, y: number, width: number, height: number, radius: number) {
  const safeRadius = Math.min(radius, width / 2, height / 2);
  context.beginPath();
  context.moveTo(x + safeRadius, y);
  context.arcTo(x + width, y, x + width, y + height, safeRadius);
  context.arcTo(x + width, y + height, x, y + height, safeRadius);
  context.arcTo(x, y + height, x, y, safeRadius);
  context.arcTo(x, y, x + width, y, safeRadius);
  context.closePath();
}

function drawLabel(context: CanvasRenderingContext2D, text: string, x: number, y: number, width: number, fill: string, stroke: string, color: string) {
  context.fillStyle = fill;
  context.strokeStyle = stroke;
  context.lineWidth = 2;
  context.shadowColor = 'rgba(23, 32, 42, 0.14)';
  context.shadowBlur = 12;
  roundRect(context, x - width / 2, y - 14, width, 28, 7);
  context.fill();
  context.shadowBlur = 0;
  context.stroke();
  context.fillStyle = color;
  context.font = '700 11px sans-serif';
  context.textAlign = 'center';
  context.textBaseline = 'middle';
  context.fillText(text, x, y, width - 12);
}

function drawBucketSummaryLabel(context: CanvasRenderingContext2D, lines: string[], x: number, bottomY: number, selected: boolean) {
  const width = selected ? 112 : 96;
  const lineHeight = 13;
  const height = lines.length * lineHeight + 10;
  const top = bottomY - height;
  context.fillStyle = selected ? '#ffffff' : 'rgba(255, 255, 255, 0.86)';
  context.strokeStyle = selected ? '#135d66' : 'rgba(13, 64, 71, 0.14)';
  context.lineWidth = selected ? 2 : 1;
  context.shadowColor = 'rgba(23, 32, 42, 0.12)';
  context.shadowBlur = selected ? 14 : 8;
  roundRect(context, x - width / 2, top, width, height, 7);
  context.fill();
  context.shadowBlur = 0;
  context.stroke();
  context.fillStyle = '#26383b';
  context.font = '700 11px sans-serif';
  context.textAlign = 'center';
  context.textBaseline = 'middle';
  lines.forEach((line, index) => {
    context.fillText(line, x, top + 7 + lineHeight / 2 + index * lineHeight, width - 12);
  });
}

function bucketCenterTime(bucket: TimelineBucket) {
  const start = Date.parse(bucket.start_time);
  const end = Date.parse(bucket.end_time);
  return Number.isFinite(start) && Number.isFinite(end) ? start + (end - start) / 2 : start;
}

function buildAxisTicks(min: number, max: number, granularity: TimelineGranularity) {
  const count = 7;
  const step = (max - min) / Math.max(1, count - 1);
  return Array.from({ length: count }, (_, index) => min + step * index);
}

function clampExtent(start: number, end: number, full: [number, number]): [number, number] {
  const span = end - start;
  if (start < full[0]) return [full[0], full[0] + span];
  if (end > full[1]) return [full[1] - span, full[1]];
  return [start, end];
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function bucketThickness(count: number, peak: number, selected: boolean) {
  const ratio = Math.max(0, Math.min(1, count / Math.max(peak, 1)));
  return 8 + ratio * 24 + (selected ? 5 : 0);
}

function bucketInRange(bucket: TimelineBucket, range: { start: string; end: string } | null) {
  if (!range) return false;
  return Date.parse(bucket.start_time) < Date.parse(range.end) && Date.parse(range.start) < Date.parse(bucket.end_time);
}

function timelineExtent(buckets: TimelineBucket[], granularity: TimelineGranularity): [number, number] {
  const firstStart = Date.parse(buckets[0].start_time);
  const lastEnd = Date.parse(buckets[buckets.length - 1].end_time);
  const span = Math.max(lastEnd - firstStart, granularityDurationMs(granularity));
  const padding = Math.max(span * 0.04, granularityDurationMs(granularity) / 2);
  return [firstStart - padding, lastEnd + padding];
}

function granularityDurationMs(granularity: TimelineGranularity) {
  switch (granularity) {
    case 'year':
      return 365 * 24 * 60 * 60 * 1000;
    case 'month':
      return 31 * 24 * 60 * 60 * 1000;
    case 'week':
      return 7 * 24 * 60 * 60 * 1000;
    case 'day':
      return 24 * 60 * 60 * 1000;
    case '15m':
      return 15 * 60 * 1000;
    case '5m':
      return 5 * 60 * 1000;
    default:
      return 60 * 60 * 1000;
  }
}

function formatTimelineAxisLabel(value: number, granularity: TimelineGranularity) {
  const options: Intl.DateTimeFormatOptions =
    granularity === 'year'
      ? { year: 'numeric' }
      : granularity === 'month'
        ? { year: '2-digit', month: '2-digit' }
        : granularity === 'week' || granularity === 'day'
          ? { year: '2-digit', month: '2-digit', day: '2-digit' }
          : { year: '2-digit', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' };
  return new Intl.DateTimeFormat('zh-CN', options).format(new Date(value));
}

function bucketStatusColor(status: unknown, selected: boolean) {
  if (selected) return '#0a3f46';
  switch (status) {
    case 'running':
      return '#c26a00';
    case 'queued':
      return '#6f5bd6';
    case 'failed':
      return '#b3261e';
    case 'completed':
      return '#20845a';
    default:
      return '#98a7aa';
  }
}

function branchStatusColor(status: unknown, selected = false) {
  if (selected) return '#12343a';
  switch (status) {
    case 'completed':
      return '#1f7a4f';
    case 'running':
      return '#c26a00';
    case 'failed':
      return '#b3261e';
    default:
      return '#0d4047';
  }
}

function formatDate(value: string) {
  if (!value || value.startsWith('0001-')) return '未知时间';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function formatBucketWindow(bucket: TimelineBucket) {
  return `${formatDate(bucket.start_time)} - ${formatDate(bucket.end_time)}`;
}

function workItemKindText(kind: string) {
  switch (kind) {
    case 'topic_summary':
      return '桶摘要';
    case 'summary_merge':
      return '合并摘要';
    case 'word_cloud':
      return '词云';
    default:
      return kind;
  }
}

function workItemTitle(item: AnalysisWorkItem) {
  const summary = topicSummaryResult(item);
  if (summary?.title) return summary.title;
  if (summary?.summary) return summary.summary;
  return `${formatDate(item.start_time)} - ${formatDate(item.end_time)}`;
}

function topicSummaryResult(item: AnalysisWorkItem) {
  if (!item.result || Array.isArray(item.result)) {
    return null;
  }
  if (typeof item.result === 'object' && 'summary' in item.result) {
    return item.result as { title?: string; summary?: string };
  }
  return null;
}
