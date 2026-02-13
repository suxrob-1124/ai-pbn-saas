export type PipelineStepDefinition = {
  id: string;
  label: string;
  progress: number;
  artifactKeys: string[];
  forceable?: boolean;
};

export const PIPELINE_STEPS: PipelineStepDefinition[] = [
  { id: "serp_analysis", label: "SERP Analysis", progress: 8, artifactKeys: ["serp_data", "analysis_csv"], forceable: true },
  { id: "competitor_analysis", label: "Competitor Analysis", progress: 16, artifactKeys: ["competitor_analysis", "llm_analysis"], forceable: true },
  { id: "technical_spec", label: "Technical Spec", progress: 24, artifactKeys: ["technical_spec"], forceable: true },
  { id: "content_generation", label: "Content Generation", progress: 32, artifactKeys: ["content_markdown"], forceable: true },
  { id: "design_architecture", label: "Design Architecture", progress: 40, artifactKeys: ["design_system"], forceable: true },
  { id: "logo_generation", label: "Logo Generation", progress: 48, artifactKeys: ["logo_svg"], forceable: true },
  { id: "html_generation", label: "HTML Generation", progress: 56, artifactKeys: ["html_raw"], forceable: true },
  { id: "css_generation", label: "CSS Generation", progress: 64, artifactKeys: ["css_content"], forceable: true },
  { id: "js_generation", label: "JS Generation", progress: 72, artifactKeys: ["js_content"], forceable: true },
  { id: "image_generation", label: "Image Generation", progress: 80, artifactKeys: ["image_prompts"], forceable: true },
  { id: "page404_generation", label: "404 Generation", progress: 88, artifactKeys: ["404_html"], forceable: true },
  { id: "assembly", label: "Final Assembly & Zip", progress: 96, artifactKeys: ["zip_archive"], forceable: true },
  { id: "publish", label: "Publish", progress: 99, artifactKeys: ["published_path"], forceable: true },
];

export function hasArtifactValue(value: any): boolean {
  if (value == null) return false;
  if (typeof value === "string") return value.trim().length > 0;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "object") return Object.keys(value).length > 0;
  return true;
}

export function isStepComplete(artifacts: Record<string, any> | undefined, keys: string[]): boolean {
  if (!artifacts) return false;
  return keys.some((key) => hasArtifactValue(artifacts[key]));
}

export function computeDisplayProgress(
  artifacts: Record<string, any> | undefined,
  progress: number | undefined,
  status?: string
): number {
  if (status === "success") return 100;

  const totalSteps = PIPELINE_STEPS.length;
  const completedSteps = PIPELINE_STEPS.filter((step) => isStepComplete(artifacts, step.artifactKeys)).length;
  const artifactProgress = Math.round((completedSteps / totalSteps) * 100);
  const backendProgress = clampInt(typeof progress === "number" ? progress : 0, 0, 99);

  const active = status === "pending" || status === "processing" || status === "pause_requested" || status === "cancelling";
  if (active) {
    const lowerBound = artifactProgress;
    const upperBound = completedSteps >= totalSteps
      ? 99
      : Math.min(99, Math.round(((completedSteps + 1) / totalSteps) * 100) - 1);
    const boundedBackend = clampInt(backendProgress, lowerBound, upperBound);
    const computed = Math.max(lowerBound, boundedBackend);
    if (computed === 0 && status === "processing") {
      return 1;
    }
    return computed;
  }

  return clampInt(Math.max(artifactProgress, backendProgress), 0, 99);
}

function clampInt(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
