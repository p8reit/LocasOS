import React from 'react';
import { fetchHardware, type Hardware } from '../services/hardware';

export function HardwarePanel() {
  const [hardware, setHardware] = React.useState<Hardware | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetchHardware(controller.signal)
      .then(data => { if (!controller.signal.aborted) setHardware(data); })
      .catch(err => { if (!controller.signal.aborted) setError(String(err.message ?? err)); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, []);

  if (loading) return <section role="status">Loading hardware telemetry…</section>;
  if (error) return <section role="alert"><h3>Hardware unavailable</h3><p>{error}</p></section>;
  if (!hardware) return <section role="status">No hardware telemetry returned.</section>;
  return <section><h3>{hardware.node}</h3>
    {hardware.gpus.length === 0 ? <p>No GPU reported.</p> :
      hardware.gpus.map((gpu, index) => <article key={gpu.id || String(index)}>
        <h4>{gpu.name}</h4>
        <p>VRAM: {gpu.memory_used_mib ?? 'Unknown'} / {gpu.memory_total_mib} MiB</p>
        <p>Utilization: {gpu.utilization_percent == null ? 'Unknown' : gpu.utilization_percent + '%'}</p>
      </article>)}
  </section>;
}
