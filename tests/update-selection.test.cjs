const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function dashboard() {
  const confirmations=[];let now=Date.now();
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
    fetch: (url,options) => new Promise((resolve, reject) => pending.push({url, options, resolve, reject})),
    confirm: message => {confirmations.push(message);return true;},
    Date: class extends Date {static now(){return now;}},
    setInterval, clearInterval
  });
  const html = fs.readFileSync(path.join(__dirname, '../cmd/locas-core/index.html'), 'utf8');
  const source = html.match(/<script>([\s\S]*?)<\/script>/)[1];
  vm.runInContext(source.replace(/initialize\(\);\s*$/, ''), context);
  return {element, pending, confirmations, advance: ms => {now+=ms;}, run: code => vm.runInContext(code, context)};
}
const sha256='a'.repeat(64),token='b'.repeat(64);
function respond(call, channel='stable', overrides={}) {
  call.resolve({ok: true, json: async () => ({
    channel, latest_release_tag: 'v1.2.3', release_url: 'https://github.com/p8reit/LocasOS/releases/tag/v1.2.3',
    package_ready: true, package_files: {}, install_ready:true,
    approval:{version:'v1.2.3',channel,sha256,token},...overrides
  })});
}
test('changing channel clears an approved selection and package links', async () => {
  const d = dashboard(); const check = d.run('checkUpdates()'); respond(d.pending[0]); await check;
  assert.equal(d.element('installupdate').disabled, false);
  d.element('updatechannel').value = 'beta'; const saving = d.element('updatechannel').listeners.change();
  d.pending.at(-1).resolve({ok:true,json:async()=>({})}); await saving;
  assert.equal(d.run('selectedUpdate'), null);
  assert.equal(d.element('installupdate').disabled, true);
  assert.equal(d.element('releaselink').children.length, 0);
  assert.equal(d.element('updatepackage').children.length, 0);
});
test('late response from old channel cannot replace newer result', async () => {
  const d = dashboard(); const first = d.run('checkUpdates()');
  d.element('updatechannel').value = 'beta'; const saving = d.element('updatechannel').listeners.change();
  d.pending.at(-1).resolve({ok:true,json:async()=>({})}); await saving;
  const second = d.run('checkUpdates()'); respond(d.pending[2], 'beta'); await second;
  respond(d.pending[0]); await first;
  assert.equal(d.run('selectedUpdate.channel'), 'beta');
  assert.equal(d.element('installupdate').disabled, false);
});
test('old failure cannot overwrite current channel result', async () => {
  const d = dashboard(); const first = d.run('checkUpdates()');
  d.element('updatechannel').value = 'beta'; const saving = d.element('updatechannel').listeners.change();
  d.pending.at(-1).resolve({ok:true,json:async()=>({})}); await saving;
  const second = d.run('checkUpdates()'); respond(d.pending[2], 'beta'); await second;
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

for(const [label,overrides] of [
  ['unsigned release',{install_ready:false}],
  ['missing approval',{approval:undefined}],
  ['wrong version',{approval:{version:'v9.0.0',channel:'stable',sha256,token}}],
  ['wrong channel',{approval:{version:'v1.2.3',channel:'beta',sha256,token}}],
  ['invalid hash',{approval:{version:'v1.2.3',channel:'stable',sha256:'A'.repeat(64),token}}],
  ['nonstring hash',{approval:{version:'v1.2.3',channel:'stable',sha256:[sha256],token}}],
  ['nonstring token',{approval:{version:'v1.2.3',channel:'stable',sha256,token:[token]}}],
  ['invalid token',{approval:{version:'v1.2.3',channel:'stable',sha256,token:'x'}}],
  ['invalid expiry',{approval_expires_at:'not-a-date'}],
  ['expired approval',{approval_expires_at:'2000-01-01T00:00:00Z'}]
])test(label+' keeps manual links but disables installation',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',overrides);await check;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
  assert.equal(d.element('releaselink').children.length,1);
  assert.ok(d.element('updatepackage').children.length>=6);
  await d.element('installupdate').listeners.click();assert.equal(d.pending.length,1);
});
test('approved hash is displayed, confirmed, sent exactly and consumed once',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0]);await check;
  assert.ok(d.element('updatepackage').children.some(e=>e.textContent==='Approved archive SHA-256: '+sha256));
  const install=d.element('installupdate').listeners.click();assert.ok(d.confirmations[0].includes(sha256));
  assert.deepEqual(JSON.parse(d.pending[1].options.body),{version:'v1.2.3',channel:'stable',sha256,token});
  assert.equal(d.pending[1].options.headers['X-Locas-Request'],'1');
  d.pending[1].resolve({ok:true,json:async()=>({})});await new Promise(resolve=>setImmediate(resolve));
  assert.equal(d.run('selectedUpdate'),null);
  d.pending[2].resolve({ok:true,json:async()=>({state:'idle'})});await install;
  assert.equal(d.element('installupdate').disabled,true);await d.element('installupdate').listeners.click();assert.equal(d.pending.length,3);
});
test('expiry disables click and idle status cannot restore expired approval',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{approval_expires_at:new Date(Date.now()+10000).toISOString()});await check;
  d.advance(20000);await d.element('installupdate').listeners.click();
  assert.equal(d.confirmations.length,0);assert.equal(d.element('installupdate').disabled,true);
  assert.match(d.element('updateerror').textContent,/expired/);
  const poll=d.run('refreshUpdateJob()');d.pending[1].resolve({ok:true,json:async()=>({state:'idle'})});await poll;
  assert.equal(d.element('installupdate').disabled,true);assert.equal(d.run('selectedUpdate'),null);
});
test('idle status itself expires selection',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{approval_expires_at:new Date(Date.now()+10000).toISOString()});await check;
  d.advance(20000);const poll=d.run('refreshUpdateJob()');d.pending[1].resolve({ok:true,json:async()=>({state:'idle'})});await poll;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
});
test('authentication loss invalidates approval and pending discovery',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0]);await check;
  const late=d.run('checkUpdates()');const poll=d.run('refreshUpdateJob()');
  d.pending[2].resolve({ok:false,status:401});await poll;respond(d.pending[1]);await late;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
});
test('logout immediately invalidates approval even before network completes',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0]);await check;
  const logout=d.element('logout').listeners.click();assert.equal(d.run('selectedUpdate'),null);
  d.pending[1].reject(Error('offline'));await logout;assert.equal(d.element('installupdate').disabled,true);
});

test('polling cannot re-enable install while enqueue is in flight',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0]);await check;
  const install=d.element('installupdate').listeners.click();
  const poll=d.run('refreshUpdateJob()');d.pending[2].resolve({ok:true,json:async()=>({state:'idle'})});await poll;
  assert.equal(d.element('installupdate').disabled,true);await d.element('installupdate').listeners.click();assert.equal(d.pending.length,3);
  d.pending[1].reject(Error('uncertain transport failure'));await new Promise(resolve=>setImmediate(resolve));
  d.pending[3].resolve({ok:true,json:async()=>({state:'idle'})});await install;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
});
