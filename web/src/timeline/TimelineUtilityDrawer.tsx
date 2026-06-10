import type { ReactNode } from 'react';
import { Database, FileText, GitBranch, Layers3, Search, X } from 'lucide-react';

export type TimelineUtilityPanel = 'scope' | 'tasks' | 'messages' | 'search' | 'branches';

const panelLabels: Record<TimelineUtilityPanel, string> = {
  scope: '范围',
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
}: {
  activePanel: TimelineUtilityPanel | null;
  title: string;
  subtitle: string;
  children: ReactNode;
  onPanelChange: (panel: TimelineUtilityPanel) => void;
  onClose: () => void;
}) {
  return (
    <>
      <div className="timelineUtilityDock" aria-label="时间轴辅助抽屉">
        {(['scope', 'tasks', 'messages', 'search', 'branches'] as const).map((panel) => (
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
            <button className="secondary iconOnly" onClick={onClose} title="关闭辅助抽屉">
              <X size={16} />
            </button>
          </div>
          <div className="timelineUtilityBody">{children}</div>
        </aside>
      )}
    </>
  );
}

function panelIcon(panel: TimelineUtilityPanel) {
  switch (panel) {
    case 'scope':
      return <Layers3 size={15} />;
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
