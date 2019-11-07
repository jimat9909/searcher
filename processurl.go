// Package contains functions for parsing a given url
package processurl

import (
	
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strings"
	"io"
	"net/http"
	"log"
	
)

// structure to hold the results for a given URL
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
	// fmt.Println("GetURL: Searching for: ", url)
	
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
		// fmt.Printf("Brand New Token: TType %v TData: %v \n", tokenType, token.Data)
		switch tokenType {
				
		case html.StartTagToken:
		
			switch data {
			case "a": 
				var newURL, title string
				noFollow := false
				
				for _, attr := range token.Attr {
				    // fmt.Printf("a: Attribute: NS: %v Key: %v Val: %v\n", attr.Namespace, attr.Key, attr.Val)
				    
				    if attr.Key == "href" {
				    
				    	if strings.HasPrefix(attr.Val, "#") {
				    		// fmt.Printf("Not chasing down Val: %v\n", attr.Val)
				    		noFollow = true
					}
				    
				        if strings.HasPrefix(attr.Val, "http:") || strings.HasPrefix(attr.Val, "https:") {
				        	// fmt.Printf("Chase down fullVal: %v\n", attr.Val)
				        	newURL = attr.Val
				        } else if strings.Contains(attr.Val, ":") {
				        	// fmt.Printf("Not chasing other protocol: %v\n", attr.Val)
				        	noFollow = true
				        } else {
				    	    	newURL = url + "/" + attr.Val
				    	    	if strings.HasPrefix (attr.Val, "/") {
				    	    		newURL = url + attr.Val
				    	    	}
					    	// fmt.Printf("Chase down internal Val: %v\n", newURL)
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
				    	// fmt.Printf("Need to index title: %v\n", attr.Val)
				    }
				} // done processing attributes
				
				// last check in case we ended up with a relative link
				isRelativeLink := checkRelativeLink.FindStringSubmatch(newURL)
							
				if isRelativeLink != nil {
					// fmt.Printf("a tag Fixing %v\n", newURL )
					newURL = isRelativeLink[1]
				}
				// fmt.Printf("a tag results: URL: %v length: %v noFollow: %v title: %v\n", newURL, len(newURL), noFollow, title )
				if !noFollow {
					embeddedURL[newURL]++
				}
			
			case "title":  
				// Next token should be the title
				// tokenType := tokenizer.Next()
				tokenizer.Next()
				token := tokenizer.Token()
				pageTitle = strings.TrimSpace(token.Data)
				// fmt.Printf("Page Title: TType %v TData: %v \n", tokenType, token.Data)
			
			
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
	// fmt.Printf("ProcessURL: GetURL: URL %v Page Title: %v URLS %v Index entries %v\n", url, pageTitle, len(embeddedURL), len(thisIndex))
	
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
		// fmt.Printf("ATI2: adding \"%v\" to map len %v bytes %v more %+q\n", token, len(token), []byte(token), token)
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
