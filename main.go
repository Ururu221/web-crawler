package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var mu sync.Mutex

var wg sync.WaitGroup

var db *gorm.DB

type Result struct {
	Index          int64
	GoTo           string
	From           string
	URL            string
	InfoAboutUrl   int
	Depth          int
	IndexForURl    int
	IsNewIteration bool
}

type InfoURLs struct {
	Info       []Result
	TotalLinks string
}

var totalAmountOfSpecialLinks int64
var totalAmountOfAllLinks int64
var linkIndexCounter int64

func crawl(url string, depth int, result *[]Result, passedLinks *sync.Map, printedLinks *sync.Map, semaphore *chan struct{}, indexToCrawl int64) error {
	*semaphore <- struct{}{}

	defer func() {
		<-*semaphore
		wg.Done()
	}()

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	depth -= 1
	allLinksOnPage := 0
	j := 1
	isNewIteration := false

	doc.Find("a").Each(func(i int, s *goquery.Selection) { // i -- это количество всех ссылок и с дубликатами
		href, exists := s.Attr("href")
		if exists {
			if strings.HasPrefix(href, "http") || strings.HasPrefix(href, "https") ||
				strings.HasPrefix(href, "/") {

				if _, loaded := printedLinks.LoadOrStore(href, true); !loaded { // проверка на дубликат
					absoluteURL, err := toAbsoluteURL(url, href)
					if err != nil {
						fmt.Println("\nPANIC ", err)
						return
					}

					crawlTo1Link := Result{
						Index:          indexToCrawl,
						GoTo:           url,
						Depth:          depth,
						URL:            absoluteURL,
						IndexForURl:    j,
						IsNewIteration: isNewIteration,
					}

					*result = append(*result, crawlTo1Link)

					j++ // счетчик ссылок без дубликатов на одной странице

					atomic.AddInt64(&totalAmountOfSpecialLinks, 1)
				}
			}
		}

		allLinksOnPage = i
		atomic.AddInt64(&totalAmountOfAllLinks, 1)
	})

	crawlTo1Link := Result{
		InfoAboutUrl:   allLinksOnPage,
		Index:          -1,
		IndexForURl:    -1,
		Depth:          -1,
		GoTo:           fmt.Sprintf("%s", url),
		IsNewIteration: true,
	}

	mu.Lock()
	*result = append(*result, crawlTo1Link)
	mu.Unlock()

	//RECURSIVE
	doc.Find("a").Each(func(i int, s *goquery.Selection) { // i -- это количество всех ссылок и с дубликатами
		href, exists := s.Attr("href")
		if exists {
			if depth > 0 && (strings.HasPrefix(href, "http") || strings.HasPrefix(href, "https") ||
				strings.HasPrefix(href, "/")) {

				if _, loaded := passedLinks.LoadOrStore(href, true); !loaded && depth > 0 { // проверка на дубликат
					absoluteURL, err := toAbsoluteURL(url, href)
					if err != nil {
						fmt.Println("\nPANIC ", err)
						return
					}

					wg.Add(1)

					atomic.AddInt64(&linkIndexCounter, 1)

					go crawl(absoluteURL, depth, result, passedLinks, printedLinks, semaphore, linkIndexCounter)
				}
			}
		}
	})

	atomic.StoreInt64(&linkIndexCounter, 0)

	defer fmt.Printf("%d: processing..\n", j)

	return nil
}

func crawlRequest(c echo.Context) error {
	_, err := template.ParseFiles("crawl-main.html")
	if err != nil {
		return err
	}

	url := c.FormValue("url")
	depth, err := strconv.Atoi(c.FormValue("depth"))
	if err != nil {
		return err
	}

	maxConcurrentRequests, err := strconv.Atoi(c.FormValue("maxConcurrentRequests"))
	if err != nil {
		return err
	}

	var result []Result // конечный массив в котором и будет информация для таблицы

	totalAmountOfSpecialLinks = 0 // количество всех ссылок на странице без дубликатов
	totalAmountOfAllLinks = 0     // количество всех ссылок на странице с дубликатами

	var passedLinks sync.Map
	var printedLinks sync.Map

	wg.Add(1)

	semaphore := make(chan struct{}, maxConcurrentRequests)

	timeStart := time.Now()

	err = crawl(url, depth, &result, &passedLinks, &printedLinks, &semaphore, 1)
	wg.Wait()
	if err != nil {
		fmt.Println(err)
		return err
	}

	totalTime := time.Since(timeStart)
	fmt.Printf("\n\n%v - total time to crawl: %s\nwith depth: %d\ntotal amount of special links: %d\nnumber of concurrent requests: %d\n",
		totalTime, url, depth, totalAmountOfSpecialLinks, maxConcurrentRequests)

	links := fmt.Sprintf("Total amount of links (without duplicates): %d\n"+
		"Total amount of links (with duplicates): %d", totalAmountOfSpecialLinks, totalAmountOfAllLinks)

	info := InfoURLs{
		Info:       result,
		TotalLinks: links,
	}

	return showURLs(c, &info)
}

func showURLs(c echo.Context, info *InfoURLs) error {
	tmpl, err := template.ParseFiles("submit.html")
	if err != nil {
		return err
	}
	return tmpl.Execute(c.Response(), info)
}

func main() {
	e := echo.New()

	//dsn := "host=localhost user=postgres password=2413050505 dbname=web_crawler port=1234 sslmode=disable"
	//var err error
	//db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	//if err != nil {
	//	log.Fatal(err)
	//}

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "authorization, dont ready yet")
	})

	e.GET("/crawl", func(c echo.Context) error {
		return c.File("crawl-main.html")
	})

	e.POST("/submit", crawlRequest)

	e.Logger.Fatal(e.Start(":5050"))
}

func toAbsoluteURL(base, href string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	relativeURL, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	absoluteURL := baseURL.ResolveReference(relativeURL)
	return absoluteURL.String(), nil
}
