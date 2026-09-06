package i18n

import (
	"encoding/json"
	"os"
)

// SupportedLangs — поддерживаемые языки сайта.
var SupportedLangs = []string{"en", "it", "de"}

// DefaultLang — язык по умолчанию, если в URL некорректный/отсутствующий код.
const DefaultLang = "en"

var translations = map[string]map[string]string{}

// Load загружает все JSON-файлы переводов из указанной директории.
// Вызывать один раз при старте приложения.
func Load(dir string) error {
	for _, lang := range SupportedLangs {
		data, err := os.ReadFile(dir + "/" + lang + ".json")
		if err != nil {
			return err
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		translations[lang] = m
	}
	return nil
}

// IsSupported проверяет, поддерживается ли код языка.
func IsSupported(lang string) bool {
	for _, l := range SupportedLangs {
		if l == lang {
			return true
		}
	}
	return false
}

// T возвращает словарь переводов для языка (используется в шаблонах как .T.key).
func T(lang string) map[string]string {
	if m, ok := translations[lang]; ok {
		return m
	}
	return translations[DefaultLang]
}
