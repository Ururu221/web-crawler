package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
	// for web crawler
	Index          int64
	GoTo           string
	From           string
	URL            string
	InfoAboutUrl   int
	Depth          int
	IndexForURl    int
	IsNewIteration bool
	// for search by word
	SentenceWithWord string
	NumberOfWord     int
}

type InfoURLs struct {
	Info       []Result
	TotalLinks string
}

var totalAmountOfSpecialLinks int64
var totalAmountOfAllLinks int64
var linkIndexCounter int64

func crawl(url string, depth int, result *[]Result, passedLinks *sync.Map, printedLinks *sync.Map, semaphore *chan struct{}, indexToCrawl int64, searchByWordFunc bool, searchWord string) error {
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

	contentType := res.Header.Get("Content-Type")

	var reader io.Reader
	if strings.Contains(contentType, "charset=windows-1251") {
		reader = transform.NewReader(res.Body, charmap.Windows1251.NewDecoder()) // Преобразование из Windows-1251 в UTF-8
	} else {
		reader = res.Body // Если кодировка UTF-8 или не указана, используем как есть
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return err
	}

	depth -= 1
	allLinksOnPage := 0
	j := 1 // счетчик ссылок без дубликатов на одной странице
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

	//ЛОГИКА ПОИСКА СЛОВА НА СТРАНИЦЕ
	if searchByWordFunc && searchWord != "" {
		doc.Find("body").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			matches := findSentencesWithWord(text, searchWord)

			if len(matches) > 0 {
				for i, match := range matches {
					crawlTo1Link := Result{
						Index:            int64(i + 1),
						GoTo:             url,
						Depth:            depth,
						SentenceWithWord: match,
						NumberOfWord:     len(matches),
					}

					mu.Lock()
					*result = append(*result, crawlTo1Link)
					mu.Unlock()
				}
			}
		})
	}
	//

	crawlTo1Link := Result{
		InfoAboutUrl:   allLinksOnPage,
		Index:          -1,
		IndexForURl:    -1,
		Depth:          -1,
		GoTo:           url,
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

					go crawl(absoluteURL, depth, result, passedLinks, printedLinks, semaphore, linkIndexCounter, false, "")
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

	err = crawl(url, depth, &result, &passedLinks, &printedLinks, &semaphore, 1, false, "")
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

	return showURLs(c, &info, "submit.html")
}

func searchByWord(c echo.Context) error {
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

	word := c.FormValue("word")

	var result []Result // конечный массив в котором и будет информация для таблицы

	totalAmountOfSpecialLinks = 0 // количество всех ссылок на странице без дубликатов
	totalAmountOfAllLinks = 0     // количество всех ссылок на странице с дубликатами

	var passedLinks sync.Map
	var printedLinks sync.Map

	wg.Add(1)

	semaphore := make(chan struct{}, maxConcurrentRequests)

	timeStart := time.Now()

	err = crawl(url, depth, &result, &passedLinks, &printedLinks, &semaphore, 1, true, word)
	wg.Wait()
	if err != nil {
		fmt.Println(err)
		return err
	}

	totalTime := time.Since(timeStart)

	fmt.Printf("\n\nWORD SEARCH\n")

	fmt.Printf("\n%v - total time to crawl: %s\nwith depth: %d\ntotal amount of special links: %d\nnumber of concurrent requests: %d\n",
		totalTime, url, depth, totalAmountOfSpecialLinks, maxConcurrentRequests)

	links := fmt.Sprintf("Total amount of links (without duplicates): %d\n"+
		"Total amount of links (with duplicates): %d", totalAmountOfSpecialLinks, totalAmountOfAllLinks)

	info := InfoURLs{
		Info:       result,
		TotalLinks: links,
	}

	return showURLs(c, &info, "submit2.html")
}

func showURLs(c echo.Context, info *InfoURLs, file string) error {
	tmpl, err := template.ParseFiles(file)
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

	e.POST("/submit2", searchByWord)

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

func findSentencesWithWord(text, searchWord string) []string {
	// Регулярное выражение для поиска предложений, заканчивающихся на ., !, ? и другие знаки
	re := regexp.MustCompile(`(?i)[^.!?…;:]*\b` + regexp.QuoteMeta(searchWord) + `\b[^.!?…;:]*[.!?…;:]`)

	matches := re.FindAllString(text, -1)

	// Если совпадений с предложениями не найдено, проверяем на простое нахождение слова
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(searchWord) + `\b`)
	simpleMatches := wordRe.FindAllString(text, -1)

	// Добавляем совпадения без предложений, если они не были найдены как предложения
	if len(matches) == 0 && len(simpleMatches) > 0 {
		matches = append(matches, simpleMatches...)
	}

	// Убираем пробелы в начале и в конце предложений
	for i, match := range matches {
		matches[i] = strings.TrimSpace(match)
	}

	// Убираем дубликаты
	uniqueMatches := make(map[string]struct{})
	cleanedMatches := []string{}
	for _, match := range matches {
		if _, exists := uniqueMatches[match]; !exists {
			uniqueMatches[match] = struct{}{}
			cleanedMatches = append(cleanedMatches, match)
		}
	}

	return cleanedMatches
}
