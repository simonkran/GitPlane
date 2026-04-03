import Link from "next/link";
import { useRouter } from "next/router";
import { ReactNode } from "react";
import { useAuth } from "@/hooks/useApi";

interface LayoutProps {
  children: ReactNode;
  title?: string;
}

export default function Layout({ children, title }: LayoutProps) {
  const router = useRouter();
  const { logout } = useAuth();

  const nav = [
    { href: "/clusters", label: "Clusters" },
    { href: "/catalog", label: "Catalog" },
    { href: "/settings", label: "Settings" },
  ];

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex">
              <Link
                href="/clusters"
                className="flex items-center px-2 text-xl font-bold text-brand-600"
              >
                GitPlane
              </Link>
              <div className="hidden sm:ml-8 sm:flex sm:space-x-4">
                {nav.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`inline-flex items-center px-3 py-2 text-sm font-medium rounded-md ${
                      router.pathname.startsWith(item.href)
                        ? "text-brand-600 bg-brand-50"
                        : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
                    }`}
                  >
                    {item.label}
                  </Link>
                ))}
              </div>
            </div>
            <div className="flex items-center">
              <button
                onClick={logout}
                className="text-sm text-gray-500 hover:text-gray-700"
              >
                Sign out
              </button>
            </div>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {title && (
          <h1 className="text-2xl font-bold text-gray-900 mb-6">{title}</h1>
        )}
        {children}
      </main>
    </div>
  );
}
