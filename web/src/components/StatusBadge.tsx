interface StatusBadgeProps {
  status: "healthy" | "degraded" | "unknown" | "offline";
  label?: string;
}

const colors = {
  healthy: "bg-green-100 text-green-800",
  degraded: "bg-yellow-100 text-yellow-800",
  unknown: "bg-gray-100 text-gray-800",
  offline: "bg-red-100 text-red-800",
};

const dots = {
  healthy: "bg-green-400",
  degraded: "bg-yellow-400",
  unknown: "bg-gray-400",
  offline: "bg-red-400",
};

export default function StatusBadge({ status, label }: StatusBadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${colors[status]}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${dots[status]}`} />
      {label || status}
    </span>
  );
}
