package llm

// Пример использования LLM клиента в Worker:
//
// func processGeneration(ctx context.Context, cfg config.Config, db *sql.DB, genID, domainID string) error {
// 	// Создаем LLM клиент
// 	llmCfg := llm.Config{
// 		APIKey:           cfg.GeminiAPIKey,
// 		DefaultModel:     cfg.GeminiDefaultModel,
// 		MaxRetries:       cfg.GeminiMaxRetries,
// 		RetryDelay:       cfg.GeminiRetryDelay,
// 		RequestTimeout:   cfg.GeminiRequestTimeout,
// 		RateLimitPerMin:  cfg.GeminiRateLimitPerMin,
// 	}
// 	llmClient := llm.NewClient(llmCfg)
// 	defer func() {
// 		// Сохраняем все LLM запросы в artifacts перед завершением
// 		llmRequests := llmClient.GetRequests()
// 		if len(llmRequests) > 0 {
// 			// Добавляем в artifacts генерации
// 			// artifacts["llm_requests"] = llmRequests
// 		}
// 		llmClient.Reset()
// 	}()
//
// 	// Создаем менеджер промптов
// 	promptStore := sqlstore.NewPromptStore(db)
// 	promptAdapter := llm.NewPromptAdapter(promptStore)
// 	promptManager := llm.NewPromptManager(promptAdapter)
//
// 	// Загружаем активный промпт
// 	promptID, systemPrompt, err := promptManager.GetActivePrompt(ctx)
// 	if err != nil {
// 		return fmt.Errorf("failed to get active prompt: %w", err)
// 	}
//
// 	// Пример: Анализ конкурентов через LLM
// 	analysisPrompt := llm.BuildPrompt(systemPrompt, "Проанализируй конкурентов...", map[string]string{
// 		"keyword": "example keyword",
// 	})
// 	analysisResult, err := llmClient.Generate(ctx, "competitor_analysis", analysisPrompt, "gemini-2.5-pro")
// 	if err != nil {
// 		return fmt.Errorf("failed to analyze competitors: %w", err)
// 	}
//
// 	// Пример: Генерация контента
// 	contentPrompt := llm.BuildPrompt(systemPrompt, "Сгенерируй контент...", nil)
// 	content, err := llmClient.Generate(ctx, "content", contentPrompt, "gemini-2.5-pro")
// 	if err != nil {
// 		return fmt.Errorf("failed to generate content: %w", err)
// 	}
//
// 	return nil
// }

