import { lazy, Suspense } from 'react';
import { Activity, Clock3, Database, Loader2, X } from 'lucide-react';
import type { AnalysisBranch, AnalysisWorkItem } from '../api';

const MarkdownRenderer = lazy(() => import('react-markdown'));

export function BranchInspector({
  branch,
  runningBranchID,
  onRun,
  onClose,
}: {
  branch: AnalysisBranch | null;
  runningBranchID: string | null;
  onRun: (branch: AnalysisBranch) => void;
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
  onClose,
}: {
  item: AnalysisWorkItem | null;
  onPrioritize: (item: AnalysisWorkItem) => void;
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
