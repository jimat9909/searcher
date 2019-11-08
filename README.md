# searcher

This program will crawl and index the terms  on a URL.  It will follow embedded links and index those pages as well, however it will only go to a depth of 3.

Building and Running
--------------------
I have used the golang.org/x/net/html package (https://godoc.org/golang.org/x/net/html). 

    go get golang.org/x/net/html
    

The searcher is run using the CLI. 

The 'index (url)' command is used to crawl and index a URL

The 'search (term)' command is used to display the results for that term.

Example session is shown below:
```
> index www.patsgames.com
Initiating crawl of http://www.patsgames.com
............................................................
Indexed 331 pages and 1182 terms

> search business
Found 2 results for search term "business" :
Pat’s Games – The Premiere Store For Magic The Gathering
http://www.patsgames.com
Occurences: 3

Our Community – Pat’s Games
http://www.patsgames.com/our-community/
Occurences: 1

>
```


The following CLI commands are supported:
```

         index (url)    This will search and index the specified url and the links
         search (term)  This will return the pages' URLS, titles and count that contain the search term
         clear  This will reset the index
         config         This will show configuration settings
         quit   This will quit the program

         set (argument)                 	set the configuration variable accordingly. Arguments are:
                case | nocase   		define case sensitivity for terms.  nocase means terms will be converted to lowercase prior to saving in the index
                indexanchors | noindexanchors   defines whether or to index the title attribute on an anchor tag
                crawlforeign | nocrawlforeign   defines whether or to crawl links pointed to hosts outside the root domain
                concurrency (integer) 		Number of concurrent crawls.  Must be 1 or more
                depth (integer) 		Number of levels to crawl, the root url being level 1.  Must be 1 or more

```

Default Configuration
----------------------
  
Searching Foreign Sites
=======================

By default, the searcher will only follow links that have the same host as the original index request. The CLI commands
```
	set crawlforeign
	set nocrawlforeign
```
will control this.  

Case Sensitivity
================

By default, terms will be converted to lower case before indexing.  A given search term will also be converted before looking up the results. The CLI commands
```
	set case
	set nocase
```
will control the case sensitivity.

Indexing Anchor Titles
======================

By default the text in an anchor tag's title attribute will be indexed.  The CLI commands 
```
	set indexanchors
	set noindexanchors
```
can be used to control this.

Depth
=====

As indicated, the searcher will follow embedded links to a depth of 3.  This may be configured by the 
```
	set depth N
```
CLI command.

Concurrency
===========

The concurrency variable is used to control the number of concurrent URL parsers running.  This may be controlled by the 
```
	set concurrency N
```
CLI command.
