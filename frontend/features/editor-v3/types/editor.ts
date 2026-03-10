export type DomainSummaryResponse = {
  domain: {
    id: string;
    project_id: string;
    url: string;
    status: string;
  };
  project_name: string;
  my_role: "admin" | "owner" | "manager" | "editor" | "viewer";
};
