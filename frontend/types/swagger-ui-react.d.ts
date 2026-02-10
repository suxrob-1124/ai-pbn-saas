declare module "swagger-ui-react" {
  import * as React from "react";

  export interface SwaggerUIProps {
    spec?: unknown;
    url?: string;
    docExpansion?: string;
    defaultModelsExpandDepth?: number;
    deepLinking?: boolean;
    [key: string]: unknown;
  }

  const SwaggerUI: React.ComponentType<SwaggerUIProps>;
  export default SwaggerUI;
}

declare module "swagger-ui-react/swagger-ui.css";
