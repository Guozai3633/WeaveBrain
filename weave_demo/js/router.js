/* ============================================================
   织脑原型 — hash 路由 + 设备切换 + 渲染入口
   ============================================================ */
(function (global) {
  'use strict';

  var WB = global.WB = global.WB || {};
  var store = WB.store;

  // 手机端屏幕白名单
  var MOBILE_SCREENS = [
    'memories', 'search', 'detail', 'echo', 'me', 'ai-settings',
    'account', 'mcp', 'workflow', 'import', 'text-note', 'capture', 'login'
  ];
  // Web 端屏幕白名单
  var WEB_SCREENS = [
    'memories', 'search', 'import', 'ai-settings', 'data', 'workflows'
  ];

  function parseHash() {
    var h = (global.location.hash || '').replace(/^#\/?/, '');
    var parts = h.split('/').filter(Boolean);
    var device = parts[0] === 'w' ? 'web' : (parts[0] === 'm' ? 'mobile' : null);
    var rest = parts.slice(1);
    var screen = rest[0] || (device === 'web' ? 'memories' : 'memories');
    var params = {};
    if (rest[1] !== undefined) params.id = rest[1];
    if (rest[2] !== undefined) params.extra = rest[2];
    return { device: device, screen: screen, params: params };
  }

  function buildHash(device, screen, params) {
    var p = device === 'web' ? 'w' : 'm';
    var s = screen;
    var id = params && params.id ? '/' + params.id : '';
    return '#/' + p + '/' + s + id;
  }

  function go(screen, params) {
    var dev = store.state.device;
    var list = dev === 'web' ? WEB_SCREENS : MOBILE_SCREENS;
    if (list.indexOf(screen) === -1) screen = dev === 'web' ? 'memories' : 'memories';
    var target = buildHash(dev, screen, params || {});
    if (global.location.hash === target) {
      // 已在该路由：仅更新 params 并重渲染
      store.set({ route: { screen: screen, params: params || {} } });
    } else {
      global.location.hash = target;
    }
  }

  function switchDevice(device) {
    if (store.state.device === device) return;
    store.set({ device: device });
    var home = device === 'web' ? 'memories' : 'memories';
    go(home);
  }

  function apply() {
    var parsed = parseHash();
    var device = parsed.device || store.state.device;
    var screen = parsed.screen;
    var list = device === 'web' ? WEB_SCREENS : MOBILE_SCREENS;
    if (list.indexOf(screen) === -1) screen = device === 'web' ? 'memories' : 'memories';
    store.set({ device: device, route: { screen: screen, params: parsed.params || {} } });
    // 设备切换按钮状态
    WB.app && WB.app.syncDeviceButtons(device);
  }

  // 监听 hash 变化
  global.addEventListener('hashchange', apply);

  // 导出
  WB.router = {
    go: go,
    switchDevice: switchDevice,
    apply: apply,
    current: function () { return store.state.route; }
  };
})(typeof window !== 'undefined' ? window : this);
