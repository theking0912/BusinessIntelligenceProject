package test

import (
	. "domain/model"
	"encoding/json"
	"infrastructure/log"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"infrastructure/gopkg.in/mgo.v2/bson"
)

type Location struct {
	Lng float64 //经度
	Lat float64 //纬度
}

type AddressComponent struct {
	country       string
	Province      string //省
	City          string //市
	District      string //区县
	Street        string //街道
	Street_number string //街道门牌号
	adcode        string
	country_code  uint64
	direction     string
	distance      string
}

type Result struct {
	Location            Location
	formatted_address   string
	business            string
	AddressComponent    AddressComponent
	poiRegions          []interface{}
	sematic_description string
	cityCode            uint64
}

type Strmap struct {
	Status int
	Result Result
}

var (
	baiduMapCityMap = map[string]string{
		"海东地区":  "海东市",
		"日喀则地区": "日喀则市",
	}
)

func (this *Crontab) GeocodingBaidu(dbname string) (isAccess bool) {
	isAccess = true
	queryMap := bson.M{"经纬度合法": "是", "手动": bson.M{"$exists": false}}
	queryMap["$or"] = []bson.M{bson.M{"地市": bson.M{"$exists": false}}, bson.M{"地市": ""}}
	queryMap["longitude"] = bson.M{"$exists": true}
	queryMap["latitude"] = bson.M{"$exists": true}
	mapapi := "http://api.map.baidu.com/geocoder/v2/?ak=******&output=json&location"
	log.Error("正在操作的数据库:", dbname)
	var count, countOK uint64 = 0, 0
	eNbLocationRecord, _ := this.GetMultiRecord(dbname, COLLECTION_ENB_LOCATION_INFO, queryMap, bson.M{"_id": 0})
	length := len(eNbLocationRecord)
	if 0 == length {
		return isAccess
	}
	for _, oneLocationRecord := range eNbLocationRecord {
		latitude := oneLocationRecord["latitude"].(float64)
		longitude := oneLocationRecord["longitude"].(float64)
		tmp1 := strconv.FormatFloat(latitude, 'f', 6, 64)
		tmp2 := strconv.FormatFloat(longitude, 'f', 6, 64)
		location := strings.Join([]string{tmp1, tmp2}, ",")
		url := strings.Join([]string{mapapi, location}, "=")
		strmap := this.fetchGeocoding(&url, &Proxy_addr)
		for retry := 1; 0 != strmap.Status && retry < Retry; retry++ {
			time.Sleep(time.Minute)
			strmap = this.fetchGeocoding(&url, &Proxy_addr)
		}
		if 0 != strmap.Status {
			log.Error("重试4次转换失败！")
			isAccess = false
			return isAccess
		}
		var updateActionMap bson.M
		match_province, _ := regexp.MatchString(DbNameToProvinceNameMap[dbname], strmap.Result.AddressComponent.Province)
		if !match_province {
			log.Error("逆解析省份与当前省份不符：", strmap.Result.AddressComponent.Province)
			count++
			continue
		} else {
			countOK++
			//要增加对海东地区的处理
			realCity := strmap.Result.AddressComponent.City
			if realName, ok := baiduMapCityMap[realCity]; ok {
				realCity = realName
			}
			updateActionMap = bson.M{"$set": bson.M{"地市": realCity}}
		}
		//log.Error(oneLocationRecord, updateActionMap)
		this.UpdateOne(dbname, COLLECTION_ENB_LOCATION_INFO, oneLocationRecord, updateActionMap)
	}
	log.Error("数据库，逆解析出省份和记录不符个数为, 逆解析成功个数为", dbname, count, countOK)
	return
}

func (this *Crontab) fetchGeocoding(url, proxy_addr *string) (strmap Strmap) {
	transport := GetTransportFieldURL(proxy_addr)
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", *url, nil)
	if err != nil {
		strmap.Status = 4
		log.Fatal(err.Error())
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		strmap.Status = 4
		log.Fatal(err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			strmap.Status = 4
			log.Fatal(err.Error())
			return
		}
		err = json.Unmarshal(body, &strmap)
		if err != nil {
			strmap.Status = 4
			log.Error(err)
			return
		}
		return strmap
	} else {
		strmap.Status = 4
		log.Fatal("get from http://api.map.baidu.com/geocoder/v2/ error")
	}
	return strmap
}

//对于经纬度无法确定地市的字段，通过提取网元中文作为key去查找站点所在地市
func (this *Crontab) DistinctCityLocations(dbName string) {
	selectMap := bson.M{"_id": 0, "地市": 1, "子网": 1, "网元名称": 1, "管理网元ID": 1, "经度": 1, "纬度": 1}
	result, _ := this.GetMultiRecord(dbName, COLLECTION_ENB_LOCATION_INFO, bson.M{"地市": bson.M{"$exists": false}}, selectMap)
	log.Error("正在操作数据库：", dbName, " 共", len(result), "个无地市站点")
	cout := 0
	if 0 == len(result) {
		log.Error(dbName, " 无不确定地市站点")
		return
	}
	for _, locations := range result {
		netName, ok := locations["网元名称"].(string)
		if !ok || netName == "" {
			continue
		}
		r := []rune(netName)
		strSlice := []string{}
		nwNameSlice := []string{}
		cnstr := ""
		str := ""
		for i := 0; i < len(r); i++ {
			str = str + string(r[i])
			nwNameSlice = append(nwNameSlice, str)
			if r[i] <= 40869 && r[i] >= 19968 {
				cnstr = cnstr + string(r[i])
				strSlice = append(strSlice, cnstr)
			}
		}

		realCityName := ""
		for i := len(nwNameSlice) - 1; i > 0; i-- {
			var networkNameslice []bson.RegEx
			networkNameslice = append(networkNameslice, bson.RegEx{nwNameSlice[i], ""})
			queryMap := bson.M{"地市": bson.M{"$exists": true}, "网元名称": bson.M{"$in": networkNameslice}}
			cityName, _ := this.Distinct(dbName, COLLECTION_ENB_LOCATION_INFO, "地市", queryMap)
			if 1 == len(cityName) {
				cout++
				realCityName = cityName[0].(string)
				break
			}
		}
		if "" != realCityName {
			log.Error(locations, bson.M{"$set": bson.M{"地市": realCityName}})
			this.UpdateOne(dbName, COLLECTION_ENB_LOCATION_INFO, locations, bson.M{"$set": bson.M{"地市": realCityName}})
			continue
		}
		for i := len(strSlice) - 1; i > 0; i-- {
			var networkNameslice []bson.RegEx
			networkNameslice = append(networkNameslice, bson.RegEx{strSlice[i], ""})
			queryMap := bson.M{"地市": bson.M{"$exists": true}, "网元名称": bson.M{"$in": networkNameslice}}
			cityName, _ := this.Distinct(dbName, COLLECTION_ENB_LOCATION_INFO, "地市", queryMap)
			if 1 == len(cityName) {
				cout++
				realCityName = cityName[0].(string)
				break
			}
		}
		if "" != realCityName {
			log.Error(locations, bson.M{"$set": bson.M{"地市": realCityName}})
			this.UpdateOne(dbName, COLLECTION_ENB_LOCATION_INFO, locations, bson.M{"$set": bson.M{"地市": realCityName}})
		}
	}
	log.Error(dbName, "  通过匹配确定了地市站点数为: ", cout)
}
