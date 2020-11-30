package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Jeffail/gabs"
)

type Coordinate struct {
	Latitude  float64
	Longitude float64
}

const weatherCityUrlTemplate string = "https://www.metaweather.com/api//location/search/?lattlong=%f,%f"
const weatherUrlTemplate string = "https://www.metaweather.com/api/location/%d/%d/%d/%d"
const cityUrls string = "https://public.opendatasoft.com/api/records/1.0/search/?dataset=geonames-all-cities-with-a-population-1000&q=&refine.country=United+States&rows=100"

func main() {

	cityData, err := doGetRequest(cityUrls)
	if err != nil {
		panic(err)
	}

	cityDataParsed, _ := gabs.ParseJSON(cityData)
	cities, _ := cityDataParsed.Path("records").Children()
	cityCoordinates := [100]Coordinate{}
	for i, city := range cities {
		coord := city.Path("fields.coordinates").Data().([]interface{})
		cityCoordinates = Coordinate{
			Latitude:  coord[0].(float64),
			Longitude: coord[1].(float64),
		}
	}

	fmt.Println(getCurrentTemperatureForCoordinates(cityCoordinates[0]))

}

func getCurrentTemperatureForCoordinates(coord Coordinate) float64 {
	weatherCityData, err := doGetRequest(fmt.Sprintf(weatherCityUrlTemplate, coord.Latitude, coord.Longitude))
	if err != nil {
		panic(err)
	}

	weatherCitiesParsed, _ := gabs.ParseJSON(weatherCityData)
	weatherCityWoeids := weatherCitiesParsed.Path("woeid").Data().([]interface{})
	weatherURLFormatted := fmt.Sprintf(weatherUrlTemplate, int64(weatherCityWoeids[0].(float64)), time.Now().Year(),
		int(time.Now().Month()), time.Now().Day())
	weatherData, err := doGetRequest(weatherURLFormatted)
	if err != nil {
		panic(err)
	}
	weatherDataParsed, _ := gabs.ParseJSON(weatherData)
	return weatherDataParsed.Path("the_temp").Data().([]interface{})[0].(float64)
}

func doGetRequest(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
