import { useEffect, useRef, useState } from 'react';
import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket, TimelineGranularity } from '../api';
import { buildTimelineScene } from './buildTimelineScene';

export function EChartsTimelineRenderer({
  buckets,
  granularity,
  selectedBucketID,
  selectedBranchID,
  selectedInsightID,
  selectedRange,
  bucketPeak,
  wordCloudItems,
  summaryItems,
  branches,
  onSelectBucket,
  onSelectRange,
  onSelectBranch,
  onSelectInsight,
}: {
  buckets: TimelineBucket[];
  granularity: TimelineGranularity;
  selectedBucketID: string;
  selectedBranchID: string;
  selectedInsightID: string;
  selectedRange: { start: string; end: string } | null;
  bucketPeak: number;
  wordCloudItems: AnalysisWorkItem[];
  summaryItems: AnalysisWorkItem[];
  branches: AnalysisBranch[];
  onSelectBucket: (bucket: TimelineBucket) => void;
  onSelectRange: (start: string, end: string, firstBucket: TimelineBucket) => void;
  onSelectBranch: (branch: AnalysisBranch) => void;
  onSelectInsight: (item: AnalysisWorkItem) => void;
}) {
  const chartRef = useRef<HTMLDivElement | null>(null);
  const [loadingRenderer, setLoadingRenderer] = useState(true);

  useEffect(() => {
    if (!chartRef.current) return;
    let disposed = false;
    let chart: any = null;
    setLoadingRenderer(true);

    void import('echarts').then((echarts) => {
      if (!chartRef.current || disposed) return;
      chart = echarts.init(chartRef.current);
      setLoadingRenderer(false);
    const scene = buildTimelineScene({ buckets, wordCloudItems, summaryItems, branches });
    const chartBuckets = scene.buckets.map((node) => node.bucket);
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
    const floatNodes = scene.insights.map((node) => [
      node.x,
      node.y,
      node.label,
      node.bucketID,
      node.item.kind,
      node.item.id,
      node.item.status,
    ]);
    const branchNodes = scene.branches.map((node) => [
      node.x,
      node.y,
      node.label,
      node.branch.id,
      'branch',
      node.branch.start_time,
      node.branch.end_time,
      node.branch.status,
    ]);

      chart.setOption({
      animation: true,
      animationDuration: 520,
      animationEasing: 'cubicOut',
      backgroundColor: 'transparent',
      grid: { left: 24, right: 18, top: 52, bottom: 58 },
      tooltip: {
        trigger: 'item',
        confine: true,
        formatter: (params: any) => {
          if (params.seriesName === 'branches') {
            const title = escapeHtml(params.data?.[2] ?? 'Branch');
            const windowText = `${formatDate(params.data?.[5])} - ${formatDate(params.data?.[6])}`;
            return `<strong>${title}</strong><br/>${windowText}<br/>${statusText(params.data?.[7])}`;
          }
          if (params.seriesName === 'insights') {
            const label = escapeHtml(params.data?.[2] ?? '');
            const time = formatTimelineAxisLabel(Number(params.data?.[0] ?? 0), granularity);
            return `<strong>${time}</strong><br/>${label}`;
          }
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
        min: -1.15,
        max: 1.15,
      },
      series: [
        {
          name: 'branch-guides',
          type: 'line',
          data: branchNodes.flatMap((node) => [
            [node[0], 0],
            [node[0], node[1]],
            [node[0], null],
          ]),
          symbol: 'none',
          lineStyle: { color: 'rgba(15, 79, 87, 0.26)', width: 1, type: 'dotted' },
          silent: true,
        },
        {
          name: 'timeline-flow',
          type: 'line',
          data: data.map((item, index) => [item[0], index % 2 === 0 ? 0.16 : -0.16]),
          symbol: 'none',
          smooth: 0.35,
          lineStyle: { color: 'rgba(19, 93, 102, 0.16)', width: 1, type: 'dashed' },
          silent: true,
        },
        {
          name: 'trunk',
          type: 'line',
          data: data.map((item) => [item[0], 0]),
          symbol: 'none',
          lineStyle: { color: '#0d4047', width: 3, shadowBlur: 12, shadowColor: 'rgba(19, 93, 102, 0.22)' },
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
          name: 'insights',
          type: 'scatter',
          data: floatNodes,
          symbolSize: 1,
          silent: false,
          itemStyle: { opacity: 0 },
          label: {
            show: true,
            formatter: (params: any) => params.data?.[2] ?? '',
            color: '#12343a',
            fontSize: 12,
            lineHeight: 18,
            backgroundColor: (params: any) => (params.data?.[5] === selectedInsightID ? '#ffffff' : 'rgba(255, 255, 255, 0.88)'),
            borderColor: (params: any) => (params.data?.[5] === selectedInsightID ? '#135d66' : 'rgba(19, 93, 102, 0.18)'),
            borderWidth: (params: any) => (params.data?.[5] === selectedInsightID ? 2 : 1),
            borderRadius: 6,
            padding: [6, 8],
            shadowBlur: (params: any) => (params.data?.[5] === selectedInsightID ? 20 : 12),
            shadowColor: 'rgba(23, 32, 42, 0.12)',
          },
          emphasis: {
            label: {
              backgroundColor: '#ffffff',
              borderColor: '#135d66',
              shadowBlur: 18,
            },
          },
        },
        {
          name: 'branches',
          type: 'scatter',
          data: branchNodes,
          symbolSize: (value: unknown[]) => (String(value[7]) === 'completed' ? 14 : 11),
          itemStyle: {
            color: (params: any) => branchStatusColor(params.data?.[7], params.data?.[3] === selectedBranchID),
            borderColor: (params: any) => (params.data?.[3] === selectedBranchID ? '#0d4047' : '#ffffff'),
            borderWidth: (params: any) => (params.data?.[3] === selectedBranchID ? 3 : 2),
            shadowBlur: (params: any) => (params.data?.[3] === selectedBranchID ? 22 : 14),
            shadowColor: 'rgba(23, 32, 42, 0.2)',
          },
          label: {
            show: true,
            position: 'bottom',
            formatter: (params: any) => params.data?.[2] ?? '',
            color: '#26383b',
            fontSize: 12,
            lineHeight: 18,
            backgroundColor: 'rgba(255, 255, 255, 0.92)',
            borderColor: 'rgba(15, 79, 87, 0.22)',
            borderWidth: 1,
            borderRadius: 6,
            padding: [5, 8],
          },
          emphasis: {
            scale: 1.15,
            label: { borderColor: '#0d4047', backgroundColor: '#ffffff' },
          },
        },
        {
          name: 'buckets',
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
      if (params.seriesName === 'insights') {
        const itemID = params.data?.[5];
        const item = [...summaryItems, ...wordCloudItems].find((candidate) => candidate.id === itemID);
        if (item) onSelectInsight(item);
        return;
      }
      if (params.seriesName === 'branches') {
        const branchID = params.data?.[3];
        const branch = branches.find((item) => item.id === branchID);
        if (branch) onSelectBranch(branch);
        return;
      }
      const bucketID = params.data?.[3];
      const bucket = chartBuckets.find((item) => item.id === bucketID);
      if (bucket) onSelectBucket(bucket);
    };
    const handleBrushSelected = (params: any) => {
      const selectedSeries = params.batch?.[0]?.selected ?? [];
      const bucketSeries = selectedSeries.find((item: any) => item.seriesName === 'buckets') ?? selectedSeries[selectedSeries.length - 1];
      const selected = bucketSeries?.dataIndex ?? [];
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

      const resizeObserver = new ResizeObserver(() => chart?.resize());
      resizeObserver.observe(chartRef.current);
      cleanup = () => {
        resizeObserver.disconnect();
        chart?.off('click', handleClick);
        chart?.off('brushSelected', handleBrushSelected);
        chart?.dispose();
      };
    });

    let cleanup = () => {
      chart?.dispose();
    };

    return () => {
      disposed = true;
      cleanup();
    };
  }, [branches, buckets, bucketPeak, granularity, onSelectBranch, onSelectBucket, onSelectInsight, onSelectRange, selectedBranchID, selectedBucketID, selectedInsightID, selectedRange, summaryItems, wordCloudItems]);

  return (
    <div className="timelineChartFrame">
      {loadingRenderer && <div className="timelineRendererLoading">正在装载时间线渲染器</div>}
      <div className="timelineChart" ref={chartRef} aria-label="聊天记录时间轴" />
    </div>
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

function branchStatusColor(status: unknown, selected = false) {
  if (selected) return '#12343a';
  switch (status) {
    case 'completed':
      return '#1f7a4f';
    case 'running':
      return '#c26a00';
    case 'failed':
      return '#b3261e';
    default:
      return '#0d4047';
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
