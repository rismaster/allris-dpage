package dpage

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common"
	"github.com/rismaster/allris-common/common/domtools"
	"github.com/rismaster/allris-common/common/files"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
	"net/url"
	"time"
)

type Vorlagenliste struct {
	app *application.AppContext
}

func NewVorlagenliste(app *application.AppContext) Vorlagenliste {
	return Vorlagenliste{
		app: app,
	}
}

func (vl *Vorlagenliste) SynchronizeSince(minTime time.Time, redownload bool) error {
	vorlagen, err := vl.downloadFromMin(minTime)
	if err != nil {
		return errors.Wrap(err, "error downloading vorlagen")
	}

	err = PublishRisDownload(vl.app, vorlagen, redownload)
	if err != nil {
		return err
	}

	allVorlagenFromRis := make(map[string]bool)
	for _, v := range vorlagen {
		vf := NewVorlage(vl.app, &v)
		allVorlagenFromRis[vf.GetPath()] = true
	}

	childFolders := []string{vl.app.Config.GetAnlagenFolder(), vl.app.Config.GetTopFolder()}
	err = files.DeleteFilesIfNotInAndAfter(vl.app, vl.app.Config.GetVorlagenFolder(), allVorlagenFromRis, childFolders, minTime)
	if err != nil {
		return errors.Wrap(err, "error deleting vorlagen")
	}
	return nil
}

func (vl *Vorlagenliste) downloadFromMin(minTime time.Time) (results []downloader.RisRessource, err error) {

	var url = vl.app.Config.GetUrlVorlagenliste()
	for i := 0; i < 1000; i++ {

		if i == 1 {
			url = vl.app.Config.GetUrlVorlagenliste() + "?shownext=true"
		}

		limitTimeReached, vorlagen, err := vl.fetch(url, i, minTime, true)
		if err != nil {
			return nil, err
		}

		for _, v := range vorlagen {
			results = append(results, v)
		}

		cnt := len(vorlagen)
		if cnt <= 0 || limitTimeReached {
			break
		}
	}

	slog.Info("loaded %d Vorlagen from %s", len(results), url)
	return results, nil
}

func (vl *Vorlagenliste) fetch(ressourceUrl string, page int, risCreatedSince time.Time, redownload bool) (limitTimeReached bool, vorlagen []downloader.RisRessource, err error) {

	uri, err := url.Parse(vl.app.Config.GetTargetToParse() + ressourceUrl)
	if err != nil {
		return false, nil, errors.Wrap(err, "cannot parse url")
	}

	srcWeb := downloader.NewRisRessource("", fmt.Sprintf("%s-%d", vl.app.Config.GetVorlagenListeType(), page), ".html", time.Now(), uri, &url.Values{})
	targetStore := files.NewFile(vl.app, srcWeb)

	err = targetStore.Fetch(files.HttpGet, srcWeb, "text/html", true)
	if err != nil {
		return false, nil, errors.Wrap(err, fmt.Sprintf("error downloading Vorlagenliste from %s", ressourceUrl))
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(targetStore.GetContent()))
	if err != nil {
		return false, nil, errors.Wrap(err, fmt.Sprintf("error create dom from %s", targetStore.GetName()))
	}

	limitTimeReached, vorlagen, err = vl.parseChildren(doc, risCreatedSince)
	if err != nil {
		return false, nil, errors.Wrap(err, fmt.Sprintf("error parsing dom from %s", targetStore.GetName()))
	}

	newHash := common.Md5HashB(targetStore.GetContent())
	err = targetStore.WriteIfMoreActualAndDifferent(newHash)
	if err != nil {
		return false, nil, errors.Wrap(err, fmt.Sprintf("error writing vorlagenliste %s", srcWeb.GetName()))
	}

	return limitTimeReached, vorlagen, nil
}

func (vl *Vorlagenliste) parseChildren(doc *goquery.Document, risCreatedSince time.Time) (limitTimeReached bool, vorlagen []downloader.RisRessource, err error) {

	selector := "tr.zl11,tr.zl12"

	limitTimeReached = false
	doc.Find(selector).Each(func(index int, dom *goquery.Selection) {
		vorlage, err := vl.parseElement(dom)
		if err == nil {
			if risCreatedSince.Before(vorlage.GetCreated()) {
				vorlagen = append(vorlagen, *vorlage)
			} else {
				limitTimeReached = true
			}
		}
	})

	return limitTimeReached, vorlagen, nil
}

func (vl *Vorlagenliste) parseElement(e *goquery.Selection) (vorlage *downloader.RisRessource, err error) {

	dom := e.Children()
	if dom.Size() < 4 {
		return nil, errors.New("false html format of Vorgangsliste")
	}

	volfdnr := domtools.ExtractIntFromInput(dom, "VOLFDNR")
	dateText := domtools.GetChildTextFromNode(dom.Get(3))

	location, err := time.LoadLocation(vl.app.Config.GetTimezone())
	if err != nil {
		return nil, err
	}

	risCreatedSince, err := time.ParseInLocation(vl.app.Config.GetDateFormat(), dateText, location)
	if err != nil {
		return nil, errors.New("false html format no created date of Vorgangsliste")
	}

	uri, err := url.Parse(vl.app.Config.GetTargetToParse() + fmt.Sprintf(vl.app.Config.GetUrlVorlageTmpl(), volfdnr))
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse url")
	}

	return downloader.NewRisRessource(vl.app.Config.GetVorlagenFolder(), fmt.Sprintf("%s-%d", vl.app.Config.GetVorlageType(), volfdnr), ".html", risCreatedSince, uri, &url.Values{}), nil
}
