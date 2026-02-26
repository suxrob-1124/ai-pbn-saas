"use client";

import { useEffect, useState } from "react";

import { authFetchCached } from "../../../lib/http";

export function useIndexMonitoringScopeLabels(projectId: string, domainScope: string) {
  const [projectName, setProjectName] = useState("");
  const [domainLabel, setDomainLabel] = useState("");

  useEffect(() => {
    let cancelled = false;
    if (!projectId) {
      setProjectName("");
      return;
    }
    setProjectName("");
    authFetchCached<{ project?: { name?: string } }>(`/api/projects/${projectId}/summary`, undefined, {
      ttlMs: 15000,
      key: `project-summary:${projectId}`,
    })
      .then((data) => {
        if (!cancelled) {
          setProjectName((data?.project?.name || "").trim());
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProjectName("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  useEffect(() => {
    let cancelled = false;
    if (!domainScope) {
      setDomainLabel("");
      return;
    }
    setDomainLabel("");
    authFetchCached<{ domain?: { url?: string } }>(`/api/domains/${domainScope}/summary?gen_limit=1&link_limit=1`, undefined, {
      ttlMs: 15000,
      key: `domain-summary:${domainScope}`,
    })
      .then((data) => {
        const label = (data?.domain?.url || "").trim();
        if (!cancelled) {
          setDomainLabel(label);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDomainLabel("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [domainScope]);

  return {
    projectName,
    domainLabel,
  };
}

