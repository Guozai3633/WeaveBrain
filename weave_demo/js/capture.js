/* ============================================================
   织脑原型 — 录音模拟状态机
   idle → requesting_mic → recording → saving → syncing → done
   停止后首文案固定「已安全保存」，随后播放同步流转。
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;
  var data = WB.data;

  var _raf = null;
  var _timer = null;
  var _wave = null;         // 波形 DOM
  var _startStamp = 0;      // recording 开始时间戳
  var _phraseIdx = 0;

  var SIM_PHRASES = [
    '今天想到一个点：记忆不应该是文件夹，而应该是可以继续思考的入口。',
    '就像把灵感交给未来的自己，如果只有一句话摘要，就丢掉了当时的语境。',
    '下一步：回看原型确认交互，然后直接开始实现。'
  ];

  var TXT_PLACEHOLDERS = [
    '「先记再想」——先安全落盘，再决定怎么处理。',
    '给每张卡一个「为什么出现」：让记忆流成为思考的历史。',
    '工具要适配生活节奏，而不是反过来。'
  ];

  function fmt(ms) { return store.fmtDuration(ms); }

  // ---------- 入口 ----------
  function start() {
    WB.modals.close();
    store.set({ capture: { phase: 'requesting_mic', elapsedMs: 0, transcript: '', startTs: 0 } });
    WB.router.go('capture');
    // 0.4s 模拟请求麦克风
    _timer = setTimeout(function () {
      store.set({ capture: { phase: 'recording', startTs: Date.now() } });
    }, 400);
  }

  function startTextNote() {
    WB.modals.close();
    store.set({ capture: { phase: 'idle' } });
    WB.router.go('text-note');
  }

  // ---------- 停止录音 → 保存 → 同步 ----------
  function stop() {
    if (store.state.capture.phase !== 'recording') return;
    _stopRaf();
    var elapsed = Date.now() - _startStamp;
    var transcript = SIM_PHRASES[_phraseIdx % SIM_PHRASES.length];
    store.set({ capture: { phase: 'saving', elapsedMs: elapsed, transcript: transcript } });
    // 0.6s 模拟落盘
    _timer = setTimeout(function () {
      _createCard(transcript, 'app', elapsed);
      store.set({ capture: { phase: 'syncing' } });
    }, 600);
  }

  // ---------- 创建新记忆卡（由 stop / text note / import 共用） ----------
  function createTextCard(text, source) {
    var now = new Date();
    var iso = now.toISOString();
    var id = 'mem-new-' + Date.now().toString(36);
    var aiEnabled = store.state.settings.aiMemoryEnabled;
    var card = {
      id: id,
      primaryType: 'idea',
      original: {
        text: text,
        kind: source === 'text' ? 'text' : 'audio',
        capturedAt: iso,
        capturedAtPrecision: 'exact',
        source: source,
        timezone: 'Asia/Shanghai',
        audio: source === 'app' ? { durationMs: store.state.capture.elapsedMs || 15000, hasWaveform: true } : null,
        transcript: source === 'app'
          ? { clean: text, userCorrected: null }
          : (text.length > 40 ? { clean: text, userCorrected: null } : null),
        context: { activity: null, locationName: null }
      },
      ai: null,
      user: { titleEdited: null, typeEdited: null, tags: null },
      processingStatus: aiEnabled ? 'processing' : 'needs_input',
      pinned: false, lifecycle: 'active',
      sync: { state: store.state.capture.offlineSim ? 'retryable_error' : 'synced' },
      relations: [],
      reminder: null,
      tags: [],
      stats: { openCount: 0, continuationCount: 0, relationCount: 0, lastOpenedAt: null, updatedAt: iso }
    };
    if (aiEnabled) {
      // AI 整理开启：先 processing，1.5s 后填充 AI 字段
      card.ai = {
        title: null, essence: null, keyPoints: [], openQuestion: null, nextStep: null,
        suggestedTags: [], suggestedType: 'idea', status: 'processing', provenance: 'ai', revision: 0
      };
      data.cards.unshift(card);
      setTimeout(function () {
        var c = store.getCard(id);
        if (!c || c.processingStatus !== 'processing') return;
        c.processingStatus = 'ready';
        c.ai.title = text.length > 18 ? text.slice(0, 18) + '…' : text;
        c.ai.essence = text;
        c.ai.keyPoints = [text.slice(0, 20) + '…', '可继续思考与续写', '保存即安全落盘'];
        c.ai.nextStep = '回看这条新记忆';
        c.ai.suggestedTags = ['新记录'];
        c.ai.suggestedType = 'idea';
        c.ai.status = 'ready';
        c.ai.revision = 1;
        store.set({}); // 触发重渲染
        WB.app.toast('AI 整理完成', 'ok');
      }, 1500);
    } else {
      // AI 关闭：纯记录
      card.ai = null;
      data.cards.unshift(card);
    }
    return card;
  }
  function _createCard(text, source, elapsed) {
    store.set({ capture: { elapsedMs: elapsed } });
    var card = createTextCard(text, source);
    if (store.state.capture.conflictSim) {
      WB.app.toast('检测到同步冲突 · 保留两版', 'err');
      store.set({ capture: { phase: 'done' } });
      WB.modals.open('conflict', { cardId: card.id });
      return;
    }
    _simulateSync(card.id);
  }

  // ---------- 同步模拟 ----------
  function _simulateSync(cardId) {
    var offline = store.state.capture.offlineSim;
    var syncBar = document.getElementById('capture-sync-bar');
    if (syncBar) {
      if (offline) {
        syncBar.innerHTML = '<span class="sync-dot retryable_error"></span><span>已安全保存 · 网络不可用，等待重试</span>';
      } else {
        syncBar.innerHTML = '<span class="sync-dot syncing"></span><span>已安全保存 · 正在同步…</span>';
      }
    }
    store.set({ capture: { phase: 'syncing' } });
    if (offline) {
      _timer = setTimeout(function () {
        var c = store.getCard(cardId);
        if (c) { c.sync.state = 'retryable_error'; }
        WB.app.toast('已安全保存 · 离线，将稍后同步', 'warn');
        store.set({ capture: { phase: 'done' } });
        _finishToMemories();
      }, 1500);
    } else {
      _timer = setTimeout(function () {
        var c = store.getCard(cardId);
        if (c) { c.sync.state = 'synced'; }
        WB.app.toast('已安全保存 · 已同步', 'ok');
        store.set({ capture: { phase: 'done' } });
        _finishToMemories();
      }, 1600);
    }
  }

  function _finishToMemories() {
    // 短暂停留展示"已安全保存"后回到记忆流
    _timer = setTimeout(function () {
      store.set({ capture: { phase: 'idle', offlineSim: false, conflictSim: false } });
      WB.router.go('memories');
    }, 900);
  }

  // ---------- 取消 ----------
  function cancel() {
    WB.modals.confirm('放弃这段录音？', '取消后本次内容不会保存。', function () {
      _stopRaf();
      store.set({ capture: { phase: 'idle', elapsedMs: 0, transcript: '' } });
      WB.router.go('memories');
    });
  }

  // ---------- 覆盖层渲染后钩子 ----------
  function onOverlayRendered() {
    var st = store.state.capture;
    var phase = st.phase;
    if (phase === 'requesting_mic') return; // start() 已调度
    if (phase === 'recording') {
      _startRaf();
      return;
    }
    if (phase === 'saving') {
      // 保存中：显示波形静止 + 转写
      var tb = document.getElementById('capture-sync-bar');
      if (tb) tb.innerHTML = '<span class="sync-dot saved_local"></span><span>已安全保存到本机</span>';
      return;
    }
    if (phase === 'syncing') {
      // 已在 _simulateSync 中设置
      return;
    }
    if (phase === 'done') {
      // 已 toast，等待返回
      return;
    }
  }

  // ---------- rAF 驱动（计时 + 波形 + 转写渐进） ----------
  function _startRaf() {
    _stopRaf();
    _startStamp = Date.now();
    _phraseIdx = Math.floor(Math.random() * SIM_PHRASES.length);
    var waveEl = document.getElementById('capture-wave');
    var timerEl = document.getElementById('capture-timer');
    var trEl = document.getElementById('capture-transcript');

    if (waveEl && !waveEl.querySelector('.bar')) {
      for (var i = 0; i < 40; i++) {
        var b = document.createElement('span');
        b.className = 'bar';
        b.style.height = '20%';
        waveEl.appendChild(b);
      }
    }

    var bars = waveEl ? waveEl.querySelectorAll('.bar') : [];

    function frame() {
      var st = store.state.capture;
      if (st.phase !== 'recording') { _stopRaf(); return; }
      var elapsed = Date.now() - _startStamp;
      // 每帧只直改 DOM（计时/波形/转写），不走 store.set，避免触发全量重渲染
      if (timerEl) timerEl.textContent = fmt(elapsed);

      // 波形动画（seeded 伪随机）
      var t = elapsed / 1000;
      if (bars.length) {
        for (var i = 0; i < bars.length; i++) {
          var h = 20 + Math.abs(Math.sin(t * 3 + i * 1.31) * 0.55 + Math.sin(t * 1.7 + i * 0.7) * 0.35) * 130;
          bars[i].style.height = Math.round(Math.min(100, h)) + '%';
        }
      }
      // 转写渐进
      if (trEl) {
        var phraseCount = Math.min(SIM_PHRASES.length, 1 + Math.floor(t / 5));
        var shown = SIM_PHRASES.slice(_phraseIdx % SIM_PHRASES.length, _phraseIdx % SIM_PHRASES.length + phraseCount);
        trEl.innerHTML = shown.join('<br>') || '<span class="cap-hint">正在聆听…</span>';
      }
      _raf = requestAnimationFrame(frame);
    }
    _raf = requestAnimationFrame(frame);
  }

  function _stopRaf() {
    if (_raf) { cancelAnimationFrame(_raf); _raf = null; }
    if (_timer) { clearTimeout(_timer); _timer = null; }
  }

  // ---------- 文字速记保存 ----------
  function saveTextNote(text) {
    var trimmed = (text || '').trim();
    if (!trimmed) { WB.app.toast('内容不能为空', 'err'); return; }
    var card = createTextCard(trimmed, 'text');
    WB.app.toast('已安全保存', 'ok');
    if (store.state.settings.aiCompletionEnabled && !store.state.settings.aiMemoryEnabled) {
      // 补全提示
      setTimeout(function () {
        WB.app.toast('还可补全 3 项（标题/类型/标签）', 'warn');
      }, 800);
    }
    store.set({ capture: { phase: 'idle' } });
    WB.router.go('memories');
  }

  // ---------- 导出 ----------
  WB.capture = {
    start: start,
    startTextNote: startTextNote,
    stop: stop,
    cancel: cancel,
    onOverlayRendered: onOverlayRendered,
    createTextCard: createTextCard,
    saveTextNote: saveTextNote
  };
})(typeof window !== 'undefined' ? window : this);
