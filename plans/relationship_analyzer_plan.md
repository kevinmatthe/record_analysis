# 基于大模型的持续感情关系分析工具方案

## 1. 项目目标

构建一套用于**持续分析两个人感情关系变化**的工具。

核心目标不是一次性总结聊天记录，而是：

1. 持续导入新的聊天记录；
2. 将聊天内容转化为结构化行为证据；
3. 基于行为证据抽取关系事件；
4. 基于事件和指标形成心理/关系维度；
5. 持续追踪关系状态变化；
6. 生成带证据链的周期性关系分析报告。

系统应避免直接输出人格诊断或绝对判断，例如：

```text
她不爱你了
他是回避型人格
这是冷暴力
你们不合适
```

更合适的输出是：

```text
本周期中，A 的确认需求信号增加，B 在冲突中的撤退/防御信号增加。聊天记录呈现出“期待未被确认 → 追问 → 防御 → 撤退”的互动循环。
```

---

## 2. 核心原则

### 2.1 分层分析

系统采用三层指标体系：

```text
L1 行为观测指标：客观、可统计、可证据化
L2 心理/关系维度：基于行为证据进行解释
L3 叙事报告：由模型基于 L1/L2 生成总结
```

不要让模型直接从原始聊天记录跳到最终结论。

正确流程：

```text
聊天消息
  → 消息清洗
  → 会话切片
  → 行为动作识别
  → 关系事件抽取
  → 指标统计
  → 心理/关系维度更新
  → 周期报告生成
```

### 2.2 证据链优先

每个重要结论必须可追溯到：

```text
报告结论
  → 心理/关系维度
  → 关系事件
  → 消息 ID
  → 原始聊天内容
```

所有模型输出中的核心判断都必须包含：

```json
{
  "claim": "...",
  "evidence_event_ids": ["EVT_001", "EVT_002"],
  "evidence_msg_ids": ["MSG_001", "MSG_002"],
  "confidence": 0.82,
  "uncertainty": "..."
}
```

### 2.3 行为化表达，不做强诊断

允许使用：

```text
焦虑激活信号
回避激活信号
回应性下降
情绪验证不足
修复尝试减少
撤退信号增加
冲突升级风险上升
```

避免使用：

```text
焦虑型人格
回避型人格
PUA
冷暴力
人格障碍
不爱了
精神控制
```

---

## 3. 总体架构

```text
聊天记录导入
   ↓
清洗与标准化
   ↓
消息去重与增量合并
   ↓
会话切片 segment
   ↓
消息级行为动作识别
   ↓
片段级关系事件抽取
   ↓
事件聚类与模式识别
   ↓
行为指标统计
   ↓
心理/关系维度计算
   ↓
关系状态快照
   ↓
周期报告生成
```

建议实体层级：

```text
message
  ↓
segment
  ↓
action
  ↓
event
  ↓
pattern
  ↓
relationship_state_snapshot
  ↓
period_report
```

---

## 4. 数据模型设计

### 4.1 messages：原始消息表

```sql
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  chat_id TEXT,
  sender TEXT NOT NULL,
  receiver TEXT,
  msg_time TIMESTAMP NOT NULL,
  msg_type TEXT,
  content TEXT,
  raw_content TEXT,
  source TEXT,
  content_hash TEXT,
  created_at TIMESTAMP DEFAULT now()
);
```

说明：

- `id` 建议使用稳定消息 ID，例如 `MSG_000001`；
- `content_hash` 用于去重；
- `sender` 建议归一化为 `PERSON_A` / `PERSON_B`。

---

### 4.2 segments：会话片段表

```sql
CREATE TABLE segments (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  start_time TIMESTAMP NOT NULL,
  end_time TIMESTAMP NOT NULL,
  message_ids JSONB NOT NULL,
  topic TEXT,
  summary TEXT,
  segment_type TEXT,
  created_at TIMESTAMP DEFAULT now()
);
```

切片策略：

1. 时间间隔超过阈值切段，例如 30 分钟；
2. 跨天切段；
3. 话题明显变化切段；
4. 冲突开始/结束信号切段。

---

### 4.3 message_actions：消息级行为动作表

```sql
CREATE TABLE message_actions (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  msg_id TEXT NOT NULL,
  segment_id TEXT,
  sender TEXT NOT NULL,

  action_type TEXT NOT NULL,
  emotion TEXT,
  intent TEXT,
  target TEXT,

  evidence_text TEXT,
  confidence DOUBLE PRECISION,
  created_at TIMESTAMP DEFAULT now()
);
```

`action_type` 推荐枚举：

```text
express_need
express_dissatisfaction
seek_reassurance
ask_question
answer_question
explain
apologize
validate_emotion
comfort
repair_attempt
criticize
defend
withdraw
stonewall
change_topic
affection
self_disclosure
make_plan
reject
accept
neutral_chat
```

---

### 4.4 relationship_events：关系事件表

```sql
CREATE TABLE relationship_events (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  segment_id TEXT,

  event_type TEXT NOT NULL,
  topic TEXT,
  trigger TEXT,
  process JSONB,
  result TEXT,

  participants JSONB,
  start_time TIMESTAMP,
  end_time TIMESTAMP,

  repair_status TEXT,
  repair_initiator TEXT,
  repair_delay_minutes INT,

  evidence_msg_ids JSONB,
  evidence_action_ids JSONB,

  confidence DOUBLE PRECISION,
  created_at TIMESTAMP DEFAULT now()
);
```

`event_type` 推荐枚举：

```text
intimacy
conflict
repair
withdrawal
plan_negotiation
emotional_support
misunderstanding
daily_chat
relationship_confirmation
boundary_setting
```

---

### 4.5 relationship_patterns：长期模式表

```sql
CREATE TABLE relationship_patterns (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,

  pattern_type TEXT NOT NULL,
  pattern_name TEXT NOT NULL,
  description TEXT,

  first_seen TIMESTAMP,
  last_seen TIMESTAMP,
  occurrence_count INT,

  related_event_ids JSONB,
  status TEXT,
  confidence DOUBLE PRECISION,

  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);
```

示例：

```json
{
  "pattern_name": "期待未被确认后的追问—防御—撤退循环",
  "description": "当一方期待回应但未得到明确反馈时，常出现追问；另一方多以解释或防御回应，随后对话中断。",
  "first_seen": "2026-04-03",
  "last_seen": "2026-06-01",
  "occurrence_count": 8,
  "status": "active"
}
```

---

### 4.6 behavior_metrics：行为指标表

```sql
CREATE TABLE behavior_metrics (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,

  metrics JSONB NOT NULL,
  created_at TIMESTAMP DEFAULT now()
);
```

推荐 L1 行为指标：

```json
{
  "message_volume": 320,
  "person_a_message_ratio": 0.58,
  "person_b_message_ratio": 0.42,
  "initiation_rate": {
    "PERSON_A": 0.61,
    "PERSON_B": 0.39
  },
  "avg_reply_latency_minutes": {
    "PERSON_A": 12.4,
    "PERSON_B": 34.8
  },
  "long_silence_count": 4,
  "question_response_rate": 0.76,
  "affection_expression_count": 12,
  "vulnerability_expression_count": 5,
  "conflict_count": 3,
  "repair_attempt_count": 4,
  "repair_success_rate": 0.67,
  "withdrawal_count": 6,
  "long_silence_after_conflict": 2
}
```

---

### 4.7 psychological_dimensions：心理/关系维度表

```sql
CREATE TABLE psychological_dimensions (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,
  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,

  dimensions JSONB NOT NULL,
  created_at TIMESTAMP DEFAULT now()
);
```

推荐 L2 维度：

```json
{
  "responsiveness": {
    "score": 0.62,
    "trend": "down",
    "evidence_event_ids": ["EVT_012", "EVT_015"],
    "confidence": 0.78,
    "notes": "情绪表达后回应减少，解释多于安抚。"
  },
  "emotional_validation": {
    "score": 0.48,
    "trend": "down",
    "evidence_event_ids": ["EVT_017"],
    "confidence": 0.74
  },
  "attachment_anxiety_activation": {
    "score": 0.71,
    "trend": "up",
    "evidence_event_ids": ["EVT_012", "EVT_018"],
    "confidence": 0.81
  },
  "attachment_avoidance_activation": {
    "score": 0.66,
    "trend": "up",
    "evidence_event_ids": ["EVT_012", "EVT_019"],
    "confidence": 0.77
  },
  "conflict_escalation": {
    "score": 0.69,
    "trend": "up"
  },
  "repair_capacity": {
    "score": 0.52,
    "trend": "down"
  },
  "self_disclosure": {
    "score": 0.44,
    "trend": "down"
  },
  "affection_warmth": {
    "score": 0.57,
    "trend": "stable"
  },
  "withdrawal_risk": {
    "score": 0.68,
    "trend": "up"
  },
  "reciprocity_balance": {
    "score": 0.55,
    "trend": "stable"
  }
}
```

---

### 4.8 relationship_state_snapshots：关系状态快照

```sql
CREATE TABLE relationship_state_snapshots (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,

  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,

  overall_summary TEXT,
  key_changes JSONB,
  active_patterns JSONB,
  risk_signals JSONB,
  positive_signals JSONB,
  evidence_event_ids JSONB,

  created_at TIMESTAMP DEFAULT now()
);
```

---

### 4.9 period_reports：周期报告表

```sql
CREATE TABLE period_reports (
  id TEXT PRIMARY KEY,
  relationship_id TEXT NOT NULL,

  period_type TEXT NOT NULL,
  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,

  report_markdown TEXT NOT NULL,
  evidence_event_ids JSONB,
  model_name TEXT,

  created_at TIMESTAMP DEFAULT now()
);
```

---

## 5. 核心分析维度

### 5.1 行为观测指标 L1

这些指标应尽可能由程序统计或由 LLM 做低层标注。

```text
message_volume
initiation_rate
reply_latency
long_silence_count
question_response_rate
affection_expression_count
vulnerability_expression_count
conflict_count
repair_attempt_count
repair_success_rate
withdrawal_count
long_silence_after_conflict
topic_continuity
plan_negotiation_count
emotional_support_count
```

---

### 5.2 心理/关系维度 L2

这些维度由 L1 指标、事件和模式综合计算。

```text
responsiveness
emotional_validation
attachment_anxiety_activation
attachment_avoidance_activation
conflict_escalation
repair_capacity
self_disclosure
affection_warmth
withdrawal_risk
reciprocity_balance
```

解释：

#### responsiveness

伴侣回应性。观察情绪表达、需求表达、提问之后是否被回应。

#### emotional_validation

情绪验证。观察是否有人回应对方感受，而不只是解释事实。

#### attachment_anxiety_activation

焦虑激活信号。观察追问、关系确认、对延迟敏感、害怕不被在乎等行为。

#### attachment_avoidance_activation

回避激活信号。观察撤退、简短回应、转移话题、长时间不回应、拒绝沟通等行为。

#### conflict_escalation

冲突升级程度。观察从不满到指责、防御、撤退的速度和频率。

#### repair_capacity

修复能力。观察冲突后是否有道歉、解释、安抚、重新连接，以及修复是否有效。

#### self_disclosure

自我暴露。观察是否分享真实感受、脆弱、压力、需求和期待。

#### affection_warmth

亲密温度。观察想念、关心、撒娇、称呼、主动分享、未来计划等信号。

#### withdrawal_risk

疏离风险。观察回复变短、主动减少、分享减少、冲突后沉默、事务性沟通增加。

#### reciprocity_balance

互动互惠平衡。观察谁更常发起、解释、道歉、让步、修复、承担情绪劳动。

---

## 6. LLM 任务拆分

不要使用一个大 prompt 完成所有任务。建议拆分为以下任务。

---

### 6.1 消息级行为识别

输入：

```json
{
  "msg_id": "MSG_001",
  "context_before": [],
  "message": {
    "sender": "PERSON_A",
    "time": "2026-06-01 21:10",
    "content": "你今天怎么又不回我"
  },
  "context_after": []
}
```

输出：

```json
{
  "msg_id": "MSG_001",
  "sender": "PERSON_A",
  "action_type": "seek_reassurance",
  "emotion": "委屈/不安",
  "intent": "希望对方解释未回复原因并确认在乎",
  "target": "PERSON_B",
  "evidence_text": "你今天怎么又不回我",
  "confidence": 0.84
}
```

---

### 6.2 片段级事件抽取

输入一个 segment 及其 message_actions。

输出：

```json
{
  "event_type": "conflict",
  "topic": "回复不及时",
  "trigger": "PERSON_B 长时间未回复，PERSON_A 追问",
  "process": [
    "PERSON_A 表达不满",
    "PERSON_B 解释自己在忙",
    "PERSON_A 认为解释没有接住情绪",
    "PERSON_B 开始防御并减少回应"
  ],
  "result": "冲突未完全修复",
  "repair_status": "partial",
  "repair_initiator": "PERSON_B",
  "evidence_msg_ids": ["MSG_001", "MSG_004", "MSG_009"],
  "confidence": 0.79
}
```

---

### 6.3 长期模式识别

输入多个事件。

输出：

```json
{
  "pattern_name": "期待未被确认后的追问—防御—撤退循环",
  "pattern_type": "conflict_cycle",
  "description": "当 PERSON_A 的回应期待未被满足时，容易出现连续追问；PERSON_B 多以解释或防御回应，随后互动减少。",
  "related_event_ids": ["EVT_012", "EVT_018", "EVT_023"],
  "first_seen": "2026-04-03",
  "last_seen": "2026-06-01",
  "occurrence_count": 3,
  "confidence": 0.81
}
```

---

### 6.4 心理/关系维度生成

输入：

```json
{
  "behavior_metrics": {},
  "events": [],
  "patterns": [],
  "previous_dimensions": {}
}
```

输出：

```json
{
  "responsiveness": {
    "score": 0.62,
    "trend": "down",
    "evidence_event_ids": ["EVT_012"],
    "confidence": 0.78,
    "notes": "需求表达后有效回应减少。"
  },
  "attachment_anxiety_activation": {
    "score": 0.71,
    "trend": "up",
    "evidence_event_ids": ["EVT_012", "EVT_018"],
    "confidence": 0.81,
    "notes": "连续追问和关系确认表达增加。"
  }
}
```

---

### 6.5 周期报告生成

输入：

```json
{
  "period": "2026-05-27 ~ 2026-06-02",
  "behavior_metrics": {},
  "psychological_dimensions": {},
  "events": [],
  "patterns": [],
  "counter_evidence": []
}
```

输出 Markdown：

```md
# 本周期关系互动报告

## 1. 总体判断

本周期关系互动呈现“靠近需求增加，但回应质量下降”的特征。

## 2. 关键变化

- PERSON_A 的确认需求信号上升，主要表现为连续追问和对回复延迟敏感。
- PERSON_B 的冲突中撤退/防御信号增加，主要表现为简短回应和中断对话。
- 冲突后的修复效率下降。

## 3. 主要互动循环

本周期最明显的循环是：

期待未被确认 → 追问/表达不满 → 解释或防御 → 感到没有被理解 → 撤退 → 冲突悬而未决。

## 4. 证据

- EVT_012：回复延迟引发追问，随后出现防御和撤退。
- EVT_015：见面安排变动后，情绪升级，修复不充分。

## 5. 不确定性

以上分析只基于文字聊天，无法观察线下互动、语音语气和现实压力。因此不能判断真实动机，只能说明聊天中呈现出的互动模式。

## 6. 建议关注

下周期可以重点观察：
- 情绪表达后是否被接住；
- 冲突后是否有明确修复；
- 双方是否能从解释事实转向回应感受。
```

---

## 7. Prompt 约束

所有 LLM 任务必须使用结构化输出，推荐 JSON Schema。

统一约束：

```text
你是关系互动分析助手。
你的任务不是诊断人格，也不是判断谁对谁错。
你只能基于输入中的聊天内容、事件、指标和证据进行分析。
不要引入聊天之外的事实。
不要使用人格障碍、PUA、冷暴力、不爱了等强诊断词。
可以使用焦虑激活信号、回避激活信号、回应性下降、情绪验证不足、修复尝试不足等行为化表达。
每个主要结论必须引用 evidence_msg_ids 或 evidence_event_ids。
如果证据不足，请输出 unknown 或 low_confidence。
```

---

## 8. 增量分析流程

持续分析场景中，每次导入新聊天记录时：

```text
1. 解析新聊天记录
2. 生成稳定 msg_id
3. 根据 content_hash 去重
4. 合并到 messages
5. 取上次分析末尾前 N 条消息作为上下文
6. 对新增消息重新切 segment
7. 识别 message_actions
8. 抽取 relationship_events
9. 判断新事件是否属于已有 pattern
10. 更新 relationship_patterns
11. 统计本周期 behavior_metrics
12. 生成 psychological_dimensions
13. 写入 relationship_state_snapshots
14. 生成 period_report
```

注意：

- 不要每次全量重跑；
- 但 segment 边界处需要带历史上下文；
- 如果模型或 prompt 版本升级，应支持全量重跑；
- 每次分析需要记录 `model_name` 和 `prompt_version`。

---

## 9. 推荐目录结构

```text
relationship-analyzer/
  importer/
    wechat_parser.py
    telegram_parser.py
    generic_text_parser.py

  pipeline/
    normalize.py
    deduplicate.py
    segment.py
    extract_actions.py
    extract_events.py
    detect_patterns.py
    compute_behavior_metrics.py
    compute_psych_dimensions.py
    generate_report.py

  schemas/
    message.py
    segment.py
    action.py
    event.py
    pattern.py
    metrics.py
    dimensions.py
    report.py

  storage/
    db.py
    migrations/
    repositories/

  llm/
    client.py
    prompts/
      action_extraction.md
      event_extraction.md
      pattern_detection.md
      dimension_generation.md
      report_generation.md
    schemas/
      action.schema.json
      event.schema.json
      pattern.schema.json
      dimensions.schema.json

  cli/
    main.py

  web/
    dashboard/
```

---

## 10. 推荐 CLI

```bash
# 导入聊天记录
rel-analyzer import ./chat.txt --relationship-id rel_001 --source wechat

# 增量分析
rel-analyzer analyze --relationship-id rel_001 --since-last

# 生成周报
rel-analyzer report --relationship-id rel_001 --period weekly

# 查看事件
rel-analyzer events --relationship-id rel_001 --from 2026-05-01 --to 2026-06-01

# 查看长期模式
rel-analyzer patterns --relationship-id rel_001

# 全量重跑
rel-analyzer rebuild --relationship-id rel_001 --prompt-version v2
```

---

## 11. MVP 范围

第一版只做以下能力：

### 输入

```text
微信聊天记录 txt/json
```

### 处理

```text
消息解析
去重
按时间切片
消息级行为识别
事件抽取
行为指标统计
心理/关系维度生成
报告生成
```

### 事件类型

```text
intimacy
conflict
repair
withdrawal
daily_chat
```

### 行为指标

```text
message_volume
initiation_rate
avg_reply_latency
affection_expression_count
vulnerability_expression_count
conflict_count
repair_attempt_count
repair_success_rate
withdrawal_count
long_silence_after_conflict
```

### 心理/关系维度

```text
responsiveness
emotional_validation
attachment_anxiety_activation
attachment_avoidance_activation
conflict_escalation
repair_capacity
affection_warmth
withdrawal_risk
reciprocity_balance
```

### 输出

```text
新增重要事件
本周期关系状态变化
重复出现的互动模式
风险信号
积极信号
证据消息
Markdown 周报
```

---

## 12. 后续增强

第二阶段可以增加：

```text
关系时间线 UI
趋势图
事件聚类
话题分布
相似历史事件检索
冲突升级链路图
修复效果分析
按人分别生成行为画像
多模型评估
人工标注修正
```

第三阶段可以增加：

```text
自动同步聊天记录
本地 LLM 隐私模式
脱敏上传云模型
RAG 检索原文证据
多关系管理
报告对比
Prompt 版本评估
```

---

## 13. 最终产品定位

产品不是“恋爱判官”，而是：

> **基于聊天记录的关系互动观察与证据化分析工具。**

系统只回答：

```text
聊天中呈现了什么互动模式？
这些模式最近是增强还是减弱？
哪些事件支持这个判断？
哪些地方证据不足？
下周期值得观察什么？
```

系统不回答：

```text
谁对谁错？
对方到底爱不爱？
对方是不是某种人格？
你们要不要分手？
```

这个边界非常重要。
