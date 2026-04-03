import { useEffect, useState } from "react";
import { useRouter } from "next/router";
import Link from "next/link";
import Layout from "@/components/Layout";
import StatusBadge from "@/components/StatusBadge";
import { apiFetch } from "@/hooks/useApi";

interface Cluster {
  id: string;
  name: string;
  stage: string;
  type: string;
  clusterSize: string;
  createdAt: string;
  status?: {
    syncReady?: boolean;
    lastSeenAt?: string;
    componentsOk?: number;
    componentsTotal?: number;
    helmreleasesRunning?: number;
    helmreleasesFailing?: number;
  };
}

function getClusterHealth(cluster: Cluster): "healthy" | "degraded" | "offline" | "unknown" {
  if (!cluster.status?.lastSeenAt) return "unknown";
  const lastSeen = new Date(cluster.status.lastSeenAt);
  const fiveMinAgo = new Date(Date.now() - 5 * 60 * 1000);
  if (lastSeen < fiveMinAgo) return "offline";
  if (cluster.status.helmreleasesFailing && cluster.status.helmreleasesFailing > 0) return "degraded";
  if (cluster.status.syncReady) return "healthy";
  return "degraded";
}

export default function ClustersPage() {
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const router = useRouter();

  useEffect(() => {
    apiFetch<Cluster[]>("/api/v1/clusters")
      .then(setClusters)
      .catch(() => router.push("/login"))
      .finally(() => setLoading(false));
  }, [router]);

  return (
    <Layout title="Clusters">
      <div className="flex justify-between items-center mb-6">
        <p className="text-gray-600">
          {clusters.length} cluster{clusters.length !== 1 ? "s" : ""}
        </p>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700"
        >
          Add cluster
        </button>
      </div>

      {loading ? (
        <div className="text-center py-12 text-gray-500">Loading...</div>
      ) : clusters.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500 mb-4">No clusters yet</p>
          <button
            onClick={() => setShowCreate(true)}
            className="px-4 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700"
          >
            Create your first cluster
          </button>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {clusters.map((cluster) => (
            <Link
              key={cluster.id}
              href={`/clusters/${cluster.id}`}
              className="block bg-white rounded-xl border border-gray-200 p-5 hover:shadow-md transition-shadow"
            >
              <div className="flex justify-between items-start mb-3">
                <h3 className="font-semibold text-gray-900">{cluster.name}</h3>
                <StatusBadge status={getClusterHealth(cluster)} />
              </div>
              <div className="space-y-1 text-sm text-gray-500">
                <p>Stage: <span className="text-gray-700">{cluster.stage}</span></p>
                <p>Type: <span className="text-gray-700">{cluster.type}</span></p>
                <p>Size: <span className="text-gray-700">{cluster.clusterSize}</span></p>
              </div>
              {cluster.status && (
                <div className="mt-3 pt-3 border-t border-gray-100 text-xs text-gray-500">
                  {cluster.status.helmreleasesRunning !== undefined && (
                    <span>
                      {cluster.status.helmreleasesRunning} HelmReleases running
                      {cluster.status.helmreleasesFailing ? `, ${cluster.status.helmreleasesFailing} failing` : ""}
                    </span>
                  )}
                </div>
              )}
            </Link>
          ))}
        </div>
      )}

      {showCreate && (
        <CreateClusterModal
          onClose={() => setShowCreate(false)}
          onCreate={(cluster) => {
            setClusters([cluster, ...clusters]);
            setShowCreate(false);
          }}
        />
      )}
    </Layout>
  );
}

interface CreateClusterModalProps {
  onClose: () => void;
  onCreate: (cluster: Cluster) => void;
}

function CreateClusterModal({ onClose, onCreate }: CreateClusterModalProps) {
  const [name, setName] = useState("");
  const [stage, setStage] = useState("dev");
  const [type, setType] = useState("worker");
  const [clusterSize, setClusterSize] = useState("medium");
  const [gitPath, setGitPath] = useState("");
  const [dnsName, setDnsName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setGitPath(name ? `clusters/${name}` : "");
  }, [name]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const cluster = await apiFetch<Cluster>("/api/v1/clusters", {
        method: "POST",
        body: { name, stage, type, clusterSize, gitPath, dnsName },
      });
      onCreate(cluster);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create cluster");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl shadow-xl max-w-lg w-full mx-4 p-6">
        <h2 className="text-lg font-semibold mb-4">Add cluster</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="bg-red-50 text-red-700 px-3 py-2 rounded text-sm">{error}</div>
          )}
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input type="text" required value={name} onChange={(e) => setName(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-brand-500 focus:border-brand-500" />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-sm font-medium text-gray-700">Stage</label>
              <select value={stage} onChange={(e) => setStage(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg">
                <option value="dev">Dev</option>
                <option value="staging">Staging</option>
                <option value="prod">Prod</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Type</label>
              <select value={type} onChange={(e) => setType(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg">
                <option value="worker">Worker</option>
                <option value="controlplane">Control Plane</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Size</label>
              <select value={clusterSize} onChange={(e) => setClusterSize(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg">
                <option value="small">Small</option>
                <option value="medium">Medium</option>
                <option value="large">Large</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">DNS Name</label>
            <input type="text" value={dnsName} onChange={(e) => setDnsName(e.target.value)} placeholder="cluster.example.com" className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-brand-500 focus:border-brand-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Git path</label>
            <input type="text" required value={gitPath} onChange={(e) => setGitPath(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-brand-500 focus:border-brand-500" />
          </div>
          <div className="flex justify-end space-x-3 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50">
              Cancel
            </button>
            <button type="submit" disabled={loading} className="px-4 py-2 text-sm font-medium text-white bg-brand-600 rounded-lg hover:bg-brand-700 disabled:opacity-50">
              {loading ? "Creating..." : "Create cluster"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
