package engine

// outputLanguageDirective returns a one-line instruction telling the model
// which language to write the generated output in. The system/task prompt
// bodies remain in Chinese; this directive only controls the output language.
func outputLanguageDirective(lang string) string {
	switch lang {
	case "en":
		return "输出语言：English"
	default:
		return "输出语言：中文"
	}
}
