import { BookmarkPlus, FileText, Loader2, Search, ZoomIn, ZoomOut } from 'lucide-react';
import type { TimelineGranularity } from '../api';

export function TimelineHeaderControls({
  timelineWindowLabel,
  granularity,
  resolvedGranularity,
  hasTimelineWindow,
  onResetTimelineWindow,
}: {
  timelineWindowLabel: string;
  granularity: 'auto' | TimelineGranularity;
  resolvedGranularity: TimelineGranularity;
  hasTimelineWindow: boolean;
  onResetTimelineWindow: () => void;
}) {
  return (
    <div className="timelineHeader">
      <span className="timelineModeLabel">{timelineWindowLabel} · {granularity === 'auto' ? `自动/${granularityLabel(resolvedGranularity)}` : granularityLabel(resolvedGranularity)}</span>
      {hasTimelineWindow && (
        <button className="secondary" onClick={onResetTimelineWindow}>
          返回全局
        </button>
      )}
    </div>
  );
}

export function TimelineActionDock({
  seedingSummaries,
  creatingMergeSummary,
  summaryReady,
  summaryMissing,
  creatingBranch,
  hasBranchPreview,
  granularity,
  visibleGranularities,
  canUseFineGranularity,
  zoomIndex,
  zoomStepsLength,
  seedingWorkItems,
  timelineLength,
  onGranularityChange,
  onZoom,
  onSeedWordClouds,
  onDrill,
  onSeedSummaries,
  onCreateMergeSummary,
  onSaveBranch,
}: {
  seedingSummaries: boolean;
  creatingMergeSummary: boolean;
  summaryReady: boolean;
  summaryMissing: number;
  creatingBranch: boolean;
  hasBranchPreview: boolean;
  granularity: 'auto' | TimelineGranularity;
  visibleGranularities: ReadonlyArray<'auto' | TimelineGranularity>;
  canUseFineGranularity: boolean;
  zoomIndex: number;
  zoomStepsLength: number;
  seedingWorkItems: boolean;
  timelineLength: number;
  onGranularityChange: (value: 'auto' | TimelineGranularity) => void;
  onZoom: (direction: 'in' | 'out') => void;
  onSeedWordClouds: () => void;
  onDrill: () => void;
  onSeedSummaries: () => void;
  onCreateMergeSummary: () => void;
  onSaveBranch: () => void;
}) {
  return (
    <div className="timelineActionDock" aria-label="当前片段操作">
      <select
        className="granularitySelect"
        value={granularity}
        onChange={(event) => onGranularityChange(event.target.value as 'auto' | TimelineGranularity)}
        title="时间粒度"
      >
        {visibleGranularities.map((value) => {
          const disabled = value !== 'auto' && isFineGranularity(value) && !canUseFineGranularity;
          return (
            <option key={value} value={value} disabled={disabled}>
              {granularityLabel(value)}
            </option>
          );
        })}
      </select>
      <button className="secondary iconOnly" onClick={() => onZoom('out')} disabled={zoomIndex <= 0 && granularity !== 'auto'} title="缩小时间尺度">
        <ZoomOut size={16} />
      </button>
      <button className="secondary iconOnly" onClick={() => onZoom('in')} disabled={zoomIndex >= zoomStepsLength - 1 && granularity !== 'auto'} title="放大时间尺度">
        <ZoomIn size={16} />
      </button>
      <button className="secondary" disabled={seedingWorkItems || timelineLength === 0} onClick={onSeedWordClouds}>
        {seedingWorkItems ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 词云
      </button>
      <button className="secondary" disabled={timelineLength === 0} onClick={onDrill}>
        <ZoomIn size={16} /> 下钻
      </button>
      <button
        className="secondary"
        disabled={seedingSummaries || timelineLength === 0}
        onClick={onSeedSummaries}
        title="生成当前可见时间线下所有子周期摘要"
      >
        {seedingSummaries ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} {summaryMissing > 0 ? `全部摘要 ${summaryMissing}` : '全部摘要'}
      </button>
      <button
        className="secondary"
        disabled={creatingMergeSummary || !summaryReady}
        onClick={onCreateMergeSummary}
        title={!summaryReady ? '需要当前范围内所有 bucket 摘要完成后才能聚合' : undefined}
      >
        {creatingMergeSummary ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 聚合
      </button>
      <button className="primary" disabled={creatingBranch || !hasBranchPreview} onClick={onSaveBranch}>
        {creatingBranch ? <Loader2 className="spin" size={16} /> : <BookmarkPlus size={16} />} Branch
      </button>
    </div>
  );
}

function granularityLabel(value: 'auto' | TimelineGranularity) {
  switch (value) {
    case 'auto':
      return '自动';
    case 'year':
      return '年';
    case 'month':
      return '月';
    case 'week':
      return '周';
    case 'day':
      return '天';
    case '15m':
      return '15 分钟';
    case '5m':
      return '5 分钟';
    default:
      return '小时';
  }
}

function isFineGranularity(value: TimelineGranularity) {
  return value === 'hour' || value === '15m' || value === '5m';
}
