package dpage

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common"
	"github.com/rismaster/allris-common/common/files"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/downloader"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Sitzungsliste struct {
	app *application.AppContext
}

type Gremium struct {
	option   int
	children []downloader.RisRessource
}

func NewSitzungsliste(app *application.AppContext) Sitzungsliste {
	return Sitzungsliste{
		app: app,
	}
}

func (sl *Sitzungsliste) SynchronizeSince(minTime time.Time, redownload bool) error {
	sitzungenRis, err := sl.fetchLongSitzungsListe(minTime, redownload)
	if err != nil {
		return errors.Wrap(err, "error fetching long sitzungsliste")
	}
	allSitzungenFromRis := make(map[string]bool)
	for _, sitzungRis := range sitzungenRis {
		slog.Info("found sitzung: %s (%s)", sitzungRis.GetName(), sitzungRis.GetCreated())
		sitzung := NewSitzung(sl.app, &sitzungRis)
		allSitzungenFromRis[sitzung.GetPath()] = true
	}

	err = PublishRisDownload(sl.app, sitzungenRis, redownload)
	if err != nil {
		return err
	}

	childFolders := []string{sl.app.Config.GetAnlagenFolder(), sl.app.Config.GetTopFolder()}
	err = files.DeleteFilesIfNotInAndAfter(sl.app, sl.app.Config.GetSitzungenFolder(), allSitzungenFromRis, childFolders, minTime)
	if err != nil {
		return errors.Wrap(err, "error deleting vorlagen")
	}
	return nil
}

func (sl *Sitzungsliste) DownloadLastNPerGremium(countPerGremium int, redownload bool) error {
	sitzungenRis, err := sl.downloadMax(countPerGremium, redownload)
	if err != nil {
		return errors.Wrap(err, "error downloading vorlagen %+v")
	}

	err = PublishRisDownload(sl.app, sitzungenRis, redownload)
	if err != nil {
		return err
	}

	return nil
}

func (sl *Sitzungsliste) downloadMax(countPerGremium int, redownload bool) (sitzungen []downloader.RisRessource, err error) {

	gremien, err := sl.fetchGremiumOptions(redownload)
	if err != nil {
		return nil, err
	}

	for _, gremium := range gremien {
		slog.Info("Gremium %d", gremium.option)
		errSizungsliste := sl.fetchSitzungsListe(gremium, redownload)
		if errSizungsliste != nil {
			slog.Error("error loading sitzungsliste for gremium %d, Reason: %v", gremium.option, errSizungsliste)
		}
		j := 0
		for _, s := range gremium.children {
			if j >= countPerGremium {
				break
			}
			if s.GetUrl() != "" {
				sitzungen = append(sitzungen, s)
				j++
			}
		}
	}

	return sitzungen, nil
}

func (sl *Sitzungsliste) fetchLongSitzungsListe(minTime time.Time, redownload bool) (sitzungen []downloader.RisRessource, err error) {

	formData := url.Values{}
	formData.Add("GRA", "99999999")
	formData.Add("filtGRA", "filter")

	uri, err := url.Parse(sl.app.Config.GetTargetToParse() + sl.app.Config.GetUrlSitzungsLangeliste())
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse url")
	}

	srcWeb := downloader.NewRisRessource("", sl.app.Config.GetAlleSitzungenType(), ".html", time.Now(), uri, &formData, redownload)
	targetStore := files.NewFile(sl.app, srcWeb)

	err = targetStore.Fetch(files.HttpPost, srcWeb, "text/html", true)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error downloading allesitzungen from %s", sl.app.Config.GetUrlSitzungsLangeliste()))
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(targetStore.GetContent()))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error create dom from %s", targetStore.GetName()))
	}

	selector := "tr.zl11,tr.zl12"
	doc.Find(selector).Each(func(index int, selection *goquery.Selection) {

		if selection.Children().Size() >= 8 {

			sitzung, err := sl.parseElement(selection, redownload)
			if err != nil {
				log.Printf("error Parse sitzung element %v", err)
			}
			if sitzung.GetUrl() != "" && sitzung.GetCreated().After(minTime) {
				sitzungen = append(sitzungen, *sitzung)
			}
		}
	})
	if len(sitzungen) == 0 {
		return nil, errors.New("keine Sitzungen (allesitzungen.html)")
	}

	newHash := common.Md5HashB(targetStore.GetContent())
	err = targetStore.WriteIfMoreActualAndDifferent(newHash)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error writing allesitzungen %s", srcWeb.GetName()))
	}
	return sitzungen, nil
}

func (sl *Sitzungsliste) fetchSitzungsListe(gremium *Gremium, redownload bool) (err error) {
	graStr := strconv.Itoa(gremium.option)

	formData := url.Values{}
	formData.Add("GRA", graStr)
	formData.Add("filtGRA", "filter")

	uri, err := url.Parse(sl.app.Config.GetTargetToParse() + sl.app.Config.GetUrlSitzungsliste())
	if err != nil {
		return errors.Wrap(err, "cannot parse url")
	}

	srcWeb := downloader.NewRisRessource("", fmt.Sprintf("%s-%d", sl.app.Config.GetGremienListeType(), gremium.option), ".html", time.Now(), uri, &formData, redownload)
	targetStore := files.NewFile(sl.app, srcWeb)

	err = targetStore.Fetch(files.HttpPost, srcWeb, "text/html", true)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error downloading Sitzungsliste from %s", sl.app.Config.GetUrlSitzungsliste()))
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(targetStore.GetContent()))
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error create dom from %s", targetStore.GetName()))
	}

	selector := "tr.zl11,tr.zl12"
	doc.Find(selector).Each(func(index int, selection *goquery.Selection) {

		if selection.Children().Size() >= 8 {

			sitzung, err := sl.parseElement(selection, redownload)
			if err != nil {
				log.Printf("error Parse sitzung element %v", err)
			}
			if sitzung != nil {
				gremium.children = append(gremium.children, *sitzung)
			}
		}
	})
	if len(gremium.children) == 0 {
		return errors.New("falsche Sitzungsliste")
	}

	newHash := common.Md5HashB(targetStore.GetContent())
	err = targetStore.WriteIfMoreActualAndDifferent(newHash)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error writing Gremienliste %s", srcWeb.GetName()))
	}
	return nil
}

func (sl *Sitzungsliste) parseElement(e *goquery.Selection, redownload bool) (*downloader.RisRessource, error) {

	lnkTr := e.Find(":nth-child(2) a")
	lnk, _ := lnkTr.Attr("href")
	lnkUrlAttr, err := url.Parse(lnk)
	if err != nil {
		return nil, err
	}

	silfdnr := lnkUrlAttr.Query().Get("SILFDNR")
	name := strings.TrimSpace(lnkTr.First().Text())
	dateText := strings.TrimSpace(e.Find(":nth-child(6) a").Text())
	timeTr := strings.Split(strings.TrimSpace(e.Find(":nth-child(7)").Text()), " - ")[0]
	dateTimetxt := fmt.Sprintf("%s %s:00", dateText, timeTr)

	localTz, _ := time.LoadLocation(sl.app.Config.GetTimezone())
	risTime, err := time.ParseInLocation(sl.app.Config.GetDateFormatWithTime(), dateTimetxt, localTz)
	if err != nil {
		return nil, err
	}

	if silfdnr != "" {
		slog.Info("Sitzung erzeugt: %s - %s / %s", lnk, name, dateText)

		silfdnrInt, err1 := strconv.Atoi(silfdnr)
		if err1 != nil {
			return nil, errors.Wrap(err1, "cannot create int from silfdnr")
		}

		uri, err2 := url.Parse(sl.app.Config.GetTargetToParse() + fmt.Sprintf(sl.app.Config.GetUrlSitzungTmpl(), silfdnrInt))
		if err2 != nil {
			return nil, errors.Wrap(err2, "cannot parse url")
		}

		sName := fmt.Sprintf("%s-%d", sl.app.Config.GetSitzungType(), silfdnrInt)

		return downloader.NewRisRessource(sl.app.Config.GetSitzungenFolder(), sName, ".html", risTime, uri, &url.Values{}, redownload), nil
		//return NewSitzung(sl.app, res), nil
	} else if dateText != "" {
		sName2 := e.Find(":nth-child(2)").Text()
		slog.Info("Kalender-Eintrag: :%s %s", dateTimetxt, sName2)

		return downloader.NewRisRessource("", sName2, "", risTime, nil, &url.Values{}, redownload), nil

		//return NewSitzung(sl.app, res), nil
	} else {
		slog.Debug("Empty: %s", name)
	}

	return nil, nil

}

func (sl *Sitzungsliste) fetchGremiumOptions(redownload bool) (gremien []*Gremium, err error) {

	uri, err := url.Parse(sl.app.Config.GetTargetToParse() + sl.app.Config.GetUrlSitzungsliste())
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse url")
	}

	srcWeb := downloader.NewRisRessource("", sl.app.Config.GetGremienOptionsType(), ".html", time.Now(), uri, &url.Values{}, redownload)

	targetStore := files.NewFile(sl.app, srcWeb)

	err = targetStore.Fetch(files.HttpGet, srcWeb, "text/html", true)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error downloading Gremienliste from %s", sl.app.Config.GetUrlSitzungsliste()))
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(targetStore.GetContent()))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error create dom from %s", targetStore.GetName()))
	}

	var options []*Gremium
	doc.Find("select[name=\"GRA\"] option").Each(func(i int, s *goquery.Selection) {
		optStr, ok := s.Attr("value")
		if ok {
			opt, intErr := strconv.Atoi(optStr)
			if intErr != nil {
				slog.Warn("error parsing opt value ignored: %s reason: %v", optStr, intErr)
			} else if opt < 1000 {
				gremium := &Gremium{option: opt}
				options = append(options, gremium)
			}
		}
	})

	newHash := common.Md5HashB(targetStore.GetContent())
	err = targetStore.WriteIfMoreActualAndDifferent(newHash)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error writing Gremienliste %s", srcWeb.GetName()))
	}

	return options, nil
}
