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
    const bucketNodes = scene.buckets;
    const chartBuckets = bucketNodes.map((node) => node.bucket);
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
    const peakMessages = Math.max(bucketPeak, ...chartBuckets.map((bucket) => bucket.message_count), 1);
    const smoothBandData = buildSmoothBandData(chartBuckets, peakMessages, selectedBucketID, selectedRange);
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
      grid: { left: 24, right: 18, top: 72, bottom: 58 },
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
          if (params.seriesName === 'bucket-band') {
            const bucketID = params.data?.[4];
            const bucket = chartBuckets.find((item) => item.id === bucketID);
            if (!bucket) return '';
            return bucketTooltip(bucket);
          }
          const index = params.dataIndex as number;
          const bucket = chartBuckets[index];
          if (!bucket) return '';
          return bucketTooltip(bucket);
        },
      },
      brush: {
        xAxisIndex: 'all',
        brushMode: 'single',
        throttleType: 'debounce',
        throttleDelay: 180,
        brushStyle: {
          color: 'rgba(19, 93, 102, 0.035)',
          borderColor: '#135d66',
          borderWidth: 2,
          borderType: 'dashed',
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
          showMinLabel: true,
          showMaxLabel: true,
          margin: 14,
          formatter: (value: number) => formatTimelineAxisLabel(value, granularity),
        },
        axisPointer: {
          show: true,
          label: {
            formatter: (params: any) => formatTimelineAxisLabel(Number(params.value), granularity),
          },
        },
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value',
        show: false,
        min: -0.9,
        max: 0.9,
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
          lineStyle: { color: 'rgba(13, 64, 71, 0.28)', width: 4, shadowBlur: 12, shadowColor: 'rgba(19, 93, 102, 0.22)' },
          silent: true,
        },
        {
          name: 'bucket-band',
          type: 'custom',
          data: smoothBandData,
          renderItem: (params: any, api: any) => {
            const status = api.value(6);
            const bucketID = api.value(4);
            const selected = bucketID === selectedBucketID || Boolean(api.value(15));
            const rangeSelected = Boolean(api.value(15));
            const points = taperedSegmentPoints(api, selected);
            return {
              type: 'path',
              shape: { pathData: taperedSegmentPath(points) },
              style: {
                fill: bucketStatusColor(status, selected),
                opacity: 1,
                shadowBlur: rangeSelected ? 28 : selected ? 22 : 14,
                shadowColor: rangeSelected ? 'rgba(13, 64, 71, 0.34)' : 'rgba(13, 64, 71, 0.24)',
              },
            };
          },
          encode: { x: [0, 2], y: 3 },
          z: 3,
          silent: false,
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
          symbol: 'roundRect',
          symbolSize: (value: unknown[]) => {
            const label = String(value[2] ?? '');
            return [Math.min(116, Math.max(48, label.length * 11 + 22)), 24];
          },
          itemStyle: {
            color: (params: any) => branchStatusColor(params.data?.[7], params.data?.[3] === selectedBranchID),
            borderColor: (params: any) => (params.data?.[3] === selectedBranchID ? '#0d4047' : '#ffffff'),
            borderWidth: (params: any) => (params.data?.[3] === selectedBranchID ? 3 : 2),
            shadowBlur: (params: any) => (params.data?.[3] === selectedBranchID ? 22 : 14),
            shadowColor: 'rgba(23, 32, 42, 0.2)',
          },
          label: {
            show: true,
            position: 'inside',
            formatter: (params: any) => params.data?.[2] ?? '',
            color: '#ffffff',
            fontSize: 11,
            lineHeight: 15,
            padding: [0, 6],
            overflow: 'truncate',
            width: 96,
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
          symbolSize: 34,
          itemStyle: { opacity: 0 },
          label: {
            show: true,
            position: 'top',
            formatter: (params: any) => {
              const bucketID = params.data?.[3];
              const bucketNode = bucketNodes.find((item) => item.bucket.id === bucketID);
              return bucketNode?.label ?? '';
            },
            color: '#26383b',
            fontSize: 11,
            fontWeight: 700,
            lineHeight: 14,
            backgroundColor: 'rgba(255, 255, 255, 0.78)',
            borderColor: 'rgba(13, 64, 71, 0.14)',
            borderWidth: 1,
            borderRadius: 6,
            padding: [5, 7],
            width: 86,
            overflow: 'break',
          },
          z: 5,
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
      if (params.seriesName === 'bucket-segments' || params.seriesName === 'bucket-band') {
        const bucketID = params.data?.[4] ?? params.data?.[3];
        const bucket = chartBuckets.find((item) => item.id === bucketID);
        if (bucket) onSelectBucket(bucket);
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

function bucketCenterTime(bucket: TimelineBucket) {
  const start = new Date(bucket.start_time).getTime();
  const end = new Date(bucket.end_time).getTime();
  if (Number.isFinite(start) && Number.isFinite(end)) {
    return start + (end - start) / 2;
  }
  return start;
}

function bucketThickness(count: number, peak: number, selected: boolean) {
  const safePeak = Math.max(peak, 1);
  const ratio = Math.max(0, Math.min(1, count / safePeak));
  return 8 + ratio * 24 + (selected ? 5 : 0);
}

function buildSmoothBandData(
  buckets: TimelineBucket[],
  peak: number,
  selectedBucketID: string,
  selectedRange: { start: string; end: string } | null,
) {
  const centers = buckets.map((bucket) => bucketCenterTime(bucket));
  return buckets.map((bucket, index) => {
    const selected = bucket.id === selectedBucketID || bucketInRange(bucket, selectedRange);
    const thickness = bucketThickness(bucket.message_count, peak, selected);
    const previousBucket = buckets[index - 1];
    const nextBucket = buckets[index + 1];
    const previousSelected = previousBucket ? previousBucket.id === selectedBucketID || bucketInRange(previousBucket, selectedRange) : selected;
    const nextSelected = nextBucket ? nextBucket.id === selectedBucketID || bucketInRange(nextBucket, selectedRange) : selected;
    const previousThickness = previousBucket ? bucketThickness(previousBucket.message_count, peak, previousSelected) : thickness;
    const nextThickness = nextBucket ? bucketThickness(nextBucket.message_count, peak, nextSelected) : thickness;
    return [
      centers[index],
      new Date(bucket.start_time).getTime(),
      new Date(bucket.end_time).getTime(),
      thickness,
      bucket.id,
      bucket.preview,
      bucket.analysis_status ?? 'unseen',
      bucket.summary_status ?? '',
      bucket.word_cloud_status ?? '',
      previousBucket ? centers[index - 1] : 0,
      nextBucket ? centers[index + 1] : 0,
      previousThickness,
      nextThickness,
      bucket.summary_title ?? '',
      bucket.total_tokens ?? 0,
      selected,
    ];
  });
}

function taperedSegmentPoints(api: any, selected = false) {
  const center = api.coord([api.value(0), 0]);
  const start = api.coord([api.value(1), 0]);
  const end = api.coord([api.value(2), 0]);
  const previousTime = Number(api.value(9));
  const nextTime = Number(api.value(10));
  const previousCenter = previousTime ? api.coord([previousTime, 0]) : null;
  const nextCenter = nextTime ? api.coord([nextTime, 0]) : null;
  const leftX = previousCenter ? (previousCenter[0] + center[0]) / 2 : start[0];
  const rightX = nextCenter ? (nextCenter[0] + center[0]) / 2 : end[0];
  const currentHalf = Number(api.value(3)) / 2;
  const leftHalf = ((previousCenter ? Number(api.value(11)) : Number(api.value(3))) + Number(api.value(3))) / 4;
  const rightHalf = ((nextCenter ? Number(api.value(12)) : Number(api.value(3))) + Number(api.value(3))) / 4;
  const rangeSelected = Boolean(api.value(15));
  const y = center[1] + (rangeSelected ? -10 : selected ? -7 : 0);
  const leftSpan = Math.max(1, center[0] - leftX);
  const rightSpan = Math.max(1, rightX - center[0]);
  return {
    leftX,
    centerX: center[0],
    rightX,
    y,
    leftHalf,
    currentHalf,
    rightHalf,
    upperLeftCPX: leftX + leftSpan * 0.58,
    upperRightCPX: center[0] - leftSpan * 0.34,
    lowerLeftCPX: leftX + leftSpan * 0.58,
    lowerRightCPX: center[0] - leftSpan * 0.34,
    nextUpperLeftCPX: center[0] + rightSpan * 0.34,
    nextUpperRightCPX: rightX - rightSpan * 0.58,
    nextLowerLeftCPX: center[0] + rightSpan * 0.34,
    nextLowerRightCPX: rightX - rightSpan * 0.58,
  };
}

function bucketInRange(bucket: TimelineBucket, range: { start: string; end: string } | null) {
  if (!range) return false;
  return rangesOverlap(bucket.start_time, bucket.end_time, range.start, range.end);
}

function rangesOverlap(startA: string, endA: string, startB: string, endB: string) {
  const aStart = new Date(startA).getTime();
  const aEnd = new Date(endA).getTime();
  const bStart = new Date(startB).getTime();
  const bEnd = new Date(endB).getTime();
  if (![aStart, aEnd, bStart, bEnd].every(Number.isFinite)) return false;
  return aStart < bEnd && aEnd > bStart;
}

function taperedSegmentPath(points: ReturnType<typeof taperedSegmentPoints>) {
  const {
    leftX,
    centerX,
    rightX,
    y,
    leftHalf,
    currentHalf,
    rightHalf,
    upperLeftCPX,
    upperRightCPX,
    lowerLeftCPX,
    lowerRightCPX,
    nextUpperLeftCPX,
    nextUpperRightCPX,
    nextLowerLeftCPX,
    nextLowerRightCPX,
  } = points;
  return [
    `M ${leftX} ${y - leftHalf}`,
    `C ${upperLeftCPX} ${y - leftHalf}, ${upperRightCPX} ${y - currentHalf}, ${centerX} ${y - currentHalf}`,
    `C ${nextUpperLeftCPX} ${y - currentHalf}, ${nextUpperRightCPX} ${y - rightHalf}, ${rightX} ${y - rightHalf}`,
    `L ${rightX} ${y + rightHalf}`,
    `C ${nextLowerRightCPX} ${y + rightHalf}, ${nextLowerLeftCPX} ${y + currentHalf}, ${centerX} ${y + currentHalf}`,
    `C ${lowerRightCPX} ${y + currentHalf}, ${lowerLeftCPX} ${y + leftHalf}, ${leftX} ${y + leftHalf}`,
    'Z',
  ].join(' ');
}

function bucketTooltip(bucket: TimelineBucket) {
  const title = bucket.summary_title || bucket.preview;
  const preview = title ? `<br/><span style="color:#5f6f72">${escapeHtml(title.slice(0, 80))}</span>` : '';
  const summaryStatus = bucket.summary_status ? `<br/>摘要：${bucket.summary_status}` : '';
  const wordCloudStatus = bucket.word_cloud_status ? ` · 词云：${bucket.word_cloud_status}` : '';
  const tokens = bucket.total_tokens ? `<br/>${bucket.total_tokens} tokens` : '';
  return `<strong>${formatBucketWindow(bucket)}</strong><br/>${bucket.message_count} 条消息 · ${bucket.participant_count} 人${summaryStatus}${wordCloudStatus}${tokens}${preview}`;
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
          ? { year: '2-digit', month: '2-digit', day: '2-digit' }
          : { year: '2-digit', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' };
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
