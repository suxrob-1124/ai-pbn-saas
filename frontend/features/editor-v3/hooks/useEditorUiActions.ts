"use client";

import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import { useCallback } from "react";

import type { AIPageApplyAction, AIPageSuggestionAsset } from "../../../lib/fileApi";
import { showToast } from "../../../lib/toastStore";

type UseEditorUIActionsParams = {
  aiCreateAssets: AIPageSuggestionAsset[];
  setAiCreateApplyPlan: Dispatch<SetStateAction<Record<string, AIPageApplyAction>>>;
  setAiCreateSkippedAssets: Dispatch<SetStateAction<string[]>>;
  setAssetUploadTargetPath: Dispatch<SetStateAction<string>>;
  assetUploadInputRef: MutableRefObject<HTMLInputElement | null>;
  failFlow: (error: unknown, fallbackMessage: string) => void;
};

export function useEditorUiActions({
  aiCreateAssets,
  setAiCreateApplyPlan,
  setAiCreateSkippedAssets,
  setAssetUploadTargetPath,
  assetUploadInputRef,
  failFlow,
}: UseEditorUIActionsParams) {
  const onSetCreatePlan = useCallback((pathValue: string, action: AIPageApplyAction) => {
    setAiCreateApplyPlan((prev) => ({ ...prev, [pathValue]: action }));
  }, [setAiCreateApplyPlan]);

  const onToggleSkipAsset = useCallback((pathValue: string) => {
    setAiCreateSkippedAssets((prev) =>
      prev.includes(pathValue) ? prev.filter((item) => item !== pathValue) : [...prev, pathValue]
    );
  }, [setAiCreateSkippedAssets]);

  const onAssetUploadPick = useCallback((pathValue: string) => {
    if (!pathValue) return;
    setAssetUploadTargetPath(pathValue);
    assetUploadInputRef.current?.click();
  }, [assetUploadInputRef, setAssetUploadTargetPath]);

  const onCopyAssetPrompt = useCallback(async (pathValue: string) => {
    const asset = aiCreateAssets.find((item) => item.path === pathValue);
    if (!asset?.prompt?.trim()) {
      showToast({
        type: "error",
        title: "Промпт отсутствует",
        message: "У ассета нет prompt для копирования.",
      });
      return;
    }
    const text = asset.prompt.trim();
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        throw new Error("Clipboard API unavailable");
      }
      showToast({
        type: "success",
        title: "Промпт скопирован",
        message: pathValue,
      });
    } catch (err: any) {
      failFlow(err, "Не удалось скопировать prompt ассета");
      showToast({
        type: "error",
        title: "Не удалось скопировать prompt",
        message: err?.message || "Clipboard недоступен",
      });
    }
  }, [aiCreateAssets, failFlow]);

  return {
    onSetCreatePlan,
    onToggleSkipAsset,
    onAssetUploadPick,
    onCopyAssetPrompt,
  };
}
