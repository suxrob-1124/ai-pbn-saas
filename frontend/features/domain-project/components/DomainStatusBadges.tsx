import { FiAlertTriangle, FiCheck, FiClock, FiPause, FiPlay, FiRefreshCw, FiX } from "react-icons/fi";
import { Badge } from "../../../components/Badge";
import { getLinkTaskStatusMeta } from "../../../lib/linkTaskStatus";
import { getGenerationStatusMeta } from "../services/statusCta";

export function DomainStatusBadge({ status }: { status: string }) {
  const meta = getGenerationStatusMeta(status);
  const icon =
    meta.icon === "play"
      ? <FiPlay />
      : meta.icon === "pause"
        ? <FiPause />
        : meta.icon === "check"
          ? <FiCheck />
          : meta.icon === "alert"
            ? <FiAlertTriangle />
            : meta.icon === "x"
              ? <FiX />
              : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

export function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === "refresh" ? <FiRefreshCw /> : meta.icon === "check" ? <FiCheck /> : meta.icon === "alert" ? <FiAlertTriangle /> : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}
