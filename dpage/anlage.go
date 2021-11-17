package dpage

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common"
	"github.com/rismaster/allris-common/common/files"
	"github.com/rismaster/allris-common/downloader"
)

type Anlage struct {
	app          *application.AppContext
	webRessource *downloader.RisRessource
	file         *files.File
}

func NewAnlage(app *application.AppContext, ris *downloader.RisRessource) *Anlage {

	return &Anlage{
		app:          app,
		webRessource: ris,
		file:         files.NewFile(app, ris),
	}
}

func (a *Anlage) GetPath() string {
	return a.file.GetPath()
}

func (a *Anlage) GetUrl() string {
	return a.webRessource.GetUrl()
}

func (a *Anlage) Download() error {
	err := a.file.Fetch(files.HttpGet, a.webRessource, "*")
	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("error downloading Vorlagenliste from %s, Error: %v", a.webRessource.GetUrl(), err))
	}

	mewHash := common.Md5HashB(a.file.GetContent())
	return a.file.WriteIfMoreActualAndDifferent(mewHash)
}
