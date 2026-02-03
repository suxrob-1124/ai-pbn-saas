package llm

import "strings"

// MaskAPIKey маскирует API ключ для безопасного логирования
// Возвращает первые 4 и последние 4 символа, остальное заменяет на "..."
// Если ключ слишком короткий, возвращает "****"
func MaskAPIKey(key string) string {
	if len(key) == 0 {
		return "[empty]"
	}
	if len(key) <= 4 {
		return "****"
	}
	if len(key) <= 8 {
		// Для ключей от 5 до 8 символов показываем только первые 4
		return key[:4] + "****"
	}
	// Для длинных ключей показываем первые 4 и последние 4 символа
	return key[:4] + "..." + key[len(key)-4:]
}

// SanitizeError удаляет потенциальные API ключи из сообщений об ошибках
// Ищет паттерны, похожие на API ключи (длинные строки с буквами и цифрами)
// и заменяет их на маскированные версии
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Ищем паттерны вида "key=XXXXX" или "?key=XXXXX" в URL
	// Это может быть API ключ в URL запроса
	if strings.Contains(errMsg, "key=") {
		// Простая замена - маскируем всё после "key=" до следующего разделителя
		parts := strings.Split(errMsg, "key=")
		if len(parts) > 1 {
			// Берем часть после "key="
			afterKey := parts[1]
			// Находим конец ключа (до &, пробела, или конца строки)
			endIdx := len(afterKey)
			for i, r := range afterKey {
				if r == '&' || r == ' ' || r == '\n' || r == '\r' {
					endIdx = i
					break
				}
			}
			keyPart := afterKey[:endIdx]
			rest := afterKey[endIdx:]
			// Заменяем ключ на маскированную версию
			masked := MaskAPIKey(keyPart)
			errMsg = parts[0] + "key=" + masked + rest
		}
	}

	// Если сообщение изменилось, создаем новую ошибку
	if errMsg != err.Error() {
		return &sanitizedError{
			original: err,
			message:  errMsg,
		}
	}

	return err
}

// sanitizedError обертка для ошибки с маскированным сообщением
type sanitizedError struct {
	original error
	message  string
}

func (e *sanitizedError) Error() string {
	return e.message
}

func (e *sanitizedError) Unwrap() error {
	return e.original
}
