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
)

var db *gorm.DB

type InfoURLs struct {
	Info string
}

var totalAmountOfLinks int

func crawl(url string, depth int, resultString *string, passedLinks *[]string, printedLinks *[]string) error {
	//*resultString += fmt.Sprintf("\ngo to %s\n", url)

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

	*resultString += fmt.Sprintf("\n- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - \n")
	*resultString += fmt.Sprintf("go to %s\n", url)

	allLinksOnPage := 0

	j := 0

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			if strings.HasPrefix(href, "http") || strings.HasPrefix(href, "https") ||
				strings.HasPrefix(href, "/") {

				isDuplicate := false

				for _, link := range *printedLinks {
					if link == href {
						isDuplicate = true
						break
					}
				}

				if !isDuplicate {
					absoluteURL, err := toAbsoluteURL(url, href)
					if err != nil {
						panic(err)
					}

					*printedLinks = append(*printedLinks, absoluteURL)
					*resultString += fmt.Sprintf("\n%d: %s, depth: %d\n", j+1, absoluteURL, depth)

					j++
					totalAmountOfLinks++
				}
			}
		}

		allLinksOnPage = i

	})

	*resultString += fmt.Sprintf("\n===================== amount of all links on this page (with duplicate) is %d\n",
		allLinksOnPage)

	//RECURSIVE
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			if depth > 0 && (strings.HasPrefix(href, "http") || strings.HasPrefix(href, "https") ||
				strings.HasPrefix(href, "/")) {
				isDuplicate := false
				for _, link := range *passedLinks {
					absoluteURL, err := toAbsoluteURL(url, href)
					if err != nil {
						panic(err)
					}

					if absoluteURL == link {
						//fmt.Printf("\n%d: %s is duplicate depth: %d\n", i+1, href, *depth)
						isDuplicate = true
						break
					}
				}

				if !isDuplicate && depth > 0 {
					absoluteURL, err := toAbsoluteURL(url, href)
					if err != nil {
						panic(err)
					}

					*resultString += fmt.Sprintf("\n%d CRAWL TO: %s, depth: %d\n", i+1, absoluteURL, depth)
					*passedLinks = append(*passedLinks, absoluteURL)

					crawl(href, depth, resultString, passedLinks, printedLinks)

				}
			}
		}
	})

	defer fmt.Printf("\n\npassed links: \n%v", *passedLinks)

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

	result := ""
	totalAmountOfLinks = 0

	err = crawl(url, depth, &result, &[]string{}, &[]string{})
	if err != nil {
		return err
	}

	links := fmt.Sprintf("total amount of links: %d", totalAmountOfLinks)

	result = fmt.Sprintf("%s\n\n%s", links, result)

	info := InfoURLs{
		Info: result,
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
