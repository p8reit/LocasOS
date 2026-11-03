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
    package_ready: true, package_files: {}, install_ready:true, install_eligible:true, comparison:'newer',
    installed_version:'v1.2.2',installed_commit:'c'.repeat(40),installed_release:'v1.2.2',last_verified_release:'v1.2.2',version_basis:'running_release',
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

for(const [comparison,message,availability] of [
  ['newer','Newer release available','update_available'],
  ['same','Installed version is up to date','up_to_date'],
  ['older','A newer version is already installed','newer_installed'],
  ['unknown','Installed version unknown; this target is NOT confirmed newer','comparison_unknown'],
  ['blocked','Installation blocked by version state','version_state_invalid']
])test('renders backend numeric comparison '+comparison+' truthfully',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{
    latest_release_tag:'v1.10.0', installed_version:'v1.9.0', comparison,availability,
    install_eligible:comparison==='newer'||comparison==='unknown',
    approval:{version:'v1.10.0',channel:'stable',sha256,token}
  });await check;
  assert.match(d.element('updateavailability').textContent,/v1\.10\.0/);
  assert.ok(d.element('updateavailability').textContent.includes(message));
  assert.doesNotMatch(d.element('updateavailability').textContent,/comparison unavailable/);
  assert.equal(d.element('installupdate').disabled,comparison!=='newer'&&comparison!=='unknown');
});
for(const comparison of ['same','older','blocked'])for(const approval of [null,{version:'v1.2.3',channel:'stable',sha256,token},{sha256:[],token:1}])test(comparison+' cannot enable install despite discovery approval '+JSON.stringify(approval),async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{comparison,install_eligible:false,install_ready:true,approval,eligibility_reason:'Version policy rejects this release'});await check;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
  assert.match(d.element('updateavailability').textContent,/Version policy rejects this release/);
  await d.element('installupdate').listeners.click();assert.equal(d.confirmations.length,0);assert.equal(d.pending.length,1);
});
test('explicit eligibility is required even with a valid newer signed approval',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{install_eligible:undefined});await check;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
});
test('legacy unknown version warns before sending exact approved upgrade identity',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{
    comparison:'unknown',availability:'comparison_unknown',installed_version:'dev',installed_release:'',installed_commit:'unknown',last_verified_release:'',version_basis:'unknown',eligibility_reason:'Signed legacy upgrade permitted; installed version is unknown'
  });await check;
  assert.equal(d.element('systemversion').textContent,'dev');assert.equal(d.element('runningrelease').textContent,'Running release: unknown');
  assert.match(d.element('updateavailability').textContent,/NOT confirmed newer/);
  const install=d.element('installupdate').listeners.click();assert.match(d.confirmations[0],/Installed version unknown; this target is NOT confirmed newer/);
  assert.deepEqual(JSON.parse(d.pending[1].options.body),{version:'v1.2.3',channel:'stable',sha256,token});
  d.pending[1].resolve({ok:true,json:async()=>({})});await new Promise(resolve=>setImmediate(resolve));
  d.pending[2].resolve({ok:true,json:async()=>({state:'idle'})});await install;
});
test('running identity and root verified record remain distinct',async()=>{
  const d=dashboard();const refresh=d.run('refreshUpdates()');d.pending[0].resolve({ok:true,json:async()=>({
    installed_version:'dev-experiment',installed_commit:'e'.repeat(40),installed_release:'',last_verified_release:'v1.9.0',version_basis:'verified_record',channel:'beta',availability:'not_checked'
  })});await refresh;
  assert.equal(d.element('systemversion').textContent,'dev-experiment');
  assert.equal(d.element('runningcommit').textContent,'Running commit: '+'e'.repeat(40));
  assert.equal(d.element('runningrelease').textContent,'Running release: unknown');
  assert.equal(d.element('verifiedrelease').textContent,'Last verified release record: v1.9.0');
  assert.equal(d.element('updatestatus').textContent,'Version basis: last verified release record');
  assert.equal(d.element('updatechannel').value,'beta');
});
for(const version_basis of ['invalid_state','inconsistent'])test(version_basis+' is displayed as blocked rather than unknown',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{version_basis,comparison:'blocked',install_eligible:false,eligibility_reason:'Installed version state requires recovery'});await check;
  assert.match(d.element('updatestatus').textContent,/installation blocked/);assert.equal(d.element('installupdate').disabled,true);
});
test('late initial identity refresh cannot overwrite a current channel discovery',async()=>{
  const d=dashboard();const initial=d.run('refreshUpdates()');const check=d.run('checkUpdates()');respond(d.pending[1]);await check;
  d.pending[0].resolve({ok:true,json:async()=>({installed_version:'obsolete',channel:'beta',availability:'not_checked'})});await initial;
  assert.equal(d.element('systemversion').textContent,'v1.2.2');assert.equal(d.element('updatechannel').value,'stable');assert.match(d.element('updateavailability').textContent,/Newer release available/);
});
for(const comparison of ['same','older','blocked'])test(comparison+' stays disabled if readiness flags contradict comparison',async()=>{
  const d=dashboard();const check=d.run('checkUpdates()');respond(d.pending[0],'stable',{comparison,install_eligible:true,install_ready:true});await check;
  assert.equal(d.run('selectedUpdate'),null);assert.equal(d.element('installupdate').disabled,true);
});
