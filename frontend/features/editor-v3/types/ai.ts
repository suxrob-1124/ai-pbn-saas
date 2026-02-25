export type AIContextMode = "auto" | "manual" | "hybrid";

export type AIFlowStatus = "idle" | "validating" | "sending" | "parsing" | "ready" | "applying" | "done" | "error";

export type AIFlowState = {
  status: AIFlowStatus;
  message?: string;
  startedAt?: string;
  finishedAt?: string;
  error?: string;
};

export type AIAssetGenerationStatus = "ok" | "broken" | "missing" | "error";

export type AIAssetGenerationResultDTO = {
  status: AIAssetGenerationStatus;
  mime_type?: string;
  size_bytes?: number;
  warnings: string[];
  error_code?: string;
  error_message?: string;
  token_usage?: {
    source?: string;
    model?: string;
    stage?: string;
    prompt_tokens?: number;
    completion_tokens?: number;
    total_tokens?: number;
    token_source?: "provider" | "estimated" | "mixed" | string;
    estimated_cost_usd?: number | null;
    input_price_usd_per_million?: number | null;
    output_price_usd_per_million?: number | null;
    [key: string]: unknown;
  };
};

export type EditorModelOption = {
  value: string;
  label: string;
};

export type AIContextModeOption = {
  value: AIContextMode;
  label: string;
};
