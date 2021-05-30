package dpage

import (
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/config"
	"github.com/rismaster/allris-common/downloader"
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
)

func Download(ctx context.Context, ris downloader.RisRessource) {

	app, err := application.NewAppContextWithContext(ctx)
	if err != nil {
		slog.Fatal("error init appContext: %+v", err)
	}

	var doc Document

	switch ris.Folder {
	case config.SitzungenFolder:
		doc = NewSitzung(app, &ris)
	case config.TopFolder:
		doc = NewTop(app, &ris)
	case config.AnlagenFolder:
		if ris.GetFormData().Get("options") != "" {
			doc = NewAnlageDocument(app, &ris)
		} else {
			doc = NewAnlage(app, &ris)
		}
	case config.VorlagenFolder:
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

	if config.Debug {

		for _, ris := range risArr {
			Download(app.Ctx(), ris)
		}

	} else {

		t := app.Publisher().Topic(config.DownloadTopic)
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
