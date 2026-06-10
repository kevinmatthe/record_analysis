# Timeline Module

This module owns the boundary between job data and the visual timeline scene.

- `types.ts` defines renderer-neutral scene node types.
- `buildTimelineScene.ts` converts buckets, work items, and branches into scene nodes.

Keep business state and API calls in `App.tsx`. Keep renderer-specific options in renderer components. The scene builder should stay pure so ECharts can later be replaced by Canvas, SVG, or Three.js without changing job-detail behavior.
