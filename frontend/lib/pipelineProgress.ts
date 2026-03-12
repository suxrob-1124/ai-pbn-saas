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

const WEBARCHIVE_PREFIX_STEPS: PipelineStepDefinition[] = [
  { id: "wayback_fetch", label: "Wayback Fetch", progress: 3, artifactKeys: ["wayback_data"] },
  { id: "keyword_generation", label: "Keyword Generation", progress: 6, artifactKeys: ["generated_keyword"] },
];

const WEBARCHIVE_STEPS: PipelineStepDefinition[] = [
  ...WEBARCHIVE_PREFIX_STEPS,
  ...PIPELINE_STEPS,
];

export function getStepsForGenerationType(generationType?: string): PipelineStepDefinition[] {
  if (generationType === "webarchive_single" || generationType === "webarchive_multi" || generationType === "webarchive_eeat") {
    return WEBARCHIVE_STEPS;
  }
  return PIPELINE_STEPS;
}

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
  status?: string,
  generationType?: string
): number {
  if (status === "success") return 100;

  const steps = getStepsForGenerationType(generationType);
  const totalSteps = steps.length;
  const completedSteps = steps.filter((step) => isStepComplete(artifacts, step.artifactKeys)).length;
  const artifactProgress = Math.round((completedSteps / totalSteps) * 100);
  const backendProgress = clampInt(typeof progress === "number" ? progress : 0, 0, 99);

  const active = status === "pending" || status === "processing" || status === "pause_requested" || status === "cancelling";
  if (active) {
    // Для активных запусков доверяем backend progress как единому источнику.
    // Это устраняет рассинхрон между экранами, где артефакты могут быть неполными/обрезанными.
    if (backendProgress > 0) {
      return backendProgress;
    }
    if (artifactProgress > 0) {
      return clampInt(artifactProgress, 1, 99);
    }
    return status === "processing" ? 1 : 0;
  }

  return clampInt(Math.max(artifactProgress, backendProgress), 0, 99);
}

function clampInt(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
