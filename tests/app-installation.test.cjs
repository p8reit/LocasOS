const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
function dashboard(){
 const elements=new Map(),pending=[],confirmations=[];let approve=true;
 function element(id){if(!elements.has(id))elements.set(id,{value:'',textContent:'',disabled:false,hidden:false,children:[],listeners:{},classList:{toggle(){}},addEventListener(n,f){this.listeners[n]=f;},replaceChildren(){this.children=[];},appendChild(c){this.children.push(c);}});return elements.get(id);}
 const context=vm.createContext({document:{getElementById:element,createElement:()=>({appendChild(){}})},fetch:(url,options)=>new Promise((resolve,reject)=>pending.push({url,options,resolve,reject})),confirm:message=>{confirmations.push(message);return approve;},setInterval,clearInterval});
 const html=fs.readFileSync(path.join(__dirname,'../cmd/locas-core/index.html'),'utf8');
 vm.runInContext(html.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/initialize\(\);\s*$/,''),context);
 return {element,pending,confirmations,run:code=>vm.runInContext(code,context),decline:()=>{approve=false;},render:data=>{context.fixture=data;vm.runInContext('renderApps(fixture)',context);}};
}
const token='a'.repeat(64),reference='b'.repeat(32);
function uncertain(overrides={}){return {job:'uncertain:open-webui',state:'uncertain',reason:'webui-docker-install',correlation_id:reference,status_token:token,ollama_installed:true,ollama_ready:false,workspace_state:'unknown',...overrides};}
function respond(call,data,status=200){call.resolve({ok:status>=200&&status<300,status,json:async()=>data});}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('binary presence does not enable WebUI or imply Ollama/API health',()=>{
 const d=dashboard();d.render({job:'idle',state:'idle',ollama_installed:true,ollama_ready:false,workspace_state:'unknown'});
 assert.equal(d.element('installwebui').disabled,true);assert.equal(d.element('installollama').disabled,false);
 assert.match(d.element('appollama').textContent,/not ready/);assert.match(d.element('appwebui').textContent,/unavailable/);
});
test('uncertain work requires deliberate acknowledgement and healthy Ollama',()=>{
 const d=dashboard();d.render(uncertain());assert.equal(d.element('installollama').disabled,true);assert.equal(d.element('installwebui').disabled,true);
 d.element('ackappstatus').listeners.click();assert.match(d.confirmations[0],/Inspect Ollama/);assert.equal(d.element('installollama').disabled,false);assert.equal(d.element('installwebui').disabled,true);
 d.render(uncertain({ollama_ready:true}));assert.equal(d.element('installwebui').disabled,false);assert.equal(d.element('installollama').disabled,true);
});
test('cancelled acknowledgement and legacy uncertainty cannot dispatch',async()=>{
 const d=dashboard();d.decline();d.render(uncertain());d.element('ackappstatus').listeners.click();await d.run("installApp('ollama')");assert.equal(d.pending.length,0);assert.equal(d.element('installollama').disabled,true);
 d.render({job:'uncertain:ollama',ollama_installed:true,ollama_ready:false});assert.equal(d.element('ackappstatus').disabled,true);await d.run("installApp('ollama')");assert.equal(d.pending.length,0);
});
test('changed status revokes acknowledgement and arbitrary diagnostic text is suppressed',()=>{
 const d=dashboard();d.render(uncertain());d.element('ackappstatus').listeners.click();d.render(uncertain({status_token:'c'.repeat(64),reason:'SECRET /private/path',correlation_id:'SECRET'}));
 assert.equal(d.element('installollama').disabled,true);assert.equal(d.element('appdetails').textContent,'');assert.doesNotMatch(d.element('appjob').textContent,/SECRET/);
});
test('retry sends current acknowledgement once and pending state prevents double enqueue',async()=>{
 const d=dashboard();d.render(uncertain());d.element('ackappstatus').listeners.click();const installing=d.run("installApp('ollama')");
 assert.equal(d.pending.length,1);assert.deepEqual(JSON.parse(d.pending[0].options.body),{app:'ollama',acknowledge_status:token});assert.equal(d.element('installollama').disabled,true);
 await d.run("installApp('ollama')");assert.equal(d.pending.length,1);
 respond(d.pending[0],{},202);await tick();respond(d.pending[1],{job:'installing:ollama',state:'installing',ollama_ready:false});await installing;
 assert.equal(d.element('installollama').disabled,true);assert.equal(d.run('appAcknowledgement'),'');
});
test('late polling cannot overwrite an install attempt or restore old consent',async()=>{
 const d=dashboard();d.render(uncertain());const oldPoll=d.run('refreshApps()');d.element('ackappstatus').listeners.click();const installing=d.run("installApp('ollama')");
 respond(d.pending[0],{job:'idle',state:'idle',ollama_ready:false});await oldPoll;assert.equal(d.run('appStatus.state'),'uncertain');assert.equal(d.element('installollama').disabled,true);
 respond(d.pending[1],{},202);await tick();respond(d.pending[2],{job:'installing:ollama',state:'installing',ollama_ready:false});await installing;
});
test('authentication loss revokes app status and acknowledgement',async()=>{
 const d=dashboard();d.render(uncertain());d.element('ackappstatus').listeners.click();const request=d.run("request('/api/apps')");respond(d.pending[0],{},401);await assert.rejects(request);
 assert.equal(d.run('appStatus'),null);assert.equal(d.run('appAcknowledgement'),'');assert.equal(d.element('installollama').disabled,true);assert.equal(d.element('installwebui').disabled,true);
});
test('failed admission requires a refreshed status and never automatically retries',async()=>{
 const d=dashboard();d.render(uncertain());d.element('ackappstatus').listeners.click();const installing=d.run("installApp('ollama')");respond(d.pending[0],{},409);await tick();respond(d.pending[1],uncertain({status_token:'d'.repeat(64)}));await installing;
 assert.equal(d.pending.filter(p=>p.options.method==='POST').length,1);assert.equal(d.element('installollama').disabled,true);assert.match(d.element('apperror').textContent,/HTTP 409/);
});
