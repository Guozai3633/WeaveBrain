/* ============================================================
   织脑原型 — 应用入口：全局事件委托 / toast / 渲染调度 / 自检
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;
  var data = WB.data;
  var R = WB.render;

  // ---------- Toast ----------
  function toast(msg, kind) {
    var root = document.getElementById('toast-root');
    if (!root) return;
    var el = document.createElement('div');
    el.className = 'toast' + (kind === 'err' ? ' toast-err' : kind === 'warn' ? ' toast-warn' : '');
    el.innerHTML = (kind === 'err'
      ? R.ic('i-close') : kind === 'warn'
        ? R.ic('i-bell') : R.ic('i-check')) + '<span>' + R.esc(msg) + '</span>';
    root.appendChild(el);
    setTimeout(function () {
      el.style.transition = 'opacity .3s, transform .3s';
      el.style.opacity = '0';
      el.style.transform = 'translateY(-10px)';
      setTimeout(function () { el.remove(); }, 320);
    }, 2600);
  }

  // ---------- 渲染调度（含焦点保持） ----------
  function renderAll() {
    var active = document.activeElement;
    var focusKey = active ? active.getAttribute('data-focus-key') : null;
    var isText = active && (active.tagName === 'INPUT' || active.tagName === 'TEXTAREA');
    var selStart = isText ? active.selectionStart : null;
    var selEnd = isText ? active.selectionEnd : null;

    var device = store.state.device;
    var phone = document.getElementById('phone-frame');
    var web = document.getElementById('web-panel');
    phone.setAttribute('data-active', device === 'mobile' ? 'true' : 'false');
    web.setAttribute('data-active', device === 'web' ? 'true' : 'false');

    if (device === 'mobile') R.renderMobile();
    else WB.renderWeb.renderWeb();

    if (focusKey) {
      var el = document.querySelector('[data-focus-key="' + focusKey + '"]');
      if (el) {
        el.focus();
        if (isText && selStart !== null) {
          try { el.setSelectionRange(selStart, selEnd); } catch (e) { /* noop */ }
        }
      }
    }
  }

  // ---------- 数据操作辅助 ----------
  function toggleFilter(key, val) {
    var f = store.state.list.filters;
    var arr = (f[key] || []).slice();
    var idx = arr.indexOf(val);
    if (idx === -1) arr.push(val); else arr.splice(idx, 1);
    store.set({ list: { filters: Object.assign({}, f, (_ = {}, _[key] = arr, _)) } });
    var _;
  }

  // ---------- 长按检测（中央 FAB：点击录音 / 长按弹层） ----------
  var lp = { timer: null, fired: false };
  function onPointerDown(e) {
    var t = e.target.closest('[data-action="start-capture"]');
    if (!t) return;
    lp.fired = false;
    lp.timer = setTimeout(function () {
      lp.fired = true;
      lp.timer = null;
      WB.modals.open('capture-sheet');
    }, 450);
  }
  function onPointerUp(e) {
    if (lp.timer) { clearTimeout(lp.timer); lp.timer = null; }
  }

  // ---------- 全局点击委托 ----------
  function onClick(e) {
    var t = e.target.closest('[data-action]');
    if (!t) return;
    var action = t.getAttribute('data-action');
    var id = t.getAttribute('data-id');
    var screen = t.getAttribute('data-screen');
    var val = t.getAttribute('data-value');

    switch (action) {
      // 设备 / 导航
      case 'switch-device':
        WB.router.switchDevice(t.getAttribute('data-device'));
        break;
      case 'nav':
      case 'go':
        WB.router.go(screen);
        break;
      case 'go-detail':
        WB.router.go('detail', { id: id });
        break;
      case 'web-nav':
        WB.router.go(screen);
        break;
      case 'web-select':
        WB.router.go('memories', { id: id });
        break;
      case 'back':
        global.history.length > 1 ? global.history.back() : WB.router.go('memories');
        break;

      // 捕捉
      case 'start-capture':
        if (lp.fired) { lp.fired = false; break; } // 长按已触发弹层，吞掉随后的 click
        WB.capture.start();
        break;
      case 'capture-sheet':
        WB.modals.close();
        WB.modals.open('capture-sheet');
        break;
      case 'capture-voice':
        WB.capture.start();
        break;
      case 'capture-text':
        WB.capture.startTextNote();
        break;
      case 'go-import':
        WB.modals.close();
        WB.router.go('import');
        break;
      case 'stop-record':
        WB.capture.stop();
        break;
      case 'cancel-record':
        WB.capture.cancel();
        break;

      // 搜索
      case 'search':
        store.set({ list: { searchQuery: val } });
        WB.router.go('search');
        break;
      case 'search-clear':
        store.set({ list: { searchQuery: '' } });
        break;

      // 记忆流
      case 'chip':
        store.set({ list: { activeChip: val, filters: { types: val === 'all' ? [] : [val] } } });
        break;
      case 'sort':
        store.set({ list: { sort: val } });
        WB.modals.close();
        break;
      case 'filter':
        toggleFilter(t.getAttribute('data-key'), val);
        break;
      case 'clear-filters':
        store.set({ list: { filters: { types: [], sources: [], status: [], dateRange: null, pinnedOnly: false }, activeChip: 'all' } });
        WB.modals.close();
        break;

      // 设置开关
      case 'toggle':
        (function () {
          var k = t.getAttribute('data-key');
          var patch = {};
          patch[k] = !store.state.settings[k];
          store.set({ settings: patch });
        })();
        break;
      case 'toggle-master':
        (function () {
          var on = !store.state.settings.aiMemoryEnabled;
          store.set({ settings: { aiMemoryEnabled: on } });
          toast(on ? 'AI 记忆整理已开启 · 新记录将后台整理' : 'AI 记忆整理已关闭 · 纯备忘录模式', on ? 'ok' : 'warn');
        })();
        break;

      // 回响
      case 'echo-tab':
        store.set({ echo: { tab: val } });
        break;
      case 'echo-feedback':
        (function () {
          var eid = t.getAttribute('data-id');
          var fb = t.getAttribute('data-fb');
          var echo = data.echoes.find(function (e) { return e.id === eid; });
          if (!echo) return;
          if (fb === 'unrelated') { echo.processed = true; toast('已标记无关，移入已处理', 'ok'); }
          else if (fb === 'done') { echo.processed = true; toast('已标记完成', 'ok'); }
          else if (fb === 'later') { toast('已稍后提醒', 'warn'); }
          else { toast('继续思考这条记忆', 'ok'); }
          store.set({ echo: { feedback: Object.assign({}, store.state.echo.feedback, (_ = {}, _[eid] = fb, _)) } });
          var _;
        })();
        break;

      // 弹窗
      case 'open-modal':
        WB.modals.open(t.getAttribute('data-modal'), { id: id, cardId: id });
        break;
      case 'close-modal':
        WB.modals.close();
        break;
      case 'no-close':
        break;
      case 'confirm-yes':
        WB.modals.resolveConfirm(true);
        break;
      case 'confirm-no':
        WB.modals.resolveConfirm(false);
        break;

      // 卡片操作
      case 'pin-toggle':
        (function () {
          var c = store.getCard(id);
          if (!c) return;
          c.pinned = !c.pinned;
          WB.modals.close();
          store.set({});
          toast(c.pinned ? '已置顶' : '已取消置顶', 'ok');
        })();
        break;
      case 'delete-card':
        WB.modals.confirm('删除这条记忆？', '会先进入回收站，保留 30 天。', function () {
          var c = store.getCard(id);
          if (c) c.lifecycle = 'deleted';
          store.set({});
          toast('已移入回收站', 'ok');
          WB.router.go('memories');
        });
        break;
      case 'set-type':
        (function () {
          var c = store.getCard(id);
          if (!c) return;
          c.primaryType = val;
          WB.modals.close();
          store.set({});
          toast('类型已更新', 'ok');
        })();
        break;

      // 提醒
      case 'save-reminder':
        (function () {
          var dt = document.querySelector('[data-focus-key="reminder-dt"]');
          var note = document.querySelector('[data-focus-key="reminder-note"]');
          var card = store.getCard(id);
          if (!card) return;
          var iso = dt && dt.value ? new Date(dt.value).toISOString() : new Date(Date.now() + 86400000).toISOString();
          card.reminder = { triggerAt: iso, reason: note ? note.value : '', status: 'active' };
          WB.modals.close();
          store.set({});
          toast('提醒已设置 · 这是你的承诺', 'ok');
        })();
        break;
      case 'reminder-preset':
        (function () {
          var days = parseInt(t.getAttribute('data-days'), 10);
          var card = store.getCard(id);
          if (!card) return;
          var d = new Date(Date.now() + days * 86400000);
          d.setHours(days === 1 ? 21 : 9, 0, 0, 0);
          card.reminder = { triggerAt: d.toISOString(), reason: '', status: 'active' };
          WB.modals.close();
          store.set({});
          toast('提醒已设置', 'ok');
        })();
        break;

      // 转写 / 续写
      case 'save-transcript':
        (function () {
          var inp = document.querySelector('[data-input="transcript"]');
          var c = store.getCard(id);
          if (c && c.original.transcript) {
            c.original.transcript.userCorrected = inp ? inp.value : '';
          }
          WB.modals.close();
          store.set({});
          toast('转写修正已保存 · 优先级最高', 'ok');
        })();
        break;
      case 'continue-append':
      case 'continue-new':
        (function () {
          var inp = document.querySelector('[data-input="continue"]');
          var txt = inp ? inp.value.trim() : '';
          if (!txt) { toast('先写点什么吧', 'warn'); return; }
          var c = store.getCard(id);
          if (c && action === 'continue-append') {
            c.original.text = c.original.text + '\n\n' + txt;
            c.stats.continuationCount = (c.stats.continuationCount || 0) + 1;
          } else if (action === 'continue-new') {
            var newCard = WB.capture.createTextCard(txt, 'text');
            newCard.original.text = txt;
            newCard.stats.continuationCount = 1;
          }
          WB.modals.close();
          store.set({});
          toast(action === 'continue-append' ? '已追加为当前片段' : '已生成独立记忆', 'ok');
        })();
        break;
      case 'continue-voice':
        WB.modals.close();
        WB.capture.start();
        break;

      // AI 补全
      case 'proposal-accept':
        toast('已接受建议', 'ok');
        break;
      case 'proposal-reject':
        t.textContent = '已忽略';
        t.disabled = true;
        break;
      case 'proposal-accept-all':
        toast('已全部采用安全字段', 'ok');
        break;
      case 'proposal-apply':
        WB.modals.close();
        toast('已应用所选补全 · 可撤销', 'ok');
        break;

      // 登录 / 数据
      case 'do-login':
        store.set({ user: { status: 'logged_in', displayName: '织脑用户', lastSyncAt: new Date().toISOString(), sessions: ['本机 · Windows', '手机 · Android'] } });
        WB.router.go('me');
        toast('登录成功 · 本地记录已合并同步', 'ok');
        break;
      case 'logout':
        store.set({ user: { status: 'guest', displayName: '本地游客', lastSyncAt: null, sessions: [], mergedCount: 0 } });
        toast('已退出登录', 'warn');
        break;
      case 'export-all':
        toast('已生成 JSON 备份（原型演示）', 'ok');
        break;
      case 'retry-sync':
        (function () {
          var c = store.getCard(id);
          if (c) c.sync.state = 'syncing';
          store.set({});
          setTimeout(function () {
            var cc = store.getCard(id);
            if (cc) { cc.sync.state = 'synced'; store.set({}); toast('重试成功 · 已同步', 'ok'); }
          }, 1400);
          toast('正在重试同步…', 'warn');
        })();
        break;

      // AI 整理
      case 'organize-once':
        WB.modals.confirm('仅整理这一条？', 'AI 记忆整理总开关当前关闭，此操作只对这一条记录生效。', function () {
          organizeCard(id, true);
        });
        break;
      case 'organize-retry':
        organizeCard(id, false);
        break;
      case 'organize-batch':
        (function () {
          var pending = data.cards.filter(function (c) { return c.lifecycle === 'active' && c.processingStatus !== 'ready' && c.processingStatus !== 'processing'; });
          if (!pending.length) { toast('没有待整理的记忆', 'warn'); return; }
          toast('开始补整理 ' + pending.length + ' 条…', 'ok');
          pending.forEach(function (c, i) {
            setTimeout(function () { organizeCard(c.id, true); }, 600 * (i + 1));
          });
        })();
        break;

      // 导入
      case 'import-preview':
        WB.modals.open('completion', { cardId: id });
        break;
      case 'do-import':
      case 'do-import-batch':
        (function () {
          var inp = document.querySelector('[data-input="import-single"], [data-input="import-batch"]');
          var txt = inp ? inp.value.trim() : '';
          if (!txt) { toast('先填写内容', 'warn'); return; }
          WB.capture.createTextCard(txt, 'import');
          store.set({});
          toast('已导入 · ' + (store.state.settings.aiMemoryEnabled ? '将后台整理' : '仅记录'), 'ok');
          WB.router.go('memories');
        })();
        break;
      case 'import-parse':
        toast('已解析 3 条 · 1 条疑似重复', 'ok');
        break;
      case 'import-completion':
        WB.modals.open('completion', {});
        break;
      case 'import-tab':
        toast(val === 'batch' ? '批量导入（原型演示预览）' : '单条导入', 'warn');
        break;

      // Web 筛选
      case 'web-filter-sort':
        store.set({ list: { sort: t.value } });
        break;
      case 'web-filter-type':
        store.set({ list: { filters: { types: t.value ? [t.value] : [] } } });
        break;
      case 'web-search-go':
        store.set({ list: { searchQuery: (document.querySelector('.web-topbar .search-box input') || {}).value || '' } });
        WB.router.go('search');
        break;

      // 占位 / 通用
      case 'planning-toast':
        toast('功能规划中，尚未开放', 'warn');
        break;
      case 'toast':
        toast('原型演示操作', 'ok');
        break;
      case 'play-audio':
        t.classList.toggle('playing');
        break;
    }
  }

  // ---------- 全局输入委托 ----------
  function onInput(e) {
    var el = e.target;
    var key = el.getAttribute('data-input');
    if (!key) return;
    if (key === 'search') {
      store.set({ list: { searchQuery: el.value } });
    }
    // 其余输入在提交动作时读取，避免每键重渲染丢失焦点
  }

  // ---------- 标题编辑（change/blur 提交） ----------
  function onChange(e) {
    var el = e.target;
    if (el.getAttribute('data-input') === 'title') {
      var id = el.getAttribute('data-id');
      var c = store.getCard(id);
      if (c) {
        if (!c.user) c.user = {};
        c.user.titleEdited = el.value.trim() || null;
        store.set({});
      }
    }
  }

  // ---------- AI 整理卡片 ----------
  function organizeCard(id, silent) {
    var c = store.getCard(id);
    if (!c) return;
    c.processingStatus = 'processing';
    if (!c.ai) c.ai = {};
    c.ai = {
      title: store.visibleTitle(c).length > 18 ? store.visibleTitle(c).slice(0, 18) + '…' : store.visibleTitle(c),
      essence: c.original.text.slice(0, 60),
      keyPoints: ['自动整理完成', '可删除 / 可重做 / 可回退', '原始记录不受影响'],
      openQuestion: null,
      nextStep: '回看这条记忆',
      suggestedTags: ['已整理'],
      suggestedType: c.primaryType,
      status: 'ready', provenance: 'ai', revision: 1
    };
    store.set({});
    setTimeout(function () {
      var cc = store.getCard(id);
      if (!cc) return;
      cc.processingStatus = 'ready';
      store.set({});
      if (!silent) toast('AI 整理完成', 'ok');
    }, 1300);
  }

  // ---------- 设备切换按钮同步 ----------
  function syncDeviceButtons(device) {
    var btns = document.querySelectorAll('.ds-btn[data-device]');
    btns.forEach(function (b) {
      b.classList.toggle('active', b.getAttribute('data-device') === device);
    });
  }

  // ---------- 状态栏时钟 ----------
  function updateClock() {
    var el = document.getElementById('phone-statusbar');
    if (el) {
      var d = new Date();
      el.innerHTML = String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0') +
        '<span class="sb-icons">' + R.ic('i-wave') + R.ic('i-mic') + '</span>';
    }
  }

  // ---------- 键盘快捷键 ----------
  function onKey(e) {
    if (e.ctrlKey && e.key === 'Tab') {
      e.preventDefault();
      WB.router.switchDevice(store.state.device === 'mobile' ? 'web' : 'mobile');
      return;
    }
    if (e.target && /INPUT|TEXTAREA|SELECT/.test(e.target.tagName)) return;
    if (e.key === 'Escape') {
      WB.modals.close();
      return;
    }
    if (e.key === 'r' || e.key === 'R') {
      if (store.state.device === 'mobile') WB.capture.start();
    }
    if (e.key === '1') WB.router.go('memories');
    if (e.key === '2') WB.router.go('echo');
    if (e.key === '3') WB.router.go('me');
  }

  // ---------- 自检 ----------
  function selfCheck() {
    var ok = true;
    var problems = [];
    if (!data.cards || data.cards.length !== 14) { ok = false; problems.push('记忆卡数量应为 14，实际 ' + (data.cards && data.cards.length)); }
    var types = Object.keys(data.typeLabels);
    var typeOk = data.cards.every(function (c) { return types.indexOf(c.primaryType) !== -1; });
    if (!typeOk) { ok = false; problems.push('存在非法类型'); }
    if (store.state.settings.aiMemoryEnabled !== false) { ok = false; problems.push('AI 总开关应默认关闭'); }
    var syncs = ['saved_local', 'pending_sync', 'syncing', 'synced', 'retryable_error', 'conflict', 'rejected'];
    data.cards.forEach(function (c) {
      if (syncs.indexOf(c.sync.state) === -1) { ok = false; problems.push('非法同步状态 ' + c.sync.state); }
    });
    console.log(ok ? '%c✓ WeaveDemo self-check passed' : '%c✗ WeaveDemo self-check failed', ok ? 'color:#10B981;font-weight:bold' : 'color:#EF4444;font-weight:bold');
    if (!ok) console.warn('问题：', problems);
  }

  // ---------- 初始化 ----------
  function init() {
    // 事件绑定
    document.addEventListener('pointerdown', onPointerDown, true);
    document.addEventListener('pointerup', onPointerUp, true);
    document.addEventListener('pointercancel', onPointerUp, true);
    document.addEventListener('pointerleave', onPointerUp, true);
    document.addEventListener('click', onClick);
    document.addEventListener('input', onInput);
    document.addEventListener('change', onChange);
    document.addEventListener('keydown', onKey);

    // 路由初始解析 + 首次渲染
    WB.router.apply();
    renderAll();
    updateClock();
    setInterval(updateClock, 30000);
    selfCheck();
  }

  // 订阅 store：状态变化即重渲染（capture 计时用 rAF 直改 DOM，不走 store）
  store.subscribe(function () { renderAll(); });

  WB.app = {
    toast: toast,
    renderAll: renderAll,
    syncDeviceButtons: syncDeviceButtons,
    init: init
  };

  // 自动启动
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})(typeof window !== 'undefined' ? window : this);
