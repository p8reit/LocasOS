const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function dashboard() {
  const elements = new Map();
  function element(id) {
    if (!elements.has(id)) elements.set(id, {
      value: id === 'updatechannel' ? 'stable' : '', disabled: false,
      textContent: '', children: [], listeners: {},
      addEventListener(name, fn) { this.listeners[name] = fn; },
      replaceChildren() { this.children = []; },
      appendChild(child) { this.children.push(child); }
    });
    return elements.get(id);
  }
  const pending = [];
  const context = vm.createContext({
    document: {getElementById: element, createElement: () => ({appendChild(){}})},
    fetch: url => new Promise((resolve, reject) => pending.push({url, resolve, reject})),
    confirm: () => {throw Error('No install confirmation expected');},
    setInterval, clearInterval
  });
  const html = fs.readFileSync(path.join(__dirname, '../cmd/locas-core/index.html'), 'utf8');
  const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];
  vm.runInContext(source.replace(/initialize\(\);\s*$/, ''), context);
  return {element, pending, run: code => vm.runInContext(code, context)};
}
function respond(call, channel='stable') {
  call.resolve({ok: true, json: async () => ({
    channel, latest_release_tag: 'v1.2.3', release_url: 'https://github.com/p8reit/LocasOS/releases/tag/v1.2.3',
    package_ready: true, package_files: {}
  })});
}
test('changing channel clears an approved selection and package links', async () => {
  const d = dashboard(); const check = d.run('checkUpdates()'); respond(d.pending[0]); await check;
  assert.equal(d.element('installupdate').disabled, false);
  d.element('updatechannel').value = 'beta'; d.element('updatechannel').listeners.change();
  assert.equal(d.run('selectedUpdate'), null);
  assert.equal(d.element('installupdate').disabled, true);
  assert.equal(d.element('releaselink').children.length, 0);
  assert.equal(d.element('updatepackage').children.length, 0);
});
test('late response from old channel cannot replace newer result', async () => {
  const d = dashboard(); const first = d.run('checkUpdates()');
  d.element('updatechannel').value = 'beta'; d.element('updatechannel').listeners.change();
  const second = d.run('checkUpdates()'); respond(d.pending[1], 'beta'); await second;
  respond(d.pending[0]); await first;
  assert.equal(d.run('selectedUpdate.channel'), 'beta');
  assert.equal(d.element('installupdate').disabled, false);
});
test('old failure cannot overwrite current channel result', async () => {
  const d = dashboard(); const first = d.run('checkUpdates()');
  d.element('updatechannel').value = 'beta'; d.element('updatechannel').listeners.change();
  const second = d.run('checkUpdates()'); respond(d.pending[1], 'beta'); await second;
  d.pending[0].reject(Error('old network error')); await first;
  assert.equal(d.element('updateerror').textContent, '');
  assert.equal(d.run('selectedUpdate.channel'), 'beta');
});
test('failed recheck leaves no installable selection', async () => {
  const d = dashboard(); const first = d.run('checkUpdates()'); respond(d.pending[0]); await first;
  const second = d.run('checkUpdates()'); d.pending[1].reject(Error('offline')); await second;
  assert.equal(d.run('selectedUpdate'), null);
  assert.equal(d.element('installupdate').disabled, true);
});
