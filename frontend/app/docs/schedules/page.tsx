import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.schedules;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsSchedulesPage() {
  return <DocsContentPage page={page} />;
}
