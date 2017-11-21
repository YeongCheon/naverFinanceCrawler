package main

import (
	"github.com/PuerkitoBio/goquery"

	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const APIURL string = "http://finance.naver.com/item/sise_day.nhn?code=%s&page=%d"

func main() {
	stockList, err := getStockListFromCsv()
	if err != nil {
		log.Fatal(err)
		return
	}

	for stockCode, _ := range stockList {
		lastPageNumber, err := getLastPage(stockCode)
		if err != nil {
			lastPageNumber = 1
			log.Println(err)
		}
		for i := 1; i <= lastPageNumber; i++ {
			parseData(stockCode, i)
			time.Sleep(2500 * time.Millisecond)
		}

	}
}

func parseData(stockCode string, pageNum int) error {
	url := fmt.Sprintf(APIURL, stockCode, pageNum)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return err
	}
	fmt.Println(url)

	doc.Find(`table.type2 tbody tr[onmouseover="mouseOver(this)"]`).Each(func(i int, selection *goquery.Selection) {
		date := selection.Find("td:nth-child(1) span").Text()                                 //날짜
		endPrice := getNumberFromPrice(selection.Find("td:nth-child(2) span").Text())         //종가
		compareYesterday := getNumberFromPrice(selection.Find("td:nth-child(3) span").Text()) //전일 대비
		price := getNumberFromPrice(selection.Find("td:nth-child(4) span").Text())            //시가
		highPrice := getNumberFromPrice(selection.Find("td:nth-child(5) span").Text())        // 고가
		lowPrice := getNumberFromPrice(selection.Find("td:nth-child(6) span").Text())         // 저가
		tradeCount := getNumberFromPrice(selection.Find("td:nth-child(7) span").Text())       //거래량

		fmt.Println(date, endPrice, compareYesterday, price, highPrice, lowPrice, tradeCount)
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
		fmt.Println("WTF!!!")
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
