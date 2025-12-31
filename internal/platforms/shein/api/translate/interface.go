package translate

type TranslateAPI interface {
	Translate(text string, from, to string) (string, error)
}
