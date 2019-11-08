package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sync"
	"strconv"
	"strings"
	"sort"
	"net/url"
	"io"
	"net/http"
	"log"
	"golang.org/x/net/html"
	
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

type UrlParseResults struct {
	URL, Title 	string
	EmbeddedURL	map[string]int
	Index		map[string]int
}

// Used in cleaning up the content on a page
var Punctuation []string
var checkRelativeLink = regexp.MustCompile(`(.+?)#`)

var CaseSensitive = false
var IndexAnchorTitles = true

// Retrieve and parse the given URL
func GetURL(url string) UrlParseResults {

	
	pageTitle := url
	inBody := false
	
	embeddedURL := make(map[string]int)
	thisIndex := make(map[string]int)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("GetURL: Error on getting %v: %v \n", url, err)
		return UrlParseResults{url, pageTitle, nil, nil}
	}
	defer resp.Body.Close()
	tokenizer := html.NewTokenizer(resp.Body)
	for {
		tokenType := tokenizer.Next()

		if tokenType == html.ErrorToken {
			err := tokenizer.Err()
			if err == io.EOF {
				//end of the file, break out of the loop
				break
			}

			log.Fatalf("error tokenizing HTML: %v", tokenizer.Err())
		}

		token := tokenizer.Token()
		data := strings.TrimSpace(token.Data)
		switch tokenType {
				
		case html.StartTagToken:
		
			switch data {
			case "a": 
				var newURL, title string
				noFollow := false
				
				for _, attr := range token.Attr {				    
				    if attr.Key == "href" {
				    	if strings.HasPrefix(attr.Val, "#") {
				    		noFollow = true
					}
				    
				        if strings.HasPrefix(attr.Val, "http:") || strings.HasPrefix(attr.Val, "https:") {
				        	newURL = attr.Val
				        } else if strings.Contains(attr.Val, ":") {
				        	noFollow = true
				        } else {
				    	    	newURL = url + "/" + attr.Val
				    	    	if strings.HasPrefix (attr.Val, "/") {
				    	    		newURL = url + attr.Val
				    	    	}
					}	
				    }
				    
				    if attr.Key == "rel" && attr.Val == "nofollow" {
				    	noFollow = true
				    }
				    
				    if attr.Key == "title" {
				    	title = attr.Val
				    	if IndexAnchorTitles {
				    		addToURLIndex(title, thisIndex)
				    	}
				    }
				} // done processing attributes
				
				// last check in case we ended up with a relative link
				isRelativeLink := checkRelativeLink.FindStringSubmatch(newURL)
							
				if isRelativeLink != nil {
					newURL = isRelativeLink[1]
				}
				if !noFollow {
					embeddedURL[newURL]++
				}
			
			case "title":  
				tokenizer.Next()
				token := tokenizer.Token()
				pageTitle = strings.TrimSpace(token.Data)			
			
			case "body": 
				inBody = true
				
			case "style":
				// skip style sheets
				for {
					tokenizer.Next()
					token := tokenizer.Token()
					if strings.TrimSpace(token.Data) == "style" {
						break
					}
				}
			case "script":
				// skip scripts
				for {
					tokenizer.Next()
					token := tokenizer.Token()
					if strings.TrimSpace(token.Data) == "script" {
						break
					}
				}
			}

		case html.TextToken:
			if inBody && len(data) > 0 {
				// fmt.Printf("Text - need to index %v \n", token.Data)
				addToURLIndex(token.Data, thisIndex)
			}
				
		}
	}	
	return UrlParseResults{url, pageTitle, embeddedURL, thisIndex}
}

// Add this text to the index for this page
func addToURLIndex (s string, m map[string]int) {
	
	for _, p := range Punctuation {
		s = strings.Replace(s, p, "", -1)
	}
	
	s = strings.Replace(s, string([]byte{194, 160}), " ", -1)
	if !CaseSensitive {
		s = strings.ToLower(s)
	}
	tokens := strings.Split(s, " ")
	for _, token := range tokens {
	        token = strings.TrimSpace(token)
	        token = strings.TrimSuffix(token, ":")
		if len(token) > 0 {
			m[token]++
		}
	}

	return
}

// Load up the Pnctuation slice.  Used to get rid of extraneous characters that affect the indexing
func InitializePunctuation () {
	emdash := []byte{226, 128, 147}
	regtm := []byte{194, 174}
	copyright := []byte{194, 169}
        Punctuation = append(Punctuation,".", ",", ";", "-", "!", string(emdash), string(regtm), string(copyright))
	return
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

func CrawlURL (url string, token chan struct{})  UrlParseResults{
	token <- struct{}{}
	theseResults := GetURL(url)
	<-token
	return theseResults
}

var crawlwg sync.WaitGroup 
func Crawl (rooturl string, maxdepth, concurrency int, visited VisitedMap, index Index, titles URLtitles)  crawlSummary {
	
	parsedrooturl, _ := url.Parse(rooturl)
	rootHost := strings.TrimPrefix(parsedrooturl.Host, "www.")
	
	// need to make this a buffered channel so we can have multiple request sets 
	requestlist := make(chan []crawlRequest, 1000)
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
		
		// wait for all these requests to process 
		// to make sure all work is done before returning results
		crawlwg.Add(len(requests))
		for _, request := range requests {
			
			// Clean up the requested url a little
			parsedRequestURL, _ := url.Parse(request.url)
			urlScheme := parsedRequestURL.Scheme + "://"
			cleanRequestURL := strings.TrimPrefix(request.url, urlScheme)
			cleanRequestURL = strings.TrimPrefix(cleanRequestURL, "www.")
			cleanRequestURL = strings.TrimSuffix(cleanRequestURL, "/")

			
			// See if we need to visit this URL.  Don't include the scheme in the check
			
			doIndexing := true
			priorDepth, ok := visited.Value(cleanRequestURL)
			
			// check to see if the url has already been scanned.  If it had been scanned a deeper level
			// rescan it but don's add to the index any terms. This gets into delete / deep delete issues  
			// we want to rescan in case there are more links that we should scan
			if ok && request.depth < priorDepth {
				ok = false
				doIndexing = false
				// fmt.Printf("Crawl: rescanning %v prior depth: %v request depth %v\n", cleanRequestURL, priorDepth, request.depth)
			}
			
			if !ok {
				visited.Visit(cleanRequestURL, request.depth)
				if request.depth < maxdepth {
					n++
				}
				uniquePages++
				go func(request crawlRequest, doIndexing bool, token chan struct{}) {
					defer crawlwg.Done()
					theseResults := CrawlURL(request.url, token)
					if doIndexing {
						_, unique := index.Add(theseResults.URL, theseResults.Index) 
						uniqueTerms += unique
						titles.Add(theseResults.URL, theseResults.Title)
					}
					
					if request.depth < maxdepth {
						newrequestlist := []crawlRequest{}
						for newurl, _ := range theseResults.EmbeddedURL {
							parsednewurl, err := url.Parse(newurl)
							if parsednewurl.Host == "" || err != nil {
								continue
							}
							newHost := strings.TrimPrefix(parsednewurl.Host, "www.")
							if newHost == rootHost || CrawlForeign {
								newrequest := crawlRequest{newurl, request.depth + 1}
								newrequestlist = append(newrequestlist, newrequest)
							}
						} // end going through the embedded urls

						if len(newrequestlist) > 0 {
							requestlist <- newrequestlist
						} else {
							n--
						}
					} // end adding more work
				}(request, doIndexing, tokens)
			} else {
				// not going to do this url, so mark it complete
				crawlwg.Done()
			}
		} // end going through this set of requests
		// wait for each request to complete
		crawlwg.Wait()
		
	}
	fmt.Printf("\n")
	return crawlSummary{uniquePages, uniqueTerms}
}

// config variables

var version = "1.0.0"
var CrawlForeign = false
var Concurrency = 10
var MaxDepth = 2


func main() {

	fmt.Printf("Searcher %v initializing\n", version)
	// Set up our main data structures 
	index := Index{entries: make(map[string][]IndexEntry)}
	visited := VisitedMap{v: make(map[string]int)}
	titles := URLtitles{titles: make(map[string]string)}
	
	InitializePunctuation()
	
	reader := bufio.NewReader(os.Stdin)
	cliLoop:
	for {
		fmt.Print("> ")
		lineIn, _ := reader.ReadString('\n')
		lineIn = strings.Replace(lineIn, "\r\n", "", -1)
		lineIn = strings.Replace(lineIn, "\n", "", -1)
		lineIn = strings.Trim(lineIn, " ");
		command := strings.SplitN(lineIn, " ", 2)
		
		switch command[0] {
			case "index", "i": 
				if command[1] != "" {
					IndexURL(command[1], visited, index, titles)
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
			case "config": 
				ShowConfig()
			case "set": 
				Set(command[1])
			case "quit", "q": 
				break	cliLoop
				
			default:
				Help()
				
		}
		
	}
	
	fmt.Printf("Searcher terminating...\n")
	
	
}

// CLI commands and utilities follow

func IndexURL (rooturl string, visited VisitedMap, index Index, titles URLtitles) {
	parsedUrl, err := url.Parse(rooturl)
	if err != nil {
		fmt.Printf("URL %v doesn't look good %v %+v\n", rooturl, err, parsedUrl)
		return
	}
	
	
	if parsedUrl.Scheme == "" {
		rooturl = "http://" + rooturl
	}
	
	
	fmt.Printf("Initiating crawl of %v \n", rooturl)
	results := Crawl(rooturl, MaxDepth, Concurrency, visited, index, titles)
	fmt.Printf("Indexed %v pages and %v terms\n\n", results.uniquePages, results.uniqueTerms)
	return
}

func DisplayTerm (term string, index Index, titles URLtitles) {
	if !CaseSensitive {
		term = strings.ToLower(term)
	}
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
	fmt.Printf("\t config \tThis will show configuration settings\n")
	fmt.Printf("\t quit \tThis will quit the program\n")
	fmt.Printf("\n\t set (argument) \t\tset the configuration variable accordingly. Arguments are:\n")
	fmt.Printf("\t\tcase | nocase\tdefine case sensitivity for terms.  nocase means terms will be converted to lowercase prior to saving in the index\n")
	fmt.Printf("\t\tindexanchors | noindexanchors\tdefines whether or to index the title attribute on an anchor tag\n")
	fmt.Printf("\t\tcrawlforeign | nocrawlforeign\tdefines whether or to crawl links pointed to hosts outside the root domain\n")
	fmt.Printf("\t\tconcurrency (integer) Number of concurrent crawls.  Must be 1 or more\n")
	fmt.Printf("\t\tdepth (integer) Number of levels to crawl, the root url being level 1.  Must be 1 or more\n")

	

}

func ShowConfig() {
	fmt.Printf("Configuration settings:\n")
	fmt.Printf("\tCase Sensitive %v\tIf false, convert terms to lower case before indexing\n", CaseSensitive)
	fmt.Printf("\tIndex Anchors %v\tIf true, index the titles of anchor tags\n", IndexAnchorTitles)
	fmt.Printf("\tCrawl Foreign %v\tIf true, crawl links to URLs outside of the root URL domain\n", CrawlForeign)
	fmt.Printf("\tMaximum Depth %v\t\tHow many levels of embedded links to crawl\n", MaxDepth + 1)
	fmt.Printf("\tConcurrency %v\t\tHow many concurrent pages to crawl\n", Concurrency)
	
}

func Set(command string) {

	commandArgs := strings.SplitN(command, " ", 2)
	switch commandArgs[0] {
		case "case": 
			CaseSensitive = true
			fmt.Printf("Indexing is now case sensitive\n")
		case "nocase":
			CaseSensitive = false
			fmt.Printf("Indexing is now case insensitive\n")
		case "indexanchors":
			IndexAnchorTitles = true
			fmt.Printf("Anchor titles will be indexed\n")
		case "noindexAnchors":
			IndexAnchorTitles = false
			fmt.Printf("Anchor titles will not be indexed\n")
		case "crawlforeign":
			CrawlForeign = true
			fmt.Printf("Links outside of the root domain will be searched\n")
		case "nocrawlforeign":
			CrawlForeign = false
			fmt.Printf("Links outside of the root domain will not be searched\n")
		case "concurrency": 
			i, err := strconv.Atoi(commandArgs[1])
			if err != nil {
				fmt.Printf("%v not integer: %v\n", commandArgs[1], err)
				break
			}
			
			if i < 1 {
				fmt.Printf("Concurrency must be greater than 0\n")
				break
			}
			Concurrency = i;
			fmt.Printf("Concurrency set to %v\n", Concurrency)
			
		case "depth": 
			i, err := strconv.Atoi(commandArgs[1])
			if err != nil {
				fmt.Printf("%v not integer: %v\n", commandArgs[1], err)
					break
				}
					
				if i < 1 {
					fmt.Printf("Depth must be greater than 0\n")
					break
				}
			MaxDepth = i - 1
			fmt.Printf("Depth set to %v\n", MaxDepth + 1)
			
			
					
		default:
			Help()
					
		}
}

func SortEntries (e []IndexEntry) []IndexEntry {
	sort.SliceStable(e, func(i, j int) bool { return e[i].Count > e[j].Count })
	return e

}
