package main

import (
	"github.com/PuerkitoBio/goquery"

	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const APIURL string = "http://finance.naver.com/item/sise_day.nhn?code=%s&page=%d"
const ES_URL = "http://127.0.0.1:9200"
const ES_INDEX_NAME string = "stock"
const ES_TYPE_NAME string = "daily"

type DailyStock struct {
	Date             string `json:"date"`
	EndPrice         int    `json:"endPrice"`
	CompareYesterday int    `json:"compareYesterday"`
	Price            int    `json:"price"`
	HighPrice        int    `json:"highPrice"`
	LowPrice         int    `json:"lowPrice"`
	TradeCount       int    `json:"tradeCount"`
}

func main() {
	stockList, err := getStockListFromCsv()
	if err != nil {
		log.Fatal(err)
		return
	}

	if !isExsitIndex(ES_INDEX_NAME) {
		createIndex(ES_INDEX_NAME)
	}

	for stockCode, _ := range stockList {
		lastPageNumber, err := getLastPage(stockCode)
		if err != nil {
			lastPageNumber = 1
			log.Println(err)
		}
		for i := 1; i <= lastPageNumber; i++ {
			parseData(stockCode, i)
			time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
		}
	}

	t := time.Now()
	for {
		if t.Format("2006-01-02") == time.Now().Format("2006-01-02") && time.Now().Hour() >= 20 { //매일 20시 이후에 한번씩 실행
			for stockCode, _ := range stockList {
				parseTodayData(stockCode, t)
			}
			t.AddDate(0, 0, 1)
		}
	}
}

func parseTodayData(stockCode string, t time.Time) {

	url := fmt.Sprintf(APIURL, stockCode, 1)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Println(err)
		return
	}

	doc.Find(`table.type2 tbody tr[onmouseover="mouseOver(this)"]`).Each(func(i int, selection *goquery.Selection) {
		date := selection.Find("td:nth-child(1) span").Text()                                 //날짜(yyyy.MM.dd)
		endPrice := getNumberFromPrice(selection.Find("td:nth-child(2) span").Text())         //종가
		compareYesterday := getNumberFromPrice(selection.Find("td:nth-child(3) span").Text()) //전일 대비
		price := getNumberFromPrice(selection.Find("td:nth-child(4) span").Text())            //시가
		highPrice := getNumberFromPrice(selection.Find("td:nth-child(5) span").Text())        // 고가
		lowPrice := getNumberFromPrice(selection.Find("td:nth-child(6) span").Text())         // 저가
		tradeCount := getNumberFromPrice(selection.Find("td:nth-child(7) span").Text())       //거래량

		date = strings.Replace(date, ".", "-", -1)
		if selection.Find("td:nth-child(3) img").AttrOr("alt", "상승") == "상승" {
			compareYesterday = compareYesterday * 1
		} else { // 하락
			compareYesterday = compareYesterday * -1
		}

		stock := DailyStock{
			Date:             date,
			EndPrice:         endPrice,
			CompareYesterday: compareYesterday,
			Price:            price,
			HighPrice:        highPrice,
			LowPrice:         lowPrice,
			TradeCount:       tradeCount,
		}

		if t.Format("2006-01-02") == date {
			insertDailyStock(stock)
		}

	})
}

func parseData(stockCode string, pageNum int) error {
	url := fmt.Sprintf(APIURL, stockCode, pageNum)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return err
	}
	fmt.Println(url)

	doc.Find(`table.type2 tbody tr[onmouseover="mouseOver(this)"]`).Each(func(i int, selection *goquery.Selection) {
		date := selection.Find("td:nth-child(1) span").Text()                                 //날짜(yyyy.MM.dd)
		endPrice := getNumberFromPrice(selection.Find("td:nth-child(2) span").Text())         //종가
		compareYesterday := getNumberFromPrice(selection.Find("td:nth-child(3) span").Text()) //전일 대비
		price := getNumberFromPrice(selection.Find("td:nth-child(4) span").Text())            //시가
		highPrice := getNumberFromPrice(selection.Find("td:nth-child(5) span").Text())        // 고가
		lowPrice := getNumberFromPrice(selection.Find("td:nth-child(6) span").Text())         // 저가
		tradeCount := getNumberFromPrice(selection.Find("td:nth-child(7) span").Text())       //거래량

		date = strings.Replace(date, ".", "-", -1)
		if selection.Find("td:nth-child(3) img").AttrOr("alt", "상승") == "상승" {
			compareYesterday = compareYesterday * 1
		} else { // 하락
			compareYesterday = compareYesterday * -1
		}

		stock := DailyStock{
			Date:             date,
			EndPrice:         endPrice,
			CompareYesterday: compareYesterday,
			Price:            price,
			HighPrice:        highPrice,
			LowPrice:         lowPrice,
			TradeCount:       tradeCount,
		}

		insertDailyStock(stock)
	})

	return nil
}

func getLastPage(stockCode string) (int, error) {
	url := fmt.Sprintf(APIURL, stockCode, 1)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return 0, err
	}

	endHref, isExist := doc.Find("table.Nnavi tbody tr td.pgRR a").Attr("href")
	if !isExist {
		return 0, nil //FIXME
	}
	tmp := strings.Split(endHref, "=")
	lastPage, _ := strconv.Atoi(tmp[len(tmp)-1])

	return lastPage, nil
}

func getStockListFromCsv() (map[string]string, error) {
	listFile, err := os.Open("./data.csv")
	if err != nil {
		return nil, err
	}

	rdr := csv.NewReader(bufio.NewReader(listFile))
	rows, err := rdr.ReadAll()

	stockMap := make(map[string]string)

	for _, row := range rows {
		stockCode := getFormattedStock(row[1])
		companyName := row[2]

		stockMap[stockCode] = companyName
	}

	return stockMap, nil
}

func getFormattedStock(stockCode string) string {
	const DIGIT_COUNT int = 6

	if len(stockCode) < DIGIT_COUNT {
		var fillingText string
		for i := 0; i < DIGIT_COUNT-len(stockCode); i++ {
			fillingText = "0" + fillingText
		}

		stockCode = fillingText + stockCode
	}

	return stockCode
}

func getNumberFromPrice(price string) int { //remove comma in price
	result, err := strconv.Atoi(strings.Replace(strings.TrimSpace(price), ",", "", -1))
	if err != nil {
		log.Println(price)
		log.Println(err)
		return 0
	}
	return result
}

func isExsitIndex(esIndexName string) bool {
	res, err := http.Head(ES_URL + "/" + esIndexName)
	if err != nil {
		log.Println(err)
		return false
	}
	if res.StatusCode == 404 {
		return false
	} else {
		return true
	}
}

func createIndex(esIndexName string) {
	client := &http.Client{}
	f, err := os.Open("./setting.json")
	if err != nil {
		log.Println(err)
	}

	req, err := http.NewRequest(http.MethodPut, ES_URL+"/"+esIndexName, f)
	if err != nil {
		log.Println(err)
	}
	req.Header.Add("content-type", "application/json")

	_, err = client.Do(req)
	if err != nil {
		log.Println(err)
	}

	log.Println("create index")
}

func insertDailyStock(dailyStock DailyStock) {
	client := &http.Client{}

	data := new(bytes.Buffer)
	json.NewEncoder(data).Encode(dailyStock)

	req, err := http.NewRequest(http.MethodPost, ES_URL+"/"+ES_INDEX_NAME+"/"+ES_TYPE_NAME, data)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Add("content-type", "application/json")

	_, err = client.Do(req)
	if err != nil {
		log.Println(err)
	}
}
