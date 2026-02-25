"use client";

import { useState } from "react";
import { FiHelpCircle, FiX, FiCopy } from "react-icons/fi";

// Доступные переменные для шаблонов промптов
export const PROMPT_VARIABLES = [
  {
    name: "keyword",
    description: "Основной ключевой запрос для генерации",
    example: "{{ keyword }}",
    usage: "Используется в промптах для анализа конкурентов и создания ТЗ",
  },
  {
    name: "contents_data",
    description: "Текстовое содержимое страниц конкурентов из SERP",
    example: "{{ contents_data }}",
    usage: "Содержит HTML-контент топ-10 сайтов, извлеченный из поисковой выдачи",
  },
  {
    name: "analysis_data",
    description: "Таблица с метриками конкурентов (CSV формат)",
    example: "{{ analysis_data }}",
    usage: "Содержит структурированные данные: позиция, домен, метрики (DR, трафик и т.д.)",
  },
  {
    name: "llm_analysis",
    description: "Результат анализа конкурентов от LLM (Markdown)",
    example: "{{ llm_analysis }}",
    usage: "Содержит структурированный анализ конкурентов, выполненный на предыдущем этапе",
  },
  {
    name: "archetype_id",
    description: "ID архетипа контента (1-4): 1=Полное руководство, 2=Сравнение, 3=Проблема/Решение, 4=Мифы и Факты",
    example: "{{ archetype_id }}",
    usage: "Используется в промпте technical_spec для выбора типа структуры контента",
  },
  {
    name: "image_style_id",
    description: "ID стиля изображений (1-4): 1=Клейморфизм, 2=Инфографика, 3=Фотореализм, 4=Плоский дизайн",
    example: "{{ image_style_id }}",
    usage: "Используется в промпте technical_spec для выбора визуального стиля",
  },
  {
    name: "header_element_id",
    description: "ID элемента шапки (1-4): 1=Нет, 2=Поиск, 3=CTA (призыв к действию), 4=Контакты",
    example: "{{ header_element_id }}",
    usage: "Используется в промпте technical_spec для выбора элемента в шапке сайта",
  },
  {
    name: "footer_variant_id",
    description: "ID варианта подвала (1-4): 1=Минималистичный, 2=Корпоративный, 3=Навигационный, 4=Комплексный",
    example: "{{ footer_variant_id }}",
    usage: "Используется в промпте technical_spec для выбора структуры подвала",
  },
  {
    name: "country",
    description: "Целевая страна для поиска",
    example: "{{ country }}",
    usage: "Код страны (например: se, us, ru)",
  },
  {
    name: "language",
    description: "Целевой язык контента",
    example: "{{ language }}",
    usage: "Код языка (например: sv, en, ru)",
  },
  {
    name: "site_context",
    description: "Сжатый контекст текущего сайта для editor AI",
    example: "{{ site_context }}",
    usage: "Включает identity, структуру страниц, дизайн-токены и ключевые файлы",
  },
  {
    name: "task_constraints",
    description: "Технические ограничения текущей AI-операции",
    example: "{{ task_constraints }}",
    usage: "Содержит operation, target_path, context_mode, язык и другие ограничения",
  },
  {
    name: "current_file_path",
    description: "Путь текущего редактируемого файла",
    example: "{{ current_file_path }}",
    usage: "Используется в AI-редактировании файла для точного таргетинга",
  },
  {
    name: "current_file_content",
    description: "Текущее содержимое редактируемого файла",
    example: "{{ current_file_content }}",
    usage: "Передается в AI для изменения существующего файла без потери контекста",
  },
  {
    name: "target_path",
    description: "Целевой путь создаваемой страницы",
    example: "{{ target_path }}",
    usage: "Используется в AI-create-page для генерации файлов в нужный путь",
  },
] as const;

export function PromptVariablesHelp() {
  const [isOpen, setIsOpen] = useState(false);

  if (!isOpen) {
    return (
      <button
        type="button"
        onClick={() => setIsOpen(true)}
        className="inline-flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300"
      >
        <FiHelpCircle /> Доступные переменные
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setIsOpen(false)}>
      <div
        className="bg-white dark:bg-slate-900 rounded-xl shadow-xl max-w-2xl w-full max-h-[80vh] overflow-auto border border-slate-200 dark:border-slate-800"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="sticky top-0 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 px-6 py-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
            📝 Переменные для шаблонов промптов
          </h3>
          <button
            onClick={() => setIsOpen(false)}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
          >
            <FiX />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <div className="text-sm text-slate-600 dark:text-slate-400 mb-4">
            Используйте переменные в формате <code className="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-800 rounded text-xs">{"{{ имя_переменной }}"}</code> в тексте промпта. 
            Они будут автоматически заменены на соответствующие данные во время генерации.
          </div>
          {PROMPT_VARIABLES.map((variable) => (
            <div
              key={variable.name}
              className="border border-slate-200 dark:border-slate-800 rounded-lg p-4 bg-slate-50/50 dark:bg-slate-900/50"
            >
              <div className="flex items-start justify-between mb-2">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <code className="px-2 py-1 bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300 rounded text-sm font-mono">
                      {"{{ " + variable.name + " }}"}
                    </code>
                    <button
                      type="button"
                      onClick={() => navigator.clipboard?.writeText("{{ " + variable.name + " }}")}
                      className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
                      title="Копировать"
                    >
                      <FiCopy className="w-3 h-3" />
                    </button>
                  </div>
                  <p className="text-sm text-slate-700 dark:text-slate-200 font-medium">{variable.description}</p>
                </div>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">{variable.usage}</p>
            </div>
          ))}
          <div className="mt-6 p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg">
            <div className="text-sm font-semibold text-amber-800 dark:text-amber-200 mb-2">💡 Пример использования:</div>
            <pre className="text-xs bg-white dark:bg-slate-900 p-3 rounded border border-amber-200 dark:border-amber-800 overflow-auto">
{`Проведи анализ контента по запросу "{{ keyword }}".

Данные конкурентов:
{{ analysis_data }}

HTML контент:
{{ contents_data }}

Страна: {{ country }}
Язык: {{ language }}`}
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}
