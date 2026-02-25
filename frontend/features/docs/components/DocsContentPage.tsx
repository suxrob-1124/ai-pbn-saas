import type { DocsPageContent } from "../../../docs-content/types";
import { AdminOnlySection } from "../../../components/AdminOnlySection";

export function DocsContentPage({ page }: { page: DocsPageContent }) {
  const tocItems = page.sections
    .map((section, index) => ({
      id: `section-${index + 1}`,
      title: section.title,
      adminOnly: section.adminOnly,
    }))
    .filter((item) => item.title && !item.adminOnly) as Array<{ id: string; title: string }>;

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">{page.title}</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{page.intro}</p>
      </header>

      {tocItems.length >= 3 ? (
        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Содержание</h2>
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            {tocItems.map((item) => (
              <a
                key={item.id}
                href={`#${item.id}`}
                className="rounded-lg border border-slate-200 px-3 py-2 text-sm text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-900/60"
              >
                {item.title}
              </a>
            ))}
          </div>
        </section>
      ) : null}

      {page.sections.map((section, index) => {
        const sectionId = `section-${index + 1}`;
        const hasCard = Boolean(
          section.title ||
            (section.listItems && section.listItems.length > 0) ||
            (section.codeBlocks && section.codeBlocks.length > 0),
        );
        const content = (
          <>
            {section.title ? (
              <h2 id={sectionId} className="scroll-mt-24 text-base font-semibold">
                {section.title}
              </h2>
            ) : null}
            {section.paragraphs?.length ? (
              <div className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
                {section.paragraphs.map((paragraph) => (
                  <p key={paragraph}>{paragraph}</p>
                ))}
              </div>
            ) : null}
            {section.listItems?.length ? (
              section.listType === "numbered" ? (
                <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
                  {section.listItems.map((item) => (
                    <li key={item}>{item}</li>
                  ))}
                </ol>
              ) : (
                <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
                  {section.listItems.map((item) => (
                    <li key={item}>{item}</li>
                  ))}
                </ul>
              )
            ) : null}
            {section.codeBlocks?.length ? (
              <div className="space-y-3">
                {section.codeBlocks.map((block) => (
                  <div key={block.code}>
                    <pre className="mt-2 overflow-x-auto rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-200">
                      <code>{block.code}</code>
                    </pre>
                    {block.caption ? (
                      <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">{block.caption}</p>
                    ) : null}
                  </div>
                ))}
              </div>
            ) : null}
          </>
        );

        const sectionNode = hasCard ? (
          <section
            key={index}
            className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60"
          >
            {content}
          </section>
        ) : (
          <section key={index}>{content}</section>
        );

        if (section.adminOnly) {
          return <AdminOnlySection key={index}>{sectionNode}</AdminOnlySection>;
        }

        if (!hasCard) {
          return <section key={index}>{content}</section>;
        }
        return sectionNode;
      })}
    </div>
  );
}
