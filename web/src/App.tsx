import { FormEvent, Suspense, lazy, useEffect, useMemo, useRef, useState } from 'react';
import {
  Activity,
  ArrowRight,
  CalendarClock,
  BookmarkPlus,
  ChevronLeft,
  Clock3,
  Database,
  FileText,
  History,
  Loader2,
  Lock,
  LogOut,
  Search,
  UploadCloud,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import * as echarts from 'echarts';
import {
  AnalysisJob,
  AnalysisBranch,
  AnalysisRecord,
  AnalysisWorkItem,
  BranchPreview,
  MessageSearchPage,
  PreviewPage,
  SystemStatus,
  TimelineBucket,
  TimelineCluster,
  TimelineGranularity,
  changePassword,
  createJobBranch,
  createSummaryMergeWorkItem,
  createAnalysisJob,
  getJob,
  getJobPreview,
  getJobTimeline,
  getTimelineBucketMessages,
  getMe,
  getReport,
  getSystemStatus,
  listAnalyses,
  listJobBranches,
  listJobs,
  login,
  logout,
  listWorkItems,
  prioritizeWorkItem,
  previewJobBranch,
  register,
  runJobBranch,
  searchJobMessages,
  seedWorkItems,
} from './api';

const MarkdownRenderer = lazy(() => import('react-markdown'));

type View = 'analysis' | 'history' | 'job' | 'report' | 'branch' | 'account';
type RouteState = { view: View; reportID?: string; jobID?: string; branchID?: string };
type TimelineRange = { start: string; end: string };

export function App() {
  const [user, setUser] = useState<string | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [route, setRoute] = useState<RouteState>(() => readRouteFromHash());
  const [records, setRecords] = useState<AnalysisRecord[]>([]);
  const [jobs, setJobs] = useState<AnalysisJob[]>([]);
  const [selectedJob, setSelectedJob] = useState<AnalysisJob | null>(null);
  const [selectedRecord, setSelectedRecord] = useState<AnalysisRecord | null>(null);
  const [report, setReport] = useState('');
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [historyFilter, setHistoryFilter] = useState('');
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    getMe()
      .then((me) => setUser(me.username))
      .catch(() => setUser(null))
      .finally(() => setAuthChecked(true));
  }, []);

  useEffect(() => {
    const onHashChange = () => setRoute(readRouteFromHash());
    window.addEventListener('hashchange', onHashChange);
    onHashChange()
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  useEffect(() => {
    if (!authChecked || !user) {
      return;
    }
    if (route.view === 'history') {
      void refreshHistory();
    }
    void getSystemStatus().then(setSystemStatus).catch(() => setSystemStatus(null));
    if (route.view === 'job' && route.jobID) {
      void openJobByID(route.jobID);
    }
    if (route.view === 'report' && route.reportID) {
      void openReportByID(route.reportID);
    }
  }, [authChecked, route, user]);

  async function refreshHistory(filter = historyFilter) {
    setHistoryLoading(true);
    try {
      const [jobData, recordData] = await Promise.all([listJobs(filter), listAnalyses(filter)]);
      setJobs(jobData.items ?? []);
      setRecords(recordData.items ?? []);
    } finally {
      setHistoryLoading(false);
    }
  }

  async function openReportByID(recordID: string) {
    const data = await getReport(recordID);
    setSelectedRecord(data.record);
    setReport(data.report_markdown);
  }

  async function openJobByID(jobID: string) {
    const job = await getJob(jobID);
    setSelectedJob(job);
    if (job.result) {
      setSelectedRecord(job.result);
    }
  }

  async function openJob(job: AnalysisJob) {
    await openJobByID(job.id);
    navigateTo({ view: 'job', jobID: job.id });
  }

  async function openReport(record: AnalysisRecord) {
    await openReportByID(record.id);
    navigateTo({ view: 'report', reportID: record.id });
  }

  async function handleLogout() {
    await logout();
    setUser(null);
    setSelectedRecord(null);
    setReport('');
    navigateTo({ view: 'analysis' });
  }

  function navigateTo(next: RouteState) {
    const hash = routeToHash(next);
    if (window.location.hash !== hash) {
      window.location.hash = hash;
      return;
    }
    setRoute(next);
  }

  if (!authChecked) {
    return <Splash />;
  }

  if (!user) {
    return <LoginScreen onLogin={setUser} />;
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brandMark">RA</div>
          <div>
            <strong>Record Analysis</strong>
            <span>Evidence workspace</span>
          </div>
        </div>
        <nav>
          <button className={route.view === 'analysis' ? 'active' : ''} onClick={() => navigateTo({ view: 'analysis' })}>
            <UploadCloud size={18} /> 分析
          </button>
          <button className={route.view === 'history' ? 'active' : ''} onClick={() => navigateTo({ view: 'history' })}>
            <History size={18} /> 历史记录
          </button>
          <button className={route.view === 'account' ? 'active' : ''} onClick={() => navigateTo({ view: 'account' })}>
            <Lock size={18} /> 账号设置
          </button>
        </nav>
        {systemStatus && <SystemStatusPanel status={systemStatus} />}
        <div className="sidebarFooter">
          <span>{user}</span>
          <button className="iconText" onClick={handleLogout}>
            <LogOut size={16} /> 退出
          </button>
        </div>
      </aside>
      <main className="workspace">
        {route.view === 'analysis' && (
          <AnalysisPage
            onCreated={async (job) => {
              await openJob(job);
              void refreshHistory();
            }}
          />
        )}
        {route.view === 'history' && (
          <HistoryPage
            records={records}
            jobs={jobs}
            filter={historyFilter}
            loading={historyLoading}
            onFilter={setHistoryFilter}
            onSearch={() => void refreshHistory()}
            onOpen={(record) => void openReport(record)}
            onOpenJob={(job) => void openJob(job)}
          />
        )}
        {route.view === 'job' && (
          <JobDetailPage
            initialJob={selectedJob}
            onBack={() => navigateTo({ view: 'history' })}
            onOpenReport={(record) => void openReport(record)}
          />
        )}
        {route.view === 'report' && (
          <ReportPage record={selectedRecord} markdown={report} onBack={() => navigateTo({ view: 'history' })} />
        )}
        {route.view === 'account' && <AccountPage username={user} />}
      </main>
    </div>
  );
}

function Splash() {
  return (
    <div className="centerScreen">
      <Loader2 className="spin" size={28} />
    </div>
  );
}

function SystemStatusPanel({ status }: { status: SystemStatus }) {
  const osStatus = status.opensearch.healthy ? 'healthy' : status.opensearch.enabled ? 'degraded' : 'off';
  return (
    <div className="systemStatusPanel">
      <div>
        <span>Postgres</span>
        <strong className={status.postgres.enabled ? 'ok' : 'off'}>{status.postgres.enabled ? 'enabled' : 'fallback'}</strong>
      </div>
      <div>
        <span>OpenSearch</span>
        <strong className={osStatus}>{osStatus}</strong>
      </div>
      {status.opensearch.reason && <p>{status.opensearch.reason}</p>}
    </div>
  );
}

function LoginScreen({ onLogin }: { onLogin: (username: string) => void }) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const result = mode === 'login' ? await login(username, password) : await register(username, password);
      if (mode === 'register') {
        await login(username, password);
      }
      onLogin(result.username || username);
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="loginScreen">
      <form className="loginPanel" onSubmit={submit}>
        <div className="loginIcon">
          <Lock size={22} />
        </div>
        <h1>{mode === 'login' ? '关系记录分析台' : '注册账号'}</h1>
        <p>{mode === 'login' ? '登录后上传聊天记录、运行 LLM 分析并查询历史报告。' : '创建账号后即可进入分析工作台。'}</p>
        <label>
          用户名
          <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
        </label>
        <label>
          密码
          <input
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            type="password"
            autoComplete="current-password"
          />
        </label>
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={loading}>
          {loading ? <Loader2 className="spin" size={16} /> : <ArrowRight size={16} />} {mode === 'login' ? '登录' : '注册'}
        </button>
        <button className="linkButton" type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
          {mode === 'login' ? '没有账号，去注册' : '已有账号，返回登录'}
        </button>
      </form>
    </div>
  );
}

function AnalysisPage({ onCreated }: { onCreated: (job: AnalysisJob) => void }) {
  const [relationshipID, setRelationshipID] = useState('rel_demo');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [maxMessages, setMaxMessages] = useState(500);
  const [analysisMode, setAnalysisMode] = useState<'quick' | 'full'>('quick');
  const [includeSystem, setIncludeSystem] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [job, setJob] = useState<AnalysisJob | null>(null);
  const [preview, setPreview] = useState<PreviewPage | null>(null);
  const [previewPage, setPreviewPage] = useState(1);

  useEffect(() => {
    if (!job || job.status === 'completed' || job.status === 'failed') return;
    const timer = window.setInterval(async () => {
      const next = await getJob(job.id);
      setJob(next);
      if (next.status === 'completed') {
        window.clearInterval(timer);
        setLoading(false);
      }
      if (next.status === 'failed') {
        window.clearInterval(timer);
        setError(next.error || '任务执行失败');
        setLoading(false);
      }
    }, 1500);
    return () => window.clearInterval(timer);
  }, [job]);

  useEffect(() => {
    if (!job) return;
    void getJobPreview(job.id, previewPage, 10).then(setPreview).catch(() => setPreview(null));
  }, [job?.id, previewPage]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!file) {
      setError('请选择聊天记录文件');
      return;
    }
    const form = new FormData();
    form.set('relationship_id', relationshipID);
    form.set('file', file);
    form.set('from', from);
    form.set('to', to);
    form.set('max_llm_messages', String(maxMessages));
    form.set('analysis_mode', analysisMode);
    if (includeSystem) form.set('include_system', 'true');
    setLoading(true);
    setError('');
    setJob(null);
    setPreview(null);
    setPreviewPage(1);
    try {
      const created = await createAnalysisJob(form);
      setJob(created);
      setLoading(created.status !== 'completed' && created.status !== 'failed');
      onCreated(created);
    } catch (err) {
      setError(err instanceof Error ? err.message : '分析失败');
      setLoading(false);
    }
  }

  return (
    <section className="page">
      <PageTitle icon={<UploadCloud size={22} />} title="新建分析" subtitle="上传 txt、csv、json 聊天记录并生成证据化报告" />
      <div className="analysisGrid">
        <form className="panel formPanel" onSubmit={submit}>
          <label>
            Relationship ID
            <input value={relationshipID} onChange={(event) => setRelationshipID(event.target.value)} required />
          </label>
          <label>
            聊天记录文件
            <input type="file" accept=".txt,.csv,.json" onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
          </label>
          <div className="twoColumns">
            <label>
              From
              <input value={from} onChange={(event) => setFrom(event.target.value)} placeholder="2026-06-01" />
            </label>
            <label>
              To
              <input value={to} onChange={(event) => setTo(event.target.value)} placeholder="2026-06-03" />
            </label>
          </div>
          <label>
            LLM 分析消息上限
            <input type="number" min={0} value={maxMessages} onChange={(event) => setMaxMessages(Number(event.target.value))} />
          </label>
          <label>
            LLM 执行模式
            <select value={analysisMode} onChange={(event) => setAnalysisMode(event.target.value as 'quick' | 'full')}>
              <option value="quick">快速报告：1 次 LLM 请求</option>
              <option value="full">完整抽取：4 阶段 LLM 请求</option>
            </select>
          </label>
          <label className="checkbox">
            <input type="checkbox" checked={includeSystem} onChange={(event) => setIncludeSystem(event.target.checked)} />
            包含系统消息
          </label>
          {error && <div className="error">{error}</div>}
          <button className="primary" disabled={loading}>
            {loading ? <Loader2 className="spin" size={16} /> : <Activity size={16} />} 开始分析
          </button>
        </form>
        <JobStatusPanel job={job} loading={loading} />
      </div>
      {job && <PreviewPanel preview={preview} page={previewPage} onPage={setPreviewPage} />}
    </section>
  );
}

function JobStatusPanel({ job, loading }: { job: AnalysisJob | null; loading: boolean }) {
  const progress = Math.max(0, Math.min(100, Math.round((job?.progress ?? 0) * 100)));
  return (
    <div className="panel statusPanel">
      <h2>任务执行状态</h2>
      <div className="statusLine">
        <span className={loading ? 'pulseDot' : 'dot'} />
        <strong>{job?.stage ?? '等待上传'}</strong>
      </div>
      <div className="progressBar">
        <span style={{ width: `${progress}%` }} />
      </div>
      <div className="progressText">{progress}%</div>
      <dl>
        <div>
          <dt>可处理消息</dt>
          <dd>{job ? `${job.message_count} 条` : '上传后显示'}</dd>
        </div>
        <div>
          <dt>实际送入 LLM</dt>
          <dd>{job ? `${job.llm_message_count} 条 / 上限 ${job.llm_message_limit || '不限'}` : '上传后显示'}</dd>
        </div>
        <div>
          <dt>执行模式</dt>
          <dd>{job ? (job.analysis_mode === 'quick' ? '快速报告' : '完整抽取') : '上传后显示'}</dd>
        </div>
        <div>
          <dt>当前处理</dt>
          <dd>{job ? `${job.processed_count} / ${job.llm_message_count || job.message_count}` : '等待任务开始'}</dd>
        </div>
        <div>
          <dt>Token 消耗</dt>
          <dd>{job ? `${job.total_tokens} total / ${job.prompt_tokens} prompt / ${job.completion_tokens} completion` : '运行后显示'}</dd>
        </div>
      </dl>
      <div className="eventLog">
        {(job?.events ?? []).slice(-8).map((event) => (
          <div key={`${event.time}-${event.message}`}>
            <span>{formatDate(event.time)}</span>
            <strong>{event.message}</strong>
          </div>
        ))}
      </div>
      {job?.error && <div className="error">{job.error}</div>}
    </div>
  );
}

function PreviewPanel({
  preview,
  page,
  onPage,
}: {
  preview: PreviewPage | null;
  page: number;
  onPage: (page: number) => void;
}) {
  return (
    <div className="panel previewPanel">
      <div className="previewHeader">
        <h2>消息分页预览</h2>
        <div className="previewControls">
          <button className="secondary" disabled={page <= 1} onClick={() => onPage(page - 1)}>
            上一页
          </button>
          <span>
            {preview?.page ?? page} / {preview?.total_pages ?? 1}
          </span>
          <button className="secondary" disabled={!preview || page >= preview.total_pages} onClick={() => onPage(page + 1)}>
            下一页
          </button>
        </div>
      </div>
      <div className="previewList">
        {(preview?.items ?? []).map((message) => (
          <div className="previewRow" key={message.id}>
            <span>{message.time}</span>
            <strong>{message.sender}</strong>
            <p>{message.content}</p>
          </div>
        ))}
        {preview && preview.items.length === 0 && <div className="empty">没有可预览消息</div>}
      </div>
    </div>
  );
}

function HistoryPage({
  jobs,
  records,
  filter,
  loading,
  onFilter,
  onSearch,
  onOpen,
  onOpenJob,
}: {
  jobs: AnalysisJob[];
  records: AnalysisRecord[];
  filter: string;
  loading: boolean;
  onFilter: (value: string) => void;
  onSearch: () => void;
  onOpen: (record: AnalysisRecord) => void;
  onOpenJob: (job: AnalysisJob) => void;
}) {
  return (
    <section className="page">
      <PageTitle icon={<History size={22} />} title="历史分析记录" subtitle="按关系 ID 查询任务状态和已完成报告" />
      <div className="toolbar">
        <div className="searchBox">
          <Search size={17} />
          <input value={filter} onChange={(event) => onFilter(event.target.value)} placeholder="relationship_id" />
        </div>
        <button className="secondary" onClick={onSearch}>
          {loading ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 查询
        </button>
      </div>
      <div className="historySections">
        <section>
          <h2>任务记录</h2>
          <div className="recordList">
            {jobs.map((job) => (
              <button className="recordRow jobRecord" key={job.id} onClick={() => onOpenJob(job)}>
                <div>
                  <strong>{job.relationship_id}</strong>
                  <span>{job.id}</span>
                </div>
                <div className="recordMeta">
                  <span>
                    <CalendarClock size={14} /> {formatDate(job.created_at)}
                  </span>
                  <span>
                    <Activity size={14} /> {job.status}
                  </span>
                  <span>
                    <Database size={14} /> {job.message_count} msgs
                  </span>
                  <span>
                    <FileText size={14} /> {job.analysis_mode === 'quick' ? '快速报告' : '完整抽取'}
                  </span>
                  <span>
                    <Clock3 size={14} /> {job.total_tokens || 0} tokens
                  </span>
                </div>
                <div className="miniProgress">
                  <span style={{ width: `${Math.max(0, Math.min(100, Math.round(job.progress * 100)))}%` }} />
                </div>
                <p>{job.stage}</p>
              </button>
            ))}
            {!loading && jobs.length === 0 && <div className="empty">暂无任务记录</div>}
          </div>
        </section>
        <section>
          <h2>完成报告</h2>
          <div className="recordList">
            {records.map((record) => (
              <button className="recordRow" key={record.id} onClick={() => onOpen(record)}>
                <div>
                  <strong>{record.relationship_id}</strong>
                  <span>{record.id}</span>
                </div>
                <div className="recordMeta">
                  <span>
                    <CalendarClock size={14} /> {formatDate(record.created_at)}
                  </span>
                  <span>
                    <Database size={14} /> {record.message_count} msgs
                  </span>
                  <span>
                    <FileText size={14} /> {record.model_name || 'n/a'}
                  </span>
                </div>
              </button>
            ))}
            {!loading && records.length === 0 && <div className="empty">暂无完成报告</div>}
          </div>
        </section>
      </div>
    </section>
  );
}

function JobDetailPage({
  initialJob,
  onBack,
  onOpenReport,
}: {
  initialJob: AnalysisJob | null;
  onBack: () => void;
  onOpenReport: (record: AnalysisRecord) => void;
}) {
  const [job, setJob] = useState<AnalysisJob | null>(initialJob);
  const [granularity, setGranularity] = useState<'auto' | TimelineGranularity>('auto');
  const [resolvedGranularity, setResolvedGranularity] = useState<TimelineGranularity>('hour');
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [clusters, setClusters] = useState<TimelineCluster[]>([]);
  const [selectedBucket, setSelectedBucket] = useState<TimelineBucket | null>(null);
  const [selectedCluster, setSelectedCluster] = useState<TimelineCluster | null>(null);
  const [selectedRange, setSelectedRange] = useState<TimelineRange | null>(null);
  const [timelineWindow, setTimelineWindow] = useState<TimelineRange | null>(null);
  const [branchPreview, setBranchPreview] = useState<BranchPreview | null>(null);
  const [branches, setBranches] = useState<AnalysisBranch[]>([]);
  const [workItems, setWorkItems] = useState<AnalysisWorkItem[]>([]);
  const [summaryItems, setSummaryItems] = useState<AnalysisWorkItem[]>([]);
  const [mergeItems, setMergeItems] = useState<AnalysisWorkItem[]>([]);
  const [seedingWorkItems, setSeedingWorkItems] = useState(false);
  const [seedingSummaries, setSeedingSummaries] = useState(false);
  const [creatingMergeSummary, setCreatingMergeSummary] = useState(false);
  const [expandedBranchID, setExpandedBranchID] = useState<string | null>(null);
  const [branchTitle, setBranchTitle] = useState('');
  const [creatingBranch, setCreatingBranch] = useState(false);
  const [runningBranchID, setRunningBranchID] = useState<string | null>(null);
  const [bucketMessages, setBucketMessages] = useState<PreviewPage | null>(null);
  const [bucketPage, setBucketPage] = useState(1);
  const [messageSearchQuery, setMessageSearchQuery] = useState('');
  const [messageSearchPage, setMessageSearchPage] = useState(1);
  const [messageSearchResults, setMessageSearchResults] = useState<MessageSearchPage | null>(null);
  const [searchingMessages, setSearchingMessages] = useState(false);
  const [loadingTimeline, setLoadingTimeline] = useState(false);

  useEffect(() => {
    setJob(initialJob);
  }, [initialJob]);

  useEffect(() => {
    if (!job || job.status === 'completed' || job.status === 'failed') return;
    const timer = window.setInterval(async () => {
      const next = await getJob(job.id);
      setJob(next);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [job?.id, job?.status]);

  useEffect(() => {
    if (!job) return;
    setLoadingTimeline(true);
    void loadAdaptiveTimeline(job.id, granularity, timelineWindow)
      .then((data) => {
        setResolvedGranularity(data.actualGranularity);
        setTimeline(data.items);
        setClusters(data.clusters ?? []);
        setSelectedBucket((current) => data.items.find((item) => item.id === current?.id) ?? data.items[0] ?? null);
        setSelectedCluster((current) => (current ? data.clusters.find((item) => item.id === current.id) ?? null : null));
      })
      .finally(() => setLoadingTimeline(false));
  }, [job?.id, granularity, timelineWindow?.start, timelineWindow?.end]);

  useEffect(() => {
    if (!job || !selectedBucket) {
      setBucketMessages(null);
      return;
    }
    void getTimelineBucketMessages(job.id, selectedBucket.id, bucketPage, 12).then(setBucketMessages).catch(() => setBucketMessages(null));
  }, [job?.id, selectedBucket?.id, bucketPage]);

  useEffect(() => {
    setBucketPage(1);
  }, [selectedBucket?.id]);

  useEffect(() => {
    setMessageSearchPage(1);
  }, [messageSearchQuery, branchPreview?.start_time, branchPreview?.end_time]);

  useEffect(() => {
    if (!job) {
      setBranchPreview(null);
      return;
    }
    if (selectedCluster && !selectedRange) {
      setBranchPreview({
        granularity: selectedCluster.granularity,
        start_time: selectedCluster.start_time,
        end_time: selectedCluster.end_time,
        message_count: selectedCluster.message_count,
        bucket_ids: selectedCluster.bucket_ids,
        cluster_id: selectedCluster.id,
        topic_hint: selectedCluster.topic_hint,
        status: selectedCluster.status,
      });
      return;
    }
    if (selectedBucket && !selectedRange) {
      setBranchPreview({
        granularity: selectedBucket.granularity,
        start_time: selectedBucket.start_time,
        end_time: selectedBucket.end_time,
        message_count: selectedBucket.message_count,
        bucket_ids: [selectedBucket.id],
        cluster_id: '',
        topic_hint: selectedBucket.preview,
        status: 'unseen',
      });
      return;
    }
    const range = selectedRange;
    if (!range) {
      setBranchPreview(null);
      return;
    }
    void previewJobBranch(job.id, {
      granularity: resolvedGranularity,
      start_time: range.start,
      end_time: range.end,
    }).then(setBranchPreview).catch(() => setBranchPreview(null));
  }, [job?.id, resolvedGranularity, selectedBucket?.id, selectedCluster?.id, selectedRange?.start, selectedRange?.end]);

  useEffect(() => {
    if (!job) {
      setBranches([]);
      return;
    }
    void listJobBranches(job.id).then((data) => setBranches(data.items ?? [])).catch(() => setBranches([]));
  }, [job?.id]);

  useEffect(() => {
    if (!job || branches.every((branch) => branch.status !== 'running')) return;
    const timer = window.setInterval(() => {
      void listJobBranches(job.id).then((data) => setBranches(data.items ?? []));
    }, 1500);
    return () => window.clearInterval(timer);
  }, [job?.id, branches]);

  useEffect(() => {
    if (!job || !resolvedGranularity) {
      setWorkItems([]);
      setSummaryItems([]);
      setMergeItems([]);
      return;
    }
    void refreshWorkItems();
    void refreshSummaryItems();
    void refreshMergeItems();
  }, [job?.id, resolvedGranularity]);

  useEffect(() => {
    const activeItems = [...workItems, ...summaryItems, ...mergeItems];
    if (!job || activeItems.every((item) => item.status !== 'queued' && item.status !== 'running')) return;
    const timer = window.setInterval(() => {
      void refreshWorkItems();
      void refreshSummaryItems();
      void refreshMergeItems();
    }, 2000);
    return () => window.clearInterval(timer);
  }, [job?.id, resolvedGranularity, workItems, summaryItems, mergeItems]);

  useEffect(() => {
    if (branchPreview) {
      setBranchTitle(branchPreview.topic_hint || '');
    }
  }, [branchPreview?.cluster_id, branchPreview?.start_time, branchPreview?.end_time]);

  if (!job) {
    return (
      <section className="page">
        <PageTitle icon={<Activity size={22} />} title="任务详情" subtitle="未找到任务数据" />
        <div className="panel empty">请从历史记录进入任务详情。</div>
      </section>
    );
  }

  const progress = Math.max(0, Math.min(100, Math.round(job.progress * 100)));
  const bucketPeak = Math.max(...timeline.map((item) => item.message_count), 1);
  const clusterDensity = clusters.length > 8 ? 'dense' : clusters.length > 4 ? 'compact' : 'regular';
  const visibleBranches = uniqueBranches(branches);
  const zoomSteps: TimelineGranularity[] = ['year', 'month', 'week', 'day', 'hour', '15m', '5m'];
  const activeGranularity = granularity === 'auto' ? resolvedGranularity : granularity;
  const zoomIndex = zoomSteps.indexOf(activeGranularity);
  const canUseFineGranularity = timelineWindow !== null;
  const visibleGranularities = ['auto', 'year', 'month', 'week', 'day', 'hour', '15m', '5m'] as const;

  async function saveBranch() {
    if (!job || !branchPreview) return;
    setCreatingBranch(true);
    try {
      const created = await createJobBranch(job.id, {
        granularity: resolvedGranularity,
        start_time: branchPreview.start_time,
        end_time: branchPreview.end_time,
        title: branchTitle.trim(),
      });
      setBranches((current) => [created, ...current.filter((item) => item.id !== created.id)]);
    } finally {
      setCreatingBranch(false);
    }
  }

  async function runBranch(branch: AnalysisBranch) {
    if (!job) return;
    setRunningBranchID(branch.id);
    try {
      const next = await runJobBranch(job.id, branch.id, { analysis_mode: 'quick', max_llm_messages: Math.max(1, branch.message_count) });
      setBranches((current) => current.map((item) => (item.id === next.id ? next : item)));
      const refreshed = await listJobBranches(job.id);
      setBranches(refreshed.items ?? []);
    } finally {
      setRunningBranchID(null);
    }
  }

  async function refreshWorkItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'word_cloud', resolvedGranularity).catch(() => ({ items: [] }));
    setWorkItems(data.items ?? []);
  }

  async function refreshSummaryItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'topic_summary', resolvedGranularity).catch(() => ({ items: [] }));
    setSummaryItems(data.items ?? []);
  }

  async function refreshMergeItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'summary_merge', resolvedGranularity).catch(() => ({ items: [] }));
    setMergeItems(data.items ?? []);
  }

  async function seedWordClouds() {
    if (!job) return;
    setSeedingWorkItems(true);
    try {
      const data = await seedWorkItems(job.id, resolvedGranularity, 'word_cloud', currentAnalysisRange());
      setWorkItems((current) => mergeWorkItems(current, data.items ?? []));
    } finally {
      setSeedingWorkItems(false);
    }
  }

  async function seedTopicSummaries() {
    if (!job) return;
    setSeedingSummaries(true);
    try {
      const data = await seedWorkItems(job.id, resolvedGranularity, 'topic_summary', currentAnalysisRange());
      setSummaryItems((current) => mergeWorkItems(current, data.items ?? []));
    } finally {
      setSeedingSummaries(false);
    }
  }

  function currentAnalysisRange(): TimelineRange | null {
    if (branchPreview) {
      return { start: branchPreview.start_time, end: branchPreview.end_time };
    }
    if (selectedBucket) {
      return { start: selectedBucket.start_time, end: selectedBucket.end_time };
    }
    return null;
  }

  async function prioritizeSelectedWorkItem(item: AnalysisWorkItem) {
    if (!job) return;
    const next = await prioritizeWorkItem(job.id, item.id);
    setWorkItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
    setSummaryItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
    setMergeItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
  }

  async function createCurrentMergeSummary() {
    if (!job || !branchPreview) return;
    setCreatingMergeSummary(true);
    try {
      const item = await createSummaryMergeWorkItem(job.id, {
        granularity: resolvedGranularity,
        start_time: branchPreview.start_time,
        end_time: branchPreview.end_time,
      });
      setMergeItems((current) => [item, ...current.filter((candidate) => candidate.id !== item.id)]);
    } finally {
      setCreatingMergeSummary(false);
    }
  }

  async function runMessageSearch(page = messageSearchPage) {
    if (!job) return;
    setSearchingMessages(true);
    try {
      const data = await searchJobMessages(job.id, messageSearchQuery, page, 10, currentAnalysisRange());
      setMessageSearchResults(data);
      setMessageSearchPage(data.page);
    } finally {
      setSearchingMessages(false);
    }
  }

  function zoomTimeline(direction: 'in' | 'out') {
    const nextIndex = direction === 'in' ? Math.min(zoomSteps.length - 1, zoomIndex + 1) : Math.max(0, zoomIndex - 1);
    const nextGranularity = zoomSteps[nextIndex];
    if (!canUseFineGranularity && isFineGranularity(nextGranularity) && selectedBucket) {
      setTimelineWindow({ start: selectedBucket.start_time, end: selectedBucket.end_time });
      setSelectedRange({ start: selectedBucket.start_time, end: selectedBucket.end_time });
      setGranularity('hour');
      return;
    }
    if (!canUseFineGranularity && isFineGranularity(nextGranularity)) {
      return;
    }
    setGranularity(nextGranularity);
  }

  function resetTimelineWindow() {
    setTimelineWindow(null);
    setSelectedRange(null);
    setGranularity('auto');
  }

  const selectedWorkItems = selectedCluster
    ? workItems.filter((item) => selectedCluster.bucket_ids.includes(item.scope_id))
    : selectedBucket
      ? workItems.filter((item) => item.scope_id === selectedBucket.id)
      : [];
  const selectedSummaryItems = branchPreview
    ? summaryItems.filter((item) => branchPreview.bucket_ids.includes(item.scope_id))
    : selectedCluster
      ? summaryItems.filter((item) => selectedCluster.bucket_ids.includes(item.scope_id))
      : selectedBucket
        ? summaryItems.filter((item) => item.scope_id === selectedBucket.id)
        : [];
  const selectedMergeItems = branchPreview
    ? mergeItems.filter(
        (item) =>
          item.start_time === branchPreview.start_time &&
          item.end_time === branchPreview.end_time &&
          item.granularity === resolvedGranularity,
      )
    : [];
  const selectedWordCloudTitle = selectedCluster ? '当前簇词云' : '当前桶词云';

  return (
    <section className="page">
      <PageTitle icon={<Activity size={22} />} title="任务时间轴" subtitle={`${job.relationship_id} · ${job.file_name}`} />
      <div className="jobLayout">
        <aside className="panel jobSidebar">
          <button className="secondary full" onClick={onBack}>
            <ChevronLeft size={16} /> 返回历史
          </button>
          <div className="metricGrid">
            <div className="metric">
              <span>任务状态</span>
              <strong>{job.status}</strong>
            </div>
            <div className="metric">
              <span>当前阶段</span>
              <strong>{job.stage}</strong>
            </div>
            <div className="metric">
              <span>可处理消息</span>
              <strong>{job.message_count}</strong>
            </div>
            <div className="metric">
              <span>送入 LLM</span>
              <strong>{job.llm_message_count}</strong>
            </div>
            <div className="metric">
              <span>Prompt Tokens</span>
              <strong>{job.prompt_tokens}</strong>
            </div>
            <div className="metric">
              <span>Total Tokens</span>
              <strong>{job.total_tokens}</strong>
            </div>
          </div>
          <div className="progressBar jobProgress">
            <span style={{ width: `${progress}%` }} />
          </div>
          <div className="progressText">{progress}%</div>
          {job.result && (
            <button className="primary full" onClick={() => onOpenReport(job.result!)}>
              <FileText size={16} /> 查看报告
            </button>
          )}
          <div className="eventLog dense">
            {job.events.slice(-12).map((event) => (
              <div key={`${event.time}-${event.message}`}>
                <span>{formatDate(event.time)}</span>
                <strong>{event.message}</strong>
              </div>
            ))}
          </div>
          {job.error && <div className="error">{job.error}</div>}
        </aside>

        <div className="jobMain">
          <div className="panel timelinePanel">
            <div className="timelineHeader">
              <div>
                <h2>时间轴</h2>
                <p>
                  {timelineWindow ? `局部 ${formatDate(timelineWindow.start)} - ${formatDate(timelineWindow.end)}` : '全局概览'}
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
                      onClick={() => setGranularity(value)}
                      title={disabled ? '先框选一段时间，再进入小时或分钟级' : undefined}
                    >
                      {granularityLabel(value)}
                    </button>
                  );
                })}
                </div>
                <div className="zoomControls">
                  <button className="secondary iconOnly" onClick={() => zoomTimeline('out')} disabled={zoomIndex <= 0 && granularity !== 'auto'} title="缩小时间尺度">
                    <ZoomOut size={16} />
                  </button>
                  <button className="secondary iconOnly" onClick={() => zoomTimeline('in')} disabled={zoomIndex >= zoomSteps.length - 1 && granularity !== 'auto'} title="放大时间尺度">
                    <ZoomIn size={16} />
                  </button>
                  {timelineWindow && (
                    <button className="secondary" onClick={resetTimelineWindow}>
                      返回全局
                    </button>
                  )}
                  <button className="secondary" disabled={seedingWorkItems || timeline.length === 0} onClick={() => void seedWordClouds()}>
                    {seedingWorkItems ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 词云预聚合
                  </button>
                </div>
              </div>
            </div>
            {loadingTimeline ? (
              <div className="timelineChartLoading">
                <Loader2 className="spin" size={16} /> 正在加载时间桶
              </div>
            ) : (
              <TimelineChart
                buckets={timeline}
                granularity={resolvedGranularity}
                selectedBucketID={selectedBucket?.id ?? ''}
                selectedRange={selectedRange}
                bucketPeak={bucketPeak}
                onSelectBucket={(bucket) => {
                  setSelectedBucket(bucket);
                  setSelectedRange(null);
                  setSelectedCluster(null);
                }}
                onSelectRange={(start, end, firstBucket) => {
                  setSelectedRange({ start, end });
                  setSelectedBucket(firstBucket);
                  setSelectedCluster(null);
                  if (!timelineWindow && !isFineGranularity(resolvedGranularity)) {
                    setTimelineWindow({ start, end });
                    setGranularity('hour');
                  }
                }}
              />
            )}
            {clusters.length > 0 && (
              <div className={`clusterStrip ${clusterDensity}`}>
                {clusters.map((cluster) => (
                  <button
                    key={cluster.id}
                    className={`clusterCard ${selectedCluster?.id === cluster.id ? 'active' : ''}`}
                    onClick={() => {
                      setSelectedCluster(cluster);
                      setSelectedRange(null);
                      const firstBucket = timeline.find((bucket) => bucket.id === cluster.bucket_ids[0]);
                      if (firstBucket) {
                        setSelectedBucket(firstBucket);
                      }
                    }}
                  >
                    <div>
                      <strong>{cluster.message_count} 条</strong>
                      <span>{cluster.bucket_count} 个时间桶</span>
                    </div>
                    <p>{cluster.topic_hint || '连续对话簇'}</p>
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="jobDetailGrid">
            <div className="panel bucketPanel">
              <div className="bucketHeader">
                <div>
                  <h2>选中时间桶</h2>
                  <p>{selectedBucket ? formatBucketWindow(selectedBucket) : '请选择一个时间桶'}</p>
                </div>
              </div>
              {selectedBucket ? (
                <>
                  <div className="bucketFacts">
                    <div className="metric">
                      <span>消息数</span>
                      <strong>{selectedBucket.message_count}</strong>
                    </div>
                    <div className="metric">
                      <span>参与人</span>
                      <strong>{selectedBucket.participant_count}</strong>
                    </div>
                    <div className="metric">
                      <span>摘要状态</span>
                      <strong>{selectedBucket.summary_status || 'unseen'}</strong>
                    </div>
                    <div className="metric">
                      <span>Token</span>
                      <strong>{selectedBucket.total_tokens || 0}</strong>
                    </div>
                  </div>
                  <BucketStatusStrip bucket={selectedBucket} />
                  <div className="participantChips">
                    {Object.entries(selectedBucket.participant_messages).map(([name, count]) => (
                      <div className="participantChip" key={name}>
                        <span>{name}</span>
                        <strong>{count}</strong>
                      </div>
                    ))}
                  </div>
                  <div className="bucketPreviewText">{selectedBucket.summary_title || selectedBucket.preview || '该时间桶没有可展示摘要。'}</div>
                  {selectedBucket.summary_topics && selectedBucket.summary_topics.length > 0 && (
                    <div className="topicChips">
                      {selectedBucket.summary_topics.slice(0, 5).map((topic) => (
                        <span key={topic}>{topic}</span>
                      ))}
                    </div>
                  )}
                  <WordCloudPanel title={selectedWordCloudTitle} items={selectedWorkItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
                </>
              ) : (
                <div className="empty">没有选中时间桶</div>
              )}
            </div>

            <div className="detailStack">
              <div className="panel clusterPanel">
                <div className="bucketHeader">
                  <div>
                    <h2>连续片段预览</h2>
                    <p>{branchPreview ? formatDate(branchPreview.start_time) + ' - ' + formatDate(branchPreview.end_time) : '请选择一个连续片段'}</p>
                  </div>
                </div>
                {branchPreview ? (
                  <>
                    <div className="bucketFacts">
                      <div className="metric">
                        <span>扩展后消息数</span>
                        <strong>{branchPreview.message_count}</strong>
                      </div>
                      <div className="metric">
                        <span>覆盖时间桶</span>
                        <strong>{branchPreview.bucket_ids.length}</strong>
                      </div>
                    </div>
                    <label>
                      片段标题
                      <input value={branchTitle} onChange={(event) => setBranchTitle(event.target.value)} placeholder="例如：午饭安排" />
                    </label>
                    <div className="bucketPreviewText">{branchPreview.topic_hint || '当前候选区间还没有主题提示。'}</div>
                    <button className="secondary full" disabled={seedingSummaries || timeline.length === 0} onClick={() => void seedTopicSummaries()}>
                      {seedingSummaries ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 生成片段摘要
                    </button>
                    <button
                      className="secondary full"
                      disabled={creatingMergeSummary || selectedSummaryItems.every((item) => item.status !== 'completed')}
                      onClick={() => void createCurrentMergeSummary()}
                    >
                      {creatingMergeSummary ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 聚合已完成摘要
                    </button>
                    <MergeSummaryPanel items={selectedMergeItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
                    <TopicSummaryPanel items={selectedSummaryItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
                    <button className="primary full" disabled={creatingBranch} onClick={saveBranch}>
                      {creatingBranch ? <Loader2 className="spin" size={16} /> : <BookmarkPlus size={16} />} 保存为 Branch
                    </button>
                  </>
                ) : (
                  <div className="empty">还没有候选连续片段</div>
                )}
              </div>

              <div className="panel previewPanel compact">
                <div className="previewHeader">
                  <h2>桶内消息</h2>
                  <div className="previewControls">
                    <button className="secondary" disabled={bucketPage <= 1} onClick={() => setBucketPage(bucketPage - 1)}>
                      上一页
                    </button>
                    <span>
                      {bucketMessages?.page ?? bucketPage} / {bucketMessages?.total_pages ?? 1}
                    </span>
                    <button
                      className="secondary"
                      disabled={!bucketMessages || bucketPage >= bucketMessages.total_pages}
                      onClick={() => setBucketPage(bucketPage + 1)}
                    >
                      下一页
                    </button>
                  </div>
                </div>
                <div className="previewList">
                  {(bucketMessages?.items ?? []).map((message) => (
                    <div className="previewRow" key={message.id}>
                      <span>{message.time}</span>
                      <strong>{message.sender}</strong>
                      <p>{message.content}</p>
                    </div>
                  ))}
                  {selectedBucket && bucketMessages && bucketMessages.items.length === 0 && <div className="empty">这个时间桶没有消息</div>}
                </div>
              </div>

              <div className="panel previewPanel compact">
                <div className="previewHeader">
                  <div>
                    <h2>消息搜索</h2>
                    <p>{messageSearchResults ? `${messageSearchResults.total} 条 · ${messageSearchResults.source}` : '按当前片段或时间桶搜索'}</p>
                  </div>
                </div>
                <div className="searchInline">
                  <input
                    value={messageSearchQuery}
                    onChange={(event) => setMessageSearchQuery(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') void runMessageSearch(1);
                    }}
                    placeholder="搜索消息内容或发送者"
                  />
                  <button className="secondary" disabled={searchingMessages} onClick={() => void runMessageSearch(1)}>
                    {searchingMessages ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 搜索
                  </button>
                </div>
                <div className="previewList">
                  {(messageSearchResults?.items ?? []).map((message) => (
                    <div className="previewRow" key={message.id}>
                      <span>{message.time}</span>
                      <strong>{message.sender}</strong>
                      <p>{message.content}</p>
                    </div>
                  ))}
                  {messageSearchResults && messageSearchResults.items.length === 0 && <div className="empty">没有搜索结果</div>}
                </div>
                {messageSearchResults && messageSearchResults.total_pages > 1 && (
                  <div className="previewControls">
                    <button
                      className="secondary"
                      disabled={messageSearchPage <= 1 || searchingMessages}
                      onClick={() => void runMessageSearch(messageSearchPage - 1)}
                    >
                      上一页
                    </button>
                    <span>
                      {messageSearchResults.page} / {messageSearchResults.total_pages}
                    </span>
                    <button
                      className="secondary"
                      disabled={messageSearchPage >= messageSearchResults.total_pages || searchingMessages}
                      onClick={() => void runMessageSearch(messageSearchPage + 1)}
                    >
                      下一页
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
          <div className="panel">
            <div className="bucketHeader">
              <div>
                <h2>已保存 Branch</h2>
                <p>这些片段已经从时间轴探索结果中固化下来，后续可以继续做深度分析。</p>
              </div>
            </div>
            <div className="recordList">
              {visibleBranches.map((branch) => {
                const expanded = expandedBranchID === branch.id;
                return (
                <div className="recordRow staticRow" key={branch.id}>
                  <div>
                    <strong>{branch.title}</strong>
                    <span>{branch.id}</span>
                  </div>
                  <div className="recordMeta">
                    <span>
                      <Clock3 size={14} /> {formatDate(branch.start_time)} - {formatDate(branch.end_time)}
                    </span>
                    <span>
                      <Database size={14} /> {branch.message_count} msgs
                    </span>
                    <span>
                      <Activity size={14} /> {branch.status}
                    </span>
                    <span>
                      <Clock3 size={14} /> {branch.total_tokens} tokens
                    </span>
                  </div>
                  <p>{branchSummary(branch)}</p>
                  <div className="branchActions">
                    <button className="secondary" disabled={runningBranchID === branch.id || branch.status === 'running'} onClick={() => void runBranch(branch)}>
                      {runningBranchID === branch.id ? <Loader2 className="spin" size={16} /> : <Activity size={16} />} 运行分析
                    </button>
                    {branch.report_markdown && (
                      <button className="secondary" onClick={() => setExpandedBranchID(expanded ? null : branch.id)}>
                        {expanded ? '收起结果' : '查看结果'}
                      </button>
                    )}
                    <span className="branchStage">{branch.stage || '等待分析'}</span>
                  </div>
                  {expanded && branch.report_markdown && (
                    <article className="branchReport markdownBody">
                      <Suspense fallback={<div className="empty">正在渲染结果</div>}>
                        <MarkdownRenderer>{branch.report_markdown}</MarkdownRenderer>
                      </Suspense>
                    </article>
                  )}
                  {branch.error && <div className="error">{branch.error}</div>}
                </div>
                );
              })}
              {visibleBranches.length === 0 && <div className="empty">还没有保存的 Branch</div>}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function ReportPage({ record, markdown, onBack }: { record: AnalysisRecord | null; markdown: string; onBack: () => void }) {
  const metrics = useMemo(() => {
    if (!record) return [];
    return [
      ['消息', record.message_count],
      ['动作', record.action_count],
      ['事件', record.event_count],
      ['模型', record.model_name || 'n/a'],
    ];
  }, [record]);

  return (
    <section className="page">
      <PageTitle icon={<FileText size={22} />} title="报告详情" subtitle={record?.relationship_id ?? '分析报告'} />
      <div className="reportLayout">
        <aside className="panel reportAside">
          <button className="secondary full" onClick={onBack}>
            返回历史
          </button>
          {metrics.map(([label, value]) => (
            <div className="metric" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
          {record?.object_key && (
            <div className="objectKey">
              <span>Object Key</span>
              <code>{record.object_key}</code>
            </div>
          )}
        </aside>
        <article className="panel reportMarkdown">
          <pre>{markdown || '暂无报告内容'}</pre>
        </article>
      </div>
    </section>
  );
}

function WordCloudPanel({
  title,
  items,
  onPrioritize,
}: {
  title: string;
  items: AnalysisWorkItem[];
  onPrioritize: (item: AnalysisWorkItem) => void;
}) {
  if (items.length === 0) {
    return <div className="wordCloudPanel mutedPanel">当前选择还没有词云任务，点击上方“词云预聚合”生成。</div>;
  }
  const terms = mergeWordCloudTerms(items);
  const runningCount = items.filter((item) => item.status === 'running').length;
  const queuedCount = items.filter((item) => item.status === 'queued').length;
  const failed = items.find((item) => item.status === 'failed');
  const prioritizable = items.find((item) => item.status === 'queued' || item.status === 'failed') ?? null;
  const completedCount = items.filter((item) => item.status === 'completed').length;
  const status = runningCount > 0 ? 'running' : queuedCount > 0 ? 'queued' : failed ? 'failed' : 'completed';
  return (
    <div className="wordCloudPanel">
      <div className="wordCloudHeader">
        <strong>{title}</strong>
        <span className={`workStatus ${status}`}>
          {status} · {completedCount}/{items.length}
        </span>
      </div>
      {terms.length > 0 ? (
        <div className="wordCloudTerms">
          {terms.slice(0, 18).map((term) => (
            <span key={term.term} style={{ fontSize: `${Math.min(1.35, 0.82 + term.count * 0.08)}rem` }}>
              {term.term}
              <small>{term.count}</small>
            </span>
          ))}
        </div>
      ) : status === 'queued' || status === 'running' ? (
        <p>{status === 'running' ? '正在处理当前选择。' : '已排队，等待后台 worker 处理。'}</p>
      ) : (
        <p>{failed?.error || '暂无词云结果'}</p>
      )}
      {prioritizable && (
        <button className="secondary" onClick={() => onPrioritize(prioritizable)}>
          插队处理
        </button>
      )}
    </div>
  );
}

function TopicSummaryPanel({ items, onPrioritize }: { items: AnalysisWorkItem[]; onPrioritize: (item: AnalysisWorkItem) => void }) {
  if (items.length === 0) {
    return <div className="wordCloudPanel mutedPanel">当前选择还没有摘要任务。点击“生成片段摘要”后，会按当前粒度逐个 bucket 生成。</div>;
  }
  const completed = items.filter((item) => item.status === 'completed');
  const runningCount = items.filter((item) => item.status === 'running').length;
  const queuedCount = items.filter((item) => item.status === 'queued').length;
  const failed = items.find((item) => item.status === 'failed');
  const prioritizable = items.find((item) => item.status === 'queued' || item.status === 'failed') ?? null;
  const status = runningCount > 0 ? 'running' : queuedCount > 0 ? 'queued' : failed ? 'failed' : 'completed';
  return (
    <div className="summaryPanel">
      <div className="wordCloudHeader">
        <strong>片段摘要</strong>
        <span className={`workStatus ${status}`}>
          {status} · {completed.length}/{items.length}
        </span>
      </div>
      {completed.length > 0 ? (
        <div className="summaryList">
          {completed.map((item) => {
            const summary = topicSummaryResult(item);
            if (!summary) return null;
            return (
              <article key={item.id} className="summaryItem">
                <strong>{summary.title || formatDate(item.start_time)}</strong>
                <p>{summary.summary}</p>
                {summary.topics?.length > 0 && <span>{summary.topics.join(' / ')}</span>}
                {summary.key_events?.length > 0 && (
                  <ul>
                    {summary.key_events.slice(0, 3).map((event) => (
                      <li key={event}>{event}</li>
                    ))}
                  </ul>
                )}
                <small>{item.total_tokens ? `${item.total_tokens} tokens` : 'token 未记录'}</small>
              </article>
            );
          })}
        </div>
      ) : (
        <p>{status === 'running' ? '正在生成摘要。' : failed?.error || '摘要任务已排队。'}</p>
      )}
      {prioritizable && (
        <button className="secondary" onClick={() => onPrioritize(prioritizable)}>
          插队生成
        </button>
      )}
    </div>
  );
}

function MergeSummaryPanel({ items, onPrioritize }: { items: AnalysisWorkItem[]; onPrioritize: (item: AnalysisWorkItem) => void }) {
  if (items.length === 0) {
    return <div className="wordCloudPanel mutedPanel">还没有合并摘要。先生成部分 bucket 摘要，再点击“聚合已完成摘要”。</div>;
  }
  const item = [...items].sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at))[0];
  const summary = topicSummaryResult(item);
  return (
    <div className="summaryPanel mergeSummaryPanel">
      <div className="wordCloudHeader">
        <strong>合并摘要</strong>
        <span className={`workStatus ${item.status}`}>{item.status}</span>
      </div>
      {item.status === 'completed' && summary ? (
        <article className="summaryItem">
          <strong>{summary.title || '当前片段'}</strong>
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
          <small>{item.total_tokens ? `${item.total_tokens} tokens` : 'token 未记录'}</small>
        </article>
      ) : (
        <p>{item.status === 'failed' ? item.error || '合并失败' : '合并摘要任务已提交。'}</p>
      )}
      {(item.status === 'queued' || item.status === 'failed') && (
        <button className="secondary" onClick={() => onPrioritize(item)}>
          插队生成
        </button>
      )}
    </div>
  );
}

function AccountPage({ username }: { username: string }) {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    setMessage('');
    try {
      await changePassword(oldPassword, newPassword);
      setMessage('密码已更新');
      setOldPassword('');
      setNewPassword('');
    } catch (err) {
      setError(err instanceof Error ? err.message : '密码更新失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="page">
      <PageTitle icon={<Lock size={22} />} title="账号设置" subtitle={username} />
      <form className="panel formPanel accountPanel" onSubmit={submit}>
        <label>
          当前密码
          <input type="password" value={oldPassword} onChange={(event) => setOldPassword(event.target.value)} />
        </label>
        <label>
          新密码
          <input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
        </label>
        {message && <div className="success">{message}</div>}
        {error && <div className="error">{error}</div>}
        <button className="primary" disabled={loading}>
          {loading ? <Loader2 className="spin" size={16} /> : <Lock size={16} />} 修改密码
        </button>
      </form>
    </section>
  );
}

function PageTitle({ icon, title, subtitle }: { icon: React.ReactNode; title: string; subtitle: string }) {
  return (
    <header className="pageTitle">
      <div className="titleIcon">{icon}</div>
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
    </header>
  );
}

function TimelineChart({
  buckets,
  granularity,
  selectedBucketID,
  selectedRange,
  bucketPeak,
  onSelectBucket,
  onSelectRange,
}: {
  buckets: TimelineBucket[];
  granularity: TimelineGranularity;
  selectedBucketID: string;
  selectedRange: { start: string; end: string } | null;
  bucketPeak: number;
  onSelectBucket: (bucket: TimelineBucket) => void;
  onSelectRange: (start: string, end: string, firstBucket: TimelineBucket) => void;
}) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current) return;

    const chart = echarts.init(chartRef.current);
    const chartBuckets = [...buckets].sort(
      (a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime(),
    );
    const data = chartBuckets.map((bucket) => [
      new Date(bucket.start_time).getTime(),
      0,
      bucket.message_count,
      bucket.id,
      bucket.end_time,
      bucket.preview,
      bucket.analysis_status ?? 'unseen',
      bucket.summary_status ?? '',
      bucket.word_cloud_status ?? '',
      bucket.summary_title ?? '',
      bucket.total_tokens ?? 0,
    ]);
    const selectedStart = selectedRange ? new Date(selectedRange.start).getTime() : null;
    const selectedEnd = selectedRange ? new Date(selectedRange.end).getTime() : null;
    const [xMin, xMax] = timelineExtent(chartBuckets, granularity);

    chart.setOption({
      animation: false,
      backgroundColor: 'transparent',
      grid: { left: 24, right: 18, top: 28, bottom: 52 },
      tooltip: {
        trigger: 'item',
        confine: true,
        formatter: (params: any) => {
          const index = params.dataIndex as number;
          const bucket = chartBuckets[index];
          if (!bucket) return '';
          const title = bucket.summary_title || bucket.preview;
          const preview = title ? `<br/><span style="color:#5f6f72">${escapeHtml(title.slice(0, 80))}</span>` : '';
          const summaryStatus = bucket.summary_status ? `<br/>摘要：${bucket.summary_status}` : '';
          const wordCloudStatus = bucket.word_cloud_status ? ` · 词云：${bucket.word_cloud_status}` : '';
          const tokens = bucket.total_tokens ? `<br/>${bucket.total_tokens} tokens` : '';
          return `<strong>${formatBucketWindow(bucket)}</strong><br/>${bucket.message_count} 条消息 · ${bucket.participant_count} 人${summaryStatus}${wordCloudStatus}${tokens}${preview}`;
        },
      },
      brush: {
        xAxisIndex: 'all',
        brushMode: 'single',
        throttleType: 'debounce',
        throttleDelay: 180,
        brushStyle: {
          color: 'rgba(19, 93, 102, 0.12)',
          borderColor: '#135d66',
          borderWidth: 1,
        },
      },
      dataZoom: [
        { type: 'inside', xAxisIndex: 0, filterMode: 'none', zoomOnMouseWheel: true, moveOnMouseMove: true },
        {
          type: 'slider',
          xAxisIndex: 0,
          bottom: 8,
          height: 20,
          filterMode: 'none',
          borderColor: '#d8e0df',
          fillerColor: 'rgba(19, 93, 102, 0.12)',
          handleStyle: { color: '#135d66' },
          textStyle: { color: '#6d7d80' },
        },
      ],
      xAxis: {
        type: 'time',
        min: xMin,
        max: xMax,
        axisLine: { lineStyle: { color: '#aebcbd' } },
        axisTick: { show: false },
        axisLabel: {
          color: '#657577',
          hideOverlap: true,
          formatter: (value: number) => formatTimelineAxisLabel(value, granularity),
        },
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value',
        show: false,
        min: -1,
        max: 1,
      },
      series: [
        {
          type: 'line',
          data: data.map((item) => [item[0], 0]),
          symbol: 'none',
          lineStyle: { color: '#b7c6c7', width: 2 },
          silent: true,
          markArea:
            selectedStart !== null && selectedEnd !== null
              ? {
                  silent: true,
                  itemStyle: { color: 'rgba(19, 93, 102, 0.08)' },
                  data: [[{ xAxis: selectedStart }, { xAxis: selectedEnd }]],
                }
              : undefined,
        },
        {
          type: 'scatter',
          data,
          symbolSize: (value: unknown[]) => {
            const count = Number(value[2] ?? 0);
            return Math.max(9, Math.min(30, 9 + (count / Math.max(bucketPeak, 1)) * 21));
          },
          itemStyle: {
            color: (params: any) => bucketStatusColor(params.data?.[6], params.data?.[3] === selectedBucketID),
            borderColor: '#ffffff',
            borderWidth: 2,
            shadowBlur: 8,
            shadowColor: 'rgba(19, 93, 102, 0.22)',
          },
          emphasis: {
            scale: 1.2,
            itemStyle: { color: '#0a3f46' },
          },
          label: {
            show: true,
            position: 'top',
            formatter: (params: any) => {
              const count = String(params.data?.[2] ?? '');
              const marker = bucketStatusMarker(params.data?.[6]);
              return `${count}${marker}`;
            },
            color: '#26383b',
            fontSize: 11,
          },
        },
      ],
      graphic:
        buckets.length === 0
          ? [
              {
                type: 'text',
                left: 'center',
                top: 'middle',
                style: { text: '暂无时间桶', fill: '#7a888a', fontSize: 14 },
              },
            ]
          : [],
    });

    if (buckets.length > 0) {
      chart.dispatchAction({
        type: 'takeGlobalCursor',
        key: 'brush',
        brushOption: { brushType: 'rect', brushMode: 'single' },
      });
    }

    const handleClick = (params: any) => {
      const bucketID = params.data?.[3];
      const bucket = chartBuckets.find((item) => item.id === bucketID);
      if (bucket) onSelectBucket(bucket);
    };
    const handleBrushSelected = (params: any) => {
      const selected = params.batch?.[0]?.selected?.[1]?.dataIndex ?? [];
      if (!Array.isArray(selected) || selected.length === 0) return;
      const selectedBuckets = selected
        .map((index: number) => chartBuckets[index])
        .filter(Boolean)
        .sort((a: TimelineBucket, b: TimelineBucket) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime());
      if (selectedBuckets.length === 0) return;
      const first = selectedBuckets[0];
      const last = selectedBuckets[selectedBuckets.length - 1];
      onSelectRange(first.start_time, last.end_time, first);
    };

    chart.on('click', handleClick);
    chart.on('brushSelected', handleBrushSelected);

    const resizeObserver = new ResizeObserver(() => chart.resize());
    resizeObserver.observe(chartRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.off('click', handleClick);
      chart.off('brushSelected', handleBrushSelected);
      chart.dispose();
    };
  }, [buckets, bucketPeak, granularity, onSelectBucket, onSelectRange, selectedBucketID, selectedRange]);

  return <div className="timelineChart" ref={chartRef} aria-label="聊天记录时间轴" />;
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

function formatBucketWindow(bucket: TimelineBucket) {
  return `${formatDate(bucket.start_time)} - ${formatDate(bucket.end_time)}`;
}

function timelineExtent(buckets: TimelineBucket[], granularity: TimelineGranularity): [number | undefined, number | undefined] {
  if (buckets.length === 0) return [undefined, undefined];
  const firstStart = new Date(buckets[0].start_time).getTime();
  const lastEnd = new Date(buckets[buckets.length - 1].end_time).getTime();
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
  const date = new Date(value);
  const options: Intl.DateTimeFormatOptions =
    granularity === 'year'
      ? { year: 'numeric' }
      : granularity === 'month'
        ? { year: '2-digit', month: '2-digit' }
        : granularity === 'week' || granularity === 'day'
          ? { month: '2-digit', day: '2-digit' }
          : { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' };
  return new Intl.DateTimeFormat('zh-CN', options).format(date);
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function formatBucketRange(bucket: TimelineBucket) {
  const start = new Date(bucket.start_time);
  let options: Intl.DateTimeFormatOptions;
  switch (bucket.granularity) {
    case 'year':
      options = { year: '2-digit' };
      break;
    case 'month':
      options = { year: '2-digit', month: '2-digit' };
      break;
    case 'week':
    case 'day':
      options = { month: '2-digit', day: '2-digit' };
      break;
    default:
      options = { hour: '2-digit', minute: '2-digit' };
  }
  return new Intl.DateTimeFormat('zh-CN', options).format(start);
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
      return '#15808c';
  }
}

function bucketStatusMarker(status: unknown) {
  switch (status) {
    case 'running':
      return ' · 运行';
    case 'queued':
      return ' · 排队';
    case 'failed':
      return ' · 失败';
    case 'completed':
      return ' · 完成';
    default:
      return '';
  }
}

function BucketStatusStrip({ bucket }: { bucket: TimelineBucket }) {
  const entries = [
    ['summary', bucket.summary_status || 'unseen'],
    ['word cloud', bucket.word_cloud_status || 'unseen'],
    ['analysis', bucket.analysis_status || 'unseen'],
  ];
  return (
    <div className="bucketStatusStrip">
      {entries.map(([label, status]) => (
        <span className={`workStatus ${status}`} key={label}>
          {label}: {status}
        </span>
      ))}
    </div>
  );
}

function uniqueBranches(branches: AnalysisBranch[]) {
  const byWindow = new Map<string, AnalysisBranch>();
  for (const branch of branches) {
    const key = `${branch.job_id}:${branch.granularity}:${branch.start_time}:${branch.end_time}`;
    const current = byWindow.get(key);
    if (!current || new Date(branch.updated_at).getTime() > new Date(current.updated_at).getTime()) {
      byWindow.set(key, branch);
    }
  }
  return Array.from(byWindow.values()).sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
}

function mergeWorkItems(current: AnalysisWorkItem[], incoming: AnalysisWorkItem[]) {
  const byID = new Map<string, AnalysisWorkItem>();
  for (const item of current) {
    byID.set(item.id, item);
  }
  for (const item of incoming) {
    byID.set(item.id, item);
  }
  return Array.from(byID.values()).sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime());
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

function mergeWordCloudTerms(items: AnalysisWorkItem[]) {
  const counts = new Map<string, number>();
  for (const item of items) {
    if (!Array.isArray(item.result)) continue;
    for (const term of item.result) {
      counts.set(term.term, (counts.get(term.term) ?? 0) + term.count);
    }
  }
  return Array.from(counts.entries())
    .map(([term, count]) => ({ term, count }))
    .sort((a, b) => (a.count === b.count ? a.term.localeCompare(b.term) : b.count - a.count));
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

function isFineGranularity(value: TimelineGranularity) {
  return value === 'hour' || value === '15m' || value === '5m';
}

async function loadAdaptiveTimeline(jobID: string, granularity: 'auto' | TimelineGranularity, range: TimelineRange | null) {
  if (granularity !== 'auto') {
    const data = await getJobTimeline(jobID, granularity, range);
    return { actualGranularity: granularity, items: data.items, clusters: data.clusters ?? [] };
  }
  const candidates: TimelineGranularity[] = range ? ['day', 'hour', '15m', '5m'] : ['year', 'month', 'week', 'day'];
  let fallback: { actualGranularity: TimelineGranularity; items: TimelineBucket[]; clusters: TimelineCluster[] } | null = null;
  let sparseCandidate: { actualGranularity: TimelineGranularity; items: TimelineBucket[]; clusters: TimelineCluster[] } | null = null;
  for (const candidate of candidates) {
    const data = await getJobTimeline(jobID, candidate, range);
    const current = { actualGranularity: candidate, items: data.items, clusters: data.clusters ?? [] };
    fallback = current;
    if (data.items.length >= 6 && data.items.length <= 80) {
      return current;
    }
    if (data.items.length > 0 && data.items.length < 6) {
      sparseCandidate = current;
    }
  }
  return sparseCandidate ?? fallback ?? { actualGranularity: range ? 'hour' : 'day', items: [], clusters: [] };
}

function readRouteFromHash(): RouteState {
  const raw = window.location.hash.replace(/^#/, '') || '/analysis';
  const path = raw.startsWith('/') ? raw : `/${raw}`;
  if (path === '/history') return { view: 'history' };
  if (path === '/account') return { view: 'account' };
  if (path.startsWith('/job/')) {
    const jobID = decodeURIComponent(path.slice('/job/'.length));
    return jobID ? { view: 'job', jobID } : { view: 'history' };
  }
  if (path.startsWith('/report/')) {
    const reportID = decodeURIComponent(path.slice('/report/'.length));
    return reportID ? { view: 'report', reportID } : { view: 'history' };
  }
  return { view: 'analysis' };
}

function routeToHash(route: RouteState): string {
  switch (route.view) {
    case 'history':
      return '#/history';
    case 'account':
      return '#/account';
    case 'job':
      return route.jobID ? `#/job/${encodeURIComponent(route.jobID)}` : '#/history';
    case 'report':
      return route.reportID ? `#/report/${encodeURIComponent(route.reportID)}` : '#/history';
    default:
      return '#/analysis';
  }
}
