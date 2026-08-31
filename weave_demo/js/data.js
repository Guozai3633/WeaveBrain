/* ============================================================
   织脑原型 — 演示种子数据
   ============================================================ */
(function (global) {
  'use strict';

  // ---------- 标签映射 ----------
  var typeLabels = { idea: '闪念', question: '问题', action: '行动', reflection: '反思', reference: '资料' };
  var sourceLabels = {
    app: '语音', shortcut: '快捷入口', widget: '桌面小组件',
    import: '导入', share: '分享', text: '文字'
  };
  var sourceIcons = { app: 'i-mic', text: 'i-text', import: 'i-import', shortcut: 'i-compass', share: 'i-share', widget: 'i-note' };
  var processingLabels = { pending: '待处理', processing: '整理中', ready: '已整理', needs_input: '待补充', failed: '失败' };
  var syncLabels = {
    saved_local: '已保存', pending_sync: '待同步', syncing: '同步中',
    synced: '已同步', retryable_error: '等待重试', conflict: '冲突'
  };
  var relationLabels = { similar: '相似', supports: '支持', contradicts: '矛盾', continues: '延续', derived_from: '衍生' };

  // ---------- 记忆卡种子 ----------
  // 字段语义：original 原始(不可覆盖) / ai 派生(可重做) / user 编辑(最高优先) / processingStatus / sync
  var cards = [
    {
      id: 'mem-001',
      primaryType: 'idea',
      original: {
        text: '跑步时想到：卡片不应该只是摘要，而应该保留继续思考的入口。\n\n就像我们把"灵感"交给未来的自己，如果只有一句话摘要，未来的自己就不知道当时在想什么了。',
        kind: 'audio', capturedAt: '2026-08-27T20:10:30+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 18000, hasWaveform: true },
        transcript: { clean: '跑步时想到：卡片不应该只是摘要，而应该保留继续思考的入口。就像我们把灵感交给未来的自己，如果只有一句话摘要，未来的自己就不知道当时在想什么了。', userCorrected: null },
        context: { activity: 'running', locationName: null }
      },
      ai: {
        title: '记忆卡要保留继续思考的入口', essence: '摘要之外，卡片应承载思考的延续。',
        keyPoints: ['卡片是"写给未来自己"的入口，不只是摘要', '缺失思考路径的摘要会丢失当时的语境', '下一步应指向具体、可继续的动作'],
        openQuestion: null, nextStep: '回看原型确认交互',
        suggestedTags: ['产品', '灵感'], suggestedType: 'idea',
        status: 'ready', provenance: 'ai', revision: 2
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-003', type: 'supports', reason: '都是"语音→行动"闭环的一部分' }],
      reminder: null,
      tags: ['产品', '灵感'],
      stats: { openCount: 5, continuationCount: 2, relationCount: 1, lastOpenedAt: '2026-08-28T09:20:00+08:00', updatedAt: '2026-08-27T20:12:40+08:00' }
    },
    {
      id: 'mem-002',
      primaryType: 'question',
      original: {
        text: '怎么让一条旧想法在半年后再次产生价值？\n回响的频率、理由、打扰程度怎么定？会不会变成又一个通知中心。',
        kind: 'audio', capturedAt: '2026-08-27T11:05:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 32000, hasWaveform: true },
        transcript: { clean: '怎么让一条旧想法在半年后再次产生价值？回响的频率、理由、打扰程度怎么定？会不会变成又一个通知中心。', userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '怎么让旧想法半年后再次有用', essence: '回响的价值取决于"理由"而非"频率"。',
        keyPoints: ['回响不是随机推送，是"有理由地重新出现"', '打扰度需要静默时段与频率上限', '每条回响必须能解释为什么再次出现'],
        openQuestion: '回响的"有用"如何定义？', nextStep: '收集 3 个真实回响案例',
        suggestedTags: ['产品', '回响'], suggestedType: 'question',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: true, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-006', type: 'continues', reason: '关于回响的产品设计，可相互印证' }],
      reminder: null,
      tags: ['产品', '回响'],
      stats: { openCount: 9, continuationCount: 1, relationCount: 1, lastOpenedAt: '2026-08-26T22:40:00+08:00', updatedAt: '2026-08-27T11:08:00+08:00' }
    },
    {
      id: 'mem-003',
      primaryType: 'action',
      original: {
        text: '给语音记录停止后的那一屏加"继续思考"入口：文字续写 + 语音续写，直接追加到当前记忆。',
        kind: 'text', capturedAt: '2026-08-26T19:30:00+08:00', capturedAtPrecision: 'exact',
        source: 'text', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '给语音记录加"继续思考"按钮', essence: '停止录音后的一屏里放文字/语音续写入口。',
        keyPoints: ['续写追加为当前记忆的新片段', '也可生成独立记忆并建立关联', '入口要 2 次操作内可达'],
        openQuestion: null, nextStep: '设置提醒',
        suggestedTags: ['产品', '行动'], suggestedType: 'action',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-001', type: 'supports', reason: '都是"语音→行动"闭环的一部分' }],
      reminder: { triggerAt: '2026-08-29T09:00:00+08:00', reason: '明天开始实现', status: 'active' },
      tags: ['产品', '行动'],
      stats: { openCount: 3, continuationCount: 1, relationCount: 1, lastOpenedAt: '2026-08-28T08:10:00+08:00', updatedAt: '2026-08-26T19:35:00+08:00' }
    },
    {
      id: 'mem-004',
      primaryType: 'reflection',
      original: {
        text: '今天 review 发现，我们总是"先想再记"：犹豫半天要不要写、写什么。其实应该"先记再想"——先安全落盘，再决定怎么处理。',
        kind: 'audio', capturedAt: '2026-08-25T22:18:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 24000, hasWaveform: true },
        transcript: { clean: '今天 review 发现，我们总是先想再记：犹豫半天要不要写、写什么。其实应该先记再想——先安全落盘，再决定怎么处理。', userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '先记再想，别先想再记', essence: '捕捉成本要低到"不值得犹豫"。',
        keyPoints: ['保存必须先于整理，不阻塞', '本地优先保证断网也能记', '"已安全保存"优先于一切进度反馈'],
        openQuestion: null, nextStep: '稍后回看',
        suggestedTags: ['工作', '反思'], suggestedType: 'reflection',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: null,
      tags: ['工作', '反思'],
      stats: { openCount: 4, continuationCount: 0, relationCount: 0, lastOpenedAt: '2026-08-27T21:00:00+08:00', updatedAt: '2026-08-25T22:20:00+08:00' }
    },
    {
      id: 'mem-005',
      primaryType: 'reference',
      original: {
        text: 'The Cult of Done 摘录：\n1. 完成胜过完美。\n2. 假装你知道自己在做什么。\n3. 停止规划，开始做事。\n4. 不要试图同时成为这么多。',
        kind: 'import', capturedAt: '2026-08-24T15:40:00+08:00', capturedAtPrecision: 'date_only',
        source: 'import', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: 'The Cult of Done 摘录', essence: '完成胜过完美，停止规划开始做事。',
        keyPoints: ['完成胜过完美', '停止规划，开始做事', '假装你知道自己在做什么'],
        openQuestion: null, nextStep: null,
        suggestedTags: ['效率', '摘录'], suggestedType: 'reference',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-007', type: 'continues', reason: '你稍后整理旧笔记时引用过它' }],
      reminder: null,
      tags: ['效率', '摘录'],
      stats: { openCount: 7, continuationCount: 0, relationCount: 1, lastOpenedAt: '2026-08-24T16:00:00+08:00', updatedAt: '2026-08-24T15:45:00+08:00' }
    },
    {
      id: 'mem-006',
      primaryType: 'idea',
      original: {
        text: '用"回响"替代通知：不打扰，只在合适的时候把旧想法带回来。把推送变成有理由的回忆。',
        kind: 'audio', capturedAt: '2026-08-28T08:02:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 16000, hasWaveform: true },
        transcript: { clean: '用回响替代通知：不打扰，只在合适的时候把旧想法带回来。把推送变成有理由的回忆。', userCorrected: null },
        context: { activity: 'commuting', locationName: null }
      },
      ai: { title: null, essence: null, keyPoints: [], openQuestion: null, nextStep: null, suggestedTags: [], suggestedType: 'idea', status: 'processing', provenance: 'ai', revision: 0 },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'processing', pinned: false, lifecycle: 'active',
      sync: { state: 'pending_sync' },
      relations: [{ toCardId: 'mem-002', type: 'continues', reason: '关于回响的产品设计，可相互印证' }],
      reminder: null,
      tags: [],
      stats: { openCount: 0, continuationCount: 0, relationCount: 0, lastOpenedAt: null, updatedAt: '2026-08-28T08:02:30+08:00' }
    },
    {
      id: 'mem-007',
      primaryType: 'action',
      original: {
        text: '整理 5 月从旧备忘录导入的 200 条笔记。大部分缺标题，类型也不对，需要批量处理。',
        kind: 'import', capturedAt: '2026-08-23T10:00:00+08:00', capturedAtPrecision: 'date_only',
        source: 'import', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '整理 5 月导入的旧笔记', essence: '迁移 200 条旧备忘录，缺标题待批量补全。',
        keyPoints: ['200 条旧记录，缺标题', '类型识别不准确需复核', '用"批量补全"降低迁移成本'],
        openQuestion: null, nextStep: '批量补全',
        suggestedTags: ['迁移'], suggestedType: 'action',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-005', type: 'continues', reason: '你稍后整理旧笔记时引用过它' }],
      reminder: null,
      tags: ['迁移'],
      stats: { openCount: 2, continuationCount: 0, relationCount: 1, lastOpenedAt: '2026-08-26T14:30:00+08:00', updatedAt: '2026-08-23T10:05:00+08:00' }
    },
    {
      id: 'mem-008',
      primaryType: 'question',
      original: {
        text: '转写准确率怎么在不暴露原文的前提下做隐私友好的评测？',
        kind: 'audio', capturedAt: '2026-08-22T20:00:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 9000, hasWaveform: true },
        transcript: { clean: '转写准确率怎么在不暴露原文的前提下做隐私友好的评测？', userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: { title: null, essence: null, keyPoints: [], openQuestion: null, nextStep: null, suggestedTags: [], suggestedType: 'question', status: 'failed', provenance: 'ai', revision: 0 },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'failed', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: null,
      tags: [],
      stats: { openCount: 1, continuationCount: 0, relationCount: 0, lastOpenedAt: '2026-08-22T20:10:00+08:00', updatedAt: '2026-08-22T20:02:00+08:00' }
    },
    {
      id: 'mem-009',
      primaryType: 'reference',
      original: {
        text: 'MCP 官方示例：让 Agent 读取 Notion 数据库并新建页面。需要配置 Notion API Token 和数据库 ID。',
        kind: 'text', capturedAt: '2026-08-21T09:15:00+08:00', capturedAtPrecision: 'exact',
        source: 'text', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: 'MCP 读 Notion 建页面（收藏）', essence: '外部工具执行示例，属于 Later 能力。',
        keyPoints: ['golang-mcp + stdio 传输', '外部副作用必须逐项授权', '与记忆链路隔离'],
        openQuestion: null, nextStep: null,
        suggestedTags: ['MCP', '集成'], suggestedType: 'reference',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: null,
      tags: ['MCP', '集成'],
      stats: { openCount: 3, continuationCount: 0, relationCount: 0, lastOpenedAt: '2026-08-25T11:20:00+08:00', updatedAt: '2026-08-21T09:20:00+08:00' }
    },
    {
      id: 'mem-010',
      primaryType: 'reflection',
      original: {
        text: '工具要适配生活节奏，而不是反过来。跑步 5 公里后的几分钟，是灵感的黄金窗口。',
        kind: 'audio', capturedAt: '2026-08-20T18:45:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 21000, hasWaveform: true },
        transcript: { clean: '工具要适配生活节奏，而不是反过来。跑步五公里后的几分钟，是灵感的黄金窗口。', userCorrected: null },
        context: { activity: 'running', locationName: null }
      },
      ai: {
        title: '工具适配生活节奏', essence: '捕捉入口要出现在"灵感发生的时刻"。',
        keyPoints: ['跑步/通勤是高价值捕捉场景', '入口要 1 次物理动作可达', '别让工具本身成为负担'],
        openQuestion: null, nextStep: null,
        suggestedTags: ['产品'], suggestedType: 'reflection',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: null,
      tags: ['产品'],
      stats: { openCount: 6, continuationCount: 0, relationCount: 0, lastOpenedAt: '2026-08-21T09:00:00+08:00', updatedAt: '2026-08-20T18:48:00+08:00' }
    },
    {
      id: 'mem-011',
      primaryType: 'idea',
      original: {
        text: '给每张卡一个"为什么出现"：让记忆流成为思考的历史，而不是零散的便签堆。',
        kind: 'text', capturedAt: '2026-08-28T09:40:00+08:00', capturedAtPrecision: 'exact',
        source: 'text', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: null, // AI 总开关关闭时创建的纯记录
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'needs_input', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: null,
      tags: [],
      stats: { openCount: 0, continuationCount: 0, relationCount: 0, lastOpenedAt: null, updatedAt: '2026-08-28T09:40:20+08:00' }
    },
    {
      id: 'mem-012',
      primaryType: 'action',
      original: {
        text: '完成回响 MVP 用户测试：本周内跑完 5 人，收集"有用/无关/稍后"的真实反馈数据。',
        kind: 'text', capturedAt: '2026-08-19T14:00:00+08:00', capturedAtPrecision: 'exact',
        source: 'text', timezone: 'Asia/Shanghai',
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '完成回响 MVP 用户测试', essence: '本周内跑完 5 人测试。',
        keyPoints: ['5 名真实用户', '记录每条的反馈原因', '重点看"无关"率'],
        openQuestion: null, nextStep: '预约时间',
        suggestedTags: ['计划'], suggestedType: 'action',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: true, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [],
      reminder: { triggerAt: '2026-08-28T18:00:00+08:00', reason: '本周五前完成', status: 'active' },
      tags: ['计划'],
      stats: { openCount: 8, continuationCount: 1, relationCount: 0, lastOpenedAt: '2026-08-27T15:30:00+08:00', updatedAt: '2026-08-19T14:05:00+08:00' }
    },
    {
      id: 'mem-013',
      primaryType: 'question',
      original: {
        text: '多语言转写，还是先专注中文？\n（从旧备忘录导入的旧问题，原始时间未知）',
        kind: 'import', capturedAt: null, capturedAtPrecision: 'unknown',
        source: 'import', timezone: null,
        audio: null,
        transcript: { clean: null, userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '多语言转写 vs 先专注中文', essence: '待补充：标题/类型/标签 AI 建议中。',
        keyPoints: [], openQuestion: '是否需要多语言 STT？', nextStep: null,
        suggestedTags: ['待补充'], suggestedType: 'question',
        status: 'needs_input', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'needs_input', pinned: false, lifecycle: 'active',
      sync: { state: 'synced' },
      relations: [{ toCardId: 'mem-008', type: 'similar', reason: '都涉及转写与评测' }],
      reminder: null,
      tags: [],
      stats: { openCount: 1, continuationCount: 0, relationCount: 1, lastOpenedAt: '2026-08-24T13:00:00+08:00', updatedAt: '2026-08-23T09:00:00+08:00' }
    },
    {
      id: 'mem-014',
      primaryType: 'idea',
      original: {
        text: '工作流触发条件设想："这条记忆两周没人打开"→ 生成一条回响，提示用户回来看看。',
        kind: 'audio', capturedAt: '2026-08-28T10:05:00+08:00', capturedAtPrecision: 'exact',
        source: 'app', timezone: 'Asia/Shanghai',
        audio: { durationMs: 12000, hasWaveform: true },
        transcript: { clean: '工作流触发条件设想：这条记忆两周没人打开，生成一条回响，提示用户回来看看。', userCorrected: null },
        context: { activity: null, locationName: null }
      },
      ai: {
        title: '触发条件：这条记忆两周没人打开', essence: '用"久未回看"作为回响触发条件。',
        keyPoints: ['触发：记录后/定时/手动', '条件：类型/标签/来源/合集', '外部副作用节点必须逐项授权'],
        openQuestion: null, nextStep: null,
        suggestedTags: ['工作流'], suggestedType: 'idea',
        status: 'ready', provenance: 'ai', revision: 1
      },
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: 'ready', pinned: false, lifecycle: 'active',
      sync: { state: 'syncing' },
      relations: [],
      reminder: null,
      tags: ['工作流'],
      stats: { openCount: 0, continuationCount: 0, relationCount: 0, lastOpenedAt: null, updatedAt: '2026-08-28T10:05:30+08:00' }
    }
  ];

  // ---------- 回响种子 ----------
  var echoes = [
    { id: 'echo-1', cardId: 'mem-006', reason: '你本周第三次提到这个主题', kind: '近期重复主题', scheduledFor: '2026-08-28T08:00:00+08:00', processed: false },
    { id: 'echo-2', cardId: 'mem-004', reason: '较久未回看的高价值记忆 · 12 天前', kind: '久未回看', scheduledFor: '2026-08-28T08:00:00+08:00', processed: false },
    { id: 'echo-3', cardId: 'mem-013', reason: '一条待补充的问题，与你上周的想法相关', kind: '待补充提醒', scheduledFor: '2026-08-28T08:00:00+08:00', processed: false }
  ];

  // ---------- MCP / 工作流 / 其他 ----------
  var mcpConnections = [
    { id: 'mcp-notion', name: 'Notion', icon: 'i-note', status: 'planning', desc: '读取数据库、创建页面' },
    { id: 'mcp-mail', name: '邮箱', icon: 'i-mail', status: 'planning', desc: '发送邮件、整理收件箱' },
    { id: 'mcp-github', name: 'GitHub', icon: 'i-github', status: 'planning', desc: 'Issue / PR 自动化' }
  ];

  var workflows = [
    { id: 'wf-1', name: '记录后自动整理', trigger: '记录后', steps: '整理 → 补全 → 关联', status: 'planning' },
    { id: 'wf-2', name: '导入后提醒回看', trigger: '导入后', steps: '补全 → 提醒', status: 'planning' }
  ];

  var recentSearches = ['回响', '跑步', 'MCP', '迁移'];

  // 导出到 WB
  var WB = global.WB = global.WB || {};
  WB.data = {
    typeLabels: typeLabels,
    sourceLabels: sourceLabels,
    sourceIcons: sourceIcons,
    processingLabels: processingLabels,
    syncLabels: syncLabels,
    relationLabels: relationLabels,
    cards: cards,
    echoes: echoes,
    mcpConnections: mcpConnections,
    workflows: workflows,
    recentSearches: recentSearches
  };
})(typeof window !== 'undefined' ? window : this);
