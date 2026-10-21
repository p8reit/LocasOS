export type GPU = {
  id: string;
  name: string;
  memory_total_mib: number;
  memory_used_mib?: number;
  utilization_percent?: number;
};
export type Hardware = { node: string; gpus: GPU[] };

export async function fetchHardware(signal?: AbortSignal): Promise<Hardware> {
  const response = await fetch('/api/v1/hardware', {
    method: 'GET',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    signal,
  });
  if (response.status === 401 || response.status === 403) {
    throw new Error('Authentication is required for hardware telemetry.');
  }
  if (!response.ok) throw new Error('Hardware telemetry is unavailable.');
  const data: unknown = await response.json();
  if (!data || typeof data !== 'object' || !('node' in data) || !('gpus' in data) ||
      typeof data.node !== 'string' || !Array.isArray(data.gpus)) {
    throw new Error('Invalid hardware response.');
  }
  return data as Hardware;
}
