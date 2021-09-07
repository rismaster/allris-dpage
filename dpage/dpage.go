package dpage

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
)

func Download(ctx context.Context, ris downloader.RisRessource) {

	app, err := application.NewAppContextWithContext(ctx)
	if err != nil {
		slog.Fatal("error init appContext: %+v", err)
	}

	var doc Document

	conf := app.Config

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

	if app.Config.GetDebug() {

		for _, ris := range risArr {
			Download(app.Ctx(), ris)
		}

	} else {

		t := app.Publisher().Topic(app.Config.GetDownloadTopic())
		for _, ris := range risArr {
			b, err := json.Marshal(ris)
			if err != nil {
				return err
			}
			t.Publish(app.Ctx(), &pubsub.Message{
				Data: b,
			})
		}
	}

	return nil
}
