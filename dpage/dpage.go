package dpage

import (
	"context"
	allris_common "github.com/rismaster/allris-common"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
)

func Download(ctx context.Context, ris downloader.RisRessource, conf allris_common.Config) {

	app, err := application.NewAppContextWithContext(ctx, conf)
	if err != nil {
		slog.Fatal("error init appContext: %+v", err)
	}

	var doc Document

	switch ris.Folder {
	case conf.GetSitzungenFolder():
		doc = NewSitzung(app, &ris)
	case conf.GetTopFolder():
		doc = NewTop(app, &ris)
	case conf.GetAnlagenFolder():
		if ris.GetFormData().Get("options") != "" {
			doc = NewAnlageDocument(app, &ris)
		} else {
			doc = NewAnlage(app, &ris)
		}
	case conf.GetVorlagenFolder():
		doc = NewVorlage(app, &ris)
	}

	if doc != nil {
		err = doc.Download()
		if err != nil {
			slog.Fatal("error downloading %+v: %+v", ris, err)
		}
	} else {
		slog.Fatal("error downloading %+v: %+v", ris, err)
	}
}

func PublishRisDownload(app *application.AppContext, risArr []downloader.RisRessource) error {

	for _, ris := range risArr {
		Download(app.Ctx(), ris, app.Config)
	}

	return nil
}
