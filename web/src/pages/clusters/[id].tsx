import { useEffect, useState } from "react";
import { useRouter } from "next/router";
import Layout from "@/components/Layout";
import StatusBadge from "@/components/StatusBadge";
import { apiFetch } from "@/hooks/useApi";
import { useClusterStatus } from "@/hooks/useClusterStatus";

interface ClusterDetail {
  id: string;
  name: string;
  stage: string;
  type: string;
  clusterSize: string;
  dnsName: string;
  gitPath: string;
  agentToken?: string;
  createdAt: string;
  status?: {
    lastSeenAt?: string;
    syncReady?: boolean;
    syncRevision?: string;
    componentsOk?: number;
    componentsTotal?: number;
    helmreleasesRunning?: number;
    helmreleasesFailing?: number;
    kustomizationsRunning?: number;
    kustomizationsFailing?: number;
  };
}

interface ServiceEntry {
  serviceName: string;
  status: string;
  catalogInfo?: {
    name: string;
    description: string;
    category: string;
    version: string;
    dependencies?: string[];
  };
}

interface HistoryEntry {
  id: string;
  gitCommitSha?: string;
  status: string;
  createdAt: string;
}

export default function ClusterDetailPage() {
  const router = useRouter();
  const { id } = router.query;
  const [cluster, setCluster] = useState<ClusterDetail | null>(null);
  const [services, setServices] = useState<ServiceEntry[]>([]);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [activeTab, setActiveTab] = useState<"overview" | "services" | "history" | "agent">("overview");
  const [generating, setGenerating] = useState(false);

  const liveStatus = useClusterStatus(id as string);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      apiFetch<ClusterDetail>(`/api/v1/clusters/${id}`),
      apiFetch<ServiceEntry[]>(`/api/v1/clusters/${id}/services`),
      apiFetch<HistoryEntry[]>(`/api/v1/clusters/${id}/history`),
    ])
      .then(([c, s, h]) => {
        setCluster(c);
        setServices(s);
        setHistory(h);
      })
      .catch(() => router.push("/clusters"));
  }, [id, router]);

  const handleToggleService = async (serviceName: string, currentStatus: string) => {
    const newStatus = currentStatus === "enabled" ? "disabled" : "enabled";
    try {
      await apiFetch(`/api/v1/clusters/${id}/services/${serviceName}`, {
        method: "PUT",
        body: { status: newStatus },
      });
      setServices(
        services.map((s) =>
          s.serviceName === serviceName ? { ...s, status: newStatus } : s
        )
      );
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to update service");
    }
  };

  const handleGenerate = async () => {
    setGenerating(true);
    try {
      await apiFetch(`/api/v1/clusters/${id}/generate`, { method: "POST" });
      const h = await apiFetch<HistoryEntry[]>(`/api/v1/clusters/${id}/history`);
      setHistory(h);
    } catch (err) {
      alert(err instanceof Error ? err.message : "Generation failed");
    } finally {
      setGenerating(false);
    }
  };

  if (!cluster) {
    return <Layout><div className="text-center py-12 text-gray-500">Loading...</div></Layout>;
  }

  const status = liveStatus || cluster.status;

  const tabs = [
    { key: "overview" as const, label: "Overview" },
    { key: "services" as const, label: "Services" },
    { key: "history" as const, label: "History" },
    { key: "agent" as const, label: "Agent" },
  ];

  return (
    <Layout>
      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{cluster.name}</h1>
          <p className="text-gray-500 mt-1">
            {cluster.stage} / {cluster.type} / {cluster.clusterSize}
          </p>
        </div>
        <button
          onClick={handleGenerate}
          disabled={generating}
          className="px-4 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700 disabled:opacity-50"
        >
          {generating ? "Generating..." : "Generate & Deploy"}
        </button>
      </div>

      <div className="border-b border-gray-200 mb-6">
        <nav className="flex space-x-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`pb-3 text-sm font-medium border-b-2 ${
                activeTab === tab.key
                  ? "border-brand-600 text-brand-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {activeTab === "overview" && (
        <div className="grid gap-6 md:grid-cols-2">
          <div className="bg-white rounded-xl border border-gray-200 p-5">
            <h3 className="font-semibold text-gray-900 mb-4">Sync Status</h3>
            {status ? (
              <div className="space-y-3">
                <div className="flex justify-between">
                  <span className="text-sm text-gray-500">Ready</span>
                  <StatusBadge
                    status={status.syncReady ? "healthy" : "degraded"}
                    label={status.syncReady ? "Synced" : "Not synced"}
                  />
                </div>
                {status.syncRevision && (
                  <div className="flex justify-between">
                    <span className="text-sm text-gray-500">Revision</span>
                    <code className="text-xs bg-gray-100 px-2 py-0.5 rounded">
                      {status.syncRevision.slice(0, 8)}
                    </code>
                  </div>
                )}
                {status.lastSeenAt && (
                  <div className="flex justify-between">
                    <span className="text-sm text-gray-500">Last seen</span>
                    <span className="text-sm text-gray-700">
                      {new Date(status.lastSeenAt).toLocaleString()}
                    </span>
                  </div>
                )}
              </div>
            ) : (
              <p className="text-sm text-gray-500">No status data. Install the agent to see live status.</p>
            )}
          </div>

          <div className="bg-white rounded-xl border border-gray-200 p-5">
            <h3 className="font-semibold text-gray-900 mb-4">Components</h3>
            {status?.componentsTotal ? (
              <div className="space-y-3">
                <div className="flex justify-between">
                  <span className="text-sm text-gray-500">Healthy</span>
                  <span className="text-sm font-medium">
                    {status.componentsOk} / {status.componentsTotal}
                  </span>
                </div>
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div
                    className="bg-green-500 h-2 rounded-full"
                    style={{
                      width: `${((status.componentsOk || 0) / status.componentsTotal) * 100}%`,
                    }}
                  />
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-500">No component data available.</p>
            )}
          </div>

          <div className="bg-white rounded-xl border border-gray-200 p-5">
            <h3 className="font-semibold text-gray-900 mb-4">Reconcilers</h3>
            {status?.helmreleasesRunning !== undefined ? (
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-gray-500">HelmReleases running</span>
                  <span className="font-medium text-green-600">{status.helmreleasesRunning}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">HelmReleases failing</span>
                  <span className="font-medium text-red-600">{status.helmreleasesFailing || 0}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Kustomizations running</span>
                  <span className="font-medium text-green-600">{status.kustomizationsRunning || 0}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Kustomizations failing</span>
                  <span className="font-medium text-red-600">{status.kustomizationsFailing || 0}</span>
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-500">No reconciler data available.</p>
            )}
          </div>

          <div className="bg-white rounded-xl border border-gray-200 p-5">
            <h3 className="font-semibold text-gray-900 mb-4">Cluster Info</h3>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-gray-500">DNS</span>
                <span className="text-gray-700">{cluster.dnsName || "—"}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Git path</span>
                <code className="text-xs bg-gray-100 px-2 py-0.5 rounded">{cluster.gitPath}</code>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Created</span>
                <span className="text-gray-700">{new Date(cluster.createdAt).toLocaleDateString()}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === "services" && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {services.map((svc) => (
            <div
              key={svc.serviceName}
              className="bg-white rounded-xl border border-gray-200 p-5"
            >
              <div className="flex justify-between items-start mb-2">
                <div>
                  <h4 className="font-medium text-gray-900">{svc.serviceName}</h4>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {svc.catalogInfo?.category} &middot; v{svc.catalogInfo?.version}
                  </p>
                </div>
                <button
                  onClick={() => handleToggleService(svc.serviceName, svc.status)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    svc.status === "enabled" ? "bg-brand-600" : "bg-gray-200"
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      svc.status === "enabled" ? "translate-x-6" : "translate-x-1"
                    }`}
                  />
                </button>
              </div>
              <p className="text-sm text-gray-600">{svc.catalogInfo?.description}</p>
              {svc.catalogInfo?.dependencies && svc.catalogInfo.dependencies.length > 0 && (
                <p className="text-xs text-gray-400 mt-2">
                  Requires: {svc.catalogInfo.dependencies.join(", ")}
                </p>
              )}
            </div>
          ))}
        </div>
      )}

      {activeTab === "history" && (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Commit</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Date</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {history.length === 0 ? (
                <tr>
                  <td colSpan={3} className="px-6 py-8 text-center text-sm text-gray-500">
                    No generation history yet
                  </td>
                </tr>
              ) : (
                history.map((entry) => (
                  <tr key={entry.id}>
                    <td className="px-6 py-4">
                      <StatusBadge
                        status={entry.status === "committed" ? "healthy" : entry.status === "failed" ? "offline" : "unknown"}
                        label={entry.status}
                      />
                    </td>
                    <td className="px-6 py-4 text-sm">
                      {entry.gitCommitSha ? (
                        <code className="bg-gray-100 px-2 py-0.5 rounded text-xs">{entry.gitCommitSha.slice(0, 8)}</code>
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {new Date(entry.createdAt).toLocaleString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === "agent" && (
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <h3 className="font-semibold text-gray-900 mb-2">Install Agent</h3>
          <p className="text-sm text-gray-600 mb-4">
            Run the following command to install the GitPlane agent in your cluster:
          </p>
          <div className="bg-gray-900 rounded-lg p-4 overflow-x-auto">
            <code className="text-green-400 text-sm whitespace-pre">
              kubectl apply -f {typeof window !== "undefined" ? window.location.origin : ""}/api/v1/clusters/{id}/agent-install
            </code>
          </div>
          <p className="text-xs text-gray-500 mt-3">
            The agent will start reporting cluster status within 60 seconds of installation.
          </p>
        </div>
      )}
    </Layout>
  );
}
