import type { ReactNode } from 'react';
import { Activity, Cloud, Database, FileText, GitBranch, Layers3, Maximize2, Search, X } from 'lucide-react';

export type TimelineUtilityPanel = 'status' | 'scope' | 'insights' | 'tasks' | 'messages' | 'search' | 'branches';

const panelLabels: Record<TimelineUtilityPanel, string> = {
  status: '状态',
  scope: '范围',
  insights: '洞察',
  tasks: '任务',
  messages: '消息',
  search: '搜索',
  branches: '分支',
};

export function TimelineUtilityDrawer({
  activePanel,
  title,
  subtitle,
  children,
  onPanelChange,
  onClose,
  onExpand,
}: {
  activePanel: TimelineUtilityPanel | null;
  title: string;
  subtitle: string;
  children: ReactNode;
  onPanelChange: (panel: TimelineUtilityPanel) => void;
  onClose: () => void;
  onExpand: (panel: TimelineUtilityPanel) => void;
}) {
  return (
    <>
      <div className="timelineUtilityDock" aria-label="时间轴辅助抽屉">
        {(['status', 'scope', 'insights', 'tasks', 'messages', 'search', 'branches'] as const).map((panel) => (
          <button key={panel} className={activePanel === panel ? 'active' : ''} onClick={() => onPanelChange(panel)} title={panelLabels[panel]}>
            {panelIcon(panel)}
            <span>{panelLabels[panel]}</span>
          </button>
        ))}
      </div>
      {activePanel && (
        <aside className={`timelineUtilityDrawer ${activePanel}`}>
          <div className="timelineUtilityHeader">
            <div>
              <span>{panelLabels[activePanel]}</span>
              <strong>{title}</strong>
              <p>{subtitle}</p>
            </div>
            <div className="panelHeaderActions">
              <button className="secondary iconOnly" onClick={() => onExpand(activePanel)} title="全屏查看">
                <Maximize2 size={16} />
              </button>
              <button className="secondary iconOnly" onClick={onClose} title="关闭辅助抽屉">
                <X size={16} />
              </button>
            </div>
          </div>
          <div className="timelineUtilityBody">{children}</div>
        </aside>
      )}
    </>
  );
}

function panelIcon(panel: TimelineUtilityPanel) {
  switch (panel) {
    case 'status':
      return <Activity size={15} />;
    case 'scope':
      return <Layers3 size={15} />;
    case 'insights':
      return <Cloud size={15} />;
    case 'tasks':
      return <FileText size={15} />;
    case 'messages':
      return <Database size={15} />;
    case 'search':
      return <Search size={15} />;
    case 'branches':
      return <GitBranch size={15} />;
  }
}
