import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "../../../features/docs/components/DocsContentPage";

const page = docsPages.projects;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsProjectsPage() {
  return <DocsContentPage page={page} />;
}
