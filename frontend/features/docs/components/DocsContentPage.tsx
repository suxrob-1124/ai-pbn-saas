import type { DocsPageContent } from "../../../docs-content/types";

export function DocsContentPage({ page }: { page: DocsPageContent }) {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">{page.title}</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{page.intro}</p>
      </header>

      {page.sections.map((section, index) => {
        const hasCard = Boolean(section.title || (section.listItems && section.listItems.length > 0));
        const content = (
          <>
            {section.title ? <h2 className="text-base font-semibold">{section.title}</h2> : null}
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
          </>
        );

        if (!hasCard) {
          return <section key={index}>{content}</section>;
        }

        return (
          <section
            key={index}
            className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60"
          >
            {content}
          </section>
        );
      })}
    </div>
  );
}
