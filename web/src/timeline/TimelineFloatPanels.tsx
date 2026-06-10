import { X } from 'lucide-react';
import type { MessageSearchPage, TimelineBucket } from '../api';

export type TimelineFloatPanelKey = 'scope' | 'bucket' | 'evidence';

export function TimelineFloatPanels({
  openPanel,
  onTogglePanel,
  onClose,
  activeScopeKind,
  activeScopeWindow,
  scopeHint,
  messageCount,
  bucketCount,
  summaryCoverageText,
  wordCloudTaskCount,
  selectedBucket,
  evidencePage,
  evidenceRangeLabel,
  loadingEvidence,
  evidencePageNumber,
  onEvidencePageChange,
}: {
  openPanel: TimelineFloatPanelKey | null;
  onTogglePanel: (panel: TimelineFloatPanelKey) => void;
  onClose: () => void;
  activeScopeKind: string;
  activeScopeWindow: string;
  scopeHint: string;
  messageCount: number;
  bucketCount: number;
  summaryCoverageText: string;
  wordCloudTaskCount: number;
  selectedBucket: TimelineBucket | null;
  evidencePage: MessageSearchPage | null;
  evidenceRangeLabel: string;
  loadingEvidence: boolean;
  evidencePageNumber: number;
  onEvidencePageChange: (page: number) => void;
}) {
  return (
    <>
      <div className="timelineFloatDock" aria-label="时间线浮动面板">
        {(['scope', 'bucket', 'evidence'] as const).map((panel) => (
          <button key={panel} className={openPanel === panel ? 'active' : ''} onClick={() => onTogglePanel(panel)}>
            <strong>{panel === 'scope' ? messageCount : panel === 'bucket' ? selectedBucket?.message_count ?? 0 : evidencePage?.total ?? 0}</strong>
            <span>{panel === 'scope' ? '当前片段' : panel === 'bucket' ? '时间桶' : '证据'}</span>
          </button>
        ))}
      </div>
      {openPanel && (
        <div className={`timelineFloatPanel ${openPanel}`}>
          <button className="secondary iconOnly floatPanelClose" onClick={onClose} title="收起浮动面板">
            <X size={16} />
          </button>
          {openPanel === 'scope' && (
            <>
              <span className="floatPanelKicker">{activeScopeKind}</span>
              <strong>{activeScopeWindow}</strong>
              <p>{scopeHint}</p>
              <div className="floatMetricGrid">
                <div>
                  <span>消息</span>
                  <strong>{messageCount}</strong>
                </div>
                <div>
                  <span>Bucket</span>
                  <strong>{bucketCount}</strong>
                </div>
                <div>
                  <span>摘要覆盖</span>
                  <strong>{summaryCoverageText}</strong>
                </div>
                <div>
                  <span>词云任务</span>
                  <strong>{wordCloudTaskCount}</strong>
                </div>
              </div>
            </>
          )}
          {openPanel === 'bucket' && (
            <>
              <span className="floatPanelKicker">选中时间桶</span>
              <strong>{selectedBucket ? formatBucketWindow(selectedBucket) : '未选中'}</strong>
              <p>{selectedBucket?.summary_title || selectedBucket?.preview || '点击时间轴上的点查看时间桶。'}</p>
              {selectedBucket && (
                <>
                  <div className="floatMetricGrid">
                    <div>
                      <span>消息</span>
                      <strong>{selectedBucket.message_count}</strong>
                    </div>
                    <div>
                      <span>参与人</span>
                      <strong>{selectedBucket.participant_count}</strong>
                    </div>
                    <div>
                      <span>摘要</span>
                      <strong>{statusText(selectedBucket.summary_status)}</strong>
                    </div>
                    <div>
                      <span>Tokens</span>
                      <strong>{selectedBucket.total_tokens || 0}</strong>
                    </div>
                  </div>
                  {selectedBucket.summary_topics && selectedBucket.summary_topics.length > 0 && (
                    <div className="topicChips">
                      {selectedBucket.summary_topics.slice(0, 5).map((topic) => (
                        <span key={topic}>{topic}</span>
                      ))}
                    </div>
                  )}
                </>
              )}
            </>
          )}
          {openPanel === 'evidence' && (
            <>
              <span className="floatPanelKicker">证据预览</span>
              <strong>{evidencePage ? `${evidencePage.total} 条消息` : evidenceRangeLabel}</strong>
              <p>{evidenceRangeLabel}</p>
              <div className="floatPreviewList">
                {(evidencePage?.items ?? []).slice(0, 6).map((message) => (
                  <div key={message.id}>
                    <span>
                      {message.time} · {message.sender}
                    </span>
                    <p>{message.content}</p>
                  </div>
                ))}
                {loadingEvidence && <p>正在加载当前范围消息...</p>}
                {evidencePage && evidencePage.items.length === 0 && <p>当前范围没有可展示消息。</p>}
                {!evidencePage && !loadingEvidence && <p>点击时间轴点位或框选范围后，这里展示证据消息。</p>}
              </div>
              {evidencePage && evidencePage.total_pages > 1 && (
                <div className="floatPager">
                  <button className="secondary" disabled={evidencePageNumber <= 1 || loadingEvidence} onClick={() => onEvidencePageChange(Math.max(1, evidencePageNumber - 1))}>
                    上一页
                  </button>
                  <span>
                    {evidencePage.page} / {evidencePage.total_pages}
                  </span>
                  <button className="secondary" disabled={evidencePageNumber >= evidencePage.total_pages || loadingEvidence} onClick={() => onEvidencePageChange(evidencePageNumber + 1)}>
                    下一页
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </>
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

function formatBucketWindow(bucket: TimelineBucket) {
  return `${formatDate(bucket.start_time)} - ${formatDate(bucket.end_time)}`;
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
