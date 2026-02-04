export type PipelineStepDefinition = {
  id: string;
  label: string;
  progress: number;
  artifactKeys: string[];
  forceable?: boolean;
};

export const PIPELINE_STEPS: PipelineStepDefinition[] = [
  { id: "serp_analysis", label: "SERP Analysis", progress: 20, artifactKeys: ["serp_data", "analysis_csv"], forceable: true },
  { id: "competitor_analysis", label: "Competitor Analysis", progress: 50, artifactKeys: ["competitor_analysis", "llm_analysis"], forceable: true },
  { id: "technical_spec", label: "Technical Spec", progress: 60, artifactKeys: ["technical_spec"], forceable: true },
  { id: "content_generation", label: "Content Generation", progress: 70, artifactKeys: ["content_markdown"], forceable: true },
  { id: "design_architecture", label: "Design Architecture", progress: 80, artifactKeys: ["design_system"], forceable: true },
  { id: "logo_generation", label: "Logo Generation", progress: 90, artifactKeys: ["logo_svg"], forceable: true },
  { id: "html_generation", label: "HTML Generation", progress: 95, artifactKeys: ["html_raw"], forceable: true },
  { id: "css_generation", label: "CSS Generation", progress: 96, artifactKeys: ["css_content"], forceable: true },
  { id: "js_generation", label: "JS Generation", progress: 97, artifactKeys: ["js_content"], forceable: true },
  { id: "image_generation", label: "Image Generation", progress: 98, artifactKeys: ["image_prompts"], forceable: true },
  { id: "assembly", label: "Final Assembly & Zip", progress: 99, artifactKeys: ["zip_archive"], forceable: true },
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
  let maxProgress = typeof progress === "number" ? progress : 0;
  for (const step of PIPELINE_STEPS) {
    if (isStepComplete(artifacts, step.artifactKeys)) {
      if (step.progress > maxProgress) {
        maxProgress = step.progress;
      }
    }
  }
  return maxProgress;
}
