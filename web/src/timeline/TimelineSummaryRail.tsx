import { useState } from 'react';
import type { AnalysisWorkItem } from '../api';

export type TimelineRange = { start: string; end: string };

export function TimelineSummaryRail({
  summaries,
  merges,
  selectedRange,
  onSelect,
}: {
  summaries: AnalysisWorkItem[];
  merges: AnalysisWorkItem[];
  selectedRange: TimelineRange | null;
  onSelect: (item: AnalysisWorkItem) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const items = [...summaries, ...merges]
    .filter((item) => item.status !== 'failed')
    .sort((a, b) => Date.parse(a.start_time) - Date.parse(b.start_time));
  const visibleItems = expanded ? items : items.slice(0, 6);
  if (items.length === 0) {
    return (
      <div className="summaryRail emptyRail">
        <span>摘要轨道</span>
        <p>生成片段摘要后，结果会按时间出现在这里。</p>
      </div>
    );
  }
  return (
    <div className="summaryRail">
      <div className="summaryRailHeader">
        <strong>摘要轨道</strong>
        <span>{items.length} 个时间结果</span>
      </div>
      <div className="summaryRailItems">
        {visibleItems.map((item) => {
          const summary = topicSummaryResult(item);
          const title = summary?.title || workItemKindText(item.kind);
          const active = selectedRange ? rangesOverlap(selectedRange.start, selectedRange.end, item.start_time, item.end_time) : false;
          return (
            <button key={item.id} className={`summaryRailItem ${item.status} ${active ? 'active' : ''}`} onClick={() => onSelect(item)}>
              <span>{formatDate(item.start_time)}</span>
              <strong>{title}</strong>
              <small>{statusText(item.status)} · {item.total_tokens || 0} tokens</small>
            </button>
          );
        })}
      </div>
      {items.length > 6 && (
        <button className="secondary summaryRailToggle" onClick={() => setExpanded((value) => !value)}>
          {expanded ? '折叠摘要轨道' : `展开全部 ${items.length} 个摘要`}
        </button>
      )}
    </div>
  );
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

function rangesOverlap(startA: string, endA: string, startB: string, endB: string) {
  return Date.parse(startA) < Date.parse(endB) && Date.parse(startB) < Date.parse(endA);
}

function statusText(status?: string) {
  switch (status) {
    case 'running':
      return '分析中';
    case 'queued':
      return '排队中';
    case 'completed':
      return '已完成';
    case 'failed':
      return '失败';
    case 'unseen':
    case '':
    case undefined:
      return '未创建';
    default:
      return status;
  }
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

function topicSummaryResult(item: AnalysisWorkItem) {
  if (!item.result || Array.isArray(item.result)) {
    return null;
  }
  if (typeof item.result === 'object' && 'summary' in item.result) {
    return item.result;
  }
  return null;
}
