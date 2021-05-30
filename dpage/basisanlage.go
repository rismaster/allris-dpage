package dpage

import (
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common"
	"github.com/rismaster/allris-common/common/files"
	"github.com/rismaster/allris-common/downloader"
	"fmt"
	"github.com/pkg/errors"
)

type AnlageDocument struct {
	app          *application.AppContext
	webRessource *downloader.RisRessource
	file         *files.File
}

func NewAnlageDocument(app *application.AppContext, ris *downloader.RisRessource) *AnlageDocument {

	return &AnlageDocument{
		app:          app,
		webRessource: ris,
		file:         files.NewFile(app, ris),
	}
}

func (d *AnlageDocument) GetPath() string {
	return d.file.GetPath()
}

func (d *AnlageDocument) GetUrl() string {
	return d.webRessource.GetUrl()
}

func (d *AnlageDocument) Download() error {

	err := d.file.Fetch(files.HttpPost, d.webRessource, "application/pdf", true)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error downloading Vorlagenliste from %s, Error: %+v", d.webRessource.GetUrl(), err))
	}

	mewHash := common.Md5HashB(d.file.GetContent())
	return d.file.WriteIfMoreActualAndDifferent(mewHash)
}
