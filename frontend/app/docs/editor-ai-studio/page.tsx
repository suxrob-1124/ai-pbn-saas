import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.editorAiStudio;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsEditorAiStudioPage() {
  return <DocsContentPage page={page} />;
}
