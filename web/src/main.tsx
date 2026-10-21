import React from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';
import { HardwarePanel } from './components/HardwarePanel';
const pages = ['Dashboard', 'Applications', 'Models', 'Jobs', 'Hardware', 'Settings'] as const;
type Page = typeof pages[number];
function App() {
  const [page, setPage] = React.useState<Page>('Dashboard');
  return <div className="app"><aside><h1>LOCAS OS</h1><p>AI Compute Control Center</p><nav aria-label="Primary navigation">{pages.map(p=><button key={p} aria-current={page===p?'page':undefined} onClick={()=>setPage(p)}>{p}</button>)}</nav></aside><main><header><h2>{page}</h2><span className="status">Development preview</span></header>{page === 'Hardware' ? <HardwarePanel /> : <section><h3>{page} service not connected</h3><p>This is an interface scaffold. Live telemetry and management actions will become available when the LOCAS API is implemented.</p></section>}</main></div>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><App/></React.StrictMode>);
