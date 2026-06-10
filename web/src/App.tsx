import { FormEvent, lazy, Suspense, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
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
  X,
  ZoomIn,
} from 'lucide-react';
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
  getBranch,
  getJob,
  getJobMessagesByIDs,
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
import {
  CanvasTimelineRenderer,
  EChartsTimelineRenderer,
  TimelineActionDock,
  TimelineDetailPanel,
  TimelineDetailSelection,
  TimelineFloatPanels,
  TimelineHeaderControls,
  TimelineUtilityDrawer,
  TimelineUtilityPanel,
} from './timeline';

const MarkdownRenderer = lazy(() => import('react-markdown'));

type View = 'analysis' | 'history' | 'job' | 'report' | 'branch' | 'account';
type RouteState = { view: View; reportID?: string; jobID?: string; branchID?: string; range?: TimelineRange; entity?: string };
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

  useEffect(() => {
    if (!authChecked || !user || route.view !== 'history') return;
    if (jobs.every((job) => job.status !== 'running' && job.status !== 'queued')) return;
    const timer = window.setInterval(() => {
      void refreshHistory();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [authChecked, user, route.view, historyJobsStatusSignature(jobs)]);

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
    <div className={`shell ${route.view === 'job' ? 'timelineMode' : ''}`}>
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
            route={route}
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
  const activeJobs = jobs.filter((job) => job.status === 'running' || job.status === 'queued');
  const latestJobs = jobs.slice(0, 3);
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
      <div className="historyOverview">
        <div className="historyOverviewHeader">
          <div>
            <span>运行中</span>
            <strong>{activeJobs.length}</strong>
          </div>
          <div>
            <span>最近任务</span>
            <strong>{latestJobs.length}</strong>
          </div>
          <div>
            <span>完成报告</span>
            <strong>{records.length}</strong>
          </div>
        </div>
        <div className="historyQuickList">
          {(activeJobs.length > 0 ? activeJobs : latestJobs).map((job) => (
            <button className="historyQuickJob" key={job.id} onClick={() => onOpenJob(job)}>
              <span className={`workStatus ${job.status}`}>{statusText(job.status)}</span>
              <strong>{job.relationship_id}</strong>
              <small>{job.stage || formatDate(job.created_at)}</small>
            </button>
          ))}
          {!loading && jobs.length === 0 && <div className="empty">暂无可恢复任务</div>}
        </div>
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
  route,
  onBack,
  onOpenReport,
}: {
  initialJob: AnalysisJob | null;
  route: RouteState;
  onBack: () => void;
  onOpenReport: (record: AnalysisRecord) => void;
}) {
  const [job, setJob] = useState<AnalysisJob | null>(initialJob);
  const [granularity, setGranularity] = useState<'auto' | TimelineGranularity>('auto');
  const [resolvedGranularity, setResolvedGranularity] = useState<TimelineGranularity>('hour');
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [insightBuckets, setInsightBuckets] = useState<TimelineBucket[]>([]);
  const [globalActionBuckets, setGlobalActionBuckets] = useState<TimelineBucket[]>([]);
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
  const [rangeMessages, setRangeMessages] = useState<MessageSearchPage | null>(null);
  const [rangeMessagesPage, setRangeMessagesPage] = useState(1);
  const [loadingRangeMessages, setLoadingRangeMessages] = useState(false);
  const [searchingMessages, setSearchingMessages] = useState(false);
  const [loadingTimeline, setLoadingTimeline] = useState(false);
  const [openTimelinePanel, setOpenTimelinePanel] = useState<'scope' | 'bucket' | 'evidence' | null>('scope');
  const [openUtilityPanel, setOpenUtilityPanel] = useState<TimelineUtilityPanel | null>(null);
  const [fullscreenUtilityPanel, setFullscreenUtilityPanel] = useState<TimelineUtilityPanel | null>(null);
  const [fullscreenBranch, setFullscreenBranch] = useState<AnalysisBranch | null>(null);
  const [loadingFullscreenBranch, setLoadingFullscreenBranch] = useState(false);
  const [detailPanelOpen, setDetailPanelOpen] = useState(false);
  const [selectedInsightID, setSelectedInsightID] = useState<string | null>(null);
  const [timelineRenderer, setTimelineRenderer] = useState<'echarts' | 'canvas'>('echarts');
  const lastTimelineSignatureRef = useRef('');
  const routeStateAppliedKey = useMemo(() => {
    if (!initialJob?.id) return '';
    return `${initialJob.id}|${window.location.hash}`;
  }, [initialJob?.id]);

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
    let cancelled = false;
    void loadAdaptiveTimeline(job.id, granularity, timelineWindow)
      .then((data) => {
        if (cancelled) return;
        const signature = timelineDataSignature(data.actualGranularity, data.items, data.clusters ?? []);
        if (signature === lastTimelineSignatureRef.current) {
          return;
        }
        lastTimelineSignatureRef.current = signature;
        setResolvedGranularity(data.actualGranularity);
        setTimeline(data.items);
        setClusters(data.clusters ?? []);
        setSelectedBucket((current) => data.items.find((item) => item.id === current?.id) ?? data.items[0] ?? null);
        setSelectedCluster((current) => (current ? data.clusters.find((item) => item.id === current.id) ?? null : null));
      })
      .finally(() => {
        if (!cancelled) setLoadingTimeline(false);
      });
    return () => {
      cancelled = true;
    };
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
    setRangeMessagesPage(1);
  }, [branchPreview?.start_time, branchPreview?.end_time, selectedRange?.start, selectedRange?.end, selectedBucket?.id]);

  useEffect(() => {
    if (openTimelinePanel !== 'evidence') return;
    void loadRangeMessages(rangeMessagesPage);
  }, [openTimelinePanel, rangeMessagesPage, branchPreview?.start_time, branchPreview?.end_time, selectedRange?.start, selectedRange?.end, selectedBucket?.id]);

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
  }, [job?.id, resolvedGranularity, selectedBucket?.id, selectedCluster?.id, selectedRange?.start, selectedRange?.end, timelineWindow?.start, timelineWindow?.end]);

  useEffect(() => {
    if (!job) {
      setInsightBuckets([]);
      return;
    }
    const range = currentNodeRange();
    if (!range) {
      setInsightBuckets([]);
      return;
    }
    void getJobTimeline(job.id, childInsightGranularity(), range)
      .then((data) => setInsightBuckets(data.items ?? []))
      .catch(() => setInsightBuckets([]));
  }, [job?.id, resolvedGranularity, selectedBucket?.id, selectedCluster?.id, selectedRange?.start, selectedRange?.end, timelineWindow?.start, timelineWindow?.end]);

  useEffect(() => {
    if (!job) {
      setGlobalActionBuckets([]);
      return;
    }
    const range = globalActionRange();
    if (!range) {
      setGlobalActionBuckets([]);
      return;
    }
    void getJobTimeline(job.id, childInsightGranularity(), range)
      .then((data) => setGlobalActionBuckets(data.items ?? []))
      .catch(() => setGlobalActionBuckets([]));
  }, [job?.id, resolvedGranularity, timelineWindow?.start, timelineWindow?.end, timeline.length, timeline[0]?.start_time, timeline[timeline.length - 1]?.end_time]);

  useEffect(() => {
    const activeItems = [...workItems, ...summaryItems, ...mergeItems];
    if (!job || activeItems.every((item) => item.status !== 'queued' && item.status !== 'running')) return;
    const timer = window.setInterval(() => {
      void refreshWorkItems();
      void refreshSummaryItems();
      void refreshMergeItems();
    }, 2000);
    return () => window.clearInterval(timer);
  }, [job?.id, resolvedGranularity, workItemStatusSignature(workItems, summaryItems, mergeItems)]);

  useEffect(() => {
    if (branchPreview) {
      setBranchTitle(branchPreview.topic_hint || '');
    }
  }, [branchPreview?.cluster_id, branchPreview?.start_time, branchPreview?.end_time]);

  useEffect(() => {
    if (!job || route.view !== 'job' || route.jobID !== job.id) return;
    if (route.range) {
      setTimelineWindow(route.range);
      setSelectedRange(route.range);
      setSelectedCluster(null);
      const bucket = timeline.find((candidate) => rangesOverlap(candidate.start_time, candidate.end_time, route.range!.start, route.range!.end));
      if (bucket) {
        setSelectedBucket(bucket);
      }
    }
    if (route.entity) {
      if (route.entity.startsWith('branch:')) {
        setExpandedBranchID(route.entity.slice('branch:'.length));
        setSelectedInsightID(null);
      }
      if (route.entity.startsWith('insight:')) {
        setSelectedInsightID(route.entity.slice('insight:'.length));
        setExpandedBranchID(null);
      }
    }
  }, [routeStateAppliedKey, job?.id, timeline.length]);

  useEffect(() => {
    if (!job || route.view !== 'job' || route.jobID !== job.id) return;
    const entity = expandedBranchID ? `branch:${expandedBranchID}` : selectedInsightID ? `insight:${selectedInsightID}` : undefined;
    const hash = routeToHash({ view: 'job', jobID: job.id, range: selectedRange ?? undefined, entity });
    if (window.location.hash !== hash) {
      window.history.replaceState(null, '', hash);
    }
  }, [job?.id, route.view, route.jobID, selectedRange?.start, selectedRange?.end, expandedBranchID, selectedInsightID]);

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
  const timelineStats = bucketAnalysisStats(timeline);

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
    } catch (err) {
      const message = err instanceof Error ? err.message : '运行分析失败';
      setBranches((current) =>
        current.map((item) =>
          item.id === branch.id
            ? {
                ...item,
                status: 'failed',
                stage: '运行分析失败',
                error: message,
              }
            : item,
        ),
      );
    } finally {
      setRunningBranchID(null);
    }
  }

  async function refreshWorkItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'word_cloud', childInsightGranularity(), currentNodeRange()).catch(() => ({ items: [] }));
    setWorkItems(data.items ?? []);
  }

  async function refreshSummaryItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'topic_summary', childInsightGranularity(), currentNodeRange()).catch(() => ({ items: [] }));
    setSummaryItems(data.items ?? []);
  }

  async function refreshMergeItems() {
    if (!job) return;
    const data = await listWorkItems(job.id, 'summary_merge', childInsightGranularity(), currentNodeRange()).catch(() => ({ items: [] }));
    setMergeItems(data.items ?? []);
  }

  async function seedWordClouds() {
    return seedWordCloudsForRange(currentNodeRange());
  }

  async function seedGlobalWordClouds() {
    return seedWordCloudsForRange(globalActionRange());
  }

  async function seedWordCloudsForRange(range: TimelineRange | null) {
    if (!job) return;
    setSeedingWorkItems(true);
    try {
      const data = await seedWorkItems(job.id, childInsightGranularity(), 'word_cloud', range);
      setWorkItems((current) => mergeWorkItems(current, data.items ?? []));
      openUtilityOverlay('insights');
    } finally {
      setSeedingWorkItems(false);
    }
  }

  async function seedTopicSummaries() {
    return seedTopicSummariesForRange(currentNodeRange());
  }

  async function seedGlobalTopicSummaries() {
    return seedTopicSummariesForRange(globalActionRange());
  }

  async function seedTopicSummariesForRange(range: TimelineRange | null) {
    if (!job) return;
    setSeedingSummaries(true);
    try {
      const data = await seedWorkItems(job.id, childInsightGranularity(), 'topic_summary', range);
      setSummaryItems((current) => mergeWorkItems(current, data.items ?? []));
      openUtilityOverlay('insights');
    } finally {
      setSeedingSummaries(false);
    }
  }

  function currentAnalysisRange(): TimelineRange | null {
    if (selectedRange) {
      return selectedRange;
    }
    if (branchPreview) {
      return { start: branchPreview.start_time, end: branchPreview.end_time };
    }
    if (selectedBucket) {
      return { start: selectedBucket.start_time, end: selectedBucket.end_time };
    }
    return null;
  }

  function currentNodeRange(): TimelineRange | null {
    if (selectedRange) {
      return selectedRange;
    }
    if (selectedCluster) {
      return { start: selectedCluster.start_time, end: selectedCluster.end_time };
    }
    if (selectedBucket) {
      return { start: selectedBucket.start_time, end: selectedBucket.end_time };
    }
    return timelineWindow ?? visibleTimelineRange();
  }

  function visibleTimelineRange(): TimelineRange | null {
    if (timeline.length === 0) return null;
    return {
      start: timeline[0].start_time,
      end: timeline[timeline.length - 1].end_time,
    };
  }

  function globalActionRange(): TimelineRange | null {
    return timelineWindow ?? visibleTimelineRange();
  }

  function childInsightGranularity(): TimelineGranularity {
    switch (resolvedGranularity) {
      case 'year':
        return 'month';
      case 'month':
        return 'week';
      case 'week':
        return 'day';
      case 'day':
        return 'hour';
      case 'hour':
        return '15m';
      default:
        return '5m';
    }
  }

  function childInsightBucketIDs() {
    return new Set(insightBuckets.map((bucket) => bucket.id));
  }

  async function prioritizeSelectedWorkItem(item: AnalysisWorkItem) {
    if (!job) return;
    const next = await prioritizeWorkItem(job.id, item.id);
    setWorkItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
    setSummaryItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
    setMergeItems((current) => current.map((candidate) => (candidate.id === next.id ? next : candidate)));
  }

  async function createCurrentMergeSummary() {
    if (!job) return;
    const range = currentNodeRange();
    if (!range) return;
    setCreatingMergeSummary(true);
    try {
      const item = await createSummaryMergeWorkItem(job.id, {
        granularity: childInsightGranularity(),
        start_time: range.start,
        end_time: range.end,
      });
      setMergeItems((current) => [item, ...current.filter((candidate) => candidate.id !== item.id)]);
      openUtilityOverlay('insights');
    } finally {
      setCreatingMergeSummary(false);
    }
  }

  async function createGlobalMergeSummary() {
    if (!job) return;
    const range = globalActionRange();
    if (!range) return;
    setCreatingMergeSummary(true);
    try {
      const item = await createSummaryMergeWorkItem(job.id, {
        granularity: childInsightGranularity(),
        start_time: range.start,
        end_time: range.end,
      });
      setMergeItems((current) => [item, ...current.filter((candidate) => candidate.id !== item.id)]);
      openUtilityOverlay('insights');
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

  async function loadRangeMessages(page = rangeMessagesPage) {
    if (!job) return;
    const range = currentAnalysisRange();
    if (!range) {
      setRangeMessages(null);
      return;
    }
    setLoadingRangeMessages(true);
    try {
      const data = await searchJobMessages(job.id, '', page, 8, range);
      setRangeMessages(data);
      setRangeMessagesPage(data.page);
    } finally {
      setLoadingRangeMessages(false);
    }
  }

  async function loadEvidenceMessages(ids: string[]) {
    if (!job || ids.length === 0) return;
    const data = await getJobMessagesByIDs(job.id, ids);
    setRangeMessages({ ...data, source: 'evidence' });
    setRangeMessagesPage(1);
    openFloatOverlay('evidence', true);
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

  function drillIntoCurrentRange() {
    const range = currentAnalysisRange() ?? globalActionRange();
    if (!range) {
      zoomTimeline('in');
      return;
    }
    setTimelineWindow(range);
    setSelectedRange(range);
    setGranularity('hour');
  }

  function clearTimelineSelection() {
    setSelectedRange(null);
    setSelectedCluster(null);
    setSelectedBucket(null);
    setExpandedBranchID(null);
    setSelectedInsightID(null);
    setBranchPreview(null);
    setDetailPanelOpen(false);
  }

  function openFloatOverlay(panel: 'scope' | 'bucket' | 'evidence', forceOpen = false) {
    setOpenUtilityPanel(null);
    setFullscreenUtilityPanel(null);
    setDetailPanelOpen(false);
    setOpenTimelinePanel((current) => (forceOpen || current !== panel ? panel : null));
  }

  function openUtilityOverlay(panel: TimelineUtilityPanel) {
    setOpenTimelinePanel(null);
    setFullscreenUtilityPanel(null);
    setDetailPanelOpen(false);
    setOpenUtilityPanel((current) => (current === panel ? null : panel));
  }

  function openFullscreenUtilityOverlay(panel: TimelineUtilityPanel) {
    setOpenTimelinePanel(null);
    setOpenUtilityPanel(null);
    setDetailPanelOpen(false);
    setFullscreenBranch(null);
    setFullscreenUtilityPanel(panel);
  }

  async function openBranchFullscreen(branch: AnalysisBranch) {
    setOpenTimelinePanel(null);
    setOpenUtilityPanel(null);
    setFullscreenUtilityPanel(null);
    setDetailPanelOpen(false);
    setFullscreenBranch(branch);
    setLoadingFullscreenBranch(true);
    try {
      const fresh = await getBranch(branch.id);
      setFullscreenBranch(fresh);
      setBranches((current) => current.map((item) => (item.id === fresh.id ? fresh : item)));
    } finally {
      setLoadingFullscreenBranch(false);
    }
  }

  function openDetailOverlay() {
    setOpenTimelinePanel(null);
    setOpenUtilityPanel(null);
    setFullscreenUtilityPanel(null);
    setFullscreenBranch(null);
    setDetailPanelOpen(true);
  }

  function selectBranchForDetail(branch: AnalysisBranch) {
    setSelectedRange({ start: branch.start_time, end: branch.end_time });
    setSelectedCluster(null);
    setSelectedInsightID(null);
    setExpandedBranchID(branch.id);
    const bucket = timeline.find((candidate) => rangesOverlap(candidate.start_time, candidate.end_time, branch.start_time, branch.end_time));
    if (bucket) {
      setSelectedBucket(bucket);
    }
    openDetailOverlay();
  }

  const activeRange = currentAnalysisRange();
  const TimelineRenderer = timelineRenderer === 'canvas' ? CanvasTimelineRenderer : EChartsTimelineRenderer;
  const insightRange = currentNodeRange();
  const insightGranularity = childInsightGranularity();
  const insightBucketIDs = childInsightBucketIDs();
  const globalActionBucketIDs = new Set(globalActionBuckets.map((bucket) => bucket.id));
  const globalSummaryItems = summaryItems.filter((item) => item.granularity === insightGranularity && (globalActionBucketIDs.has(item.scope_id) || (globalActionRange() ? rangeContains(globalActionRange()!.start, globalActionRange()!.end, item.start_time, item.end_time) : true)));
  const globalSummaryCoverage = summaryCoverageStats(globalActionBucketIDs.size, globalSummaryItems);
  const insightWorkItems = insightRange
    ? workItems.filter((item) => item.granularity === insightGranularity && (insightBucketIDs.has(item.scope_id) || rangeContains(insightRange.start, insightRange.end, item.start_time, item.end_time)))
    : [];
  const selectedSummaryItems = insightRange
    ? summaryItems.filter((item) => item.granularity === insightGranularity && (insightBucketIDs.has(item.scope_id) || rangeContains(insightRange.start, insightRange.end, item.start_time, item.end_time)))
    : [];
  const selectedMergeItems = insightRange
    ? mergeItems.filter(
        (item) =>
          item.start_time === insightRange.start &&
          item.end_time === insightRange.end &&
          item.granularity === insightGranularity,
      )
    : [];
  const summaryCoverage = summaryCoverageStats(insightBucketIDs.size, selectedSummaryItems);
  const selectedWordCloudTitle = selectedRange ? '子周期词云' : selectedCluster ? '簇内子周期词云' : selectedBucket ? '桶内子周期词云' : '当前节点子周期词云';
  const activeScopeKind = selectedRange ? '框选范围' : selectedCluster ? '连续对话簇' : selectedBucket ? '单个时间桶' : '未选择';
  const activeScopeWindow = activeRange ? `${formatDate(activeRange.start)} - ${formatDate(activeRange.end)}` : '请选择时间轴上的点或范围';
  const insightScopeWindow = insightRange ? `${formatDate(insightRange.start)} - ${formatDate(insightRange.end)}` : activeScopeWindow;
  const searchScopeLabel = activeRange ? `${activeScopeKind} · ${activeScopeWindow}` : '全量消息';
  const selectedBranch = visibleBranches.find((branch) => branch.id === expandedBranchID) ?? null;
  const insightItems = [...summaryItems, ...mergeItems, ...workItems];
  const selectedInsight = insightItems.find((item) => item.id === selectedInsightID) ?? null;
  const visibleTokenTotal = timeline.reduce((sum, bucket) => sum + (bucket.total_tokens || 0), 0);
  const currentInsightTokenTotal = selectedMergeItems.reduce((sum, item) => sum + (item.total_tokens || 0), 0) + selectedSummaryItems.reduce((sum, item) => sum + (item.total_tokens || 0), 0);
  const selectedTokenTotal = selectedBranch?.total_tokens || selectedInsight?.total_tokens || currentInsightTokenTotal || selectedBucket?.total_tokens || 0;
  const evidencePage = rangeMessages ?? (bucketMessages ? { ...bucketMessages, source: 'bucket' } : null);
  const evidenceRangeLabel = activeRange ? `${formatDate(activeRange.start)} - ${formatDate(activeRange.end)}` : selectedBucket ? formatBucketWindow(selectedBucket) : '未选中范围';
  const timelineDetailSelection: TimelineDetailSelection | null = selectedBranch
    ? { kind: 'branch', branch: selectedBranch }
    : selectedInsight
      ? { kind: 'insight', item: selectedInsight }
      : selectedRange
        ? {
            kind: 'range',
            range: selectedRange,
            title: '框选范围',
            messageCount: branchPreview?.message_count ?? selectedBucket?.message_count ?? 0,
            bucketCount: branchPreview?.bucket_ids.length ?? insightBucketIDs.size,
            coverageText: `${summaryCoverage.completed}/${summaryCoverage.expected}`,
            wordCloudTaskCount: insightWorkItems.length,
            tokenCount: selectedTokenTotal,
            hint: branchPreview?.topic_hint || selectedBucket?.summary_title || selectedBucket?.preview || '当前框选范围还没有稳定摘要。',
          }
        : selectedCluster
          ? {
              kind: 'range',
              range: { start: selectedCluster.start_time, end: selectedCluster.end_time },
              title: '连续对话簇',
              messageCount: selectedCluster.message_count,
              bucketCount: selectedCluster.bucket_count,
              coverageText: `${summaryCoverage.completed}/${summaryCoverage.expected}`,
              wordCloudTaskCount: insightWorkItems.length,
              tokenCount: selectedTokenTotal,
              hint: selectedCluster.topic_hint || '连续对话簇',
            }
          : selectedBucket
            ? {
                kind: 'bucket',
                bucket: selectedBucket,
                coverageText: `${summaryCoverage.completed}/${summaryCoverage.expected}`,
                wordCloudTaskCount: insightWorkItems.length,
              }
            : null;
  const utilityTitle = openUtilityPanel === 'insights' ? '词云与摘要' : openUtilityPanel === 'tasks' ? '当前片段任务' : openUtilityPanel === 'messages' ? '桶内消息' : openUtilityPanel === 'search' ? '消息搜索' : openUtilityPanel === 'branches' ? '已保存 Branch' : activeScopeKind;
  const fullscreenUtilityTitle = fullscreenUtilityPanel === 'insights' ? '词云与摘要' : fullscreenUtilityPanel === 'tasks' ? '当前片段任务' : fullscreenUtilityPanel === 'messages' ? '桶内消息' : fullscreenUtilityPanel === 'search' ? '消息搜索' : fullscreenUtilityPanel === 'branches' ? '已保存 Branch' : fullscreenUtilityPanel === 'status' ? '任务状态' : activeScopeKind;
  const utilitySubtitle =
    openUtilityPanel === 'status'
      ? `${job.status} · ${progress}%`
      : openUtilityPanel === 'messages'
      ? selectedBucket
        ? formatBucketWindow(selectedBucket)
        : '请选择一个时间桶'
      : openUtilityPanel === 'search'
        ? messageSearchResults
          ? `${messageSearchResults.total} 条 · ${messageSearchResults.source}`
          : '按当前片段或时间桶搜索'
        : openUtilityPanel === 'branches'
          ? `${visibleBranches.length} 个已保存片段`
        : openUtilityPanel === 'insights'
          ? `当前节点的 ${granularityText(insightGranularity)} 子周期 · ${summaryCoverage.completed}/${summaryCoverage.expected} 个摘要`
        : activeScopeWindow;
  const activeUtilityPanel = fullscreenUtilityPanel ?? openUtilityPanel;
  const utilityPanel = (
    <>
      {activeUtilityPanel === 'status' && (
        <div className="utilityStack">
          <InteractionFlow />
          <div className="timelineStatusStrip drawerStatusStrip">
            <div>
              <span>任务状态</span>
              <strong>{job.status}</strong>
            </div>
            <div>
              <span>消息</span>
              <strong>{job.message_count}</strong>
            </div>
            <div>
              <span>LLM</span>
              <strong>{job.llm_message_count}</strong>
            </div>
            <div>
              <span>Tokens</span>
              <strong>{job.total_tokens}</strong>
            </div>
          </div>
          <div className="progressBar jobProgress">
            <span style={{ width: `${progress}%` }} />
          </div>
          <div className="progressText">{progress}% · {job.stage}</div>
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
        </div>
      )}
      {activeUtilityPanel === 'insights' && (
        <div className="utilityStack">
          <InsightWorkflowPanel
            scopeWindow={insightScopeWindow}
            childGranularity={granularityText(insightGranularity)}
            coverage={summaryCoverage}
            summaryCount={selectedSummaryItems.length}
            mergeCount={selectedMergeItems.length}
            wordCloudCount={insightWorkItems.length}
            wordCloudItems={insightWorkItems}
            seedingSummaries={seedingSummaries}
            creatingMergeSummary={creatingMergeSummary}
            seedingWorkItems={seedingWorkItems}
            onSeedSummaries={() => void seedTopicSummaries()}
            onMerge={() => void createCurrentMergeSummary()}
            onSeedWordClouds={() => void seedWordClouds()}
          />
          <WordCloudPanel title={selectedWordCloudTitle} items={insightWorkItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
          <MergeSummaryPanel items={selectedMergeItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} coverage={summaryCoverage} />
          <TopicSummaryPanel items={selectedSummaryItems} coverage={summaryCoverage} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
        </div>
      )}
      {activeUtilityPanel === 'scope' && (
        <div className="utilityStack">
          <div className="floatMetricGrid">
            <div>
              <span>消息</span>
              <strong>{branchPreview?.message_count ?? selectedBucket?.message_count ?? 0}</strong>
            </div>
            <div>
              <span>Bucket</span>
              <strong>{branchPreview?.bucket_ids.length ?? (selectedBucket ? 1 : 0)}</strong>
            </div>
            <div>
              <span>摘要覆盖</span>
              <strong>{summaryCoverage.completed}/{summaryCoverage.expected}</strong>
            </div>
            <div>
              <span>词云任务</span>
              <strong>{insightWorkItems.length}</strong>
            </div>
          </div>
          <div className="scopeNotice">
            <strong>操作对象</strong>
            <span>{selectedRange ? '当前框选节点' : selectedCluster ? '当前连续对话簇节点' : selectedBucket ? '当前时间桶节点' : '未选择'}</span>
          </div>
          <label>
            Branch 标题
            <input value={branchTitle} onChange={(event) => setBranchTitle(event.target.value)} placeholder="例如：午饭安排" />
          </label>
          <div className="bucketPreviewText">{branchPreview?.topic_hint || selectedBucket?.summary_title || selectedBucket?.preview || '当前候选区间还没有主题提示。'}</div>
          <button className="primary full" disabled={creatingBranch || !branchPreview} onClick={saveBranch}>
            {creatingBranch ? <Loader2 className="spin" size={16} /> : <BookmarkPlus size={16} />} 保存为 Branch
          </button>
        </div>
      )}
      {activeUtilityPanel === 'tasks' && (
        <div className="utilityStack">
          <WorkItemOverview
            title="当前片段任务"
            items={[...selectedSummaryItems, ...selectedMergeItems]}
            expectedSummaries={branchPreview?.bucket_ids.length ?? 0}
            onPrioritize={(item) => void prioritizeSelectedWorkItem(item)}
          />
          <button className="secondary full" disabled={seedingSummaries || timeline.length === 0} onClick={() => void seedTopicSummaries()}>
            {seedingSummaries ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} {summaryCoverage.missing > 0 ? `生成缺失摘要 (${summaryCoverage.missing})` : '重新检查片段摘要'}
          </button>
          <button
            className="secondary full"
            disabled={creatingMergeSummary || !summaryCoverage.ready}
            onClick={() => void createCurrentMergeSummary()}
            title={!summaryCoverage.ready ? '需要当前范围内所有 bucket 摘要完成后才能聚合' : undefined}
          >
            {creatingMergeSummary ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 聚合完整周期摘要
          </button>
          <MergeSummaryPanel items={selectedMergeItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} coverage={summaryCoverage} />
          <TopicSummaryPanel items={selectedSummaryItems} coverage={summaryCoverage} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
          <WordCloudPanel title={selectedWordCloudTitle} items={insightWorkItems} onPrioritize={(item) => void prioritizeSelectedWorkItem(item)} />
        </div>
      )}
      {activeUtilityPanel === 'messages' && (
        <div className="utilityStack">
          <div className="previewHeader">
            <h2>桶内消息</h2>
            <div className="previewControls">
              <button className="secondary" disabled={bucketPage <= 1} onClick={() => setBucketPage(bucketPage - 1)}>
                上一页
              </button>
              <span>
                {bucketMessages?.page ?? bucketPage} / {bucketMessages?.total_pages ?? 1}
              </span>
              <button className="secondary" disabled={!bucketMessages || bucketPage >= bucketMessages.total_pages} onClick={() => setBucketPage(bucketPage + 1)}>
                下一页
              </button>
            </div>
          </div>
          <div className="previewList utilityPreviewList">
            {(bucketMessages?.items ?? []).map((message) => (
              <div className="previewRow" key={message.id}>
                <span>{message.time}</span>
                <strong>{message.sender}</strong>
                <p>{message.content}</p>
              </div>
            ))}
            {selectedBucket && bucketMessages && bucketMessages.items.length === 0 && <div className="empty">这个时间桶没有消息</div>}
            {!selectedBucket && <div className="empty">请先在时间轴上选择一个时间桶</div>}
          </div>
        </div>
      )}
      {activeUtilityPanel === 'search' && (
        <div className="utilityStack">
          <div className="scopeNotice searchScopeNotice">
            <strong>搜索范围</strong>
            <span>{searchScopeLabel}</span>
            {activeRange && (
              <button
                className="secondary"
                onClick={() => {
                  setSelectedRange(null);
                  setSelectedCluster(null);
                  setSelectedBucket(null);
                  setBranchPreview(null);
                  setMessageSearchResults(null);
                }}
              >
                解除限定
              </button>
            )}
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
          <div className="previewList utilityPreviewList">
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
              <button className="secondary" disabled={messageSearchPage <= 1 || searchingMessages} onClick={() => void runMessageSearch(messageSearchPage - 1)}>
                上一页
              </button>
              <span>
                {messageSearchResults.page} / {messageSearchResults.total_pages}
              </span>
              <button className="secondary" disabled={messageSearchPage >= messageSearchResults.total_pages || searchingMessages} onClick={() => void runMessageSearch(messageSearchPage + 1)}>
                下一页
              </button>
            </div>
          )}
        </div>
      )}
      {activeUtilityPanel === 'branches' && (
        <div className="recordList utilityBranchList">
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
                  <button className="secondary" disabled={runningBranchID === branch.id || branch.status === 'running' || branch.message_count <= 0} onClick={() => void runBranch(branch)}>
                    {runningBranchID === branch.id ? <Loader2 className="spin" size={16} /> : <Activity size={16} />} 运行分析
                  </button>
                  <button className="secondary" onClick={() => void openBranchFullscreen(branch)}>
                    全文
                  </button>
                  <button
                    className="secondary"
                    onClick={() => {
                      if (expanded) {
                        setExpandedBranchID(null);
                        setDetailPanelOpen(false);
                        return;
                      }
                      selectBranchForDetail(branch);
                    }}
                  >
                    {expanded ? '关闭面板' : '打开面板'}
                  </button>
                  <span className="branchStage">{branch.message_count <= 0 ? '当前片段没有可分析消息' : branch.stage || '等待分析'}</span>
                </div>
                {branch.error && <div className="error">{branch.error}</div>}
              </div>
            );
          })}
          {visibleBranches.length === 0 && <div className="empty">还没有保存的 Branch</div>}
        </div>
      )}
    </>
  );

  return (
    <section className="page timelinePage">
      <div className={`timelineShell ${openUtilityPanel ? 'drawerOpen' : ''} ${openTimelinePanel ? 'floatOpen' : ''}`}>
        <div className="timelineTopBar">
          <button className="secondary" onClick={onBack}>
            <ChevronLeft size={16} /> 返回历史
          </button>
          <div className="timelineTitleBlock">
            <span>任务时间轴</span>
            <strong>{job.relationship_id}</strong>
            <p>{job.file_name}</p>
          </div>
          <div className="timelineStatusStrip">
            <div>
              <span>状态</span>
              <strong>{job.status}</strong>
            </div>
            <div>
              <span>阶段</span>
              <strong>{job.stage}</strong>
            </div>
            <div>
              <span>消息</span>
              <strong>{job.message_count}</strong>
            </div>
            <div>
              <span>Tokens</span>
              <strong>{job.total_tokens}</strong>
            </div>
          </div>
        </div>
        <div className="timelineMain">
          <div className="timelinePanel">
            {loadingTimeline ? (
              <div className="timelineChartLoading">
                <Loader2 className="spin" size={16} /> 正在加载时间桶
              </div>
            ) : (
              <div className="timelineStage">
                <div className="timelineGlassPanel timelineMetaFloat">
                  <div className="timelineMetaPrimary">
                    <span><strong>{timeline.length}</strong> 时间桶</span>
                    <span><strong>{timelineStats.completed}</strong> 已完成</span>
                    <span><strong>{timelineStats.running + timelineStats.queued}</strong> 处理中</span>
                    <span><strong>{timelineStats.failed}</strong> 失败</span>
                  </div>
                  <div className="timelineTokenStrip">
                    <span>
                      <strong>{visibleTokenTotal}</strong>
                      可见 Tokens
                    </span>
                    <span>
                      <strong>{selectedTokenTotal}</strong>
                      当前 Tokens
                    </span>
                  </div>
                  <div className="timelineLegend">
                    {(['completed', 'running', 'queued', 'failed', 'unseen'] as const).map((status) => (
                      <span key={status}>
                        <i style={{ background: bucketStatusColor(status, false) }} />
                        {statusText(status)}
                      </span>
                    ))}
                  </div>
                  {selectedRange && (
                    <div className="timelineSelection">
                      已选区间 {formatDate(selectedRange.start)} - {formatDate(selectedRange.end)}
                    </div>
                  )}
                </div>
                <div className="timelineControlDock">
                  <TimelineHeaderControls
                    timelineWindowLabel={timelineWindow ? `局部 ${formatDate(timelineWindow.start)} - ${formatDate(timelineWindow.end)}` : '全局概览'}
                    granularity={granularity}
                    resolvedGranularity={resolvedGranularity}
                    hasTimelineWindow={Boolean(timelineWindow)}
                    onResetTimelineWindow={resetTimelineWindow}
                  />
                  <TimelineActionDock
                    seedingSummaries={seedingSummaries}
                    creatingMergeSummary={creatingMergeSummary}
                    summaryReady={globalSummaryCoverage.ready}
                    summaryMissing={globalSummaryCoverage.missing}
                    creatingBranch={creatingBranch}
                    hasBranchPreview={Boolean(branchPreview)}
                    granularity={granularity}
                    visibleGranularities={visibleGranularities}
                    canUseFineGranularity={canUseFineGranularity}
                    zoomIndex={zoomIndex}
                    zoomStepsLength={zoomSteps.length}
                    seedingWorkItems={seedingWorkItems}
                    timelineLength={timeline.length}
                    onGranularityChange={setGranularity}
                    onZoom={zoomTimeline}
                    onSeedWordClouds={() => void seedGlobalWordClouds()}
                    onDrill={drillIntoCurrentRange}
                    onSeedSummaries={() => void seedGlobalTopicSummaries()}
                    onCreateMergeSummary={() => void createGlobalMergeSummary()}
                    onSaveBranch={saveBranch}
                  />
                  <div className="rendererSwitch" aria-label="时间线渲染器">
                    <button className={timelineRenderer === 'echarts' ? 'active' : ''} onClick={() => setTimelineRenderer('echarts')}>
                      ECharts
                    </button>
                    <button className={timelineRenderer === 'canvas' ? 'active' : ''} onClick={() => setTimelineRenderer('canvas')}>
                      Canvas
                    </button>
                  </div>
                </div>
                {timelineDetailSelection && !detailPanelOpen && (
                  <button className="timelineDetailReopen" onClick={openDetailOverlay}>
                    打开当前详情
                  </button>
                )}
                <TimelineFloatPanels
                  openPanel={openTimelinePanel}
                  onTogglePanel={openFloatOverlay}
                  onClose={() => setOpenTimelinePanel(null)}
                  activeScopeKind={activeScopeKind}
                  activeScopeWindow={activeScopeWindow}
                  scopeHint={branchPreview?.topic_hint || selectedBucket?.summary_title || selectedBucket?.preview || '当前选择还没有稳定摘要。'}
                  messageCount={branchPreview?.message_count ?? selectedBucket?.message_count ?? 0}
                  bucketCount={branchPreview?.bucket_ids.length ?? (selectedBucket ? 1 : 0)}
                  summaryCoverageText={`${summaryCoverage.completed}/${summaryCoverage.expected}`}
                  wordCloudTaskCount={insightWorkItems.length}
                  selectedBucket={selectedBucket}
                  evidencePage={evidencePage}
                  evidenceRangeLabel={evidenceRangeLabel}
                  loadingEvidence={loadingRangeMessages}
                  evidencePageNumber={rangeMessagesPage}
                  onEvidencePageChange={setRangeMessagesPage}
                />
                <TimelineUtilityDrawer
                  activePanel={openUtilityPanel}
                  title={utilityTitle}
                  subtitle={utilitySubtitle}
                  onPanelChange={openUtilityOverlay}
                  onClose={() => setOpenUtilityPanel(null)}
                  onExpand={openFullscreenUtilityOverlay}
                >
                  {utilityPanel}
                </TimelineUtilityDrawer>
                <TimelineRenderer
                  buckets={timeline}
                  granularity={resolvedGranularity}
                  selectedBucketID={selectedBucket?.id ?? ''}
                  selectedBranchID={expandedBranchID ?? ''}
                  selectedInsightID={selectedInsightID ?? ''}
                  selectedRange={selectedRange}
                  bucketPeak={bucketPeak}
                  wordCloudItems={workItems}
                  summaryItems={[...summaryItems, ...mergeItems]}
                  branches={visibleBranches}
                  onSelectBucket={(bucket) => {
                    setSelectedBucket(bucket);
                    setSelectedRange(null);
                    setSelectedCluster(null);
                    setExpandedBranchID(null);
                    setSelectedInsightID(null);
                    openDetailOverlay();
                  }}
                  onSelectRange={(start, end, firstBucket) => {
                    setSelectedRange({ start, end });
                    setSelectedBucket(firstBucket);
                    setSelectedCluster(null);
                    setExpandedBranchID(null);
                    setSelectedInsightID(null);
                    openDetailOverlay();
                  }}
                  onSelectBranch={selectBranchForDetail}
                  onSelectInsight={(item) => {
                    setSelectedInsightID(item.id);
                    setExpandedBranchID(null);
                    setSelectedRange({ start: item.start_time, end: item.end_time });
                    setSelectedCluster(null);
                    openDetailOverlay();
                    const bucket = timeline.find((candidate) => candidate.id === item.scope_id) ?? timeline.find((candidate) => rangesOverlap(candidate.start_time, candidate.end_time, item.start_time, item.end_time));
                    if (bucket) {
                      setSelectedBucket(bucket);
                    }
                  }}
                />
                {timelineDetailSelection && detailPanelOpen && (
                  <div className="timelineStageInspector">
                    <TimelineDetailPanel
                      selection={timelineDetailSelection}
                      runningBranchID={runningBranchID}
                      onRunBranch={(branch) => void runBranch(branch)}
                      onPrioritize={(item) => void prioritizeSelectedWorkItem(item)}
                      onEvidence={(ids) => void loadEvidenceMessages(ids)}
                      onExpandBranch={(branch) => void openBranchFullscreen(branch)}
                      onClose={() => {
                        clearTimelineSelection();
                      }}
                    />
                  </div>
                )}
              </div>
            )}
            {fullscreenUtilityPanel && (
              <div className="timelineFullscreenPanel">
                <div className="timelineUtilityHeader">
                  <div>
                    <span>全屏</span>
                    <strong>{fullscreenUtilityTitle}</strong>
                    <p>{activeScopeWindow}</p>
                  </div>
                  <button className="secondary iconOnly" onClick={() => setFullscreenUtilityPanel(null)} title="关闭全屏面板">
                    <X size={16} />
                  </button>
                </div>
                <div className="timelineUtilityBody fullBody">{utilityPanel}</div>
              </div>
            )}
            {fullscreenBranch && (
              <div className="timelineFullscreenPanel branchFullscreenPanel">
                <div className="timelineUtilityHeader">
                  <div>
                    <span>Branch 全文</span>
                    <strong>{fullscreenBranch.title || fullscreenBranch.topic_hint || fullscreenBranch.id}</strong>
                    <p>{formatDate(fullscreenBranch.start_time)} - {formatDate(fullscreenBranch.end_time)} · {fullscreenBranch.total_tokens} tokens</p>
                  </div>
                  <button className="secondary iconOnly" onClick={() => setFullscreenBranch(null)} title="关闭 Branch 全文">
                    <X size={16} />
                  </button>
                </div>
                <article className="timelineUtilityBody fullBody markdownBody branchFullscreenBody">
                  {loadingFullscreenBranch ? (
                    <div className="empty">
                      <Loader2 className="spin" size={16} /> 正在加载 Branch 全文
                    </div>
                  ) : fullscreenBranch.report_markdown ? (
                    <Suspense fallback={<div className="empty">正在渲染结果</div>}>
                      <MarkdownRenderer>{fullscreenBranch.report_markdown}</MarkdownRenderer>
                    </Suspense>
                  ) : (
                    <pre className="branchPlainReport">{fullscreenBranch.error || '暂无报告内容。请先运行 Branch 分析，或等待运行完成后重新打开。'}</pre>
                  )}
                </article>
              </div>
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
        <WordCloudCanvas terms={terms} limit={28} />
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

function InsightWorkflowPanel({
  scopeWindow,
  childGranularity,
  coverage,
  summaryCount,
  mergeCount,
  wordCloudCount,
  wordCloudItems,
  seedingSummaries,
  creatingMergeSummary,
  seedingWorkItems,
  onSeedSummaries,
  onMerge,
  onSeedWordClouds,
}: {
  scopeWindow: string;
  childGranularity: string;
  coverage: ReturnType<typeof summaryCoverageStats>;
  summaryCount: number;
  mergeCount: number;
  wordCloudCount: number;
  wordCloudItems: AnalysisWorkItem[];
  seedingSummaries: boolean;
  creatingMergeSummary: boolean;
  seedingWorkItems: boolean;
  onSeedSummaries: () => void;
  onMerge: () => void;
  onSeedWordClouds: () => void;
}) {
  const terms = mergeWordCloudTerms(wordCloudItems);
  const wordCloudStatus = wordCloudItems.some((item) => item.status === 'running')
    ? 'running'
    : wordCloudItems.some((item) => item.status === 'queued')
      ? 'queued'
      : wordCloudItems.some((item) => item.status === 'failed')
        ? 'failed'
        : terms.length > 0
          ? 'completed'
          : 'unseen';
  return (
    <div className="insightWorkflow">
      <div className="wordCloudHeader">
        <strong>当前节点的子周期洞察</strong>
        <span className={`workStatus ${coverage.ready ? 'completed' : coverage.completed > 0 ? 'running' : 'unseen'}`}>
          {coverage.completed}/{coverage.expected}
        </span>
      </div>
      <p>{scopeWindow} · 子周期粒度：{childGranularity}</p>
      <div className="inlineWordCloud">
        <div className="wordCloudHeader">
          <strong>词云结果</strong>
          <span className={`workStatus ${wordCloudStatus}`}>{statusText(wordCloudStatus)}</span>
        </div>
        {terms.length > 0 ? (
          <WordCloudCanvas terms={terms} limit={18} compact />
        ) : (
          <p>{wordCloudStatus === 'running' ? '正在生成词云。' : wordCloudStatus === 'queued' ? '词云任务已排队。' : '还没有词云结果。点击“生成词云”。'}</p>
        )}
      </div>
      <div className="workOverviewGrid">
        <div>
          <span>桶摘要</span>
          <strong>{summaryCount}</strong>
        </div>
        <div>
          <span>聚合摘要</span>
          <strong>{mergeCount}</strong>
        </div>
        <div>
          <span>词云任务</span>
          <strong>{wordCloudCount}</strong>
        </div>
        <div>
          <span>可聚合</span>
          <strong>{coverage.ready ? '是' : '否'}</strong>
        </div>
      </div>
      <div className="insightWorkflowActions">
        <button className="secondary" disabled={seedingSummaries || coverage.expected === 0} onClick={onSeedSummaries}>
          {seedingSummaries ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} {coverage.missing > 0 ? `补齐桶摘要 (${coverage.missing})` : '刷新桶摘要'}
        </button>
        <button className="primary" disabled={creatingMergeSummary || !coverage.ready} onClick={onMerge}>
          {creatingMergeSummary ? <Loader2 className="spin" size={16} /> : <FileText size={16} />} 聚合当前节点
        </button>
        <button className="secondary" disabled={seedingWorkItems || coverage.expected === 0} onClick={onSeedWordClouds}>
          {seedingWorkItems ? <Loader2 className="spin" size={16} /> : <Search size={16} />} 生成词云
        </button>
      </div>
    </div>
  );
}

function TopicSummaryPanel({ items, coverage, onPrioritize }: { items: AnalysisWorkItem[]; coverage: ReturnType<typeof summaryCoverageStats>; onPrioritize: (item: AnalysisWorkItem) => void }) {
  const [expanded, setExpanded] = useState(false);
  if (items.length === 0) {
    return <div className="wordCloudPanel mutedPanel">当前节点还没有子周期摘要。先补齐子周期摘要，再聚合当前节点。</div>;
  }
  const completed = items.filter((item) => item.status === 'completed');
  const runningCount = items.filter((item) => item.status === 'running').length;
  const queuedCount = items.filter((item) => item.status === 'queued').length;
  const failed = items.find((item) => item.status === 'failed');
  const prioritizable = items.find((item) => item.status === 'queued' || item.status === 'failed') ?? null;
  const status = runningCount > 0 ? 'running' : queuedCount > 0 ? 'queued' : failed ? 'failed' : 'completed';
  const visibleCompleted = expanded ? completed : completed.slice(0, 3);
  return (
    <div className="summaryPanel">
      <div className="wordCloudHeader">
        <strong>子周期摘要</strong>
        <span className={`workStatus ${status}`}>
          {coverage.completed}/{coverage.expected} · {status}
        </span>
      </div>
      {completed.length > 0 ? (
        <div className="summaryList">
          {visibleCompleted.map((item) => {
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
          {completed.length > 3 && (
            <button className="secondary summaryRailToggle" onClick={() => setExpanded((value) => !value)}>
              {expanded ? '折叠摘要列表' : `展开全部 ${completed.length} 个摘要`}
            </button>
          )}
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

function MergeSummaryPanel({ items, coverage, onPrioritize }: { items: AnalysisWorkItem[]; coverage?: ReturnType<typeof summaryCoverageStats>; onPrioritize: (item: AnalysisWorkItem) => void }) {
  if (items.length === 0) {
    return <div className="wordCloudPanel mutedPanel">{coverage?.ready ? '子周期摘要已齐，可以聚合当前节点。' : '还没有当前节点聚合摘要。先补齐这个节点里的所有子周期摘要。'}</div>;
  }
  const item = [...items].sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at))[0];
  const summary = topicSummaryResult(item);
  return (
    <div className="summaryPanel mergeSummaryPanel">
      <div className="wordCloudHeader">
        <strong>当前节点聚合摘要</strong>
        <span className={`workStatus ${item.status}`}>{statusText(item.status)}</span>
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

function WorkItemOverview({
  title,
  items,
  expectedSummaries = 0,
  onPrioritize,
}: {
  title: string;
  items: AnalysisWorkItem[];
  expectedSummaries?: number;
  onPrioritize: (item: AnalysisWorkItem) => void;
}) {
  const stats = workItemStats(items);
  const coverage = summaryCoverageStats(expectedSummaries, items.filter((item) => item.kind === 'topic_summary'));
  const active = [...items]
    .filter((item) => item.status === 'running' || item.status === 'failed')
    .sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at))[0];
  const prioritizable = items.find((item) => item.status === 'queued' || item.status === 'failed') ?? null;
  if (items.length === 0) {
    return (
      <div className="workOverview mutedPanel">
        <div className="wordCloudHeader">
          <strong>{title}</strong>
          <span className="workStatus unseen">未创建</span>
        </div>
        <p>当前片段还没有摘要任务。点击“生成片段摘要”后只会处理当前选中范围。</p>
      </div>
    );
  }
  return (
    <div className="workOverview">
      <div className="wordCloudHeader">
        <strong>{title}</strong>
        <span className={`workStatus ${stats.primaryStatus}`}>{statusText(stats.primaryStatus)}</span>
      </div>
      <div className="workOverviewGrid">
        <div>
          <span>总任务</span>
          <strong>{items.length}</strong>
        </div>
        <div>
          <span>已完成</span>
          <strong>{stats.completed}</strong>
        </div>
        <div>
          <span>排队/运行</span>
          <strong>{stats.queued + stats.running}</strong>
        </div>
        <div>
          <span>Tokens</span>
          <strong>{stats.totalTokens}</strong>
        </div>
        <div>
          <span>摘要覆盖</span>
          <strong>{coverage.completed}/{coverage.expected}</strong>
        </div>
      </div>
      {!coverage.ready && coverage.expected > 0 && (
        <p className="workOverviewWarning">完整周期摘要还缺 {coverage.missing} 个 bucket 摘要，先生成缺失摘要。</p>
      )}
      <div className="statusChips">
        {(['completed', 'running', 'queued', 'failed'] as const).map((status) => (
          <span className={`workStatus ${status}`} key={status}>
            {statusText(status)} {stats[status]}
          </span>
        ))}
      </div>
      {active && (
        <p className="workOverviewActive">
          {statusText(active.status)} · {workItemKindText(active.kind)} · {formatDate(active.start_time)} - {formatDate(active.end_time)}
          {active.error ? ` · ${active.error}` : ''}
        </p>
      )}
      {prioritizable && (
        <button className="secondary" onClick={() => onPrioritize(prioritizable)}>
          插队下一条
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

function InteractionFlow() {
  return (
    <div className="interactionFlow">
      <div>
        <strong>1</strong>
        <span>选择时间</span>
      </div>
      <div>
        <strong>2</strong>
        <span>插队分析</span>
      </div>
      <div>
        <strong>3</strong>
        <span>阅读证据</span>
      </div>
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

function formatBucketWindow(bucket: TimelineBucket) {
  return `${formatDate(bucket.start_time)} - ${formatDate(bucket.end_time)}`;
}

function rangesOverlap(startA: string, endA: string, startB: string, endB: string) {
  return Date.parse(startA) < Date.parse(endB) && Date.parse(startB) < Date.parse(endA);
}

function rangeContains(containerStart: string, containerEnd: string, itemStart: string, itemEnd: string) {
  return Date.parse(itemStart) >= Date.parse(containerStart) && Date.parse(itemEnd) <= Date.parse(containerEnd);
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
      options = { year: 'numeric', month: '2-digit', day: '2-digit' };
      break;
    default:
      options = { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' };
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
    case 'unseen':
      return '#98a7aa';
    default:
      return '#98a7aa';
  }
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

function workItemStats(items: AnalysisWorkItem[]) {
  const stats = {
    completed: 0,
    running: 0,
    queued: 0,
    failed: 0,
    totalTokens: 0,
    primaryStatus: 'unseen',
  };
  for (const item of items) {
    if (item.status === 'completed') stats.completed++;
    if (item.status === 'running') stats.running++;
    if (item.status === 'queued') stats.queued++;
    if (item.status === 'failed') stats.failed++;
    stats.totalTokens += item.total_tokens || 0;
  }
  stats.primaryStatus = stats.running > 0 ? 'running' : stats.queued > 0 ? 'queued' : stats.failed > 0 ? 'failed' : stats.completed > 0 ? 'completed' : 'unseen';
  return stats;
}

function summaryCoverageStats(expected: number, items: AnalysisWorkItem[]) {
  const uniqueCompletedScopes = new Set<string>();
  for (const item of items) {
    if (item.status === 'completed') {
      uniqueCompletedScopes.add(item.scope_id);
    }
  }
  const completed = uniqueCompletedScopes.size;
  const missing = Math.max(0, expected - completed);
  return {
    expected,
    completed,
    missing,
    ready: expected > 0 && completed >= expected,
  };
}

function bucketAnalysisStats(buckets: TimelineBucket[]) {
  const stats = { completed: 0, running: 0, queued: 0, failed: 0, unseen: 0 };
  for (const bucket of buckets) {
    const status = bucket.analysis_status || 'unseen';
    if (status === 'completed') stats.completed++;
    else if (status === 'running') stats.running++;
    else if (status === 'queued') stats.queued++;
    else if (status === 'failed') stats.failed++;
    else stats.unseen++;
  }
  return stats;
}

function timelineDataSignature(granularity: TimelineGranularity, buckets: TimelineBucket[], clusters: TimelineCluster[]) {
  return JSON.stringify({
    granularity,
    buckets: buckets.map((bucket) => [
      bucket.id,
      bucket.message_count,
      bucket.analysis_status || '',
      bucket.summary_status || '',
      bucket.word_cloud_status || '',
      bucket.summary_title || '',
      bucket.total_tokens || 0,
    ]),
    clusters: clusters.map((cluster) => [cluster.id, cluster.message_count, cluster.bucket_count, cluster.status, cluster.topic_hint]),
  });
}

function BucketStatusStrip({ bucket }: { bucket: TimelineBucket }) {
  const entries = [
    ['摘要', bucket.summary_status || 'unseen'],
    ['词云', bucket.word_cloud_status || 'unseen'],
    ['整体', bucket.analysis_status || 'unseen'],
  ];
  return (
    <div className="bucketStatusStrip">
      {entries.map(([label, status]) => (
        <span className={`workStatus ${status}`} key={label}>
          {label}: {statusText(status)}
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

function workItemStatusSignature(...groups: AnalysisWorkItem[][]) {
  return groups
    .flat()
    .map((item) => `${item.id}:${item.status}:${item.progress}:${item.updated_at}`)
    .sort()
    .join('|');
}

function historyJobsStatusSignature(jobs: AnalysisJob[]) {
  return jobs
    .map((job) => `${job.id}:${job.status}:${job.progress}:${job.updated_at}`)
    .sort()
    .join('|');
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

function WordCloudCanvas({
  terms,
  limit = 24,
  compact = false,
}: {
  terms: Array<{ term: string; count: number }>;
  limit?: number;
  compact?: boolean;
}) {
  const visibleTerms = terms.slice(0, limit);
  const maxCount = Math.max(...visibleTerms.map((term) => term.count), 1);
  const minCount = Math.min(...visibleTerms.map((term) => term.count), maxCount);
  return (
    <div className={`wordCloudCanvas${compact ? ' compact' : ''}`} aria-label="词云结果">
      {visibleTerms.map((term, index) => {
        const weight = maxCount === minCount ? 0.65 : (term.count - minCount) / (maxCount - minCount);
        const angle = (index * 137.5 * Math.PI) / 180;
        const radius = Math.sqrt((index + 1) / Math.max(visibleTerms.length, 1));
        const x = 50 + Math.cos(angle) * radius * (compact ? 36 : 41);
        const y = 50 + Math.sin(angle) * radius * (compact ? 30 : 34);
        const fontSize = compact ? 0.72 + weight * 0.68 : 0.78 + weight * 0.92;
        const opacity = 0.58 + weight * 0.38;
        return (
          <span
            key={term.term}
            className="wordCloudBubble"
            style={
              {
                left: `${clamp(x, 10, 90)}%`,
                top: `${clamp(y, 12, 88)}%`,
                fontSize: `${fontSize}rem`,
                opacity,
                '--word-weight': weight,
              } as CSSProperties
            }
            title={`${term.term} · ${term.count}`}
          >
            {term.term}
          </span>
        );
      })}
    </div>
  );
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
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

function granularityText(value: TimelineGranularity) {
  switch (value) {
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
  const [rawPath, rawQuery = ''] = raw.split('?');
  const path = rawPath.startsWith('/') ? rawPath : `/${rawPath}`;
  const params = new URLSearchParams(rawQuery);
  if (path === '/history') return { view: 'history' };
  if (path === '/account') return { view: 'account' };
  if (path.startsWith('/job/')) {
    const jobID = decodeURIComponent(path.slice('/job/'.length));
    const range = parseHashRange(params.get('range'));
    const entity = params.get('entity') || undefined;
    return jobID ? { view: 'job', jobID, range, entity } : { view: 'history' };
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
      if (!route.jobID) return '#/history';
      return `#/job/${encodeURIComponent(route.jobID)}${jobHashQuery(route)}`;
    case 'report':
      return route.reportID ? `#/report/${encodeURIComponent(route.reportID)}` : '#/history';
    default:
      return '#/analysis';
  }
}

function jobHashQuery(route: RouteState) {
  const params = new URLSearchParams();
  if (route.range) {
    params.set('range', `${route.range.start},${route.range.end}`);
  }
  if (route.entity) {
    params.set('entity', route.entity);
  }
  const query = params.toString();
  return query ? `?${query}` : '';
}

function parseHashRange(value: string | null): TimelineRange | undefined {
  if (!value) return undefined;
  const [start, end] = value.split(',');
  if (!start || !end) return undefined;
  return { start, end };
}
