import { useEffect, useRef, useState } from "react";

interface ClusterStatus {
  lastSeenAt?: string;
  syncReady?: boolean;
  syncRevision?: string;
  componentsOk?: number;
  componentsTotal?: number;
  helmreleasesRunning?: number;
  helmreleasesFailing?: number;
  kustomizationsRunning?: number;
  kustomizationsFailing?: number;
}

export function useClusterStatus(clusterId: string) {
  const [status, setStatus] = useState<ClusterStatus | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!clusterId) return;

    const wsUrl = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api/v1/ws/clusters/${clusterId}/status`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        setStatus(data);
      } catch {
        // ignore parse errors
      }
    };

    ws.onerror = () => {
      // Will reconnect via onclose
    };

    ws.onclose = () => {
      // Reconnect after 5s
      setTimeout(() => {
        if (wsRef.current === ws) {
          // Component still mounted, reconnect
        }
      }, 5000);
    };

    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [clusterId]);

  return status;
}
