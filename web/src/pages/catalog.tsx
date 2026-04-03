import { useEffect, useState } from "react";
import { useRouter } from "next/router";
import Layout from "@/components/Layout";
import { apiFetch } from "@/hooks/useApi";

interface Service {
  name: string;
  description: string;
  category: string;
  helmChart: string;
  version: string;
  dependencies?: string[];
  defaultValues?: Record<string, Record<string, unknown>>;
}

interface Catalog {
  services: Service[];
}

const categoryColors: Record<string, string> = {
  gitops: "bg-purple-100 text-purple-800",
  security: "bg-blue-100 text-blue-800",
  networking: "bg-green-100 text-green-800",
  observability: "bg-amber-100 text-amber-800",
  backup: "bg-red-100 text-red-800",
  operations: "bg-gray-100 text-gray-800",
};

export default function CatalogPage() {
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const router = useRouter();

  useEffect(() => {
    apiFetch<Catalog>("/api/v1/catalog")
      .then(setCatalog)
      .catch(() => router.push("/login"));
  }, [router]);

  if (!catalog) {
    return <Layout title="Service Catalog"><div className="text-center py-12 text-gray-500">Loading...</div></Layout>;
  }

  const categories = [...new Set(catalog.services.map((s) => s.category))];

  return (
    <Layout title="Service Catalog">
      <p className="text-gray-600 mb-6">
        Browse the curated platform services available for your clusters.
      </p>

      {categories.map((category) => (
        <div key={category} className="mb-8">
          <h2 className="text-lg font-semibold text-gray-900 mb-3 capitalize">{category}</h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {catalog.services
              .filter((s) => s.category === category)
              .map((svc) => (
                <div
                  key={svc.name}
                  className="bg-white rounded-xl border border-gray-200 p-5"
                >
                  <div className="flex justify-between items-start mb-2">
                    <h3 className="font-medium text-gray-900">{svc.name}</h3>
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                        categoryColors[svc.category] || "bg-gray-100 text-gray-800"
                      }`}
                    >
                      {svc.category}
                    </span>
                  </div>
                  <p className="text-sm text-gray-600 mb-3">{svc.description}</p>
                  <div className="flex justify-between items-center text-xs text-gray-500">
                    <span>v{svc.version}</span>
                    <span>{svc.helmChart}</span>
                  </div>
                  {svc.dependencies && svc.dependencies.length > 0 && (
                    <div className="mt-2 pt-2 border-t border-gray-100 text-xs text-gray-400">
                      Depends on: {svc.dependencies.join(", ")}
                    </div>
                  )}
                </div>
              ))}
          </div>
        </div>
      ))}
    </Layout>
  );
}
