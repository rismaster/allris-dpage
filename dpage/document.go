package dpage

type Document interface {
	GetPath() string
	GetUrl() string
	Download(redownload bool) error
}
