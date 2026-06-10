import { lazy, Suspense } from 'react';
import { Activity, Clock3, Database, Loader2, X } from 'lucide-react';
import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket } from '../api';

const MarkdownRenderer = lazy(() => import('react-markdown'));

export type TimelineDetailSelection =
  | { kind: 'branch'; branch: AnalysisBranch }
  | { kind: 'insight'; item: AnalysisWorkItem }
  | { kind: 'bucket'; bucket: TimelineBucket; coverageText: string; wordCloudTaskCount: number }
  | {
      kind: 'range';
      range: { start: string; end: string };
      title: string;
      messageCount: number;
      bucketCount: number;
      coverageText: string;
      wordCloudTaskCount: number;
      tokenCount: number;
      hint: string;
    };

export function TimelineDetailPanel({
  selection,
  runningBranchID,
  onRunBranch,
  onPrioritize,
  onEvidence,
  onExpandBranch,
  onClose,
}: {
  selection: TimelineDetailSelection | null;
  runningBranchID: string | null;
  onRunBranch: (branch: AnalysisBranch) => void;
  onPrioritize: (item: AnalysisWorkItem) => void;
  onEvidence: (ids: string[]) => void;
  onExpandBranch: (branch: AnalysisBranch) => void;
  onClose: () => void;
}) {
  if (!selection) {
    return (
      <aside className="branchInspector emptyInspector">
        <div className="branchInspectorHeader">
          <div>
            <span>详情</span>
            <strong>未选中节点</strong>
          </div>
        </div>
        <p>点击时间线节点、框选一段时间，或打开 Branch / 洞察节点查看详情。</p>
      </aside>
    );
  }
  switch (selection.kind) {
    case 'branch':
      return <BranchInspector branch={selection.branch} runningBranchID={runningBranchID} onRun={onRunBranch} onExpand={onExpandBranch} onClose={onClose} />;
    case 'insight':
      return <InsightInspector item={selection.item} onPrioritize={onPrioritize} onEvidence={onEvidence} onClose={onClose} />;
    case 'bucket':
      return <BucketDetailPanel bucket={selection.bucket} coverageText={selection.coverageText} wordCloudTaskCount={selection.wordCloudTaskCount} onClose={onClose} />;
    case 'range':
      return <RangeDetailPanel selection={selection} onClose={onClose} />;
  }
}

function BucketDetailPanel({
  bucket,
  coverageText,
  wordCloudTaskCount,
  onClose,
}: {
  bucket: TimelineBucket;
  coverageText: string;
  wordCloudTaskCount: number;
  onClose: () => void;
}) {
  return (
    <aside className="branchInspector bucketInspector">
      <div className="branchInspectorHeader">
        <div>
          <span>时间桶</span>
          <strong>{formatDate(bucket.start_time)} - {formatDate(bucket.end_time)}</strong>
        </div>
        <button className="secondary iconOnly" onClick={onClose} title="关闭详情面板">
          <X size={16} />
        </button>
      </div>
      <div className="branchInspectorMeta">
        <span>
          <Database size={14} /> {bucket.message_count} msgs
        </span>
        <span>
          <Activity size={14} /> {statusText(bucket.analysis_status)}
        </span>
        <span>
          <Clock3 size={14} /> 摘要 {statusText(bucket.summary_status)}
        </span>
        <span>
          <Clock3 size={14} /> 词云 {statusText(bucket.word_cloud_status)}
        </span>
      </div>
      <p className="branchInspectorSummary">{bucket.summary_title || bucket.preview || '当前时间桶还没有稳定摘要。'}</p>
      {bucket.summary_topics && bucket.summary_topics.length > 0 && (
        <div className="topicChips">
          {bucket.summary_topics.slice(0, 8).map((topic) => (
            <span key={topic}>{topic}</span>
          ))}
        </div>
      )}
      <div className="floatMetricGrid">
        <div>
          <span>参与人</span>
          <strong>{bucket.participant_count}</strong>
        </div>
        <div>
          <span>Tokens</span>
          <strong>{bucket.total_tokens || 0}</strong>
        </div>
        <div>
          <span>子摘要</span>
          <strong>{coverageText}</strong>
        </div>
        <div>
          <span>词云任务</span>
          <strong>{wordCloudTaskCount}</strong>
        </div>
      </div>
    </aside>
  );
}

function RangeDetailPanel({
  selection,
  onClose,
}: {
  selection: Extract<TimelineDetailSelection, { kind: 'range' }>;
  onClose: () => void;
}) {
  return (
    <aside className="branchInspector rangeInspector">
      <div className="branchInspectorHeader">
        <div>
          <span>{selection.title}</span>
          <strong>{formatDate(selection.range.start)} - {formatDate(selection.range.end)}</strong>
        </div>
        <button className="secondary iconOnly" onClick={onClose} title="关闭详情面板">
          <X size={16} />
        </button>
      </div>
      <div className="branchInspectorMeta">
        <span>
          <Database size={14} /> {selection.messageCount} msgs
        </span>
        <span>
          <Activity size={14} /> {selection.bucketCount} buckets
        </span>
        <span>
          <Clock3 size={14} /> 摘要 {selection.coverageText}
        </span>
        <span>
          <Clock3 size={14} /> 词云 {selection.wordCloudTaskCount}
        </span>
        <span>
          <Clock3 size={14} /> {selection.tokenCount} tokens
        </span>
      </div>
      <p className="branchInspectorSummary">{selection.hint}</p>
    </aside>
  );
}

export function BranchInspector({
  branch,
  runningBranchID,
  onRun,
  onExpand,
  onClose,
}: {
  branch: AnalysisBranch | null;
  runningBranchID: string | null;
  onRun: (branch: AnalysisBranch) => void;
  onExpand?: (branch: AnalysisBranch) => void;
  onClose: () => void;
}) {
  if (!branch) {
    return (
      <aside className="branchInspector emptyInspector">
        <div className="branchInspectorHeader">
          <div>
            <span>Branch</span>
            <strong>未选中分支</strong>
          </div>
        </div>
        <p>点击时间轴下方的分支节点，或从 Branch 索引打开一个分支。</p>
      </aside>
    );
  }
  const running = runningBranchID === branch.id || branch.status === 'running';
  return (
    <aside className="branchInspector">
      <div className="branchInspectorHeader">
        <div>
          <span>Branch</span>
          <strong>{branch.title || branch.topic_hint || branch.id}</strong>
        </div>
        <button className="secondary iconOnly" onClick={onClose} title="关闭分支面板">
          <X size={16} />
        </button>
      </div>
      <div className="branchInspectorMeta">
        <span>
          <Clock3 size={14} /> {formatDate(branch.start_time)} - {formatDate(branch.end_time)}
        </span>
        <span>
          <Database size={14} /> {branch.message_count} msgs
        </span>
        <span>
          <Activity size={14} /> {statusText(branch.status)}
        </span>
        <span>
          <Clock3 size={14} /> {branch.total_tokens} tokens
        </span>
      </div>
      <p className="branchInspectorSummary">{branchSummary(branch)}</p>
      <div className="branchActions">
        <button className="primary" disabled={running || branch.message_count <= 0} onClick={() => onRun(branch)}>
          {running ? <Loader2 className="spin" size={16} /> : <Activity size={16} />} 运行分析
        </button>
        {onExpand && (
          <button className="secondary" onClick={() => onExpand(branch)}>
            全屏阅读全文
          </button>
        )}
        <span className="branchStage">{branch.message_count <= 0 ? '当前片段没有可分析消息' : branch.stage || '等待分析'}</span>
      </div>
      {branch.error && <div className="error">{branch.error}</div>}
      {branch.report_markdown ? (
        <article className="branchInspectorReport markdownBody">
          <Suspense fallback={<div className="empty">正在渲染结果</div>}>
            <MarkdownRenderer>{branch.report_markdown}</MarkdownRenderer>
          </Suspense>
        </article>
      ) : (
        <div className="empty">还没有深度分析报告。运行分析后，结果会在这里展开。</div>
      )}
    </aside>
  );
}

export function InsightInspector({
  item,
  onPrioritize,
  onEvidence,
  onClose,
}: {
  item: AnalysisWorkItem | null;
  onPrioritize: (item: AnalysisWorkItem) => void;
  onEvidence: (ids: string[]) => void;
  onClose: () => void;
}) {
  if (!item) {
    return null;
  }
  const summary = topicSummaryResult(item);
  const terms = Array.isArray(item.result) ? item.result : [];
  const canPrioritize = item.status === 'queued' || item.status === 'failed';
  return (
    <aside className="branchInspector insightInspector">
      <div className="branchInspectorHeader">
        <div>
          <span>{workItemKindText(item.kind)}</span>
          <strong>{summary?.title || `${formatDate(item.start_time)} - ${formatDate(item.end_time)}`}</strong>
        </div>
        <button className="secondary iconOnly" onClick={onClose} title="关闭详情面板">
          <X size={16} />
        </button>
      </div>
      <div className="branchInspectorMeta">
        <span>
          <Clock3 size={14} /> {formatDate(item.start_time)} - {formatDate(item.end_time)}
        </span>
        <span>
          <Activity size={14} /> {statusText(item.status)}
        </span>
        <span>
          <Database size={14} /> {item.message_count} msgs
        </span>
        <span>
          <Clock3 size={14} /> {item.total_tokens} tokens
        </span>
      </div>
      {summary ? (
        <article className="insightSummary">
          <p>{summary.summary}</p>
          {summary.topics?.length > 0 && <span>{summary.topics.join(' / ')}</span>}
          {summary.key_events?.length > 0 && (
            <ul>
              {summary.key_events.map((event) => (
                <li key={event}>{event}</li>
              ))}
            </ul>
          )}
          {summary.evidence_msg_ids?.length > 0 && (
            <div className="evidenceChips">
              {summary.evidence_msg_ids.slice(0, 8).map((id) => (
                <button key={id} className="secondary" onClick={() => onEvidence([id])}>
                  {id}
                </button>
              ))}
              {summary.evidence_msg_ids.length > 1 && (
                <button className="primary" onClick={() => onEvidence(summary.evidence_msg_ids)}>
                  查看证据
                </button>
              )}
            </div>
          )}
          {summary.uncertainty && <small>{summary.uncertainty}</small>}
        </article>
      ) : terms.length > 0 ? (
        <div className="wordCloudTerms inspectorTerms">
          {terms.slice(0, 24).map((term) => (
            <span key={term.term} style={{ fontSize: `${Math.min(1.35, 0.82 + term.count * 0.08)}rem` }}>
              {term.term}
              <small>{term.count}</small>
            </span>
          ))}
        </div>
      ) : (
        <div className="empty">{item.error || '该节点还没有可展示结果。'}</div>
      )}
      {item.error && <div className="error">{item.error}</div>}
      {canPrioritize && (
        <button className="secondary" onClick={() => onPrioritize(item)}>
          插队处理
        </button>
      )}
    </aside>
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

function branchSummary(branch: AnalysisBranch) {
  const markdownSummary = firstMarkdownParagraph(branch.report_markdown ?? '');
  if (markdownSummary) {
    return markdownSummary;
  }
  if (branch.topic_hint) {
    return branch.topic_hint;
  }
  return '暂无摘要';
}

function firstMarkdownParagraph(markdown: string) {
  const lines = markdown
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .filter((line) => !line.startsWith('#'))
    .filter((line) => !/^[-*_]{3,}$/.test(line));
  const first = lines[0] ?? '';
  return first.length > 180 ? `${first.slice(0, 180)}...` : first;
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
