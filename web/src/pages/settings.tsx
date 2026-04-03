import { useEffect, useState } from "react";
import { useRouter } from "next/router";
import Layout from "@/components/Layout";
import { apiFetch } from "@/hooks/useApi";

interface GitConnection {
  id: string;
  provider: string;
  repoUrl: string;
  defaultBranch: string;
  createdAt: string;
}

export default function SettingsPage() {
  const [connections, setConnections] = useState<GitConnection[]>([]);
  const [showConnect, setShowConnect] = useState(false);
  const router = useRouter();

  useEffect(() => {
    apiFetch<GitConnection[]>("/api/v1/git/status")
      .then(setConnections)
      .catch(() => router.push("/login"));
  }, [router]);

  const handleDisconnect = async () => {
    if (!confirm("Disconnect git repository?")) return;
    try {
      await apiFetch("/api/v1/git/disconnect", { method: "DELETE" });
      setConnections([]);
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to disconnect");
    }
  };

  return (
    <Layout title="Settings">
      <div className="space-y-8">
        <section className="bg-white rounded-xl border border-gray-200 p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Git Connection</h2>
          {connections.length > 0 ? (
            <div className="space-y-4">
              {connections.map((conn) => (
                <div key={conn.id} className="flex justify-between items-center p-4 bg-gray-50 rounded-lg">
                  <div>
                    <p className="font-medium text-gray-900">{conn.repoUrl}</p>
                    <p className="text-sm text-gray-500">
                      {conn.provider} &middot; branch: {conn.defaultBranch}
                    </p>
                  </div>
                  <button
                    onClick={handleDisconnect}
                    className="text-sm text-red-600 hover:text-red-800"
                  >
                    Disconnect
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div>
              <p className="text-sm text-gray-500 mb-3">No git repository connected.</p>
              <button
                onClick={() => setShowConnect(true)}
                className="px-4 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700"
              >
                Connect repository
              </button>
            </div>
          )}
        </section>

        <section className="bg-white rounded-xl border border-gray-200 p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Organization</h2>
          <p className="text-sm text-gray-500">
            Organization management coming soon.
          </p>
        </section>
      </div>

      {showConnect && (
        <ConnectGitModal
          onClose={() => setShowConnect(false)}
          onConnect={(conn) => {
            setConnections([conn, ...connections]);
            setShowConnect(false);
          }}
        />
      )}
    </Layout>
  );
}

interface ConnectGitModalProps {
  onClose: () => void;
  onConnect: (conn: GitConnection) => void;
}

function ConnectGitModal({ onClose, onConnect }: ConnectGitModalProps) {
  const [provider, setProvider] = useState("github");
  const [repoUrl, setRepoUrl] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const conn = await apiFetch<GitConnection>("/api/v1/git/connect", {
        method: "POST",
        body: { provider, repoUrl, accessToken, defaultBranch },
      });
      onConnect(conn);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to connect");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl shadow-xl max-w-lg w-full mx-4 p-6">
        <h2 className="text-lg font-semibold mb-4">Connect Git Repository</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="bg-red-50 text-red-700 px-3 py-2 rounded text-sm">{error}</div>
          )}
          <div>
            <label className="block text-sm font-medium text-gray-700">Provider</label>
            <select value={provider} onChange={(e) => setProvider(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg">
              <option value="github">GitHub</option>
              <option value="gitlab">GitLab</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Repository URL</label>
            <input type="text" required value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} placeholder="https://github.com/org/repo" className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Access Token</label>
            <input type="password" required value={accessToken} onChange={(e) => setAccessToken(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg" />
            <p className="mt-1 text-xs text-gray-500">Personal access token with repo read/write permissions.</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Default Branch</label>
            <input type="text" value={defaultBranch} onChange={(e) => setDefaultBranch(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-lg" />
          </div>
          <div className="flex justify-end space-x-3 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={loading} className="px-4 py-2 text-sm font-medium text-white bg-brand-600 rounded-lg hover:bg-brand-700 disabled:opacity-50">
              {loading ? "Connecting..." : "Connect"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
