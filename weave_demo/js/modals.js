/* ============================================================
   织脑原型 — 弹窗系统（底部弹层 / 居中弹窗）
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;
  var data = WB.data;
  var R = WB.render;

  var rootEl = null;
  var confirmCb = null;

  // ---------- 基础 ----------
  function sheet(inner) {
    return '<div class="modal-mask" data-action="close-modal"><div class="sheet" data-action="no-close">' +
      '<div class="sheet-grabber"></div>' + inner + '</div></div>';
  }
  function dialog(inner, extraCls) {
    return '<div class="modal-mask modal-center" data-action="close-modal"><div class="dialog' + (extraCls ? ' ' + extraCls : '') + '" data-action="no-close">' +
      inner + '</div></div>';
  }

  function renderModal(type, d) {
    d = d || {};
    switch (type) {
      case 'capture-sheet': return sheet(captureSheetHTML());
      case 'more': return sheet(moreMenuHTML(d.id));
      case 'filter-sort': return sheet(filterSortHTML());
      case 'reminder': return sheet(reminderHTML(d.id));
      case 'type': return sheet(typeHTML(d.id));
      case 'transcript': return sheet(transcriptHTML(d.id));
      case 'continue-text': return sheet(continueHTML(d.id));
      case 'completion': return sheet(completionHTML(d.cardId));
      case 'trash': return dialog(trashHTML());
      case 'delete-account': return dialog(deleteAccountHTML());
      case 'conflict': return dialog(conflictHTML(d.cardId));
      case 'confirm': return dialog(confirmHTML(d.title, d.sub));
      default: return '';
    }
  }

  // ---------- 捕捉入口弹层 ----------
  function captureSheetHTML() {
    var opts = [
      { a: 'capture-voice', i: 'i-mic', t: '语音记录', s: '点击即录' },
      { a: 'capture-text', i: 'i-text', t: '文字速记', s: '2 次完成' },
      { a: 'go-import', i: 'i-import', t: '单条导入', s: '粘贴/上传' },
      { a: 'go-import', i: 'i-download', t: '批量导入', s: 'CSV/JSONL' }
    ];
    var items = opts.map(function (o) {
      return '<button class="capture-entry-item" data-action="' + o.a + '">' + R.ic(o.i) +
        '<span>' + o.t + '</span><small>' + o.s + '</small></button>';
    }).join('');
    return '<div class="t-lg t-weight-600" style="margin-bottom:14px">快速捕捉</div>' +
      '<div class="capture-entry-options">' + items + '</div>';
  }

  // ---------- 卡片更多菜单 ----------
  function moreMenuHTML(id) {
    var card = store.getCard(id);
    if (!card) return '';
    var actions = [
      { a: 'pin-toggle', i: 'i-pin', t: card.pinned ? '取消置顶' : '置顶' },
      { a: 'toast', i: 'i-archive', t: '归档' },
      { a: 'toast', i: 'i-export', t: '导出/分享' },
      { a: 'toast', i: 'i-edit', t: '标记待补充' },
      { a: 'delete-card', i: 'i-trash', t: '删除（进入回收站）', danger: true }
    ];
    var rows = actions.map(function (x) {
      return '<div class="list-item" data-action="' + x.a + '" data-id="' + id + '"' +
        (x.danger ? ' style="color:var(--err)"' : '') + '>' +
        '<div class="list-item-icon" ' + (x.danger ? 'style="background:#FEF2F2;color:var(--err)"' : '') + '>' + R.ic(x.i) + '</div>' +
        '<div class="list-item-main"><div class="list-item-title">' + x.t + '</div></div></div>';
    }).join('');
    return '<div class="t-lg t-weight-600" style="margin-bottom:8px">更多操作</div>' + rows;
  }

  // ---------- 筛选/排序面板 ----------
  function filterSortHTML() {
    var list = store.state.list;
    var sorts = [
      { v: 'recent', t: '最近记录', sub: '默认，按 captured_at 倒序' },
      { v: 'earliest', t: '最早记录', sub: '按 captured_at 正序' },
      { v: 'updated', t: '最近更新', sub: '续写/编辑/提醒' },
      { v: 'frequent', t: '常用记忆', sub: '按有效行为聚合' }
    ];
    var sortRows = sorts.map(function (s) {
      var on = list.sort === s.v;
      return '<div class="list-item" data-action="sort" data-value="' + s.v + '">' +
        '<div class="list-item-main"><div class="list-item-title">' + s.t + '</div>' +
        '<div class="list-item-sub">' + s.sub + '</div></div>' +
        (on ? R.ic('i-check', '') + '<span style="color:var(--primary)"></span>' : '') +
        (on ? '<svg class="icon" style="color:var(--primary)"><use href="#i-check"/></svg>' : '') +
      '</div>';
    }).join('');

    function filterChip(key, val, label) {
      var arr = list.filters[key] || [];
      var on = arr.indexOf(val) !== -1;
      return '<button class="chip' + (on ? ' chip-active' : '') + '" data-action="filter" data-key="' + key + '" data-value="' + val + '">' + label + '</button>';
    }

    return '' +
      '<div class="t-lg t-weight-600" style="margin-bottom:4px">排序</div>' + sortRows +
      '<div class="t-sm t-weight-600 t-mid" style="margin:14px 0 8px">筛选 · 类型</div>' +
      '<div class="row gap-8" style="flex-wrap:wrap;margin-bottom:6px">' +
        Object.keys(data.typeLabels).map(function (k) { return filterChip('types', k, data.typeLabels[k]); }).join('') +
      '</div>' +
      '<div class="t-sm t-weight-600 t-mid" style="margin:10px 0 8px">筛选 · 处理状态</div>' +
      '<div class="row gap-8" style="flex-wrap:wrap;margin-bottom:6px">' +
        filterChip('status', 'ready', '已整理') + filterChip('status', 'processing', '整理中') +
        filterChip('status', 'needs_input', '待补充') + filterChip('status', 'failed', '失败') +
      '</div>' +
      '<div class="row gap-8" style="flex-wrap:wrap;margin-bottom:6px">' +
        filterChip('sources', 'app', '语音') + filterChip('sources', 'text', '文字') + filterChip('sources', 'import', '导入') +
      '</div>' +
      '<div class="row gap-8" style="margin-top:12px">' +
        '<button class="btn btn-outline btn-sm" data-action="clear-filters">清除筛选</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-primary btn-sm" data-action="close-modal">完成</button>' +
      '</div>';
  }

  // ---------- 提醒 ----------
  function reminderHTML(id) {
    var card = store.getCard(id);
    if (!card) return '';
    var presets = [
      { t: '今晚 21:00', d: 1 },
      { t: '明天 09:00', d: 2 },
      { t: '本周五 18:00', d: 5 },
      { t: '下周', d: 7 }
    ];
    var chips = presets.map(function (p, i) {
      return '<button class="chip" data-action="reminder-preset" data-days="' + p.d + '" data-id="' + id + '" data-testid="reminder-preset-' + i + '">' + p.t + '</button>';
    }).join('');
    return '' +
      '<div class="t-lg t-weight-600">设置提醒</div>' +
      '<div class="t-xs t-low" style="margin:4px 0 12px">提醒是<strong>你的承诺</strong>，与系统建议的"回响"不同。</div>' +
      '<div class="row gap-8" style="flex-wrap:wrap;margin-bottom:14px">' + chips + '</div>' +
      '<div class="field"><label>自定义时间</label><input class="input" type="datetime-local" data-focus-key="reminder-dt"></div>' +
      '<div class="field"><label>提醒原因（可选）</label><input class="input" placeholder="例如：明天开始实现" data-focus-key="reminder-note"></div>' +
      '<button class="btn btn-primary btn-block" data-action="save-reminder" data-id="' + id + '">保存提醒</button>';
  }

  // ---------- 改类型 ----------
  function typeHTML(id) {
    var card = store.getCard(id);
    if (!card) return '';
    var opts = Object.keys(data.typeLabels).map(function (k) {
      var on = card.primaryType === k;
      return '<button class="chip' + (on ? ' chip-active' : '') + '" data-action="set-type" data-id="' + id + '" data-value="' + k + '">' + data.typeLabels[k] + '</button>';
    }).join('');
    return '<div class="t-lg t-weight-600" style="margin-bottom:12px">修改记忆类型</div>' +
      '<div class="row gap-8" style="flex-wrap:wrap">' + opts + '</div>' +
      '<div class="t-xs t-low" style="margin-top:12px">类型不是文件夹，一条记忆只有一个主要类型。</div>';
  }

  // ---------- 修正转写 ----------
  function transcriptHTML(id) {
    var card = store.getCard(id);
    if (!card) return '';
    var cur = (card.original.transcript && (card.original.transcript.userCorrected || card.original.transcript.clean)) || '';
    return '<div class="t-lg t-weight-600" style="margin-bottom:4px">修正转写</div>' +
      '<div class="t-xs t-low" style="margin-bottom:12px">你的修正版优先级最高，AI 不会覆盖它。</div>' +
      '<textarea class="input" rows="4" data-input="transcript" data-focus-key="transcript">' + R.esc(cur) + '</textarea>' +
      '<button class="btn btn-primary btn-block" style="margin-top:10px" data-action="save-transcript" data-id="' + id + '">保存修正</button>';
  }

  // ---------- 继续思考 ----------
  function continueHTML(id) {
    return '' +
      '<div class="t-lg t-weight-600" style="margin-bottom:4px">继续思考</div>' +
      '<div class="t-xs t-low" style="margin-bottom:12px">文字续写会追加为当前记忆的新片段，也可选择生成独立记忆。</div>' +
      '<textarea class="input" rows="4" placeholder="接着往下想…" data-input="continue" data-focus-key="continue"></textarea>' +
      '<div class="row gap-8" style="margin-top:10px">' +
        '<button class="btn btn-soft btn-sm" data-action="continue-append" data-id="' + id + '">追加为片段</button>' +
        '<button class="btn btn-outline btn-sm" data-action="continue-new" data-id="' + id + '">生成独立记忆</button>' +
      '</div>';
  }

  // ---------- AI 补全预览 ----------
  function completionHTML(cardId) {
    var fields = [
      { f: '标题', orig: '（空）', sug: '记忆卡要保留继续思考的入口', ev: '来自原文首句', safe: true },
      { f: '类型', orig: '（空）', sug: '闪念', ev: '原文为创意、突然想到', safe: true },
      { f: '标签', orig: '（空）', sug: '产品 · 灵感', ev: '与历史标签相似', safe: true },
      { f: '产生时间', orig: '未知', sug: '2026-08-27（估算）', ev: '文本无明确时间', safe: false, note: '仅建议 · 估算' }
    ];
    var rows = fields.map(function (f) {
      return '<div class="proposal-field">' +
        '<div class="proposal-field-head"><span>' + f.f + '</span>' +
          (f.note ? '<span class="badge badge-st st-needs">' + f.note + '</span>' : '') +
          (f.safe ? '<span class="badge-planning" style="margin-left:auto">安全字段</span>' : '') +
          '<span class="spacer"></span>' +
          '<button class="btn btn-soft btn-sm" data-action="proposal-accept" data-id="' + cardId + '">接受</button>' +
          '<button class="btn btn-outline btn-sm" data-action="proposal-reject">忽略</button>' +
        '</div>' +
        '<div class="proposal-body">' +
          '<div class="t-xs t-low">原值：' + f.orig + '</div>' +
          '<div class="t-sm t-weight-600" style="margin:4px 0">AI 建议：' + f.sug + '</div>' +
          '<div class="proposal-evidence">证据：' + f.ev + '</div>' +
        '</div>' +
      '</div>';
    }).join('');
    return '' +
      '<div class="t-lg t-weight-600">AI 补全 · 可补全 4 项</div>' +
      '<div class="t-xs t-low" style="margin:4px 0 12px">AI 默认只填空字段，不覆盖你填的值或导入原值；事实类建议必须有证据。</div>' +
      rows +
      '<div class="proposal-protected">' + R.ic('i-lock') + '已填字段受保护，不会被预览或应用覆盖</div>' +
      '<div class="row gap-8" style="margin-top:12px">' +
        '<button class="btn btn-outline btn-sm" data-action="proposal-accept-all" data-id="' + cardId + '">全部采用安全字段</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-primary btn-sm" data-action="proposal-apply" data-id="' + cardId + '">应用所选</button>' +
      '</div>';
  }

  // ---------- 回收站 / 删除账号 / 冲突 ----------
  function trashHTML() {
    return '' +
      '<div class="dialog-title">回收站</div>' +
      '<div class="dialog-sub">删除的记忆进入回收站保留 30 天，到期后永久清理。永久删除会级联删除音频、转写、向量和派生内容。</div>' +
      '<div class="row gap-8">' +
        '<button class="btn btn-outline btn-sm" data-action="close-modal">关闭</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-danger btn-sm" data-action="toast">清空回收站</button>' +
      '</div>';
  }
  function deleteAccountHTML() {
    return '' +
      '<div class="dialog-title" style="color:var(--err)">删除账号</div>' +
      '<div class="dialog-sub">此操作会永久删除账号、所有记忆、音频、转写、向量和关联。删除后不可恢复。确定要继续吗？</div>' +
      '<div class="row gap-8">' +
        '<button class="btn btn-outline btn-sm" data-action="close-modal">取消</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-danger btn-sm" data-action="toast">确认删除</button>' +
      '</div>';
  }
  function conflictHTML(cardId) {
    return '' +
      '<div class="dialog-title" style="color:var(--err)">同步冲突</div>' +
      '<div class="dialog-sub">本机与服务端版本不一致，为防静默覆盖，已保留两个版本。请选择保留哪个。</div>' +
      '<div class="row gap-8">' +
        '<button class="btn btn-outline btn-sm" data-action="close-modal">保留本机版</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-primary btn-sm" data-action="close-modal">保留服务端版</button>' +
      '</div>';
  }
  function confirmHTML(title, sub) {
    return '<div class="dialog dialog-sm">' +
      '<div class="dialog-title">' + R.esc(title || '确认操作') + '</div>' +
      (sub ? '<div class="dialog-sub">' + R.esc(sub) + '</div>' : '') +
      '<div class="row gap-8">' +
        '<button class="btn btn-outline btn-sm" data-action="confirm-no">取消</button>' +
        '<span class="spacer"></span>' +
        '<button class="btn btn-danger btn-sm" data-action="confirm-yes">确定</button>' +
      '</div></div>';
  }

  // ---------- 对外接口 ----------
  function open(type, data) {
    if (!rootEl) rootEl = document.getElementById('modal-root');
    rootEl.innerHTML = renderModal(type, data);
  }
  function close() {
    if (!rootEl) rootEl = document.getElementById('modal-root');
    rootEl.innerHTML = '';
    confirmCb = null;
  }
  function isOpen() {
    if (!rootEl) rootEl = document.getElementById('modal-root');
    return !!rootEl.innerHTML;
  }
  function confirm(title, sub, cb) {
    confirmCb = cb;
    open('confirm', { title: title, sub: sub });
  }
  function resolveConfirm(yes) {
    var cb = confirmCb;
    confirmCb = null;
    close();
    if (yes && cb) cb();
  }

  WB.modals = {
    open: open,
    close: close,
    isOpen: isOpen,
    confirm: confirm,
    resolveConfirm: resolveConfirm
  };
})(typeof window !== 'undefined' ? window : this);
