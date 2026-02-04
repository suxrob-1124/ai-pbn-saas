// Этапы генерации контента и соответствующие промпты
export const GENERATION_STAGES = [
  {
    value: "competitor_analysis",
    label: "Анализ конкурентов",
    description: "Промпт для анализа конкурентов на основе SERP данных и контента",
    variables: ["keyword", "analysis_data", "contents_data", "country", "language"],
  },
  {
    value: "technical_spec",
    label: "Техническое задание",
    description: "Промпт для создания технического задания на основе анализа конкурентов",
    variables: ["llm_analysis", "keyword", "country", "language"],
  },
  {
    value: "content_generation",
    label: "Генерация контента",
    description: "Промпт для генерации Markdown контента статьи",
    variables: ["technical_spec", "keyword", "country", "language"],
  },
  {
    value: "design_architecture",
    label: "Дизайн-система",
    description: "Промпт для сборки дизайн-системы (цвета, шрифты, layout)",
    variables: ["content", "keyword", "country", "language"],
  },
  {
    value: "logo_generation",
    label: "Генерация логотипа",
    description: "Промпт для генерации SVG логотипа на основе дизайн-системы",
    variables: ["design_system", "keyword"],
  },
  {
    value: "image_prompt_generation",
    label: "Генерация изображений (промпты)",
    description: "Промпт для генерации промптов для создания изображений",
    variables: ["technical_spec", "design_system", "keyword"],
  },
  {
    value: "css_generation",
    label: "Генерация CSS",
    description: "Промпт для генерации CSS стилей сайта",
    variables: ["technical_spec", "content"],
  },
  {
    value: "js_generation",
    label: "Генерация JavaScript",
    description: "Промпт для генерации JavaScript кода",
    variables: ["technical_spec", "content"],
  },
  {
    value: "html_generation",
    label: "Генерация HTML",
    description: "Промпт для генерации HTML каркаса страницы",
    variables: ["technical_spec", "content", "css", "js"],
  },
  {
    value: "svg_generation",
    label: "Генерация SVG",
    description: "Промпт для генерации SVG логотипа и фавикона",
    variables: ["technical_spec", "keyword"],
  },
  {
    value: "404_page",
    label: "Генерация 404 страницы",
    description: "Промпт для генерации контента страницы 404 (страница не найдена)",
    variables: ["design_system", "html_raw", "css_content", "js_content", "language"],
  },
] as const;

export type GenerationStage = (typeof GENERATION_STAGES)[number]["value"];

export function getStageLabel(stage: string): string {
  const found = GENERATION_STAGES.find((s) => s.value === stage);
  return found?.label || stage;
}

export function getStageDescription(stage: string): string {
  const found = GENERATION_STAGES.find((s) => s.value === stage);
  return found?.description || "";
}
