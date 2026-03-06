import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.queue;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsQueuePage() {
  return <DocsContentPage page={page} />;
}
