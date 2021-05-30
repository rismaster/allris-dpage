package dpage

import (
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common"
	"github.com/rismaster/allris-common/common/domtools"
	"github.com/rismaster/allris-common/common/files"
	"github.com/rismaster/allris-common/common/slog"
	"github.com/rismaster/allris-common/config"
	"github.com/rismaster/allris-common/downloader"
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AnlageContainer struct {
	app          *application.AppContext
	webRessource *downloader.RisRessource
	file         *files.File
}

func NewVorlage(app *application.AppContext, ris *downloader.RisRessource) *AnlageContainer {

	return NewAnlageContainer(app,ris)
}

func NewSitzung(app *application.AppContext, ris *downloader.RisRessource) *AnlageContainer {

	return NewAnlageContainer(app,ris)
}

func NewTop(app *application.AppContext, ris *downloader.RisRessource) *AnlageContainer {

	return NewAnlageContainer(app,ris)
}

func NewAnlageContainer(app *application.AppContext, ris *downloader.RisRessource) *AnlageContainer {

	return &AnlageContainer{
		app:          app,
		webRessource: ris,
		file:         files.NewFile(app, ris),
	}
}

func (a *AnlageContainer) GetName() string {
	return a.file.GetNameWithoutExtension()
}

func (a *AnlageContainer) GetPath() string {
	return a.file.GetPath()
}

func (a *AnlageContainer) GetFolder() string {
	return a.file.GetFolder()
}

func (a *AnlageContainer) GetUrl() string {
	return a.webRessource.GetUrl()
}

func (a *AnlageContainer) Download() error {

	dom, err := a.downloadAndSave()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error downloading: %s %+v", a.GetPath()))
	}

	existingAnlagen := make(map[string]bool)

	selector := "#allriscontainer"
	var risToDownload []downloader.RisRessource

	dom.Find(selector).Each(func(index int, dom *goquery.Selection) {
		risAnlagen := a.extractAnlagen(dom)
		risAnlageDocs := a.extractBasisAnlagen(dom)
		for _, anlageRis := range risAnlagen {
			anlage := NewAnlage(a.app, &anlageRis)
			existingAnlagen[anlage.GetPath()] = true
			risToDownload = append(risToDownload, anlageRis)
		}
		for _, ad := range risAnlageDocs {
			anlageDoc := NewAnlageDocument(a.app, &ad)
			existingAnlagen[anlageDoc.GetPath()] = true
			risToDownload = append(risToDownload, ad)
		}
	})

	slog.Info("loaded %d anlagen of %s", len(risToDownload), a.file.GetPath())

	var tops []*AnlageContainer
	existingTops := make(map[string]bool)
	if a.GetFolder() == config.SitzungenFolder {
		tops = a.extractTops(dom)
		for _, top := range tops {
			existingTops[top.GetPath()] = true
			risToDownload = append(risToDownload, *top.webRessource)
		}
	}

	childFolders := []string{}
	err = files.DeleteFilesIfNotInAndAfter(a.app, config.AnlagenFolder+a.GetName()+"-anlage-", existingAnlagen, childFolders, time.Time{})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error deleting %s", config.AnlagenFolder+a.GetName()))
	}

	if a.GetFolder() == config.SitzungenFolder {
		childFolders = []string{config.AnlagenFolder}
		err = files.DeleteFilesIfNotInAndAfter(a.app, config.TopFolder+a.GetName()+"-top-", existingTops, childFolders, time.Time{})
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error deleting %s", config.TopFolder+a.GetName()))
		}
	}

	return PublishRisDownload(a.app, risToDownload)
}

func (a *AnlageContainer) downloadAndSave() (*goquery.Document, error) {

	err := a.file.Fetch(files.HttpGet, a.webRessource, "text/html", true)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error downloading file from %s, Error: %+v", a.GetUrl(), err))
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(a.file.GetContent()))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error create dom from %s, Error: %+v", a.GetUrl(), err))
	}

	docCopy := doc.Clone()

	aLink := doc.Find("a")
	aLinkText := aLink.Text()

	docCopy.Find("a").ReplaceWith(aLinkText)

	docCleaned, _ := docCopy.Html()
	hash := common.Md5HashStr(docCleaned)

	err = a.file.WriteIfMoreActualAndDifferent(hash)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error writing to storage: %s, Error: %+v", a.GetUrl(), err))
	}

	return doc, nil
}

func (a *AnlageContainer) extractTops(dom *goquery.Document) (tops []*AnlageContainer) {

	if a.GetFolder() != config.SitzungenFolder {
		return tops
	}
	topRows := dom.Find("table a")
	topRows.Each(func(i int, selection *goquery.Selection) {
		lnk, _ := selection.Attr("href")
		if lnk == "" || !strings.HasPrefix(lnk, "to020.asp?TOLFDNR=") {
			return
		}
		lnkUrlAttr, _ := url.Parse(lnk)
		if lnkUrlAttr != nil {

			var validID = regexp.MustCompile(`to020\.asp\?TOLFDNR=([0-9]+)`)
			matches := validID.FindStringSubmatch(lnkUrlAttr.String())
			if len(matches) >= 2 {

				tolfdnr, errT := strconv.Atoi(matches[1])
				if errT == nil && tolfdnr > 0 {

					name := fmt.Sprintf("%s-%s-%d", a.webRessource.GetName(), config.TopType, tolfdnr)
					ending := ".html" //filename contains ending
					created := a.webRessource.GetCreated()
					uri, err := url.Parse(config.TargetToParse + fmt.Sprintf("to020.asp?TOLFDNR=%d", tolfdnr))
					if err == nil {
						doc := downloader.NewRisRessource(config.TopFolder, name, ending, created, uri, &url.Values{})
						tops = append(tops, NewTop(a.app, doc))
					}
				}
			}
		}
	})
	slog.Info("loaded %d tops of %s", len(tops), a.file.GetPath())
	return tops
}

func (a *AnlageContainer) extractAnlagen(dom *goquery.Selection) (docs []downloader.RisRessource) {

	theAnlagenTables := dom.Find("table.tk1")
	if theAnlagenTables.Size() <= 1 {
		return docs
	}

	trs := theAnlagenTables.Last().Find("tr")
	if trs.Size() < 2 || trs.Next().Children().Size() < 2 {
		return docs
	}

	trs.Each(func(i int, selection *goquery.Selection) {
		tds := selection.Find("td")
		if i > 2 && tds.Size() >= 3 {

			lnk := tds.Get(2).FirstChild
			if lnk != nil {
				href := domtools.GetAttrFromNode(lnk, "href")

				description := domtools.GetChildTextFromNode(lnk)
				rp := regexp.MustCompile("(.*)[(]([0-9]+ KB)[)]")
				groups := rp.FindAllStringSubmatch(description, -1) // ["a
				var size = "0 kb"
				if groups != nil && len(groups) > 0 && len(groups[0]) > 2 {
					size = domtools.CleanText(groups[0][2])
				}

				name := fmt.Sprintf("%s-%s-%s-%s", a.webRessource.GetName(), config.AnlageType, size, filepath.Base(href))
				ending := "" //filename contains ending
				created := a.webRessource.GetCreated()
				uri, err := url.Parse(config.TargetToParse + href)
				if err == nil {
					doc := downloader.NewRisRessource(config.AnlagenFolder, name, ending, created, uri, &url.Values{})
					docs = append(docs, *doc)
				}
			}
		}
	})
	return docs
}

func (a *AnlageContainer) extractBasisAnlagen(dom *goquery.Selection) (docs []downloader.RisRessource) {

	theTopTable := dom.Find(".me1 > table.tk1").First()
	selector := "form[action=\"" + config.UrlAnlagedoc + "\"]"
	var form = theTopTable.Find(selector)
	for ; form.Nodes != nil; form = form.NextFiltered(selector) {
		dolfdnr := domtools.ExtractIntFromInput(form, "DOLFDNR")
		opts := domtools.ExtractIntFromInput(form, "options")
		annots := domtools.ExtractIntFromInput(form, "annots")
		formData := url.Values{}
		formData.Add("options", strconv.Itoa(opts))
		formData.Add("DOLFDNR", strconv.Itoa(dolfdnr))
		formData.Add("annots", strconv.Itoa(annots))

		name := fmt.Sprintf("%s-%s-%d-%d", a.webRessource.GetName(), config.AnlageDocumentType, dolfdnr, dolfdnr%100)
		ending := ".pdf"
		created := a.webRessource.GetCreated()
		uri, err := url.Parse(config.TargetToParse + config.UrlAnlagedoc)

		if err == nil {
			doc := downloader.NewRisRessource(config.AnlagenFolder, name, ending, created, uri, &formData)
			docs = append(docs, *doc)
		}

	}
	return docs
}
