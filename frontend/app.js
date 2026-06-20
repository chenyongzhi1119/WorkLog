// ── utils ──────────────────────────────────────────────────────
const today     = () => new Date().toISOString().split('T')[0];
const yesterday = () => { const d=new Date(); d.setDate(d.getDate()-1); return d.toISOString().split('T')[0]; };
const thisMonth = () => new Date().toISOString().slice(0,7);
const thisMonday = () => { const d=new Date(), day=d.getDay()||7; d.setDate(d.getDate()-day+1); return d.toISOString().split('T')[0]; };
const fmtTime = iso => iso ? iso.slice(11,16) : '';
const fmtDate = iso => iso ? iso.slice(0,10) : '';
const esc = s => (s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
const ea  = s => (s||'').replace(/"/g,'&quot;').replace(/'/g,'&#39;');

function toast(msg, ms=2400) {
  const el=document.getElementById('toast');
  el.textContent=msg; el.style.display='block'; el.style.opacity='1';
  clearTimeout(el._t);
  el._t=setTimeout(()=>{el.style.opacity='0';setTimeout(()=>el.style.display='none',250);},ms);
}

async function api(path, method='GET', body=null) {
  const opts={method,headers:{'Content-Type':'application/json'}};
  if(body) opts.body=JSON.stringify(body);
  const r=await fetch('/api'+path,opts);
  if(!r.ok) throw new Error(await r.text()||r.statusText);
  return r.json();
}

function decodeChunk(raw) {
  if(raw==='[DONE]'||raw.startsWith('[ERROR]')) return raw;
  try{return JSON.parse(raw);}catch{return raw;}
}

// ── Markdown ───────────────────────────────────────────────────
function md(text) {
  if(!text) return '';
  const blocks=[];
  const protect=html=>{blocks.push(html);return `\x00B${blocks.length-1}\x00`;};

  let t=text.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  t=t.replace(/```[\w]*\n?([\s\S]*?)```/g,(_,c)=>protect(`<pre><code>${c.trim()}</code></pre>`));
  t=t.replace(/^(\|.+\|)\n(?:\|[-| :]+\|\n)?((?:\|.+\|\n?)+)/gm,(_,hdr,body)=>{
    const pr=row=>row.split('|').filter(c=>c.trim()).map(c=>c.trim());
    const th=pr(hdr).map(c=>`<th>${mi(c)}</th>`).join('');
    const rows=body.trim().split('\n').filter(l=>l.includes('|')&&!/^[\|\-\:\s]+$/.test(l))
      .map(row=>`<tr>${pr(row).map(c=>`<td>${mi(c)}</td>`).join('')}</tr>`).join('');
    return protect(`<table><thead><tr>${th}</tr></thead><tbody>${rows}</tbody></table>`);
  });
  t=t.replace(/`([^`]+)`/g,(_,c)=>protect(`<code>${c}</code>`));
  t=t.replace(/\*\*([^*\n]+)\*\*/g,'<strong>$1</strong>');
  t=t.replace(/\*([^*\n]+)\*/g,'<em>$1</em>');
  t=t.replace(/^### (.+)$/gm,'<h3>$1</h3>');
  t=t.replace(/^## (.+)$/gm,'<h2>$1</h2>');
  t=t.replace(/^# (.+)$/gm,'<h1>$1</h1>');
  t=t.replace(/^---+$/gm,'<hr>');
  t=t.replace(/^- (.+)$/gm,'<li>$1</li>');
  t=t.replace(/(<li>[^<]*<\/li>\n?)+/g,m=>`<ul>${m}</ul>`);
  t=t.replace(/^\d+\. (.+)$/gm,'<li>$1</li>');
  t=t.replace(/(<li>[^<]*<\/li>\n?)+/g,m=>`<ol>${m}</ol>`);
  t=t.replace(/\n\n+/g,'<br><br>');
  t=t.replace(/([^\n>])\n(?=[^\n<\x00])/g,'$1<br>');
  t=t.replace(/\x00B(\d+)\x00/g,(_,i)=>blocks[parseInt(i)]);
  return t;
}
const mi=s=>s.replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>').replace(/\*([^*]+)\*/g,'<em>$1</em>');

// ── Tabs ───────────────────────────────────────────────────────
function switchTab(name) {
  document.querySelectorAll('.tab').forEach(b=>b.classList.toggle('active',b.dataset.tab===name));
  document.querySelectorAll('.tab-content').forEach(c=>c.classList.toggle('active',c.id==='tab-'+name));
  if(name==='reports')  loadDailyList();
  if(name==='goals')    loadGoals();
  if(name==='daily')    loadDailyTab();
  if(name==='overview') loadOverview();
}
document.querySelectorAll('.tab').forEach(btn=>btn.addEventListener('click',()=>switchTab(btn.dataset.tab)));
document.querySelectorAll('.rtab').forEach(btn=>{
  btn.addEventListener('click',()=>{
    document.querySelectorAll('.rtab').forEach(b=>b.classList.remove('active'));
    document.querySelectorAll('.rtab-content').forEach(c=>c.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('rtab-'+btn.dataset.rtab).classList.add('active');
    if(btn.dataset.rtab==='weekly')  loadWeeklyList();
    if(btn.dataset.rtab==='monthly') loadMonthlyList();
  });
});
document.getElementById('logo').addEventListener('click',()=>switchTab('chat'));

// Modal tabs (Settings sub-sections)
document.querySelectorAll('.modal-tab').forEach(btn=>{
  btn.addEventListener('click',()=>{
    document.querySelectorAll('.modal-tab').forEach(b=>b.classList.remove('active'));
    document.querySelectorAll('.modal-tab-content').forEach(c=>c.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('mtab-'+btn.dataset.mtab).classList.add('active');
  });
});

// ── Init ───────────────────────────────────────────────────────
function init() {
  const d=new Date();
  document.getElementById('date-display').textContent=
    d.toLocaleDateString('zh-CN',{year:'numeric',month:'long',day:'numeric',weekday:'short'});
  document.getElementById('filter-month').value=thisMonth();
  document.getElementById('weekly-start').value=thisMonday();
  document.getElementById('monthly-month').value=thisMonth();
  updateProviderBadge();
  loadSidebarGoals();
  loadHistory();
  loadGoals();
  setupNotifications();
}

// ── SIDEBAR GOALS ──────────────────────────────────────────────
async function loadSidebarGoals() {
  try{
    const goals=await api('/goals');
    const el=document.getElementById('sb-goals');
    if(!goals||!goals.length){el.innerHTML='<p class="hint">暂无目标</p>';return;}
    el.innerHTML=goals.slice(0,6).map(g=>{
      const pct=g.progress||0;
      return `<div class="sb-goal-item">
        <div class="sb-goal-title">${esc(g.title)}</div>
        <div class="sb-goal-meta">
          <span class="sb-goal-lv">${{long_term:'长期',monthly:'月',weekly:'周'}[g.level]}</span>
          <div class="sb-progress-wrap"><div class="sb-progress-fill${pct>=100?' done':''}" style="width:${pct}%"></div></div>
          <span class="sb-progress-pct">${pct}%</span>
        </div>
      </div>`;
    }).join('');
  }catch(_){}
}

// ── OVERVIEW / DASHBOARD ───────────────────────────────────────
let calMonth = thisMonth();

async function loadOverview() {
  try{
    const data=await api('/stats?month='+calMonth);
    // Stats
    document.getElementById('stat-streak').textContent = data.streak||'0';
    document.getElementById('stat-month').textContent  = data.month_count||'0';
    document.getElementById('stat-week').textContent   = data.week_count||'0';
    const rate = data.tasks_total_week > 0
      ? Math.round(data.tasks_done_week/data.tasks_total_week*100)+'%'
      : '—';
    document.getElementById('stat-tasks').textContent = rate;

    // Calendar
    buildCalendar(calMonth, data.calendar_dates||[]);

    // Goals overview
    const oel=document.getElementById('ov-goals');
    const lvName={long_term:'长期',monthly:'月目标',weekly:'周目标'};
    if(!data.goals||!data.goals.length){oel.innerHTML='<p class="hint" style="padding:12px 0">暂无目标</p>';return;}
    oel.innerHTML=data.goals.map(g=>{
      const pct=g.progress||0;
      return `<div class="ov-goal-item">
        <div class="ov-goal-hd">
          <span class="ov-goal-title">${esc(g.title)}</span>
          <span class="ov-goal-pct">${pct}%</span>
        </div>
        <div class="ov-goal-lv">${lvName[g.level]||g.level}</div>
        <div class="ov-progress"><div class="ov-progress-fill${pct>=100?' done':''}" style="width:${pct}%"></div></div>
      </div>`;
    }).join('');
  }catch(e){console.error(e);}
}

function buildCalendar(month, reportDates) {
  const [yr,mo]=month.split('-').map(Number);
  const title=`${yr} 年 ${mo} 月`;
  document.getElementById('cal-title').textContent=title;
  const datesSet=new Set(reportDates);
  const todayStr=today();
  const firstDay=new Date(yr,mo-1,1);
  const lastDate=new Date(yr,mo,0).getDate();
  const startWd=(firstDay.getDay()+6)%7; // Mon=0

  let html='<div class="cal-grid">';
  ['一','二','三','四','五','六','日'].forEach(d=>html+=`<div class="cal-hd">${d}</div>`);
  for(let i=0;i<startWd;i++) html+='<div class="cal-day cal-empty"></div>';
  for(let d=1;d<=lastDate;d++){
    const ds=`${yr}-${String(mo).padStart(2,'0')}-${String(d).padStart(2,'0')}`;
    const has=datesSet.has(ds), isTd=ds===todayStr;
    html+=`<div class="cal-day${has?' cal-has-report':''}${isTd?' cal-today':''}" data-date="${ds}">${d}</div>`;
  }
  html+='</div>';
  const container=document.getElementById('cal-container');
  container.innerHTML=html;
  container.querySelectorAll('.cal-has-report').forEach(el=>{
    el.addEventListener('click',()=>showCalDetail(el.dataset.date,datesSet));
  });
  document.getElementById('cal-detail').style.display='none';
}

async function showCalDetail(date, _datesSet) {
  const det=document.getElementById('cal-detail');
  det.innerHTML='<div class="cal-detail-hd">'+date+'</div><p class="hint">加载中...</p>';
  det.style.display='block';
  try{
    const list=await api('/reports/daily?date='+date);
    const r=list&&list.length?list[0]:null;
    if(!r){det.innerHTML='<p class="hint">无日报记录</p>';return;}
    det.innerHTML=`<div class="cal-detail-hd">${r.date}</div>
      <div class="cal-detail-section"><div class="cal-detail-label">今日完成</div>${esc(r.completed)}</div>
      ${r.plan?`<div class="cal-detail-section"><div class="cal-detail-label">明日计划</div>${esc(r.plan)}</div>`:''}
      ${r.issues?`<div class="cal-detail-section"><div class="cal-detail-label">问题风险</div>${esc(r.issues)}</div>`:''}`;
  }catch(e){det.innerHTML='<p class="hint">加载失败</p>';}
}

document.getElementById('cal-prev').addEventListener('click',()=>{
  const [yr,mo]=calMonth.split('-').map(Number);
  const d=new Date(yr,mo-2,1); calMonth=d.toISOString().slice(0,7);
  loadOverview();
});
document.getElementById('cal-next').addEventListener('click',()=>{
  const [yr,mo]=calMonth.split('-').map(Number);
  const d=new Date(yr,mo,1); calMonth=d.toISOString().slice(0,7);
  loadOverview();
});

// ── CHAT ───────────────────────────────────────────────────────
let homeStreaming=false, lastMsgIsPlan=false, abortCtrl=null;

function addBubble(role,content,streaming=false) {
  const el=document.getElementById('home-msgs');
  const div=document.createElement('div');
  div.className='cmsg '+role;
  div.innerHTML=`<div class="cavatar">${role==='user'?'我':'师'}</div>
    <div class="cbubble">${streaming
      ?'<div class="dots"><span></span><span></span><span></span></div>'
      :(role==='assistant'?md(content):esc(content))
    }</div>`;
  el.appendChild(div); el.scrollTop=el.scrollHeight;
  return div.querySelector('.cbubble');
}

async function loadHistory() {
  try{
    const list=await api('/mentor/conversations');
    const el=document.getElementById('home-msgs');el.innerHTML='';
    (list||[]).forEach(c=>addBubble(c.role,c.content,false));
    el.scrollTop=el.scrollHeight;
  }catch(_){}
}

async function sendHomeMsg(msg) {
  if(homeStreaming||!msg.trim()) return;
  document.getElementById('home-input').value='';
  addBubble('user',msg);
  const bubble=addBubble('assistant','',true);
  homeStreaming=true;
  document.getElementById('home-send').disabled=true;
  document.getElementById('home-stop').style.display='inline-flex';

  let buf='',reportJson=null;
  abortCtrl=new AbortController();
  try{
    const resp=await fetch('/api/mentor/stream?message='+encodeURIComponent(msg),{signal:abortCtrl.signal});
    const reader=resp.body.getReader();const decoder=new TextDecoder();let rem='';
    outer:while(true){
      const{done,value}=await reader.read();if(done) break;
      rem+=decoder.decode(value,{stream:true});
      const lines=rem.split('\n');rem=lines.pop();
      for(const line of lines){
        if(!line.startsWith('data: ')) continue;
        const chunk=decodeChunk(line.slice(6));
        if(chunk==='[DONE]'||chunk.startsWith('[ERROR]')){
          if(chunk.startsWith('[ERROR]')) bubble.innerHTML=`<span style="color:var(--danger)">${esc(chunk.slice(8))}</span>`;
          break outer;
        }
        buf+=chunk;bubble.textContent=buf;
        document.getElementById('home-msgs').scrollTop=9999;
      }
    }
  }catch(e){if(e.name!=='AbortError') bubble.textContent='发送失败：'+e.message;}

  homeStreaming=false;
  document.getElementById('home-send').disabled=false;
  document.getElementById('home-stop').style.display='none';
  abortCtrl=null;
  if(!buf) return;

  const jm=buf.match(/日报整理好了[：:]\s*```(?:json)?\s*(\{[\s\S]*?\})\s*```/i)||buf.match(/日报整理好了[：:]\s*(\{[\s\S]*?\})/i);
  if(jm){try{reportJson=JSON.parse(jm[1]);}catch(_){}}
  bubble.innerHTML=md(buf);

  const actions=[];
  if(!reportJson){
    const lp=lastMsgIsPlan||/\b(09|10|11|13|14|15|16|17):\d{2}/.test(buf);
    if(lp&&buf.length>80){actions.push({label:'📋 添加任务到日报',fn:()=>savePlanAsTasks(buf)});lastMsgIsPlan=false;}
  }
  if(reportJson) actions.push({label:'✅ 应用到日报',fn:()=>applyAndGoToDaily(reportJson)});
  if(actions.length){
    const div=document.createElement('div');div.className='chat-actions';
    actions.forEach(a=>{const b=document.createElement('button');b.className='chat-action-btn';b.textContent=a.label;b.addEventListener('click',a.fn);div.appendChild(b);});
    bubble.appendChild(div);
  }
  document.getElementById('home-msgs').scrollTop=9999;
}

document.getElementById('home-stop').addEventListener('click',()=>{if(abortCtrl){abortCtrl.abort();abortCtrl=null;}});
let homeComposing=false;
document.getElementById('home-send').addEventListener('click',()=>sendHomeMsg(document.getElementById('home-input').value));
document.getElementById('home-input').addEventListener('compositionstart',()=>homeComposing=true);
document.getElementById('home-input').addEventListener('compositionend',()=>homeComposing=false);
document.getElementById('home-input').addEventListener('keydown',e=>{
  if(e.key==='Enter'&&!e.shiftKey&&!homeComposing){e.preventDefault();sendHomeMsg(document.getElementById('home-input').value);}
});
document.querySelectorAll('.chip').forEach(btn=>{
  if(btn.id==='btn-clear-chat') return;
  btn.addEventListener('click',()=>{lastMsgIsPlan=btn.dataset.msg?.includes('计划');sendHomeMsg(btn.dataset.msg);});
});
document.getElementById('btn-clear-chat').addEventListener('click',async()=>{
  if(!confirm('清空所有对话历史？（导师记忆不受影响）')) return;
  try{await api('/mentor/conversations','DELETE');document.getElementById('home-msgs').innerHTML='';toast('对话已清空');}
  catch(e){toast('失败：'+e.message);}
});

function extractTaskLines(text){
  const lines=text.split('\n'),tasks=[];
  for(const raw of lines){
    const line=raw.trim();if(!line) continue;
    if(/^#{1,4}\s/.test(line)||/^\|[-| :]+\|/.test(line)) continue;
    let task='';
    if(/^[-*•]\s+(.+)/.test(line)) task=line.replace(/^[-*•]\s+/,'');
    else if(/^\d+[.)]\s+(.+)/.test(line)) task=line.replace(/^\d+[.)]\s+/,'');
    else if(/^\|/.test(line)){const cells=line.split('|').map(c=>c.trim()).filter(Boolean);if(cells.length>=2) task=cells[1];}
    else if(/^\d{2}:\d{2}/.test(line)) task=line.replace(/^\d{2}:\d{2}[–\-~到至]\d{2}:\d{2}\s*/,'');
    if(task&&task.length>1&&task!=='任务'&&task!=='关联目标') tasks.push(task.replace(/\*\*/g,'').trim());
  }
  return [...new Set(tasks)];
}
async function savePlanAsTasks(content){
  const tasks=extractTaskLines(content);if(!tasks.length){toast('未解析到任务条目');return;}
  let added=0;for(const t of tasks){try{await api('/tasks','POST',{content:t,date:today()});added++;}catch(_){}}
  toast(`已添加 ${added} 条任务`);
  if(document.getElementById('tab-daily').classList.contains('active')) loadDrTasks();
}
function applyAndGoToDaily(data){
  switchTab('daily');
  if(data.plan)   document.getElementById('dr-plan').value  =data.plan;
  if(data.issues) document.getElementById('dr-issues').value=data.issues;
  if(data.completed&&data.completed.trim()){
    document.getElementById('dr-ai-completed-hint').style.display='block';
    document.getElementById('dr-ai-completed-text').textContent=data.completed;
  }
  toast('已填写明日计划和问题风险，今日完成请手动勾选任务');
}

// ── DAILY TAB ──────────────────────────────────────────────────
async function loadDailyTab(){await Promise.all([loadDrTasks(),loadDrYesterday(),loadDrWlogs()]);}

async function loadDrYesterday(){
  try{
    const list=await api('/reports/daily?date='+yesterday());
    const r=list&&list.length?list[0]:null;
    if(r&&r.plan&&r.plan.trim()){
      document.getElementById('dr-yesterday-text').textContent=r.plan;
      document.getElementById('dr-yesterday').style.display='flex';
    }
  }catch(_){}
}

async function loadDrTasks(){try{const t=await api('/tasks?date='+today());renderDrTasks(t||[]);}catch(_){}}

function renderDrTasks(tasks){
  const list=document.getElementById('dr-task-list');
  const badge=document.getElementById('task-badge');
  if(!tasks.length){list.innerHTML='<p class="hint" style="padding:4px 0;font-size:12px">暂无事项，在下方输入添加</p>';badge.style.display='none';return;}
  const done=tasks.filter(t=>t.done).length;
  badge.textContent=`${done}/${tasks.length} 完成`;badge.style.display='inline';
  list.innerHTML=tasks.map(t=>`
    <div class="dr-task-item">
      <div class="task-cb${t.done?' checked':''}" data-id="${t.id}" data-done="${t.done}"></div>
      <span class="task-text${t.done?' done':''}" data-id="${t.id}" title="双击编辑">${esc(t.content)}</span>
      <button class="task-del" data-id="${t.id}" title="删除">×</button>
    </div>`).join('');
  list.querySelectorAll('.task-cb').forEach(cb=>{
    cb.addEventListener('click',async()=>{
      try{await api('/tasks/'+cb.dataset.id,'PUT',{done:cb.dataset.done!=='true'});loadDrTasks();}catch(e){toast('更新失败：'+e.message);}
    });
  });
  list.querySelectorAll('.task-del').forEach(btn=>{
    btn.addEventListener('click',async()=>{try{await api('/tasks/'+btn.dataset.id,'DELETE');loadDrTasks();}catch(e){toast('删除失败：'+e.message);}});
  });
  list.querySelectorAll('.task-text').forEach(span=>{
    span.addEventListener('dblclick',()=>{
      const id=span.dataset.id,old=span.textContent;
      const inp=document.createElement('input');inp.type='text';inp.value=old;inp.className='dr-task-input';inp.style.flex='1';
      span.replaceWith(inp);inp.focus();inp.select();
      const save=async()=>{
        const val=inp.value.trim();
        if(val&&val!==old){try{await api('/tasks/'+id,'DELETE');await api('/tasks','POST',{content:val,date:today()});}catch(e){toast('编辑失败：'+e.message);}}
        loadDrTasks();
      };
      inp.addEventListener('blur',save);
      inp.addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();inp.blur();}if(e.key==='Escape'){loadDrTasks();}});
    });
  });
}

async function addDrTask(content){if(!content.trim()) return;try{await api('/tasks','POST',{content:content.trim(),date:today()});loadDrTasks();}catch(e){toast('添加失败：'+e.message);}}
let dtc=false;
document.getElementById('btn-add-dr-task').addEventListener('click',()=>{const i=document.getElementById('dr-task-input');addDrTask(i.value);i.value='';i.focus();});
document.getElementById('dr-task-input').addEventListener('compositionstart',()=>dtc=true);
document.getElementById('dr-task-input').addEventListener('compositionend',()=>dtc=false);
document.getElementById('dr-task-input').addEventListener('keydown',e=>{if(e.key==='Enter'&&!dtc){e.preventDefault();const i=document.getElementById('dr-task-input');addDrTask(i.value);i.value='';}});

async function loadDrWlogs(){
  try{
    const logs=await api('/worklogs?date='+today());
    const el=document.getElementById('dr-wlog-list');
    if(!logs||!logs.length){el.innerHTML='';return;}
    el.innerHTML=logs.map(w=>`
      <div class="dr-wlog-item">
        <span class="dr-wlog-time">${fmtDate(w.created_at)} ${fmtTime(w.created_at)}</span>
        <span class="dr-wlog-content">${esc(w.content)}</span>
        <button class="task-del dr-wlog-del" data-id="${w.id}">×</button>
      </div>`).join('');
    el.querySelectorAll('.dr-wlog-del').forEach(btn=>{
      btn.addEventListener('click',async()=>{try{await api('/worklogs/'+btn.dataset.id,'DELETE');loadDrWlogs();}catch(e){toast('删除失败：'+e.message);}});
    });
  }catch(_){}
}
let dwc=false;
document.getElementById('btn-add-dr-wlog').addEventListener('click',async()=>{
  const i=document.getElementById('dr-wlog-input');const c=i.value.trim();if(!c){i.focus();return;}
  try{await api('/worklogs','POST',{content:c,date:today()});i.value='';loadDrWlogs();}catch(e){toast('记录失败：'+e.message);}
});
document.getElementById('dr-wlog-input').addEventListener('compositionstart',()=>dwc=true);
document.getElementById('dr-wlog-input').addEventListener('compositionend',()=>dwc=false);
document.getElementById('dr-wlog-input').addEventListener('keydown',e=>{
  if(e.key==='Enter'&&!dwc){e.preventDefault();const i=document.getElementById('dr-wlog-input');const c=i.value.trim();if(!c) return;api('/worklogs','POST',{content:c,date:today()}).then(()=>{i.value='';loadDrWlogs();}).catch(e=>toast(e.message));}
});
document.getElementById('btn-wlog-to-task').addEventListener('click',async()=>{
  const logs=await api('/worklogs?date='+today());if(!logs||!logs.length){toast('今日暂无随手记');return;}
  for(const w of logs){try{await api('/tasks','POST',{content:w.content,date:today()});}catch(_){}}
  toast(`已导入 ${logs.length} 条随手记`);loadDrTasks();
});

document.getElementById('btn-submit-dr').addEventListener('click',async()=>{
  const btn=document.getElementById('btn-submit-dr'),msgEl=document.getElementById('dr-msg');
  const tasks=await api('/tasks?date='+today());
  const completed=(tasks||[]).map((t,i)=>`${i+1}. ${t.content}${t.done?'':'（进行中）'}`).join('\n');
  const plan=document.getElementById('dr-plan').value.trim();
  const issues=document.getElementById('dr-issues').value.trim();
  if(!completed&&!plan){toast('请至少填写今日任务或明日计划');return;}
  btn.disabled=true;btn.textContent='提交中...';
  try{
    await api('/reports/daily','POST',{date:today(),completed,plan,issues});
    msgEl.textContent='日报提交成功！';msgEl.className='smsg smsg-ok';msgEl.style.display='block';
    toast('日报已提交');setTimeout(()=>msgEl.style.display='none',3000);
  }catch(err){msgEl.textContent='提交失败：'+err.message;msgEl.className='smsg smsg-err';msgEl.style.display='block';}
  btn.disabled=false;btn.textContent='提交日报';
});

document.getElementById('btn-copy-dr').addEventListener('click',async()=>{
  const tasks=await api('/tasks?date='+today());
  const lines=(tasks||[]).map((t,i)=>`${i+1}. ${t.content}${t.done?'':'（进行中）'}`).join('\n');
  const plan=document.getElementById('dr-plan').value.trim();
  const issues=document.getElementById('dr-issues').value.trim();
  const text=`【日报 · ${today()}】\n\n一、今日工作完成情况\n${lines||'（无）'}\n\n二、明日工作计划\n${plan||'（无）'}\n\n三、问题和风险\n${issues||'无'}`;
  navigator.clipboard.writeText(text).then(()=>toast('已复制到剪贴板 ✓')).catch(()=>toast('复制失败'));
});

// ── REPORTS TAB ────────────────────────────────────────────────
async function loadDailyList(){
  const month=document.getElementById('filter-month').value;
  const el=document.getElementById('daily-list');el.innerHTML='<p class="hint">加载中...</p>';
  try{
    const list=await api('/reports/daily?month='+month);
    if(!list||!list.length){el.innerHTML='<p class="hint">本月暂无日报</p>';return;}
    el.innerHTML=list.map(r=>`
      <div class="ritem">
        <div class="ritem-hd"><span class="rdate">${r.date}</span></div>
        <div class="ritem-body">
          <div><div class="rsec-label">一、今日完成</div><div class="rsec-body">${esc(r.completed)}</div></div>
          ${r.plan?`<div><div class="rsec-label">二、明日计划</div><div class="rsec-body">${esc(r.plan)}</div></div>`:''}
          ${r.issues?`<div><div class="rsec-label">三、问题和风险</div><div class="rsec-body">${esc(r.issues)}</div></div>`:''}
        </div>
      </div>`).join('');
  }catch(e){el.innerHTML='<p class="hint" style="color:var(--danger)">加载失败</p>';}
}
document.getElementById('btn-load-daily').addEventListener('click',loadDailyList);

async function loadWeeklyList(){
  const el=document.getElementById('weekly-list');el.innerHTML='<p class="hint">加载中...</p>';
  try{
    const list=await api('/reports/weekly');
    if(!list||!list.length){el.innerHTML='<p class="hint">暂无周报</p>';return;}
    el.innerHTML=list.map(r=>`<div class="ritem"><div class="ritem-hd"><span class="rdate">周报 · ${r.week_start}</span>${r.auto_generated?'<span class="badge">AI</span>':''}</div><div class="ritem-body">${md(r.content)}</div></div>`).join('');
  }catch(e){el.innerHTML='<p class="hint" style="color:var(--danger)">加载失败</p>';}
}
document.getElementById('btn-gen-weekly').addEventListener('click',async()=>{
  const ws=document.getElementById('weekly-start').value;if(!ws){toast('请选择周一日期');return;}
  const btn=document.getElementById('btn-gen-weekly');btn.disabled=true;btn.textContent='生成中...';
  try{await api('/reports/weekly','POST',{week_start:ws});toast('周报生成成功');await loadWeeklyList();}catch(e){toast('失败：'+e.message);}
  btn.disabled=false;btn.textContent='AI 生成周报';
});

async function loadMonthlyList(){
  const el=document.getElementById('monthly-list');el.innerHTML='<p class="hint">加载中...</p>';
  try{
    const list=await api('/reports/monthly');
    if(!list||!list.length){el.innerHTML='<p class="hint">暂无月报</p>';return;}
    el.innerHTML=list.map(r=>`<div class="ritem"><div class="ritem-hd"><span class="rdate">月报 · ${r.month}</span>${r.auto_generated?'<span class="badge">AI</span>':''}</div><div class="ritem-body">${md(r.content)}</div></div>`).join('');
  }catch(e){el.innerHTML='<p class="hint" style="color:var(--danger)">加载失败</p>';}
}
document.getElementById('btn-gen-monthly').addEventListener('click',async()=>{
  const m=document.getElementById('monthly-month').value;if(!m){toast('请选择月份');return;}
  const btn=document.getElementById('btn-gen-monthly');btn.disabled=true;btn.textContent='生成中...';
  try{await api('/reports/monthly','POST',{month:m});toast('月报生成成功');await loadMonthlyList();}catch(e){toast('失败：'+e.message);}
  btn.disabled=false;btn.textContent='AI 生成月报';
});

// ── GOALS TAB ──────────────────────────────────────────────────
const lvName={long_term:'长期',monthly:'月目标',weekly:'周目标'};

async function loadGoals(){
  const el=document.getElementById('goals-list');
  try{
    const list=await api('/goals');
    if(!list||!list.length){el.innerHTML='<p class="hint">暂无目标</p>';return;}
    const groups={long_term:[],monthly:[],weekly:[]};list.forEach(g=>groups[g.level]?.push(g));
    let html='';
    for(const[lv,items]of Object.entries(groups)){
      if(!items.length) continue;
      html+=`<div class="gsec-label">${lvName[lv]}</div>`;
      html+=items.map(g=>{
        const pct=g.progress||0;
        return `<div class="gitem">
          <span class="glevel lv-${g.level}">${lvName[g.level]}</span>
          <div class="gtext">
            <div class="gtitle">${esc(g.title)}</div>
            ${g.description?`<div class="gdesc">${esc(g.description)}</div>`:''}
            <div class="goal-progress">
              <div class="progress-bar-wrap"><div class="progress-bar-fill${pct>=100?' done':''}" style="width:${pct}%"></div></div>
              <div class="progress-controls"><span class="progress-pct">${pct}%</span>
                <div class="progress-btns">
                  <button class="progress-btn" data-id="${g.id}" data-delta="-10">−</button>
                  <button class="progress-btn" data-id="${g.id}" data-delta="10">+</button>
                </div>
              </div>
            </div>
          </div>
          <button class="gdone" data-id="${g.id}" data-title="${ea(g.title)}" data-desc="${ea(g.description)}">完成</button>
        </div>`;
      }).join('');
    }
    el.innerHTML=html;
    el.querySelectorAll('.gdone').forEach(btn=>{
      btn.addEventListener('click',async()=>{
        if(!confirm(`确认将「${btn.dataset.title}」标记为完成？`)) return;
        try{await api('/goals/'+btn.dataset.id,'PUT',{title:btn.dataset.title,description:btn.dataset.desc,status:'completed'});toast('目标已完成 ✓');loadGoals();loadSidebarGoals();}
        catch(e){toast('失败：'+e.message);}
      });
    });
    el.querySelectorAll('.progress-btn').forEach(btn=>{
      btn.addEventListener('click',async()=>{
        const id=btn.dataset.id,delta=parseInt(btn.dataset.delta);
        const pctEl=btn.closest('.gitem').querySelector('.progress-pct');
        const cur=Math.max(0,Math.min(100,parseInt(pctEl.textContent)+delta));
        try{await api('/goals/'+id,'PUT',{progress:cur});pctEl.textContent=cur+'%';
          const fill=btn.closest('.gitem').querySelector('.progress-bar-fill');fill.style.width=cur+'%';fill.classList.toggle('done',cur>=100);
          loadSidebarGoals();}catch(e){toast('更新失败：'+e.message);}
      });
    });
  }catch(e){el.innerHTML='<p class="hint" style="color:var(--danger)">加载失败</p>';}
}
document.getElementById('btn-refresh-goals').addEventListener('click',loadGoals);
document.getElementById('goal-form').addEventListener('submit',async e=>{
  e.preventDefault();const btn=e.target.querySelector('button[type=submit]');btn.disabled=true;btn.textContent='添加中...';
  try{await api('/goals','POST',{title:document.getElementById('g-title').value,description:document.getElementById('g-desc').value,level:document.getElementById('g-level').value});
    document.getElementById('g-title').value='';document.getElementById('g-desc').value='';
    toast('目标已添加');loadGoals();loadSidebarGoals();}catch(e){toast('失败：'+e.message);}
  btn.disabled=false;btn.textContent='添加';
});

// ── NOTIFICATIONS ──────────────────────────────────────────────
function setupNotifications() {
  updateNotifStatus();
  // Check reminder every minute
  setInterval(checkReminder, 60000);
  checkReminder();
}

function updateNotifStatus() {
  const el=document.getElementById('notif-status');
  if(!('Notification' in window)){el.textContent='不支持';return;}
  const perm=Notification.permission;
  el.textContent = perm==='granted'?'已授权':perm==='denied'?'已拒绝':'未授权';
  el.className='notif-status'+(perm==='granted'?' granted':'');
}

document.getElementById('btn-req-notif').addEventListener('click',async()=>{
  if(!('Notification' in window)){toast('该浏览器不支持通知');return;}
  const perm=await Notification.requestPermission();
  updateNotifStatus();
  if(perm==='granted') toast('通知权限已授权');
  else toast('通知权限被拒绝，请在浏览器设置中手动允许');
});

async function checkReminder() {
  if(Notification.permission!=='granted') return;
  try{
    const profile=await api('/profile');
    const reminderTime=profile['profile_reminder_time'];
    if(!reminderTime) return;
    const now=new Date();
    const[rh,rm]=reminderTime.split(':').map(Number);
    if(now.getHours()!==rh||now.getMinutes()!==rm) return;
    // Check if today's report has been submitted
    const list=await api('/reports/daily?date='+today());
    if(list&&list.length) return; // already submitted
    new Notification('WorkLog 日报提醒', {
      body: `现在是 ${reminderTime}，别忘了填写今天的日报！`,
      icon: '/favicon.ico',
    });
  }catch(_){}
}

// ── SETTINGS ──────────────────────────────────────────────────
const PROVIDERS=[
  {id:'deepseek', name:'DeepSeek',  type:'openai_compat',base:'https://api.deepseek.com/v1',                       model:'deepseek-chat',    reg:'https://platform.deepseek.com/'},
  {id:'anthropic',name:'Anthropic', type:'anthropic',    base:'',                                                   model:'claude-sonnet-4-6',reg:'https://console.anthropic.com/'},
  {id:'openai',   name:'OpenAI',    type:'openai_compat',base:'https://api.openai.com/v1',                          model:'gpt-4o',           reg:'https://platform.openai.com/'},
  {id:'qwen',     name:'通义千问',  type:'openai_compat',base:'https://dashscope.aliyuncs.com/compatible-mode/v1', model:'qwen-plus',        reg:'https://bailian.console.aliyun.com/'},
  {id:'zhipu',    name:'智谱 GLM',  type:'openai_compat',base:'https://open.bigmodel.cn/api/paas/v4',              model:'glm-4-flash',      reg:'https://open.bigmodel.cn/'},
  {id:'custom',   name:'自定义',    type:'openai_compat',base:'',                                                   model:'',                 reg:''},
];
function matchPreset(type,base){if(type==='anthropic') return PROVIDERS.find(p=>p.id==='anthropic');return PROVIDERS.find(p=>p.type===type&&p.base===base)||PROVIDERS.find(p=>p.id==='custom');}

async function openSettings(){
  // Load provider config
  let cfg={};try{cfg=await api('/settings');}catch(_){}
  const ap=cfg['provider_type']?matchPreset(cfg['provider_type'],cfg['provider_base_url']||''):null;
  const body=document.getElementById('settings-body');body.innerHTML='';
  PROVIDERS.forEach(p=>{
    const isA=ap?.id===p.id,hasK=isA&&cfg['provider_api_key']!=='';
    const row=document.createElement('div');
    row.className=`provider-row${hasK?' has-key':''}${isA?' active-provider':''}`;row.id=`prow-${p.id}`;
    const sc=isA?'active':(hasK?'ready':'none'),st=isA?'当前使用':(hasK?'已配置':'未配置');
    const kv=isA?(cfg['provider_api_key']||''):'',md2=isA?(cfg['provider_model']||p.model):p.model,bd=isA?(cfg['provider_base_url']||p.base):p.base;
    row.innerHTML=`
      <div class="provider-row-header"><span class="provider-row-name">${p.name}</span><span class="provider-status ${sc}" id="pstatus-${p.id}">${st}</span>${p.reg?`<a class="provider-register" href="${p.reg}" target="_blank">注册 →</a>`:''}</div>
      <div class="key-row"><input class="key-input" id="pkey-${p.id}" type="password" placeholder="API Key" value="${esc(kv)}" oninput="onKeyInput('${p.id}')" autocomplete="off"/><button class="key-clear" onclick="clearKey('${p.id}')" title="清除">✕</button></div>
      <span class="advanced-toggle" id="padv-toggle-${p.id}" onclick="toggleAdv('${p.id}')">${isA?'▾':'▸'} 高级选项</span>
      <div class="advanced-fields${isA?' open':''}" id="padv-${p.id}">
        <label>模型名称（默认：${p.model||'—'}）</label><input id="pmodel-${p.id}" placeholder="${p.model}" value="${esc(md2)}" autocomplete="off"/>
        ${p.type==='openai_compat'?`<label>API 地址${p.base?'（默认：'+p.base+'）':''}</label><input id="pbase-${p.id}" placeholder="${p.base}" value="${esc(bd)}" autocomplete="off"/>`:''}
      </div>`;
    body.appendChild(row);
  });

  // Load profile
  try{
    const p=await api('/profile');
    document.getElementById('p-name').value    =p['profile_name']||'';
    document.getElementById('p-role').value    =p['profile_role']||'';
    document.getElementById('p-team').value    =p['profile_team']||'';
    document.getElementById('p-stage').value   =p['profile_stage']||'';
    document.getElementById('p-projects').value=p['profile_projects']||'';
    document.getElementById('p-reminder').value=p['profile_reminder_time']||'17:30';
  }catch(_){}

  document.getElementById('settings-modal').classList.add('open');
}

function toggleAdv(id){const el=document.getElementById(`padv-${id}`),tg=document.getElementById(`padv-toggle-${id}`);tg.textContent=(el.classList.toggle('open')?'▾':'▸')+' 高级选项';}
function onKeyInput(id){
  const key=document.getElementById(`pkey-${id}`).value.trim();
  const row=document.getElementById(`prow-${id}`),status=document.getElementById(`pstatus-${id}`);
  if(key&&!row.classList.contains('active-provider')){
    document.querySelectorAll('.provider-row').forEach(r=>{if(r.id!==`prow-${id}`){r.classList.remove('active-provider','has-key');const s=r.querySelector('.provider-status');if(s&&s.classList.contains('active')){s.className='provider-status none';s.textContent='未配置';}}});
  }
  if(key){row.classList.add('has-key','active-provider');status.className='provider-status active';status.textContent='当前使用';}
  else{row.classList.remove('has-key','active-provider');status.className='provider-status none';status.textContent='未配置';}
}
function clearKey(id){document.getElementById(`pkey-${id}`).value='';onKeyInput(id);}
function closeSettings(){document.getElementById('settings-modal').classList.remove('open');}

document.getElementById('btn-open-settings').addEventListener('click',openSettings);
document.getElementById('btn-close-settings').addEventListener('click',closeSettings);
document.getElementById('btn-cancel-settings').addEventListener('click',closeSettings);
document.getElementById('btn-cancel-profile').addEventListener('click',closeSettings);
document.getElementById('settings-modal').addEventListener('click',e=>{if(e.target===document.getElementById('settings-modal'))closeSettings();});

document.getElementById('btn-save-settings').addEventListener('click',async()=>{
  const btn=document.getElementById('btn-save-settings');
  let chosen=null;
  for(const p of PROVIDERS){const key=document.getElementById(`pkey-${p.id}`)?.value.trim();if(key){chosen={p,key};break;}}
  if(!chosen){toast('请在至少一个供应商处填写 API Key');return;}
  const{p,key}=chosen;
  const model=document.getElementById(`pmodel-${p.id}`)?.value.trim()||p.model;
  const base=document.getElementById(`pbase-${p.id}`)?.value.trim()||p.base;
  btn.disabled=true;btn.textContent='保存中...';
  try{await api('/settings','POST',{provider_type:p.type,api_key:key,base_url:base,model});toast(`已切换到 ${p.name}`);updateProviderBadge();setTimeout(closeSettings,800);}
  catch(e){toast('保存失败：'+e.message);}
  btn.disabled=false;btn.textContent='保存配置';
});

document.getElementById('btn-save-profile').addEventListener('click',async()=>{
  const btn=document.getElementById('btn-save-profile');
  btn.disabled=true;btn.textContent='保存中...';
  try{
    await api('/profile','POST',{
      profile_name:    document.getElementById('p-name').value.trim(),
      profile_role:    document.getElementById('p-role').value.trim(),
      profile_team:    document.getElementById('p-team').value.trim(),
      profile_stage:   document.getElementById('p-stage').value,
      profile_projects:document.getElementById('p-projects').value.trim(),
      profile_reminder_time: document.getElementById('p-reminder').value,
    });
    toast('个人档案已保存，AI 导师将在下次对话中使用');
    setTimeout(closeSettings,800);
  }catch(e){toast('保存失败：'+e.message);}
  btn.disabled=false;btn.textContent='保存档案';
});

async function updateProviderBadge(){
  try{const cfg=await api('/settings');if(!cfg['provider_type']){document.getElementById('provider-badge').textContent='配置 AI';return;}const p=matchPreset(cfg['provider_type'],cfg['provider_base_url']||'');document.getElementById('provider-badge').textContent=p?.name||'已配置';}
  catch(_){document.getElementById('provider-badge').textContent='配置 AI';}
}

// ── Boot ───────────────────────────────────────────────────────
init();
