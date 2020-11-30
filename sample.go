package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

const (
	cityURL                = "https://public.opendatasoft.com/api/records/1.0/search/?rows=100&disjunctive.country=true&refine.country=United+States&sort=population&start=0&fields=ascii_name,population,latitude,longitude&dataset=geonames-all-cities-with-a-population-1000&timezone=UTC&lang=en"
	weatherCityURLTemplate = "https://www.metaweather.com/api/location/search/?lattlong=%f,%f"
	weatherURLTemplate     = "https://www.metaweather.com/api/location/%d/%d/%d/%d"
)

var (

	// ErrNot100Cities indicates the HTTP request to get the most populous 100 cities in the US did not include at least
	// 100 cities.
	ErrNot100Cities = errors.New("100 cities were not returned by the HTTP response")

	// ErrNoWoe indicates the HTTP request to get a city's WOE ID did not include a WOE ID.
	ErrNoWoe = errors.New("a Where On Earth ID was not returned by the HTTP response")
)

type coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func main() {

	// Create a logger.
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Create an HTTP client that will be reused for requests.
	//
	// By using an http.Client, we can more easily switch out the code to use proxies later, if desired.
	httpClient := &http.Client{}

	// Get the 100 largest US cities.
	var err error
	var coords [100]coordinates
	if coords, err = largest100USCities(httpClient, cityURL); err != nil {
		logger.Fatalf("Failed to get the 100 largest US cities.\nError: %s", err.Error())
	}

	//fmt.Println(getCurrentTemperatureForCoordinates(cityCoordinates[0]))
}

func coordinateTemperature(coords coordinates, httpClient *http.Client, woeIDURL string) (temperature float64, err error) {

	// Perform the request to get the Where On Earth (woe) ID.
	var resp *http.Response
	if resp, err = httpClient.Get(woeIDURL); err != nil {
		return 0, err
	}
	defer resp.Body.Close() // Ignore this error, if any.

	// Read the body of the response.
	var respJSON []byte
	if respJSON, err = ioutil.ReadAll(resp.Body); err != nil {
		return 0, err
	}

	// Get the WOE ID of the closest city.
	woeID := gjson.GetBytes(respJSON, "0.woeid").String()
	if woeID == "" {
		return 0, ErrNoWoe
	}

}

func largest100USCities(httpClient *http.Client, urlWithParams string) (coords [100]coordinates, err error) {

	// Perform the HTTP request given the HTTP client.
	var resp *http.Response
	if resp, err = httpClient.Get(urlWithParams); err != nil {
		return [100]coordinates{}, err
	}
	defer resp.Body.Close() // Ignore this error, if any.

	// Read the body of the response.
	var respJSON []byte
	if respJSON, err = ioutil.ReadAll(resp.Body); err != nil {
		return [100]coordinates{}, err
	}

	// Create a gjson.Result that will let us iterate through the coords returned in the response.
	records := gjson.GetBytes(respJSON, "records.#.fields")

	// Declare these variables in the outer scope so that the index can be referenced once the loop is completed.
	var index int
	var cityJSON gjson.Result

	// Iterate through the coords in the response.
	for index, cityJSON = range records.Array() {

		// Create the current coordinates.
		currentCords := &coordinates{}

		// Turn the JSON into a Go structure.
		if err = json.Unmarshal([]byte(cityJSON.Raw), currentCords); err != nil {
			return [100]coordinates{}, err
		}

		// Put the coords into the array of coords.
		coords[index] = *currentCords
	}

	// Confirm the index is at 99 to ensure 100 city coordinates were gathered.
	if index != 99 {
		return coords, ErrNot100Cities
	}

	return coords, nil
}

func getCurrentTemperatureForCoordinates(coord Coordinate) float64 {
	weatherCityData, err := doGetRequest(fmt.Sprintf(weatherCityURLTemplate, coord.Latitude, coord.Longitude))
	if err != nil {
		panic(err)
	}

	weatherCitiesParsed, _ := gabs.ParseJSON(weatherCityData)
	weatherCityWoeids := weatherCitiesParsed.Path("woeid").Data().([]interface{})
	weatherURLFormatted := fmt.Sprintf(weatherURLTemplate, int64(weatherCityWoeids[0].(float64)), time.Now().Year(),
		int(time.Now().Month()), time.Now().Day())
	weatherData, err := doGetRequest(weatherURLFormatted)
	if err != nil {
		panic(err)
	}
	weatherDataParsed, _ := gabs.ParseJSON(weatherData)
	return weatherDataParsed.Path("the_temp").Data().([]interface{})[0].(float64)
}
