/* ============================================================
   织脑原型 — 单一状态仓库 + 派生选择器
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var data = WB.data;

  // ---------- 初始状态 ----------
  var state = {
    device: 'mobile',               // 'mobile' | 'web'
    route: { screen: 'memories', params: {} },

    settings: {                     // 对齐 UserAISettings 契约
      aiMemoryEnabled: false,       // 默认关（spec 硬性要求）
      aiCompletionEnabled: true,    // 补全入口独立于总开关
      sttEnabled: true,             // 云端转写独立开关
      autoSummary: true,
      autoTags: true,
      autoRelation: true,
      autoEcho: true,
      allowCloudAudio: true,
      allowCloudText: true,
      completionMode: 'suggest_only',
      pendingTaskPolicy: 'finish_running_cancel_queued',
      audioRetention: '30_days',
      enabledAt: null,
      consentedAt: null,
      revision: 1,
      updatedAt: null
    },

    user: {
      status: 'guest',              // guest | logged_in
      displayName: '本地游客',
      lastSyncAt: null,
      sessions: [],
      mergedCount: 0
    },

    list: {
      sort: 'recent',               // recent | earliest | updated | frequent
      filters: { types: [], sources: [], status: [], dateRange: null, pinnedOnly: false },
      searchQuery: '',
      activeChip: 'all'
    },

    capture: {
      phase: 'idle',                // idle | requesting_mic | recording | saving | syncing | done | error
      startTs: 0,
      elapsedMs: 0,
      transcript: '',
      offlineSim: false,
      conflictSim: false
    },

    echo: { tab: 'today', feedback: {} },
    toasts: [],
    modal: null,
    _pendingOrganizeCleared: false
  };

  var listeners = [];

  function notify() {
    listeners.forEach(function (fn) { try { fn(state); } catch (e) { /* noop */ } });
  }

  function set(patch) {
    // 深层浅合并：只处理已知顶层键
    Object.keys(patch).forEach(function (k) {
      var v = patch[k];
      if (v && typeof v === 'object' && !Array.isArray(v) && typeof state[k] === 'object' && state[k] && !Array.isArray(state[k])) {
        state[k] = Object.assign({}, state[k], v);
      } else {
        state[k] = v;
      }
    });
    notify();
  }

  function getCard(id) {
    return data.cards.find(function (c) { return c.id === id; }) || null;
  }

  // ---------- 派生选择器 ----------
  // 有效标题：user > ai > 原文回退
  function visibleTitle(card) {
    if (card.user && card.user.titleEdited) return card.user.titleEdited;
    if (card.ai && card.ai.title) return card.ai.title;
    return fallbackTitle(card);
  }
  function fallbackTitle(card) {
    var t = (card.original && card.original.text || '').trim().replace(/\s+/g, ' ');
    if (t.length <= 30) return t;
    return t.slice(0, 30) + '…';
  }

  function effectiveTags(card) {
    if (card.user && card.user.tags && card.user.tags.length) return card.user.tags;
    if (card.ai && card.ai.suggestedTags && card.ai.suggestedTags.length) return card.ai.suggestedTags;
    return card.tags || [];
  }

  function isPinned(card) { return !!card.pinned; }

  // 筛选 + 排序
  function filteredCards() {
    var s = state.list;
    var f = s.filters;
    var q = (s.searchQuery || '').trim().toLowerCase();

    var out = data.cards.filter(function (c) {
      if (c.lifecycle !== 'active') return false;
      if (f.types.length && f.types.indexOf(c.primaryType) === -1) return false;
      if (f.sources.length && f.sources.indexOf(c.original.source) === -1) return false;
      if (f.status.length && f.status.indexOf(c.processingStatus) === -1) return false;
      if (f.pinnedOnly && !c.pinned) return false;
      if (f.dateRange) {
        var t = new Date(c.original.capturedAt || 0).getTime();
        if (f.dateRange.from && t < f.dateRange.from) return false;
        if (f.dateRange.to && t > f.dateRange.to) return false;
      }
      if (q) {
        var hay = (visibleTitle(c) + ' ' + (c.original.text || '') + ' ' + effectiveTags(c).join(' ')).toLowerCase();
        if (hay.indexOf(q) === -1) return false;
      }
      return true;
    });

    var sort = s.sort;
    out.sort(function (a, b) {
      // 置顶优先独立层
      if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
      if (sort === 'earliest') {
        return (a.original.capturedAt || '0') < (b.original.capturedAt || '0') ? -1 : 1;
      }
      if (sort === 'updated') {
        return ((b.stats && b.stats.updatedAt) || '0') < ((a.stats && a.stats.updatedAt) || '0') ? -1 : 1;
      }
      if (sort === 'frequent') {
        var an = (a.stats && a.stats.openCount || 0) + (a.stats && a.stats.continuationCount || 0) * 2;
        var bn = (b.stats && b.stats.openCount || 0) + (b.stats && b.stats.continuationCount || 0) * 2;
        return bn - an;
      }
      // recent 默认
      return (a.original.capturedAt || '0') < (b.original.capturedAt || '0') ? 1 : -1;
    });

    return out;
  }

  // 按日分组（captured_at 真实日期）
  function dayKey(iso) {
    if (!iso) return '时间未知';
    var d = new Date(iso);
    var y = d.getFullYear();
    var m = String(d.getMonth() + 1).padStart(2, '0');
    var day = String(d.getDate()).padStart(2, '0');
    return y + '-' + m + '-' + day;
  }
  function groupByDay(cards) {
    var groups = [];
    var map = {};
    cards.forEach(function (c) {
      var k = dayKey(c.original.capturedAt);
      if (!map[k]) { map[k] = { date: k, cards: [] }; groups.push(map[k]); }
      map[k].cards.push(c);
    });
    return groups;
  }

  // 可补整理的卡片数（AI 未整理、非失败、非 processing）
  function pendingOrganizeCount() {
    return data.cards.filter(function (c) {
      return c.lifecycle === 'active' &&
        c.processingStatus !== 'ready' &&
        c.processingStatus !== 'processing';
    }).length;
  }

  // 回响项目
  function echoItems() {
    return data.echoes;
  }

  // ---------- 工具：时间格式化 ----------
  function fmtTime(iso) {
    if (!iso) return '';
    var d = new Date(iso);
    return String(d.getMonth() + 1).padStart(2, '0') + '-' +
           String(d.getDate()).padStart(2, '0') + ' ' +
           String(d.getHours()).padStart(2, '0') + ':' +
           String(d.getMinutes()).padStart(2, '0');
  }
  function fmtDuration(ms) {
    var s = Math.floor(ms / 1000);
    var m = Math.floor(s / 60);
    return String(m).padStart(2, '0') + ':' + String(s % 60).padStart(2, '0');
  }
  function fmtDayLabel(key) {
    if (key === '时间未知') return key;
    var d = new Date(key + 'T00:00:00');
    var today = new Date();
    today.setHours(0, 0, 0, 0);
    var diff = Math.round((today - d) / 86400000);
    if (diff === 0) return '今天';
    if (diff === 1) return '昨天';
    if (diff === 2) return '前天';
    var m = d.getMonth() + 1;
    return m + '月' + d.getDate() + '日';
  }

  WB.store = {
    state: state,
    set: set,
    getCard: getCard,
    subscribe: function (fn) { listeners.push(fn); },
    // selectors
    visibleTitle: visibleTitle,
    effectiveTags: effectiveTags,
    filteredCards: filteredCards,
    groupByDay: groupByDay,
    pendingOrganizeCount: pendingOrganizeCount,
    echoItems: echoItems,
    // helpers
    fmtTime: fmtTime,
    fmtDuration: fmtDuration,
    fmtDayLabel: fmtDayLabel
  };
})(typeof window !== 'undefined' ? window : this);
