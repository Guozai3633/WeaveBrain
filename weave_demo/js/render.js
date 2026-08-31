/* ============================================================
   织脑原型 — 渲染引擎（共享组件 + 手机端屏幕）
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;
  var data = WB.data;

  // ---------- 基础工具 ----------
  function esc(s) {
    if (s === null || s === undefined) return '';
    return String(s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }
  function ic(name, extra) {
    return '<svg class="icon' + (extra ? ' ' + extra : '') + '"><use href="#' + name + '"/></svg>';
  }
  function highlight(text, q) {
    if (!q) return esc(text);
    var t = esc(text);
    var qq = esc(q).toLowerCase();
    var idx = t.toLowerCase().indexOf(qq);
    if (idx === -1) return t;
    return t.slice(0, idx) + '<mark>' + t.slice(idx, idx + q.length) + '</mark>' + t.slice(idx + q.length);
  }

  // ---------- 卡片构建 ----------
  function badgeTypeHTML(card) {
    var label = data.typeLabels[card.primaryType] || card.primaryType;
    return '<span class="badge badge-type type-' + card.primaryType + '">' + label + '</span>';
  }
  function badgeStatusHTML(card) {
    var p = card.processingStatus;
    if (card.ai === null && p === 'needs_input') {
      return '<span class="badge badge-st st-needs">仅记录</span>';
    }
    var label = data.processingLabels[p] || p;
    return '<span class="badge badge-st st-' + p + '">' + label + '</span>';
  }
  function syncDotHTML(card) {
    var s = (card.sync && card.sync.state) || 'synced';
    return '<span class="sync-dot ' + s + '" title="' + (data.syncLabels[s] || s) + '"></span>';
  }
  function cardContextHTML(card) {
    var o = card.original;
    var ctx = [];
    ctx.push('<span class="ctx-item">' + ic('i-clock') + store.fmtTime(o.capturedAt) + '</span>');
    var src = data.sourceLabels[o.source] || o.source;
    ctx.push('<span class="ctx-item">' + ic(data.sourceIcons[o.source] || 'i-mic') + src + '</span>');
    if (o.context && o.context.activity) {
      ctx.push('<span class="ctx-item">' + ic('i-compass') + activityLabel(o.context.activity) + '</span>');
    }
    if (o.context && o.context.locationName) {
      ctx.push('<span class="ctx-item">' + ic('i-pin') + esc(o.context.locationName) + '</span>');
    }
    return ctx.join('');
  }
  function activityLabel(a) {
    var map = { running: '跑步', walking: '散步', commuting: '通勤' };
    return map[a] || a;
  }
  function cardSupplyHTML(card) {
    var tags = store.effectiveTags(card);
    var parts = [];
    var shown = tags.slice(0, 2);
    shown.forEach(function (t) {
      parts.push('<span class="tag">' + esc(t) + '</span>');
    });
    if (tags.length > 2) parts.push('<span class="tag">+' + (tags.length - 2) + '</span>');
    var rc = (card.stats && card.stats.relationCount) || 0;
    if (rc > 0) parts.push('<span class="tag">' + ic('i-link', 'icon-sm') + ' ' + rc + '</span>');
    if (card.reminder) parts.push('<span class="tag" style="color:#065F46;background:#ECFDF5">' + ic('i-bell', 'icon-sm') + ' ' + store.fmtTime(card.reminder.triggerAt) + '</span>');
    if (card.ai && card.ai.nextStep) {
      parts.push('<span class="next-step">' + ic('i-chev-r') + esc(card.ai.nextStep) + '</span>');
    }
    return parts.join('');
  }
  function cardActionsHTML(card, opts) {
    var quick = [];
    quick.push('<button class="card-action" data-action="go-detail" data-id="' + card.id + '">' + ic('i-edit') + '继续</button>');
    quick.push('<button class="card-action" data-action="open-modal" data-modal="reminder" data-id="' + card.id + '">' + ic('i-bell') + '提醒</button>');
    quick.push('<button class="card-action" data-action="open-modal" data-modal="more" data-id="' + card.id + '">' + ic('i-more') + '更多</button>');
    return quick.join('');
  }
  function cardHTML(card, opts) {
    opts = opts || {};
    var title = store.visibleTitle(card);
    var excerpt = card.original.text || '';
    var pinned = card.pinned ? '<span class="card-pin">' + ic('i-pin') + '</span>' : '';
    return '' +
      '<div class="card' + (card.pinned ? ' pinned-card' : '') + '" data-action="go-detail" data-id="' + card.id + '">' +
        '<div class="card-meta">' +
          badgeTypeHTML(card) + badgeStatusHTML(card) + pinned +
          '<span class="spacer"></span>' + syncDotHTML(card) +
        '</div>' +
        '<div class="card-title">' + esc(title) + '</div>' +
        '<div class="card-excerpt t-2line">' + esc(excerpt) + '</div>' +
        '<div class="card-context">' + cardContextHTML(card) + '</div>' +
        '<div class="card-supp">' + cardSupplyHTML(card) + '</div>' +
        '<div class="card-actions">' + cardActionsHTML(card) + '</div>' +
      '</div>';
  }
  function relationCardHTML(rel, card) {
    var target = store.getCard(rel.toCardId);
    if (!target) return '';
    var label = data.relationLabels[rel.type] || rel.type;
    return '' +
      '<div class="relation-card" data-action="go-detail" data-id="' + target.id + '">' +
        '<div class="relation-card-main">' +
          '<div class="relation-card-title">' + esc(store.visibleTitle(target)) + '</div>' +
          '<div class="relation-reason">' + ic('i-link') + '<span>' + esc(rel.reason || '') + '</span></div>' +
        '</div>' +
        '<span class="badge badge-st">' + label + '</span>' +
      '</div>';
  }
  function audioWaveHTML(card) {
    if (!card.original.audio) return '';
    var bars = '';
    var n = 24;
    for (var i = 0; i < n; i++) {
      var h = 30 + Math.abs(Math.sin(i * 1.7) * 0.5 + Math.sin(i * 0.9) * 0.4) * 90;
      bars += '<span class="bar" style="height:' + Math.round(h) + '%"></span>';
    }
    return '' +
      '<div class="waveform" data-action="play-audio" data-id="' + card.id + '">' + bars + '</div>' +
      '<div style="display:flex;align-items:center;gap:7px;margin-top:6px;font-size:12px;color:var(--text-low)">' +
        ic('i-play') + store.fmtDuration(card.original.audio.durationMs) + ' · 原始音频 · 点击播放' +
      '</div>';
  }

  // ---------- 底部导航 ----------
  function mobileNav(active) {
    function item(screen, label, icon, activeScreen) {
      return '<button class="nav-item' + (activeScreen === screen ? ' active' : '') + '" data-action="nav" data-screen="' + screen + '">' +
        ic(icon) + '<span>' + label + '</span></button>';
    }
    var isMem = active === 'memories' || active === 'search' || active === 'detail';
    var isEcho = active === 'echo';
    var isMe = active === 'me' || active === 'ai-settings' || active === 'account' || active === 'mcp' || active === 'workflow';
    return '' +
      '<nav class="mobile-nav">' +
        item('memories', '记忆', 'i-inbox', isMem ? 'memories' : '') +
        '<div class="nav-gap">' +
          '<button class="fab-record" data-action="start-capture" title="点击录音 · 长按更多入口">' + ic('i-mic') + '</button>' +
        '</div>' +
        item('echo', '回响', 'i-wave', isEcho ? 'echo' : '') +
        item('me', '我的', 'i-user', isMe ? 'me' : '') +
      '</nav>';
  }
  function mobileShell(active, content) {
    return '<div class="mobile-view">' + content + mobileNav(active) + '</div>';
  }

  // ---------- 空状态 ----------
  function emptyState(icon, title, sub) {
    return '<div class="empty-state">' + ic(icon) + '<div class="empty-state-title">' + title + '</div>' +
      '<div class="empty-state-sub">' + sub + '</div></div>';
  }

  // ============================================================
  // 手机端：记忆流
  // ============================================================
  function screenMemories() {
    var s = store.state.list;
    var chips = [
      { v: 'all', label: '全部' },
      { v: 'idea', label: '闪念' },
      { v: 'action', label: '行动' },
      { v: 'question', label: '问题' },
      { v: 'reflection', label: '反思' },
      { v: 'reference', label: '资料' }
    ];
    var chipHTML = chips.map(function (c) {
      var active = s.activeChip === c.v;
      return '<button class="chip' + (active ? ' chip-active' : '') + '" data-action="chip" data-value="' + c.v + '">' + c.label + '</button>';
    }).join('');

    var cards = store.filteredCards();
    var groups = store.groupByDay(cards);
    var listHTML = '';
    groups.forEach(function (g) {
      listHTML += '<div class="day-group-title">' + store.fmtDayLabel(g.date) + ' · ' + g.cards.length + '</div>';
      g.cards.forEach(function (c) { listHTML += cardHTML(c); });
    });
    if (!listHTML) {
      listHTML = emptyState('i-inbox', '还没有记忆', '点击下方按钮，随手说一句');
    }

    var syncInfo = store.state.user.status === 'guest'
      ? '<span class="t-mid">游客模式 · 仅本机</span>'
      : '<span class="t-mid">' + (store.state.user.lastSyncAt ? '已同步 ' + store.fmtTime(store.state.user.lastSyncAt) : '已登录') + '</span>';

    var content = '' +
      '<div class="appbar">' +
        '<div style="flex:1"><div class="appbar-title">记忆</div><div class="appbar-sub">' + syncInfo + '</div></div>' +
        '<button class="appbar-action" data-action="go" data-screen="search">' + ic('i-search') + '</button>' +
        '<button class="appbar-action" data-action="open-modal" data-modal="filter-sort">' + ic('i-filter') + '</button>' +
      '</div>' +
      '<div class="chip-row">' + chipHTML + '</div>' +
      '<div class="mobile-scroll"><div class="memories-list">' + listHTML + '</div></div>';

    return mobileShell('memories', content);
  }

  // ============================================================
  // 手机端：搜索
  // ============================================================
  function screenSearch() {
    var s = store.state.list;
    var q = s.searchQuery;
    var cards = store.filteredCards();
    var recents = data.recentSearches.map(function (r) {
      return '<div class="recent-item" data-action="search" data-q="' + esc(r) + '">' + ic('i-clock') + esc(r) + '</div>';
    }).join('');

    var resultsHTML = '';
    if (q) {
      if (cards.length === 0) {
        resultsHTML = emptyState('i-search', '没有找到「' + esc(q) + '」', '试试换一个关键词，或清除筛选');
      } else {
        cards.forEach(function (c) {
          var title = store.visibleTitle(c);
          var excerpt = (c.original.text || '').slice(0, 80);
          resultsHTML += '' +
            '<div class="card" data-action="go-detail" data-id="' + c.id + '">' +
              '<div class="card-meta">' + badgeTypeHTML(c) + '<span class="spacer"></span>' + syncDotHTML(c) + '</div>' +
              '<div class="card-title">' + highlight(title, q) + '</div>' +
              '<div class="card-excerpt t-2line">' + highlight(excerpt, q) + '</div>' +
            '</div>';
        });
      }
    }

    var content = '' +
      '<div class="search-topbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="search-box">' + ic('i-search') +
          '<input data-input="search" value="' + esc(q) + '" placeholder="搜索记忆…" autocomplete="off" data-focus-key="search">' +
        '</div>' +
        (q ? '<button class="btn-icon" data-action="search-clear">' + ic('i-close') + '</button>' : '') +
      '</div>' +
      '<div class="mobile-scroll">' +
        (q ? '<div class="search-results">' + resultsHTML + '</div>'
           : '<div class="recent-searches"><div class="day-group-title">最近搜索</div>' + recents + '</div>') +
      '</div>';

    return mobileShell('memories', content);
  }

  // ============================================================
  // 手机端：记忆详情（三层分离）
  // ============================================================
  function screenDetail(id) {
    var card = store.getCard(id);
    if (!card) return mobileShell('memories', emptyState('i-close', '未找到记忆', '这条记忆不存在或已被删除'));

    // 三层物理分区
    var originalBlock = '' +
      '<div class="detail-block block-original">' +
        '<div class="detail-block-title">' + ic('i-note') + '原始记忆 · 不可被覆盖</div>' +
        '<div class="original-text">' + esc(card.original.text) + '</div>' +
        (card.original.transcript && card.original.transcript.clean ? '' +
          '<div style="margin-top:10px;padding-top:10px;border-top:1px dashed var(--outline)">' +
            '<div class="t-xs t-low" style="margin-bottom:5px">转写（' +
              (card.original.transcript.userCorrected ? '你的修正版' : '可修正') + '）</div>' +
            '<div class="t-sm t-mid">' + esc(card.original.transcript.userCorrected || card.original.transcript.clean) + '</div>' +
            '<div class="transcript-fix" data-action="open-modal" data-modal="transcript" data-id="' + card.id + '">查看并修正转写</div>' +
          '</div>' : '') +
        (card.original.audio ? audioWaveHTML(card) : '') +
      '</div>';

    var aiBlock = '';
    if (card.ai === null) {
      // 总开关关闭时创建的纯记录
      aiBlock = '' +
        '<div class="detail-block block-ai">' +
          '<div class="detail-block-title">' + ic('i-sparkle') + 'AI 整理 · 未启用</div>' +
          '<div class="t-sm t-mid" style="margin-bottom:10px">这条记录保存时「AI 记忆整理」处于关闭状态，因此只保存了原文。</div>' +
          '<button class="btn btn-soft btn-sm" data-action="organize-once" data-id="' + card.id + '">' + ic('i-sparkle') + '仅整理这一条</button>' +
        '</div>';
    } else if (card.ai.title === null && card.processingStatus === 'processing') {
      aiBlock = '' +
        '<div class="detail-block block-ai">' +
          '<div class="detail-block-title">' + ic('i-sparkle') + 'AI 整理 · 整理中</div>' +
          '<div class="skeleton" style="height:14px;width:70%;margin-bottom:8px"></div>' +
          '<div class="skeleton" style="height:14px;width:92%"></div>' +
        '</div>';
    } else if (card.processingStatus === 'failed') {
      aiBlock = '' +
        '<div class="detail-block block-ai">' +
          '<div class="detail-block-title">' + ic('i-sparkle') + 'AI 整理 · 失败</div>' +
          '<div class="t-sm t-mid" style="margin-bottom:10px">整理任务失败，原音频与原文仍可用。你可以重试整理。</div>' +
          '<button class="btn btn-soft btn-sm" data-action="organize-retry" data-id="' + card.id + '">' + ic('i-refresh') + '重试整理</button>' +
        '</div>';
    } else {
      var ai = card.ai;
      var aiParts = ['<span class="ai-label">' + ic('i-sparkle') + 'AI 建议 · v' + (ai.revision || 1) + '</span>'];
      if (ai.essence) aiParts.push('<div class="essence">' + esc(ai.essence) + '</div>');
      if (ai.keyPoints && ai.keyPoints.length) {
        var kp = ai.keyPoints.map(function (k) { return '<div class="keypoint">' + esc(k) + '</div>'; }).join('');
        aiParts.push('<div class="keypoints">' + kp + '</div>');
      }
      if (ai.openQuestion) aiParts.push('<div class="open-q">' + ic('i-question') + '<span>' + esc(ai.openQuestion) + '</span></div>');
      if (ai.nextStep) aiParts.push('<div class="next-step-box">' + ic('i-chev-r') + esc(ai.nextStep) + '</div>');
      aiBlock = '' +
        '<div class="detail-block block-ai">' +
          '<div class="detail-block-title">' + ic('i-sparkle') + 'AI 整理 · 可重做</div>' +
          aiParts.join('') +
          '<div style="display:flex;gap:8px;margin-top:12px">' +
            '<button class="btn btn-soft btn-sm" data-action="organize-retry" data-id="' + card.id + '">' + ic('i-refresh') + '重做</button>' +
            '<span class="tag tag-provenance" style="align-self:center">来源：模型 · 可删除可回退</span>' +
          '</div>' +
        '</div>';
    }

    // 用户编辑区
    var hasUserEdit = card.user && (card.user.titleEdited || (card.user.tags && card.user.tags.length));
    var userBlock = '';
    if (hasUserEdit) {
      userBlock = '' +
        '<div class="detail-block block-user">' +
          '<div class="detail-block-title">' + ic('i-edit') + '你的修改 · 最高优先级</div>' +
          (card.user.titleEdited ? '<div class="essence">' + esc(card.user.titleEdited) + '</div>' : '') +
          (card.user.tags && card.user.tags.length
            ? '<div style="display:flex;gap:6px;flex-wrap:wrap">' + card.user.tags.map(function (t) { return '<span class="tag">' + esc(t) + '</span>'; }).join('') + '</div>'
            : '') +
        '</div>';
    }

    // 关联
    var relHTML = '';
    if (card.relations && card.relations.length) {
      relHTML = card.relations.map(function (r) { return relationCardHTML(r, card); }).join('');
    } else {
      relHTML = '<div class="t-xs t-low">暂未发现关联记忆。</div>';
    }

    // 提醒
    var reminderHTML = '';
    if (card.reminder) {
      reminderHTML = '<div class="reminder-banner">' + ic('i-bell') +
        '<span><strong>提醒：</strong>' + store.fmtTime(card.reminder.triggerAt) + ' · ' + esc(card.reminder.reason || '') +
        '<span class="t-xs" style="opacity:.75">（这是你的承诺，不是系统建议）</span></span>' +
        '<span style="flex:1"></span>' +
        '<button class="btn btn-outline btn-sm" data-action="open-modal" data-modal="reminder" data-id="' + card.id + '">修改</button>' +
        '</div>';
    } else {
      reminderHTML = '<button class="btn btn-outline btn-block" data-action="open-modal" data-modal="reminder" data-id="' + card.id + '">' + ic('i-bell') + '设置提醒</button>';
    }

    var syncInfo = data.syncLabels[card.sync.state] || card.sync.state;

    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">记忆详情</div>' +
        '<button class="btn-icon" data-action="pin-toggle" data-id="' + card.id + '" title="置顶">' + ic('i-pin') + '</button>' +
        '<button class="btn-icon" data-action="open-modal" data-modal="more" data-id="' + card.id + '">' + ic('i-more') + '</button>' +
      '</div>' +
      '<div class="mobile-scroll"><div class="detail-scroll">' +
        '<div class="detail-hero">' +
          '<div class="card-meta" style="margin-bottom:8px">' + badgeTypeHTML(card) + badgeStatusHTML(card) +
            (card.pinned ? '<span class="card-pin">' + ic('i-pin') + '</span>' : '') +
            '<span class="spacer"></span>' +
            '<button class="card-action" data-action="open-modal" data-modal="type" data-id="' + card.id + '">' + ic('i-edit') + '改类型</button>' +
          '</div>' +
          '<textarea class="detail-title-input" rows="1" data-input="title" data-id="' + card.id + '" data-focus-key="title">' + esc(store.visibleTitle(card)) + '</textarea>' +
          '<div class="detail-meta">' + cardContextHTML(card) + '</div>' +
          (store.effectiveTags(card).length
            ? '<div class="detail-meta" style="margin-top:6px">' + store.effectiveTags(card).map(function (t) { return '<span class="tag">' + esc(t) + '</span>'; }).join('') + '</div>'
            : '') +
        '</div>' +
        originalBlock +
        aiBlock +
        userBlock +
        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-edit') + '继续思考</div>' +
          '<div class="row gap-8">' +
            '<button class="btn btn-soft btn-sm" data-action="open-modal" data-modal="continue-text" data-id="' + card.id + '">' + ic('i-text') + '文字续写</button>' +
            '<button class="btn btn-soft btn-sm" data-action="continue-voice" data-id="' + card.id + '">' + ic('i-mic') + '语音续写</button>' +
          '</div>' +
        '</div>' +
        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-link') + '关联记忆</div>' + relHTML +
        '</div>' +
        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-bell') + '提醒与回响</div>' + reminderHTML +
        '</div>' +
        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-clock') + '历史与元数据</div>' +
          '<div class="t-xs t-low" style="line-height:1.9">' +
            '创建 ' + store.fmtTime(card.original.capturedAt) + ' · 同步 ' + syncInfo +
            '<br>来源 ' + (data.sourceLabels[card.original.source] || card.original.source) +
            ' · 回看 ' + ((card.stats && card.stats.openCount) || 0) + ' 次 · 续写 ' + ((card.stats && card.stats.continuationCount) || 0) + ' 次' +
            (card.ai ? '<br>AI 版本 v' + card.ai.revision + ' · 用户编辑优先' : '') +
          '</div>' +
        '</div>' +
        '<div class="sync-strip">' + syncDotHTML(card) + '本地同步状态：' + syncInfo +
          (card.sync.state === 'retryable_error' ? '<button class="btn btn-outline btn-sm" data-action="retry-sync" data-id="' + card.id + '">重试</button>' : '') +
        '</div>' +
      '</div></div>';

    return mobileShell('detail', content);
  }

  // ============================================================
  // 手机端：回响
  // ============================================================
  function screenEcho() {
    var tabs = [
      { v: 'today', label: '今日回顾' },
      { v: 'unfinished', label: '未完成' },
      { v: 'relations', label: '新关联' },
      { v: 'done', label: '已处理' }
    ];
    var active = store.state.echo.tab;
    var seg = tabs.map(function (t) {
      return '<button class="' + (active === t.v ? 'active' : '') + '" data-action="echo-tab" data-value="' + t.v + '">' + t.label + '</button>';
    }).join('');

    var items = store.echoItems();
    var listHTML = '';
    items.forEach(function (e) {
      var card = store.getCard(e.cardId);
      if (!card) return;
      if (active === 'done' && !e.processed) return;
      if (active === 'today' && e.processed) return;
      if (active === 'unfinished' && (e.kind !== '待补充提醒' && e.kind !== '久未回看')) return;
      if (active === 'relations' && e.kind !== '近期重复主题') return;
      var fb = store.state.echo.feedback[e.id];
      var processedClass = fb === 'unrelated' ? 'opacity:.55' : '';
      listHTML += '' +
        '<div class="echo-card" style="' + processedClass + '">' +
          '<div class="echo-reason">' + ic('i-wave') + '<span>' + esc(e.reason) + '</span></div>' +
          cardHTML(card, { mini: true }) +
          '<div class="echo-actions">' +
            '<button class="btn btn-soft btn-sm" data-action="echo-feedback" data-id="' + e.id + '" data-fb="continue">' + ic('i-edit') + '继续</button>' +
            '<button class="btn btn-outline btn-sm" data-action="echo-feedback" data-id="' + e.id + '" data-fb="done">' + ic('i-check') + '完成</button>' +
            '<button class="btn btn-outline btn-sm" data-action="echo-feedback" data-id="' + e.id + '" data-fb="later">' + ic('i-clock') + '稍后</button>' +
            '<button class="btn btn-outline btn-sm" data-action="echo-feedback" data-id="' + e.id + '" data-fb="unrelated">' + ic('i-close') + '无关</button>' +
          '</div>' +
        '</div>';
    });
    if (!listHTML) listHTML = emptyState('i-wave', '这里空空如也', '回响会在合适的时候带回旧想法');

    var content = '' +
      '<div class="appbar">' +
        '<div style="flex:1"><div class="appbar-title">回响</div><div class="appbar-sub">有理由地重新出现，不是随机推送</div></div>' +
      '</div>' +
      '<div class="echo-seg"><div class="segmented">' + seg + '</div></div>' +
      '<div class="mobile-scroll"><div class="echo-view">' + listHTML + '</div></div>';

    return mobileShell('echo', content);
  }

  // ============================================================
  // 手机端：我的
  // ============================================================
  function screenMe() {
    var u = store.state.user;
    var isGuest = u.status === 'guest';
    var acct = '' +
      '<div class="account-card">' +
        '<div class="row gap-12">' +
          '<div class="account-avatar">' + (isGuest ? ic('i-user', 'icon-lg') : '织') + '</div>' +
          '<div style="flex:1">' +
            '<div class="account-name">' + esc(u.displayName) + '</div>' +
            '<div class="account-sub">' + (isGuest ? '仅本机 · 数据不离开设备' : '已登录并同步' + (u.lastSyncAt ? ' · ' + store.fmtTime(u.lastSyncAt) : '')) + '</div>' +
          '</div>' +
          '<button class="btn" style="background:rgba(255,255,255,.18);color:#fff" data-action="go" data-screen="account">' + (isGuest ? '登录' : '管理') + '</button>' +
        '</div>' +
      '</div>';

    function item(screen, icon, title, sub, accent) {
      return '<div class="setting-row" data-action="go" data-screen="' + screen + '">' +
        '<div class="setting-row-icon ' + (accent || '') + '">' + ic(icon) + '</div>' +
        '<div class="setting-row-main"><div class="setting-row-title">' + title + '</div>' +
        (sub ? '<div class="setting-row-sub">' + sub + '</div>' : '') + '</div>' +
        ic('i-chev-r', 'chev') + '</div>';
    }

    var content = '' +
      '<div class="appbar"><div class="appbar-title">我的</div></div>' +
      acct +
      '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-num">' + store.pendingOrganizeCount() + '</div><div class="stat-label">待整理</div></div>' +
        '<div class="stat-card"><div class="stat-num">' + store.state.user.mergedCount + '</div><div class="stat-label">已合并</div></div>' +
        '<div class="stat-card"><div class="stat-num">30天</div><div class="stat-label">回收站</div></div>' +
      '</div>' +
      '<div class="mobile-scroll" style="padding-bottom:30px">' +
        '<div class="me-section">' +
          '<div class="me-section-title">账户</div>' +
          '<div class="me-card-group">' +
            item('account', 'i-user', '登录与同步', isGuest ? '本地游客 · 未同步' : '已同步', '') +
            item('account', 'i-share', '导出全部数据', 'JSON 备份', 'accent-green') +
          '</div>' +
        '</div>' +
        '<div class="me-section">' +
          '<div class="me-section-title">AI 与自动化</div>' +
          '<div class="me-card-group">' +
            item('ai-settings', 'i-sparkle', 'AI 与自动化', store.state.settings.aiMemoryEnabled ? 'AI 记忆整理 已开启' : 'AI 记忆整理 关闭', '') +
            item('mcp', 'i-external', 'MCP 集成', data.mcpConnections.length + ' 个连接 · 规划中', '') +
            item('workflow', 'i-compass', '工作流方案', '规划中', 'accent-amber') +
          '</div>' +
        '</div>' +
        '<div class="me-section">' +
          '<div class="me-section-title">数据与隐私</div>' +
          '<div class="me-card-group">' +
            item('account', 'i-calendar', '提醒与静默时段', '', '') +
            item('account', 'i-lock', '隐私与数据', '音频保留 30 天', 'accent-red') +
          '</div>' +
        '</div>' +
        '<div class="me-section" style="padding:0 4px">' +
          '<div class="t-xs t-low" style="text-align:center;padding:16px">织脑 WeaveBrain · 产品原型 v0.3<br>快速捕捉 → 安全保存 → AI 可选整理 → 回看续写</div>' +
        '</div>' +
      '</div>';

    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：AI 设置
  // ============================================================
  function screenAISettings() {
    var st = store.state.settings;
    var masterOn = st.aiMemoryEnabled;
    var pending = store.pendingOrganizeCount();
    var disabledAttr = masterOn ? '' : ' disabled';

    function subSwitch(key, title, sub, enabled) {
      return '<div class="switch-row">' +
        '<div class="switch-row-main"><div class="switch-row-title">' + title + '</div>' +
        '<div class="switch-row-sub">' + sub + '</div></div>' +
        '<label class="switch">' +
          '<input type="checkbox" data-action="toggle" data-key="' + key + '"' + (st[key] ? ' checked' : '') + (enabled ? '' : ' disabled') + '>' +
          '<span class="track"></span>' +
        '</label></div>';
    }

    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">AI 与自动化</div>' +
      '</div>' +
      '<div class="mobile-scroll"><div class="ai-settings-view">' +
        '<div class="ai-hero">' +
          '<div class="ai-hero-title">' + ic('i-sparkle') + 'AI 记忆整理</div>' +
          '<div class="ai-hero-desc">开启后，新记录会在后台生成标题、核心、要点、标签、关联与回响；关闭后，只保存为普通备忘录，不再触发任何 AI 任务。</div>' +
          '<div class="ai-switch-box">' +
            '<div><div class="ai-switch-box-title">' + (masterOn ? '已开启' : '已关闭') + '</div>' +
            '<div class="ai-switch-box-sub">' + (masterOn ? '新记录将后台整理' : '纯备忘录模式 · AI 调用为零') + '</div></div>' +
            '<label class="switch switch-lg">' +
              '<input type="checkbox" data-action="toggle-master" data-key="aiMemoryEnabled"' + (masterOn ? ' checked' : '') + '>' +
              '<span class="track"></span>' +
            '</label>' +
          '</div>' +
        '</div>' +

        '<div class="ai-sub-section">' +
          '<div class="ai-sub-title">子开关' + (masterOn ? '' : '（总开关关闭时不可用，原值保留）') + '</div>' +
          '<div class="ai-sub-group">' +
            subSwitch('autoSummary', '自动摘要', '生成一句话核心与要点', masterOn) +
            subSwitch('autoTags', '自动标签', '推荐不超过 2 个标签', masterOn) +
            subSwitch('autoRelation', '关联记忆', '找到相关旧想法并说明原因', masterOn) +
            subSwitch('autoEcho', '主动回响', '在合适时间带回旧想法', masterOn) +
          '</div>' +
        '</div>' +

        '<div class="ai-sub-section">' +
          '<div class="ai-sub-title">语音与隐私</div>' +
          '<div class="ai-sub-group">' +
            subSwitch('sttEnabled', '云端转写', '独立于 AI 整理 · 关掉整理仍可转写', true) +
            subSwitch('allowCloudAudio', '允许云端处理音频', '音频默认保留 30 天', true) +
            subSwitch('allowCloudText', '允许云端处理文本', '关闭后模型无法读取你的文字', true) +
          '</div>' +
        '</div>' +

        (pending > 0 ? '' +
          '<div class="batch-organize-btn">' +
            '<button class="btn btn-primary btn-block" data-action="organize-batch">' + ic('i-sparkle') + '补整理未处理记忆（' + pending + ' 条）</button>' +
            '<div class="t-xs t-low" style="text-align:center;margin-top:6px;padding:0 20px">只处理之后的新记录与本次手动补整理，不会静默回溯历史。</div>' +
          '</div>' : '') +

        '<div class="ai-sub-section">' +
          '<div class="ai-sub-title">AI 补全</div>' +
          '<div class="ai-sub-group">' +
            subSwitch('aiCompletionEnabled', 'AI 补全入口', '检测缺失字段 → 建议 → 预览 → 逐字段采用', true) +
          '</div>' +
        '</div>' +
      '</div></div>';

    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：登录与同步
  // ============================================================
  function screenAccount() {
    var u = store.state.user;
    var isGuest = u.status === 'guest';
    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">登录与同步</div>' +
      '</div>' +
      '<div class="mobile-scroll" style="padding:14px">' +
        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-user') + '当前状态</div>' +
          '<div style="display:flex;align-items:center;gap:12px">' +
            '<div class="account-avatar" style="background:var(--primary-container);color:var(--primary)">' + (isGuest ? ic('i-user') : '织') + '</div>' +
            '<div><div class="t-lg t-weight-600">' + (isGuest ? '仅本机' : '已登录并同步') + '</div>' +
            '<div class="t-xs t-low">' + (isGuest ? '可记录、回看和导出，仅本机存储' : (u.lastSyncAt ? '最后同步 ' + store.fmtTime(u.lastSyncAt) : '')) + '</div></div>' +
          '</div>' +
          (isGuest
            ? '<button class="btn btn-primary btn-block" style="margin-top:14px" data-action="go" data-screen="login">' + ic('i-arrow-up') + '登录或绑定账号</button>'
            : '<button class="btn btn-danger btn-block" style="margin-top:14px" data-action="logout">退出登录</button>') +
        '</div>' +

        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-sync') + '同步状态</div>' +
          '<div class="t-sm t-mid" style="line-height:1.7">' +
            (isGuest
              ? '游客模式：本地记录会保留，登录后由当前账户认领并同步。'
              : '本地待同步记录会自动上传，断网时保留原文并重试。') +
          '</div>' +
        '</div>' +

        '<div class="detail-block" style="background:var(--surface);border:1px solid var(--outline)">' +
          '<div class="detail-block-title">' + ic('i-export') + '数据</div>' +
          '<div class="row gap-8">' +
            '<button class="btn btn-outline btn-sm" data-action="export-all">导出全部数据</button>' +
            '<button class="btn btn-outline btn-sm" data-action="open-modal" data-modal="trash">回收站 (30 天)</button>' +
          '</div>' +
        '</div>' +
      '</div>';

    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：登录
  // ============================================================
  function screenLogin() {
    var content = '' +
      '<div class="login-view">' +
        '<div class="appbar"><button class="btn-icon" data-action="back">' + ic('i-back') + '</button></div>' +
        '<div class="login-brand">' +
          '<div class="ds-logo">' + ic('i-wave') + '</div>' +
          '<h1>织脑 WeaveBrain</h1>' +
          '<p>以语音为入口、AI 可选的个人思维外脑</p>' +
        '</div>' +
        '<div class="field"><label>手机号</label><input class="input" value="138 0000 0000" data-focus-key="phone"></div>' +
        '<div class="field"><label>验证码</label><div class="row gap-8">' +
          '<input class="input" style="flex:1" value="123456" data-focus-key="code">' +
          '<button class="btn btn-outline">获取验证码</button>' +
        '</div></div>' +
        '<button class="btn btn-primary btn-block" style="margin-top:6px" data-action="do-login">' + ic('i-arrow-up') + '登录 / 注册</button>' +
        '<div class="t-xs t-low" style="text-align:center;margin-top:14px">登录后本地记录会合并到当前账号并同步</div>' +
      '</div>';
    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：MCP
  // ============================================================
  function screenMcp() {
    var items = data.mcpConnections.map(function (m) {
      return '<div class="workflow-card">' +
        '<div class="wf-icon">' + ic(m.icon) + '</div>' +
        '<div class="setting-row-main"><div class="setting-row-title">' + esc(m.name) + '</div>' +
        '<div class="setting-row-sub">' + esc(m.desc) + '</div></div>' +
        '<span class="badge-planning">规划中</span>' +
      '</div>';
    }).join('');
    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">MCP 集成</div>' +
      '</div>' +
      '<div class="mobile-scroll"><div class="placeholder-list">' +
        '<div class="t-xs t-low" style="margin-bottom:12px;padding:0 4px">MCP 将记忆转为外部行动（Notion、邮箱等）。属于 Later 能力，当前仅保留入口与契约。</div>' +
        items +
        '<button class="btn btn-outline btn-block" style="margin-top:6px" data-action="planning-toast">' + ic('i-plus') + '新建连接</button>' +
      '</div></div>';
    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：工作流
  // ============================================================
  function screenWorkflow() {
    var items = data.workflows.map(function (w) {
      return '<div class="workflow-card">' +
        '<div class="wf-icon">' + ic('i-compass') + '</div>' +
        '<div class="setting-row-main"><div class="setting-row-title">' + esc(w.name) + '</div>' +
        '<div class="setting-row-sub">触发：' + esc(w.trigger) + ' · 步骤：' + esc(w.steps) + '</div></div>' +
        '<span class="badge-planning">规划中</span>' +
      '</div>';
    }).join('');
    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">工作流方案</div>' +
      '</div>' +
      '<div class="mobile-scroll"><div class="placeholder-list">' +
        '<div class="t-xs t-low" style="margin-bottom:12px;padding:0 4px">设计"触发 → 条件 → 步骤 → 输出"的记忆处理方案。本阶段仅预留入口，不提供可用的编辑器。</div>' +
        items +
        '<button class="btn btn-outline btn-block" style="margin-top:6px" data-action="planning-toast">' + ic('i-plus') + '新建方案</button>' +
      '</div></div>';
    return mobileShell('me', content);
  }

  // ============================================================
  // 手机端：文字速记
  // ============================================================
  function screenTextNote() {
    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">文字速记</div>' +
        '<button class="btn btn-primary btn-sm" data-action="save-note">保存</button>' +
      '</div>' +
      '<div class="text-note-view">' +
        (store.state.settings.aiCompletionEnabled
          ? '<div class="completion-hint" data-action="open-modal" data-modal="completion">' + ic('i-sparkle') + '保存后可补全 3 项（标题 / 类型 / 标签）</div>'
          : '') +
        '<textarea class="text-note-area" placeholder="随手记下…" data-input="note" data-focus-key="note"></textarea>' +
        '<div class="t-xs t-low" style="text-align:center;padding-bottom:8px">保存即安全落盘 · ' + (store.state.settings.aiMemoryEnabled ? '将后台整理' : '仅记录（AI 整理已关闭）') + '</div>' +
      '</div>';
    return mobileShell('memories', content);
  }

  // ============================================================
  // 手机端：导入
  // ============================================================
  function screenImport() {
    var content = '' +
      '<div class="appbar">' +
        '<button class="btn-icon" data-action="back">' + ic('i-back') + '</button>' +
        '<div class="appbar-title t-sm">导入</div>' +
      '</div>' +
      '<div class="mobile-scroll"><div class="import-view">' +
        '<div class="import-seg"><div class="segmented">' +
          '<button class="active" data-action="import-tab" data-value="single">单条</button>' +
          '<button data-action="import-tab" data-value="batch">批量</button>' +
        '</div></div>' +
        '<div class="field"><label>内容</label><textarea class="input" placeholder="粘贴一段文字，或描述一条记忆…" data-input="import-single" data-focus-key="import-single"></textarea></div>' +
        '<div class="field"><label>选填：标题</label><input class="input" placeholder="留空则由 AI 建议" data-focus-key="import-title"></div>' +
        '<div class="row gap-8" style="margin-bottom:14px">' +
          '<div class="field" style="flex:1"><label>类型</label><select class="input"><option value="">AI 建议</option>' +
            Object.keys(data.typeLabels).map(function (k) { return '<option value="' + k + '">' + data.typeLabels[k] + '</option>'; }).join('') +
          '</select></div>' +
          '<div class="field" style="flex:1"><label>产生时间</label><input class="input" type="date"></div>' +
        '</div>' +
        '<button class="btn btn-primary btn-block" data-action="import-preview">' + ic('i-sparkle') + 'AI 补全缺失项</button>' +
        '<button class="btn btn-outline btn-block" style="margin-top:8px" data-action="do-import">按原样导入</button>' +
        '<div class="t-xs t-low" style="text-align:center;margin-top:14px">导入最终复用同一捕捉管线，缺时间不伪造、地点不猜测。</div>' +
      '</div></div>';
    return mobileShell('memories', content);
  }

  // ============================================================
  // 手机端：捕捉 Overlay
  // ============================================================
  function screenCapture() {
    var st = store.state.capture;
    var content = '' +
      '<div class="capture-overlay" id="capture-overlay">' +
        '<div class="capture-top">' +
          '<button class="cap-btn" data-action="cancel-record">' + ic('i-close') + '取消</button>' +
          '<button class="cap-btn" data-action="capture-sheet">' + ic('i-arrow-up') + '更多入口</button>' +
        '</div>' +
        '<div class="capture-timer" id="capture-timer">' + store.fmtDuration(st.elapsedMs) + '</div>' +
        '<div class="capture-wave" id="capture-wave"></div>' +
        '<div class="capture-transcript" id="capture-transcript">' +
          (st.transcript ? esc(st.transcript) : '<span class="cap-hint">正在聆听…（原型为模拟转写）</span>') +
        '</div>' +
        '<div class="capture-record-zone">' +
          '<button class="capture-stop-btn" data-action="stop-record">' + ic('i-stop') + '</button>' +
          '<div class="capture-stop-hint">点击停止 · 松开即保存</div>' +
        '</div>' +
        '<div class="capture-sync-bar" id="capture-sync-bar">' +
          '<span class="sync-dot syncing"></span><span>已安全保存 · 正在同步</span>' +
        '</div>' +
      '</div>';
    return content;
  }

  // ---------- 手机端分发 ----------
  function renderMobile() {
    var route = store.state.route;
    var s = route.screen;
    var el = document.getElementById('mobile-screen');
    var html;
    switch (s) {
      case 'search': html = screenSearch(); break;
      case 'detail': html = screenDetail(route.params.id); break;
      case 'echo': html = screenEcho(); break;
      case 'me': html = screenMe(); break;
      case 'ai-settings': html = screenAISettings(); break;
      case 'account': html = screenAccount(); break;
      case 'login': html = screenLogin(); break;
      case 'mcp': html = screenMcp(); break;
      case 'workflow': html = screenWorkflow(); break;
      case 'import': html = screenImport(); break;
      case 'text-note': html = screenTextNote(); break;
      case 'capture': html = screenCapture(); break;
      default: html = screenMemories();
    }
    el.innerHTML = html;
    // 捕捉覆盖层专用：驱动计时/波形
    if (s === 'capture') { WB.capture && WB.capture.onOverlayRendered(); }
  }

  // ---------- 导出 ----------
  WB.render = {
    renderMobile: renderMobile,
    esc: esc,
    ic: ic,
    highlight: highlight,
    cardHTML: cardHTML,
    badgeTypeHTML: badgeTypeHTML,
    badgeStatusHTML: badgeStatusHTML,
    syncDotHTML: syncDotHTML,
    cardContextHTML: cardContextHTML
  };
})(typeof window !== 'undefined' ? window : this);
