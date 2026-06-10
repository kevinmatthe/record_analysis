import type { AnalysisBranch, AnalysisWorkItem, TimelineBucket } from '../api';

export type TimelineBucketNode = {
  bucket: TimelineBucket;
  x: number;
  y: number;
};

export type TimelineInsightNode = {
  item: AnalysisWorkItem;
  x: number;
  y: number;
  label: string;
  bucketID: string;
};

export type TimelineBranchNode = {
  branch: AnalysisBranch;
  x: number;
  y: number;
  label: string;
};

export type TimelineScene = {
  buckets: TimelineBucketNode[];
  insights: TimelineInsightNode[];
  branches: TimelineBranchNode[];
};
