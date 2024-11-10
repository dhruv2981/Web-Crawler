Web Crawler extracts data from web pages, following the specified CSS Selectors.It uses multiple threads running concurrently to speed up data collection.Each thread operates independently, allowing simultaneous data fetching from multiple web pages.
You can use it in many ways for data mining, data processing or archiving.

## The Web Scraping Pipeline
Web-scraping pipeline consists of 3 general components:

- **Downloading** an HTML web-page. (Fetch Service)
- **Parsing** an HTML page and retrieving data we're interested in (Parse Service)
- **Encoding** parsed data to CSV, MS Excel, JSON,or XML format.

## Fetch service
**fetch.d** server is intended for html web pages content download. 
Depending on Fetcher type, web page content is downloaded using either Base Fetcher or Chrome fetcher. 

Base fetcher uses standard golang http client to fetch pages as is. 
It works faster than Chrome fetcher. But Base fetcher cannot render dynamic javascript driven web pages. 

Chrome fetcher is intended for rendering dynamic javascript based content. It sends requests to Chrome running in headless mode.  

A fetched web page is passed to parse.d service. 

## Parse service
**parse.d** is the service that extracts data from downloaded web page following the rules listed in configuration JSON file. Extracted data is returned in CSV, MS Excel, JSON or XML format.

*Note: Sometimes Parse service cannot extract data from some pages retrieved by default Base fetcher. Empty results may be returned while parsing Java Script generated pages. Parse service then attempts to force Chrome fetcher to render the same dynamic javascript driven content automatically. Have a look at https://scrape.dataflowkit.com/persons/page-0 which is a sample of JavaScript driven web page.*   

## Benefits:

- Scraping of JavaScript generated pages;
- Data extraction from paginated websites;
- Cookies and sessions handling;
- Following links and detailed pages processing;
- Saving intermediate data in Diskv or Mongodb. Storage interface is flexible enough to add more storage types easily;
- Encode results to CSV, MS Excel, JSON(Lines), XML formats;
- It takes about 4-6 seconds to fetch and then parse 50 pages.
- Provides a built-in analytics dashboard to visualize and track scraped data,helping users quickly identify trends and make informed decisions.
- Utilizes state-of-the-art techniques like IP rotation, CAPTCHA solving, and header customization to avoid detection and ensure uninterrupted crawling. 

### Docker
1. Install [Docker](https://www.docker.com) and [Docker Compose](https://docs.docker.com/compose/install/)

2. Start services.

```
cd $GOPATH/src/github.com/dhruv2981/Web-Crawler && docker-compose up
```
This command fetches docker images automatically and starts services.

3. Launch parsing in the second terminal window by sending POST request to parse daemon. Some json configuration files for testing are available in /examples folder.
```
curl -XPOST  127.0.0.1:8001/parse --data-binary "@$GOPATH/src/github.com/dhruv2981/Web-Crawler/examples/books.toscrape.com.json"
```
Here is the sample json configuration file:

```
{
	"name":"collection",
	"request":{
	   "url":"https://example.com"
	},
	"fields":[
	   {
		  "name":"Title",
		  "selector":".product-container a",
		  "extractor":{
			 "types":["text", "href"],
			 "filters":[
				"trim",
				"lowerCase"
			 ],
			 "params":{
				"includeIfEmpty":false
			 }
		  }
	   },
	   {
		  "name":"Image",
		  "selector":"#product-container img",
		  "extractor":{
			 "types":["alt","src","width","height"],
			 "filters":[
				"trim",
				"upperCase"
			 ]
		  }
	   },
	   {
		  "name":"Buyinfo",
		  "selector":".buy-info",
		  "extractor":{
			 "types":["text"],
			 "params":{
				"includeIfEmpty":false
			 }
		  }
	   }
	],
	"paginator":{
	   "selector":".next",
	   "attr":"href",
	   "maxPages":3
	},
	"format":"json",
	"fetcherType":"chrome",
	"paginateResults":false
}
```
Read more information about scraper configuration JSON files at our [GoDoc reference](https://godoc.org/)


4. To stop services just press Ctrl+C and run 
``` 
cd $GOPATH/src/github.com/slotix/dataflowkit && docker-compose down --remove-orphans --volumes
```

### Manual way

1. Start Chrome docker container 
``` 
docker run --init -it --rm -d --name chrome --shm-size=1024m -p=127.0.0.1:9222:9222 --cap-add=SYS_ADMIN \
yukinying/chrome-headless-browser
```


[Headless Chrome](https://developers.google.com/web/updates/2017/04/headless-chrome) is used for fetching web pages to feed a Dataflow kit parser. 

2. Build and run fetch.d service
```
cd $GOPATH/src/github.com/dhruv2981/Web-Crawler/cmd/fetch.d && go build && ./fetch.d
```
3. In new terminal window build and run parse.d service
```
cd $GOPATH/src/github.com/dhruv2981/Web-Crawler/cmd/parse.d && go build && ./parse.d
```
4. Launch parsing. See step 3. from the previous section. 

### Run tests
- ```docker-compose -f test-docker-compose.yml up -d```
- ```./test.sh```
- To stop services just run ```docker-compose -f test-docker-compose.yml down```

### Group 20 Project

#### Team Members
- **Arpitha Goyal**  
  Roll Number: 22114010  

- **Dhruv**  
  Roll Number: 22114029  

- **Shreya Jain**  
  Roll Number: 22114091   

- **Somya Chawla**  
  Roll Number: 22114095  
 

> Created with 💖 by **Group 20**



