import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.errors;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsErrorsPage() {
  return <DocsContentPage page={page} />;
}
