package ports

type TextOutput interface {
	WriteLine(text string) error
}
