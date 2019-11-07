package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"strings"
	"sort"
	"net/url"
	"github.com/jimat9909/processurl"
	
	
)
// Global Index of terms 

type IndexEntry struct {
	URL	string
	Count	int
}

type Index struct {
	entries map[string][]IndexEntry
	mux     sync.Mutex
}

func (gi *Index) Add(url string, results map[string]int) (int, int) {
	gi.mux.Lock()
	defer gi.mux.Unlock()
	
	total, unique := 0, 0
	for t, c := range results {
		_, ok := gi.entries[t]
		 if !ok {
		     unique++
		 }
		total++
		gi.entries[t] = append(gi.entries[t],IndexEntry{url, c})		
	}	
	return total, unique
}

func (gi *Index) GetTerm(term string) []IndexEntry{
	gi.mux.Lock()
	defer gi.mux.Unlock()
	
	info, ok := gi.entries[term]
	
	if !ok {
		return nil
	}
	if len(info) > 1 {
		info = SortEntries(info)
	}
	return info
}

func (gi *Index) Reset(){
	gi.mux.Lock()
	defer gi.mux.Unlock()
	for key, _ := range gi.entries {
		delete (gi.entries, key)
	}
}

// Map the URL's to their titles
// Used when diplaying results

type URLtitles struct {
	titles map[string]string
	mux    sync.Mutex
}

func (ut *URLtitles) Add(url, title string) {
	ut.mux.Lock()
	ut.titles[url] = title
	ut.mux.Unlock()
}

func (ut *URLtitles) Get(url string) (string, bool) {
	ut.mux.Lock()
	defer ut.mux.Unlock()
	title, ok := ut.titles[url]
	return title, ok
}

func (ut *URLtitles) Reset(){
	ut.mux.Lock()
	defer ut.mux.Unlock()
	for key, _ := range ut.titles {
		delete (ut.titles, key)
	}
}



// Keep track of the visited URL's and at what depth
type VisitedMap struct {
	v   map[string]int
	mux sync.Mutex
}

func (vm *VisitedMap) Visit(url string, depth int) {
	vm.mux.Lock()
	vm.v[url] = depth
	vm.mux.Unlock()
}

func (vm *VisitedMap) Value(url string) (int, bool) {
	vm.mux.Lock()
	defer vm.mux.Unlock()
	depth, ok := vm.v[url]
	return depth, ok
}

func (vm *VisitedMap) Reset(){
	vm.mux.Lock()
	defer vm.mux.Unlock()
	for key, _ := range vm.v {
		delete (vm.v, key)
	}
}

type crawlResult struct {
	url, title string
	depth, linkCount, indexCount int
}

type crawlSummary struct {
	uniquePages, uniqueTerms int
}

type crawlRequest struct {
	url 	string
	depth 	int
}

func CrawlURL (url string, token chan struct{})  processurl.UrlParseResults{
	token <- struct{}{}
	theseResults := processurl.GetURL(url)
	<-token
	return theseResults
}

func Crawl (rooturl string, depth, concurrency int, visited VisitedMap, index Index, titles URLtitles)  crawlSummary {

	// fmt.Printf("Crawl: Starting to retrieve root %v Depth %v\n", rooturl, depth)
	
	parsedrooturl, _ := url.Parse(rooturl)
	rootHost := strings.TrimPrefix(parsedrooturl.Host, "www.")
	//fmt.Printf("Crawl: rooturl scheme %v host %v\n", parsedrooturl.Scheme, rootHost)
	
	
	requestlist := make(chan []crawlRequest)
	tokens := make( chan struct{}, concurrency)
	
	uniquePages := 0
	uniqueTerms := 0
	rootRequest := crawlRequest{rooturl, 0}
	go func() {requestlist <- []crawlRequest{rootRequest} }()
	var n int
	n++
	for ; n > 0; n-- {
		
		fmt.Printf(".")
		requests := <-requestlist
		for _, request := range requests {
			
			// Clean up the requested url a little
			parsedRequestURL, _ := url.Parse(request.url)
			urlScheme := parsedRequestURL.Scheme + "://"
			cleanRequestURL := strings.TrimPrefix(request.url, urlScheme)
			cleanRequestURL = strings.TrimPrefix(cleanRequestURL, "www.")
			cleanRequestURL = strings.TrimSuffix(cleanRequestURL, "/")

			// Only check the Host, not the Scheme
			// Add in the returned depth for re-crawling at a "higher" level
			_, ok := visited.Value(cleanRequestURL)
			// fmt.Printf("CRAWL: Looking at request.url %v, clean %v, OK %v\n", request.url, cleanRequestURL, ok)
			if !ok {
				visited.Visit(cleanRequestURL, request.depth)
				if request.depth < 2 {
					n++
				}
				uniquePages++
				go func(request crawlRequest, token chan struct{}) {
					theseResults := CrawlURL(request.url, token)
					_, unique := index.Add(theseResults.URL, theseResults.Index) 
					//fmt.Printf("URL %v had  %v were unique\n", theseResults.URL, unique)
					titles.Add(theseResults.URL, theseResults.Title)
					uniqueTerms += unique
					if request.depth < 2 {
						newrequestlist := []crawlRequest{}
						for newurl, _ := range theseResults.EmbeddedURL {
							parsednewurl, err := url.Parse(newurl)
							if parsednewurl.Host == "" || err != nil {
								continue
							}
							newHost := strings.TrimPrefix(parsednewurl.Host, "www.")
							if newHost == rootHost {
								newrequest := crawlRequest{newurl, request.depth + 1}
								newrequestlist = append(newrequestlist, newrequest)
							}
							
							
						} // end going through the embedded urls
						// fmt.Printf("Crawl gofunc: Adding %v requests to list\n", len(newrequestlist))

						if len(newrequestlist) > 0 {
							requestlist <- newrequestlist
						} else {
							n--
						}
					} // end adding more work
				
				}(request, tokens)
			} 
		} // end going through this set of requests
		
	}
	// fmt.Printf("****** Crawl: Finally done with crawling root: Pages %v Terms %v\n", uniquePages, uniqueTerms)
	fmt.Printf("\n")
	return crawlSummary{uniquePages, uniqueTerms}
}



func main() {

	
	// Set up our main data structures 
	index := Index{entries: make(map[string][]IndexEntry)}
	visited := VisitedMap{v: make(map[string]int)}
	titles := URLtitles{titles: make(map[string]string)}
	
	processurl.InitializePunctuation()

	
	reader := bufio.NewReader(os.Stdin)
	cliLoop:
	for {
		fmt.Print("> ")
		lineIn, _ := reader.ReadString('\n')
		lineIn = strings.Replace(lineIn, "\r\n", "", -1)
		lineIn = strings.Trim(lineIn, " ");
		command := strings.SplitN(lineIn, " ", 2)
		
		switch command[0] {
			case "index", "i": 
				if command[1] != "" {
					GetURL(command[1], visited, index, titles)
				} else {
					fmt.Printf ("index command needs a url to crawl\n")
					Help()
				}
			case "search", "s": 
				if command[1] != "" {
					DisplayTerm(command[1], index, titles) 
				} else {
					fmt.Printf ("search command needs a term to look for\n")
					Help()
				}
		
			case "clear": 
				Reset(visited, index, titles)
			case "quit", "q": 
				break	cliLoop
				
			default:
				Help()
				
		}
		
	}
	
	fmt.Printf("Searcher terminating...\n")
	
	
}

// CLI commands and utilities follow

func GetURL (rooturl string, visited VisitedMap, index Index, titles URLtitles) {
	parsedUrl, err := url.Parse(rooturl)
	if err != nil {
		fmt.Printf("URL %v doesn't look good %v %+v\n", rooturl, err, parsedUrl)
		return
	}
	
	
	if parsedUrl.Scheme == "" {
		rooturl = "http://" + rooturl
	}
	
	
	fmt.Printf("Initiating crawl of %v \n", rooturl)
	results := Crawl(rooturl, 0, 10, visited, index, titles)
	fmt.Printf("Indexed %v pages and %v terms\n\n", results.uniquePages, results.uniqueTerms)
	return
}

func DisplayTerm (term string, index Index, titles URLtitles) {
	
	termList := index.GetTerm(term) 
	if termList == nil {
		fmt.Printf("Search term \"%v\" not found\n\n", term)
		return
	}
	fmt.Printf("Found %v results for search term \"%v\" :\n", len(termList), term)
	for _, entry := range termList {
		title, ok := titles.Get(entry.URL)
		if !ok {
			title = "UNKNOWN"
		}
		fmt.Printf("%v\n%v\nOccurences: %v\n\n", title,  entry.URL, entry.Count)
	}
	return
}

func Reset (visited VisitedMap, index Index, titles URLtitles) {
	index.Reset()
	visited.Reset()
	titles.Reset()
	fmt.Printf("Reset Index\n\n")

} 

func Help() {
	fmt.Printf("This search will crawl a URL and index the terms it finds. It will follow embedded links to a depth of 3, \n")
	fmt.Printf("however it will only follow links with the same hostname as that originally supplied. \n\n")
	fmt.Printf("The following commands are available:\n\n")
	fmt.Printf("\t index (url) \tThis will search and index the specified url and the links\n")
	fmt.Printf("\t search (term) \tThis will return the pages' URLS, titles and count that contain the search term\n")
	fmt.Printf("\t clear \tThis will reset the index\n")
	fmt.Printf("\t quit \tThis will quit the program\n")

}

func SortEntries (e []IndexEntry) []IndexEntry {
	sort.SliceStable(e, func(i, j int) bool { return e[i].Count > e[j].Count })
	return e

}
