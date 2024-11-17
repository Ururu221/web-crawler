# Web Crawler Project

## Overview

This project is a multi-threaded web crawler built using Go. The crawler fetches URLs from a starting page, follows links recursively based on the specified depth, and displays results in a web interface using an HTML table.

## Features

- Multi-threaded crawling using Goroutines and synchronization mechanisms (WaitGroup, sync.Map).
- Customizable crawling depth and concurrency control.
- Results displayed in an HTML table with detailed information for each link, including:
  - Index of the link
  - Go To (source URL)
  - From (referenced page)
  - URL (linked URL)
  - Depth
  - Total links found with and without duplicates.
- User-friendly web interface built with the Echo framework for web server management and GoQuery for HTML parsing.


## Usage

1. Go to [http://localhost:5050/crawl](http://localhost:5050/crawl) in your web browser.
2. Enter the starting URL and the desired crawling depth.
3. Optionally, specify the maximum number of concurrent requests.
4. Click the "Submit" button to start crawling.

The results will be displayed in a table format with details about each link.

## Project Structure

```
.
├── main.go               # Main application logic for the web crawler
├── crawl-main.html       # HTML form for inputting crawl parameters
├── submit.html           # HTML template for displaying crawl results
├── README.md             # Project documentation
└── go.mod                # Go module file for dependency management
```

## Configuration

- `maxConcurrentRequests`: Maximum number of concurrent requests can be set via the form input on the web interface.
- `depth`: Maximum depth for crawling links from the starting URL.

## Technologies Used

- [Go](https://golang.org/)
- [GoQuery](https://github.com/PuerkitoBio/goquery) - HTML parsing
- [Echo](https://echo.labstack.com/) - Web framework
- [sync](https://pkg.go.dev/sync) - Synchronization primitives for multi-threading

## Customization

To customize the web crawling behavior or change the HTML templates, you can modify `main.go`, `crawl-main.html`, and `submit.html`.

### Example Configuration

```go
// Example configuration for maximum concurrency and depth
maxConcurrentRequests := 5
depth := 3
```

## Contributing

Feel free to submit issues or fork the repository and open a pull request with your improvements.
