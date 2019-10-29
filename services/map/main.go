package main

import (
	"GitHub.com/tidwall/gjson"
	"fmt"
	"github.com/Luxurioust/excelize"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func getLonLat(p_address string) {
	time.Sleep(time.Duration(1) * time.Second)
	var resp, _ = http.Get("https://apis.map.qq.com/ws/geocoder/v1/?address=" + p_address + "&key=AIABZ-VEHW6-3EJSW-MI7VB-NPHK7-PBFNA")

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	sbody := string(body)

	p_lng := gjson.Get(sbody, "result.location.lng")
	p_lat := gjson.Get(sbody, "result.location.lat")

	fmt.Print(p_lng)
	fmt.Print(" ")
	fmt.Println(p_lat)

	if err != nil {
		log.Fatal(err)
	}
}

func getExcelData() {
	xlsx, err := excelize.OpenFile("C:\\Users\\37646\\Downloads\\地图定点绘制.xlsx")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	xlsx.GetRows(xlsx.GetSheetName(xlsx.GetActiveSheetIndex()))

	rows, err := xlsx.GetRows("Sheet1")
	for _, row := range rows {
		for cidx, colCell := range row {
			if cidx == 4 || cidx == 6 {
				fmt.Print(colCell, "\t")
				getLonLat(colCell)
			}
		}
		fmt.Println()
	}
}

func main() {
	getExcelData()
}
