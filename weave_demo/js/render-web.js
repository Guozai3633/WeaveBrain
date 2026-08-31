/* ============================================================
   织脑原型 — Web 端渲染（回顾与生产力大屏）
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;
  var data = WB.data;
  var R = WB.render;

  // ---------- Web 布局骨架 ----------
  function sidebar(active) {
    function item(screen, label, icon) {
      return '<div class="web-sidebar-item' + (active === screen ? ' active' : '') + '" data-action="web-nav" data-screen="' + screen + '">' +
        R.ic(icon) + '<span>' + label + '</span></div>';
    }
    return '' +
      '<aside class="web-sidebar">' +
        '<div class="web-sidebar-brand"><div class="ds-logo">' + R.ic('i-wave') + '</div>织脑</div>' +
        '<div class="web-sidebar-section">回顾</div>' +
        item('memories', '记忆流', 'i-inbox') +
        item('search', '搜索', 'i-search') +
        '<div class="web-sidebar-section">生产力</div>' +
        item('import', '导入', 'i-import') +
        item('ai-settings', 'AI 设置', 'i-sparkle') +
        item('data', '数据管理', 'i-export') +
        '<div class="web-sidebar-section">预留</div>' +
        item('workflows', '工作流', 'i-compass') +
        '<div class="web-sidebar-footer">' + (store.state.settings.aiMemoryEnabled ? 'AI 记忆整理 已开启' : 'AI 记忆整理 关闭') +
          ' · 原型 v0.3</div>' +
      '</aside>';
  }

  function topbar() {
    var st = store.state.settings;
    var u = store.state.user;
    return '' +
      '<header class="web-topbar">' +
        '<div class="search-box">' + R.ic('i-search') +
          '<input placeholder="搜索记忆…（关键词 / 语义混合）" data-action="web-search-go">' +
        '</div>' +
        '<div class="spacer"></div>' +
        '<label class="switch" title="AI 记忆整理总开关">' +
          '<input type="checkbox" data-action="toggle-master" data-key="aiMemoryEnabled"' + (st.aiMemoryEnabled ? ' checked' : '') + '>' +
          '<span class="track"></span>' +
        '</label>' +
        '<span class="t-xs t-low">AI 整理</span>' +
        '<div class="web-user-chip">' +
          '<div class="avatar-sm">' + (u.status === 'guest' ? '游' : '织') + '</div>' +
          '<span>' + (u.status === 'guest' ? '本地游客' : u.displayName) + '</span>' +
        '</div>' +
      '</header>';
  }

  function layout(active, content) {
    return '<div class="web-layout">' + sidebar(active) +
      '<main class="web-main">' + topbar() + '<div class="web-content">' + content + '</div></main>' +
      '</div>';
  }

  // ---------- Web 记忆流（主从） ----------
  function webMemories() {
    var params = store.state.route.params;
    var selectedId = params.id || null;
    var cards = store.filteredCards();

    var listItems = cards.map(function (c) {
      var sel = selectedId === c.id ? ' selected' : '';
      var title = store.visibleTitle(c);
      var excerpt = (c.original.text || '').slice(0, 90);
      return '' +
        '<div class="web-card' + sel + '" data-action="web-select" data-id="' + c.id + '">' +
          '<div class="card-meta">' + R.badgeTypeHTML(c) + R.badgeStatusHTML(c) +
            (c.pinned ? '<span class="card-pin">' + R.ic('i-pin') + '</span>' : '') +
            '<span class="spacer"></span>' + R.syncDotHTML(c) +
          '</div>' +
          '<div class="card-title">' + R.esc(title) + '</div>' +
          '<div class="card-excerpt t-2line">' + R.esc(excerpt) + '</div>' +
        '</div>';
    }).join('');

    var detailPane = '';
    if (selectedId) {
      detailPane = webDetailPane(selectedId);
    } else {
      detailPane = '' +
        '<div class="web-detail-empty">' + R.ic('i-inbox') +
          '<div>从左侧选择一条记忆查看详情</div>' +
          '<div class="t-xs" style="margin-top:4px">原始记忆 · AI 整理 · 用户编辑 三层分离</div>' +
        '</div>';
    }

    var content = '' +
      '<div class="web-master-detail" style="flex:1;min-height:0">' +
        '<div class="web-list-pane">' +
          '<div class="web-filter-bar">' +
            '<select data-action="web-filter-sort">' +
              '<option value="recent"' + (store.state.list.sort === 'recent' ? ' selected' : '') + '>最近记录</option>' +
              '<option value="earliest"' + (store.state.list.sort === 'earliest' ? ' selected' : '') + '>最早记录</option>' +
              '<option value="updated"' + (store.state.list.sort === 'updated' ? ' selected' : '') + '>最近更新</option>' +
              '<option value="frequent"' + (store.state.list.sort === 'frequent' ? ' selected' : '') + '>常用记忆</option>' +
            '</select>' +
            '<select data-action="web-filter-type">' +
              '<option value="">全部类型</option>' +
              Object.keys(data.typeLabels).map(function (k) {
                return '<option value="' + k + '"' + (store.state.list.filters.types.indexOf(k) !== -1 ? ' selected' : '') + '>' + data.typeLabels[k] + '</option>';
              }).join('') +
            '</select>' +
            '<span class="count-info">共 ' + cards.length + ' 条</span>' +
          '</div>' +
          '<div class="web-list-scroll">' + listItems + '</div>' +
        '</div>' +
        '<div class="web-detail-pane">' + detailPane + '</div>' +
      '</div>';

    return layout('memories', content);
  }

  // ---------- Web 详情窗格（两栏） ----------
  function webDetailPane(id) {
    var card = store.getCard(id);
    if (!card) return '<div class="web-detail-empty">' + R.ic('i-close') + '记忆不存在</div>';

    // 左栏：核心 + 原始
    var leftCol = '' +
      '<div class="web-section">' +
        '<textarea class="web-detail-title-input" rows="1" data-input="title" data-id="' + card.id + '" data-focus-key="title">' + R.esc(store.visibleTitle(card)) + '</textarea>' +
        '<div class="web-detail-meta">' + R.badgeTypeHTML(card) + R.badgeStatusHTML(card) +
          '<span class="t-xs">' + store.fmtTime(card.original.capturedAt) + '</span>' +
          '<span class="t-xs">' + (data.sourceLabels[card.original.source] || card.original.source) + '</span>' +
          (card.pinned ? '<span class="card-pin">' + R.ic('i-pin') + '置顶</span>' : '') +
        '</div>' +
      '</div>' +
      '<div class="web-section">' +
        '<div class="web-section-title">' + R.ic('i-note') + '原始记忆 · 不可被覆盖</div>' +
        '<div class="original-text">' + R.esc(card.original.text) + '</div>' +
        (card.original.transcript && card.original.transcript.clean ? '' +
          '<div style="margin-top:12px;padding-top:12px;border-top:1px dashed var(--outline)">' +
            '<div class="t-xs t-low" style="margin-bottom:5px">转写</div>' +
            '<div class="t-sm">' + R.esc(card.original.transcript.userCorrected || card.original.transcript.clean) + '</div>' +
          '</div>' : '') +
        (card.original.audio ? '' +
          '<div style="margin-top:14px">' +
            '<div class="waveform" data-action="play-audio" data-id="' + card.id + '">' + waveBars(26) + '</div>' +
            '<div class="t-xs t-low" style="margin-top:6px">' + R.ic('i-play') + ' ' + store.fmtDuration(card.original.audio.durationMs) + ' · 原始音频</div>' +
          '</div>' : '') +
      '</div>';

    // 右栏：AI + 关联 + 提醒 + 历史
    var aiHTML = '';
    if (card.ai === null) {
      aiHTML = '<div class="t-sm t-mid">这条记录保存时 AI 整理关闭，仅保留原文。' +
        '<button class="btn btn-soft btn-sm" style="margin-top:8px" data-action="organize-once" data-id="' + card.id + '">仅整理这一条</button></div>';
    } else if (card.ai.title === null && card.processingStatus === 'processing') {
      aiHTML = '<div class="skeleton" style="height:14px;width:60%;margin-bottom:8px"></div><div class="skeleton" style="height:14px;width:85%"></div>';
    } else if (card.processingStatus === 'failed') {
      aiHTML = '<div class="t-sm t-mid">整理失败，原音频可用。<button class="btn btn-soft btn-sm" data-action="organize-retry" data-id="' + card.id + '">重试整理</button></div>';
    } else if (card.ai) {
      var ai = card.ai;
      aiHTML = '<span class="ai-label">' + R.ic('i-sparkle') + 'AI 建议 · v' + (ai.revision || 1) + '</span>';
      if (ai.essence) aiHTML += '<div class="essence">' + R.esc(ai.essence) + '</div>';
      if (ai.keyPoints && ai.keyPoints.length) {
        aiHTML += '<div class="keypoints">' + ai.keyPoints.map(function (k) { return '<div class="keypoint">' + R.esc(k) + '</div>'; }).join('') + '</div>';
      }
      if (ai.openQuestion) aiHTML += '<div class="open-q">' + R.ic('i-question') + '<span>' + R.esc(ai.openQuestion) + '</span></div>';
      if (ai.nextStep) aiHTML += '<div class="next-step-box">' + R.ic('i-chev-r') + R.esc(ai.nextStep) + '</div>';
      aiHTML += '<div style="margin-top:10px"><button class="btn btn-soft btn-sm" data-action="organize-retry" data-id="' + card.id + '">' + R.ic('i-refresh') + '重做 AI 整理</button></div>';
    }

    var relHTML = (card.relations && card.relations.length)
      ? card.relations.map(function (r) { return relationHTML(r); }).join('')
      : '<div class="t-xs t-low">暂无关联</div>';

    var remHTML = card.reminder
      ? '<div class="reminder-banner">' + R.ic('i-bell') + '<span>' + store.fmtTime(card.reminder.triggerAt) + ' · ' + R.esc(card.reminder.reason || '') + '</span></div>'
      : '<button class="btn btn-outline btn-sm" data-action="open-modal" data-modal="reminder" data-id="' + card.id + '">' + R.ic('i-bell') + '设置提醒</button>';

    var rightCol = '' +
      '<div class="web-section">' +
        '<div class="web-section-title">' + R.ic('i-sparkle') + 'AI 整理</div>' + aiHTML +
      '</div>' +
      '<div class="web-section">' +
        '<div class="web-section-title">' + R.ic('i-link') + '关联记忆</div>' + relHTML +
      '</div>' +
      '<div class="web-section">' +
        '<div class="web-section-title">' + R.ic('i-bell') + '提醒与回响</div>' + remHTML +
        '<div class="row gap-8" style="margin-top:10px">' +
          '<button class="btn btn-soft btn-sm" data-action="open-modal" data-modal="continue-text" data-id="' + card.id + '">继续思考</button>' +
          '<button class="btn btn-outline btn-sm" data-action="pin-toggle" data-id="' + card.id + '">' + (card.pinned ? '取消置顶' : '置顶') + '</button>' +
        '</div>' +
      '</div>' +
      '<div class="web-section">' +
        '<div class="web-section-title">' + R.ic('i-clock') + '历史与元数据</div>' +
        '<div class="t-xs t-low" style="line-height:1.9">' +
          '创建 ' + store.fmtTime(card.original.capturedAt) +
          ' · 同步 ' + (data.syncLabels[card.sync.state] || card.sync.state) +
          ' · 回看 ' + ((card.stats && card.stats.openCount) || 0) + ' 次' +
          '<br>来源 ' + (data.sourceLabels[card.original.source] || card.original.source) +
          (card.original.context && card.original.context.activity ? ' · ' + card.original.context.activity : '') +
        '</div>' +
      '</div>';

    return '<div class="web-two-col" style="grid-template-columns:1.05fr .95fr">' +
      '<div>' + leftCol + '</div>' + '<div>' + rightCol + '</div>' +
      '</div>';
  }

  function relationHTML(rel) {
    var target = store.getCard(rel.toCardId);
    if (!target) return '';
    return '' +
      '<div class="relation-card" data-action="web-select" data-id="' + target.id + '">' +
        '<div class="relation-card-main"><div class="relation-card-title">' + R.esc(store.visibleTitle(target)) + '</div>' +
        '<div class="relation-reason">' + R.ic('i-link') + '<span>' + R.esc(rel.reason || '') + '</span></div></div>' +
      '</div>';
  }

  function waveBars(n) {
    var b = '';
    for (var i = 0; i < n; i++) {
      var h = 30 + Math.abs(Math.sin(i * 1.7) * 0.5 + Math.sin(i * 0.9) * 0.4) * 90;
      b += '<span class="bar" style="height:' + Math.round(h) + '%"></span>';
    }
    return b;
  }

  // ---------- Web 搜索 ----------
  function webSearch() {
    var q = store.state.list.searchQuery;
    var cards = store.filteredCards();
    var results = '';
    if (q) {
      results = cards.map(function (c) {
        return '' +
          '<div class="web-card" data-action="web-select" data-id="' + c.id + '" style="max-width:760px">' +
            '<div class="card-meta">' + R.badgeTypeHTML(c) + R.badgeStatusHTML(c) +
              '<span class="spacer"></span>' + R.syncDotHTML(c) + '</div>' +
            '<div class="card-title">' + R.highlight(store.visibleTitle(c), q) + '</div>' +
            '<div class="card-excerpt t-2line">' + R.highlight((c.original.text || '').slice(0, 100), q) + '</div>' +
          '</div>';
      }).join('');
      if (!results) results = '<div class="empty-state" style="max-width:760px">' + R.ic('i-search') + '未找到相关记忆</div>';
    } else {
      results = '<div class="t-low t-sm" style="max-width:760px;padding:30px 16px">输入关键词，体验关键词 + 语义混合搜索（命中片段会高亮）。</div>';
    }

    var content = '' +
      '<div class="web-content" style="padding:0">' +
        '<div style="flex:1;overflow-y:auto;padding:20px 26px">' +
          '<div class="search-box" style="max-width:760px;background:#fff;border:1px solid var(--outline)">' +
            R.ic('i-search') +
            '<input value="' + R.esc(q) + '" placeholder="搜索记忆…" data-input="search" data-focus-key="search">' +
          '</div>' +
          '<div style="margin-top:16px">' + results + '</div>' +
        '</div>' +
      '</div>';

    return layout('search', content);
  }

  // ---------- Web 导入 ----------
  function webImport() {
    var content = '' +
      '<div class="web-content" style="padding:0">' +
        '<div style="flex:1;overflow-y:auto;padding:22px 26px">' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-import') + '批量导入</div>' +
            '<div class="segmented" style="max-width:360px;margin-bottom:16px">' +
              '<button class="active">批量文本</button>' +
              '<button>CSV</button>' +
              '<button>JSONL</button>' +
            '</div>' +
            '<div class="field"><label>多段文本（用 <code>---</code> 分隔多条记录）</label>' +
            '<textarea class="input" rows="7" data-input="import-batch" data-focus-key="import-batch">第一条记忆正文，可以有多个段落。\n\n这是同一条的第二段。\n---\n第二条记忆正文。\n---\n第三条：跑步时想到的灵感。</textarea></div>' +
            '<div class="row gap-8" style="flex-wrap:wrap">' +
              '<button class="btn btn-outline btn-sm" data-action="import-parse">解析并预览</button>' +
              '<button class="btn btn-soft btn-sm" data-action="import-completion">' + R.ic('i-sparkle') + 'AI 补全缺失项</button>' +
              '<button class="btn btn-primary btn-sm" data-action="do-import-batch">确认导入</button>' +
            '</div>' +
          '</div>' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-check') + '预览（前 3 条）</div>' +
            '<div class="import-card">' +
              '<div class="import-preview-row"><span class="preview-row-num">1</span>' +
                '<div><div class="t-sm t-weight-600">第一条记忆正文…</div>' +
                '<div class="t-xs t-low">标题缺 · <span class="tag tag-provenance">AI 建议</span> · 时间缺 · 保持未知</div></div></div>' +
              '<div class="import-preview-row"><span class="preview-row-num">2</span>' +
                '<div><div class="t-sm t-weight-600">第二条记忆正文。</div>' +
                '<div class="t-xs t-low">标题缺 · <span class="tag tag-provenance">AI 建议</span> · 时间缺 · 保持未知</div></div></div>' +
              '<div class="import-preview-row"><span class="preview-row-num">3</span>' +
                '<div><div class="t-sm t-weight-600">第三条：跑步时想到的灵感。</div>' +
                '<div class="t-xs t-low">已就绪 · 类型：闪念（AI 建议）</div></div></div>' +
            '</div>' +
            '<div class="row gap-8" style="margin-top:10px">' +
              '<span class="tag">3 条待导入</span><span class="tag">1 条疑似重复</span><span class="tag">0 错误</span>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';
    return layout('import', content);
  }

  // ---------- Web AI 设置 ----------
  function webAISettings() {
    var st = store.state.settings;
    var masterOn = st.aiMemoryEnabled;
    var pending = store.pendingOrganizeCount();

    function sw(key, title, sub, enabled) {
      return '<div class="switch-row">' +
        '<div class="switch-row-main"><div class="switch-row-title">' + title + '</div>' +
        '<div class="switch-row-sub">' + sub + '</div></div>' +
        '<label class="switch"><input type="checkbox" data-action="toggle" data-key="' + key + '"' + (st[key] ? ' checked' : '') + (enabled ? '' : ' disabled') + '><span class="track"></span></label>' +
      '</div>';
    }

    var content = '' +
      '<div class="web-content" style="padding:0">' +
        '<div style="flex:1;overflow-y:auto;padding:22px 26px">' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-sparkle') + 'AI 记忆整理 · 总开关</div>' +
            '<div class="ai-hero" style="margin:0">' +
              '<div class="ai-hero-desc">开启后，新记录会在后台生成标题、核心、要点、标签、关联与回响；关闭后，只保存为普通备忘录，不再触发任何 AI 任务。</div>' +
              '<div class="ai-switch-box">' +
                '<div><div class="ai-switch-box-title">' + (masterOn ? '已开启' : '已关闭') + '</div>' +
                '<div class="ai-switch-box-sub">' + (masterOn ? '新记录将后台整理' : '纯备忘录模式 · AI 调用为零') + '</div></div>' +
                '<label class="switch switch-lg"><input type="checkbox" data-action="toggle-master" data-key="aiMemoryEnabled"' + (masterOn ? ' checked' : '') + '><span class="track"></span></label>' +
              '</div>' +
            '</div>' +
          '</div>' +
          '<div class="web-two-col">' +
            '<div>' +
              '<div class="web-section">' +
                '<div class="web-section-title">子开关</div>' +
                sw('autoSummary', '自动摘要', '生成一句话核心与要点', masterOn) +
                sw('autoTags', '自动标签', '推荐不超过 2 个标签', masterOn) +
                sw('autoRelation', '关联记忆', '找到相关旧想法并说明原因', masterOn) +
                sw('autoEcho', '主动回响', '在合适时间带回旧想法', masterOn) +
              '</div>' +
              '<div class="web-section">' +
                '<div class="web-section-title">AI 补全</div>' +
                sw('aiCompletionEnabled', 'AI 补全入口', '检测缺失 → 建议 → 预览 → 逐字段采用', true) +
              '</div>' +
            '</div>' +
            '<div>' +
              '<div class="web-section">' +
                '<div class="web-section-title">语音与隐私</div>' +
                sw('sttEnabled', '云端转写', '独立于 AI 整理', true) +
                sw('allowCloudAudio', '允许云端处理音频', '音频默认保留 30 天', true) +
                sw('allowCloudText', '允许云端处理文本', '关闭后模型无法读取文字', true) +
              '</div>' +
              '<div class="web-section">' +
                '<div class="web-section-title">补整理未处理记忆</div>' +
                (pending > 0
                  ? '<button class="btn btn-primary" data-action="organize-batch">' + R.ic('i-sparkle') + '补整理未处理记忆（' + pending + ' 条）</button>'
                  : '<span class="t-sm t-low">暂无未处理记忆</span>') +
                '<div class="t-xs t-low" style="margin-top:8px">只处理之后的新记录与本次手动补整理，不会静默回溯历史。</div>' +
              '</div>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';
    return layout('ai-settings', content);
  }

  // ---------- Web 数据管理 ----------
  function webData() {
    var content = '' +
      '<div class="web-content" style="padding:0">' +
        '<div style="flex:1;overflow-y:auto;padding:22px 26px;max-width:860px">' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-export') + '导出全部数据</div>' +
            '<div class="t-sm t-mid" style="margin-bottom:12px">导出包含原始内容、用户编辑、AI 派生、时间情境和附件清单。点击后生成 JSON 备份文件。</div>' +
            '<button class="btn btn-primary btn-sm" data-action="export-all">' + R.ic('i-download') + '导出 JSON</button>' +
          '</div>' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-settings') + '存储占用</div>' +
            '<div class="storage-row"><span>音频</span><div class="progress-track"><div class="progress-fill" style="width:62%"></div></div><span class="t-xs">62%</span></div>' +
            '<div class="storage-row"><span>文本与向量</span><div class="progress-track"><div class="progress-fill" style="width:23%"></div></div><span class="t-xs">23%</span></div>' +
            '<div class="storage-row"><span>缓存</span><div class="progress-track"><div class="progress-fill" style="width:15%"></div></div><span class="t-xs">15%</span></div>' +
          '</div>' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-trash') + '回收站（保留 30 天）</div>' +
            '<table class="data-table">' +
              '<thead><tr><th>记忆</th><th>删除时间</th><th>永久删除剩余</th><th style="text-align:right">操作</th></tr></thead>' +
              '<tbody>' +
                '<tr><td>跑完步的复盘草稿</td><td>2026-08-20</td><td>21 天</td>' +
                  '<td class="table-actions"><button class="btn btn-outline btn-sm" data-action="toast">恢复</button><button class="btn btn-outline btn-sm" style="color:var(--err)" data-action="open-modal" data-modal="trash">永久删除</button></td></tr>' +
                '<tr><td>旧项目废弃方案</td><td>2026-08-18</td><td>19 天</td>' +
                  '<td class="table-actions"><button class="btn btn-outline btn-sm" data-action="toast">恢复</button><button class="btn btn-outline btn-sm" style="color:var(--err)" data-action="open-modal" data-modal="trash">永久删除</button></td></tr>' +
              '</tbody>' +
            '</table>' +
          '</div>' +
          '<div class="web-section">' +
            '<div class="web-section-title" style="color:var(--err)">' + R.ic('i-trash') + '危险区</div>' +
            '<button class="btn btn-outline btn-sm" style="color:var(--err);border-color:#FECACA" data-action="open-modal" data-modal="delete-account">删除账号并清除所有数据</button>' +
          '</div>' +
        '</div>' +
      '</div>';
    return layout('data', content);
  }

  // ---------- Web 工作流 ----------
  function webWorkflows() {
    var items = data.workflows.map(function (w) {
      return '<div class="wf-card">' +
        '<div class="wf-icon">' + R.ic('i-compass') + '</div>' +
        '<div class="wf-card-main"><div class="wf-card-title">' + R.esc(w.name) + '</div>' +
        '<div class="wf-card-sub">触发：' + R.esc(w.trigger) + ' · 步骤：' + R.esc(w.steps) + '</div></div>' +
        '<span class="badge-planning">规划中</span>' +
      '</div>';
    }).join('');
    var content = '' +
      '<div class="web-content" style="padding:0">' +
        '<div style="flex:1;overflow-y:auto;padding:22px 26px;max-width:860px">' +
          '<div class="web-section">' +
            '<div class="web-section-title">' + R.ic('i-compass') + '工作流方案</div>' +
            '<div class="t-sm t-mid" style="margin-bottom:14px">设计"触发 → 条件 → 步骤 → 输出"的记忆处理方案。当前 capabilities 明确不可用（designer=false, execution=false），本页仅为入口预留。</div>' +
            items +
            '<button class="btn btn-outline" data-action="planning-toast">' + R.ic('i-plus') + '新建方案</button>' +
          '</div>' +
        '</div>' +
      '</div>';
    return layout('workflows', content);
  }

  // ---------- Web 分发 ----------
  function renderWeb() {
    var route = store.state.route;
    var el = document.getElementById('web-screen');
    var html;
    switch (route.screen) {
      case 'search': html = webSearch(); break;
      case 'import': html = webImport(); break;
      case 'ai-settings': html = webAISettings(); break;
      case 'data': html = webData(); break;
      case 'workflows': html = webWorkflows(); break;
      default: html = webMemories();
    }
    el.innerHTML = html;
  }

  WB.renderWeb = { renderWeb: renderWeb };
})(typeof window !== 'undefined' ? window : this);
