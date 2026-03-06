"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  extractCssVariables,
  applyCssVariables,
  type CSSVariable,
} from "../services/cssVariableExtraction";

export function useCssVariables(cssContent: string) {
  const [variables, setVariables] = useState<CSSVariable[]>([]);
  const [cssDirty, setCssDirty] = useState(false);
  const lastCssRef = useRef("");

  useEffect(() => {
    if (cssContent === lastCssRef.current) return;
    lastCssRef.current = cssContent;
    setVariables(extractCssVariables(cssContent));
    setCssDirty(false);
  }, [cssContent]);

  const updateVariable = useCallback((index: number, value: string) => {
    setVariables((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], value };
      return updated;
    });
    setCssDirty(true);
  }, []);

  const buildUpdatedCss = useCallback(() => {
    return applyCssVariables(cssContent, variables);
  }, [cssContent, variables]);

  return { variables, cssDirty, updateVariable, buildUpdatedCss };
}
