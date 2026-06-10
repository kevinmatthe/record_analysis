import { BookmarkPlus, FileText, Loader2, Search, ZoomIn, ZoomOut } from 'lucide-react';
import type { TimelineGranularity } from '../api';

export function TimelineHeaderControls({
  timelineWindowLabel,
  granularity,
  resolvedGranularity,
  visibleGranularities,
  canUseFineGranularity,
  zoomIndex,
  zoomStepsLength,
  hasTimelineWindow,
  seedingWorkItems,
  timelineLength,
  onGranularityChange,
  onZoom,
  onResetTimelineWindow,
  onSeedWordClouds,
}: {
  timelineWindowLabel: string;
  granularity: 'auto' | TimelineGranularity;
  resolvedGranularity: TimelineGranularity;
  visibleGranularities: ReadonlyArray<'auto' | TimelineGranularity>;
  canUseFineGranularity: boolean;
  zoomIndex: number;
  zoomStepsLength: number;
  hasTimelineWindow: boolean;
  seedingWorkItems: boolean;
  timelineLength: number;
  onGranularityChange: (value: 'auto' | TimelineGranularity) => void;
  onZoom: (direction: 'in' | 'out') => void;
  onResetTimelineWindow: () => void;
  onSeedWordClouds: () => void;
}) {
  return (
    <div className="timelineHeader">
      <div>
        <h2>时间轴</h2>
        <p>
          {timelineWindowLabel}
          {' · '}
          当前按 {granularity === 'auto' ? `自动粒度 (${granularityLabel(resolvedGranularity)})` : granularityLabel(resolvedGranularity)} 聚合消息桶。
        </p>
      </div>
      <div className="timelineControls">
        <div className="segmented">
          {visibleGranularities.map((value) => {
            const disabled = value !== 'auto' && isFineGranularity(value) && !canUseFineGranularity;
            return (
              <button
                key={value}
                className={granularity === value ? 'active' : ''}
                disabled={disabled}
                onClick={() => onGranularityChange(value)}
                title={disabled ? '先框选一段时间，再进入小时或分钟级' : undefined}
              >
                {granularityLabel(value)}
              </button>
            );
          })}
        </div>
        <div className="zoomControls">
          <button className="secondary iconOnly" onClick={() => onZoom('out')} disabled={zoomIndex <= 0 && granularity !== 'auto'} title="缩小时间尺度">
            <ZoomOut size={16} />
          </button>
          <button className="secondary iconOnly" onClick={() => onZoom('in')} disabled={zoomIndex >= zoomStepsLength - 1 && granularity !== 'auto'} title="放大时间尺度">
            <ZoomIn size={16} />
          </button>
          {hasTimelineWindow && (
            <button className="secondary" onClick={onResetTimelineWindow}>
              返回全局
            </button>
          )}
          <button className="secondary" disabled={seedingWorkItems || timelineLength === 0} onClick={onSeedWordClouds}>
            {seedingWorkItems ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 词云预聚合
          </button>
        </div>
      </div>
    </div>
  );
}

export function TimelineActionDock({
  hasActiveRange,
  seedingSummaries,
  creatingMergeSummary,
  summaryReady,
  creatingBranch,
  hasBranchPreview,
  onDrill,
  onSeedSummaries,
  onCreateMergeSummary,
  onSaveBranch,
}: {
  hasActiveRange: boolean;
  seedingSummaries: boolean;
  creatingMergeSummary: boolean;
  summaryReady: boolean;
  creatingBranch: boolean;
  hasBranchPreview: boolean;
  onDrill: () => void;
  onSeedSummaries: () => void;
  onCreateMergeSummary: () => void;
  onSaveBranch: () => void;
}) {
  return (
    <div className="timelineActionDock" aria-label="当前片段操作">
      <button className="secondary" disabled={!hasActiveRange} onClick={onDrill}>
        <ZoomIn size={16} /> 下钻
      </button>
      <button className="secondary" disabled={seedingSummaries || !hasActiveRange} onClick={onSeedSummaries}>
        {seedingSummaries ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 摘要
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
