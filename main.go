package main

import (
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

	// cityURL is the URL with the query to the most populous US cities.
	//
	// I modified this URL a bit to add sorting and only the fields I cared about. Let the server do all the work!
	cityURL = "https://public.opendatasoft.com/api/records/1.0/search/?rows=100&disjunctive.country=true&refine.country=United+States&sort=population&start=0&fields=coordinates&dataset=geonames-all-cities-with-a-population-1000&timezone=UTC&lang=en"

	// The URL template that will turn a pair of coordinates into a Where On Earth (WOE) ID.
	woeIDURLTemplate = "https://www.metaweather.com/api/location/search/?lattlong=%f,%f"

	// The URL template that will turn a WOE ID and date into a temperature reading.
	temperatureURLTemplate = "https://www.metaweather.com/api/location/%d/%d/%d/%d"
)

var (

	// ErrNot100Cities indicates the HTTP request to get the most populous 100 cities in the US did not include at least
	// 100 cities.
	ErrNot100Cities = errors.New("100 cities were not returned by the HTTP response")

	// ErrNoTemperature indicates the HTTP request to get a city's temperature did not include a temperature.
	ErrNoTemperature = errors.New("a temperature reading was not returned by the HTTP response")

	// ErrNoWoe indicates the HTTP request to get a city's WOE ID did not include a WOE ID.
	ErrNoWoe = errors.New("a Where On Earth ID was not returned by the HTTP response")
)

type coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func main() {

	// Create a logger.
	//
	// It will behave as an async safe printer, but could be updated for later.
	logger := log.New(os.Stdout, "", 0)

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

	// Create a temperature sum and count of how many temperatures are from that sum.
	tempCount := float64(0)
	tempSum := float64(0)

	// Iterate through the coordinates and get their temperatures.
	//
	// It'd be pretty easy to make this faster by working asynchronously. I'd use a *sync.Mutex to lock the float64's
	// above and put what's in this loop into some different goroutines. Ask me to do it if you want!
	for _, coord := range coords {

		// Get the Where On Earth ID of the city.
		var woeID int64
		if woeID, err = coordinateWOEID(coord, httpClient, woeIDURLTemplate); err != nil {
			logger.Fatalf("Failed to get a WOE ID.\nError: %s", err.Error())
		}

		// Get the temperature of the city.
		var temperature float64
		if temperature, err = woeIDTemperature(httpClient, temperatureURLTemplate, woeID); err != nil {
			if errors.Is(err, ErrNoTemperature) {
				logger.Printf("Failed to get temperature for WOE ID: %d. Continuing anyways.", woeID)
			} else {
				logger.Fatalf("Failed to get temperature from WOE ID.\nError: %s", err.Error())
			}
		}

		// Add to the total temperature sum.
		tempCount += 1
		tempSum += temperature
	}

	// Divide by the number of temperature summed and print the desired number.
	logger.Printf("The average temperature in the %d most populous US cities is: %.2fF", int(tempCount), tempSum/tempCount)
}

// coordinateWOEID turns a pair of coordinates into a WOE ID.
func coordinateWOEID(coords coordinates, httpClient *http.Client, urlTemplate string) (woeID int64, err error) {

	// Perform the request to get the Where On Earth (woe) ID.
	var resp *http.Response
	if resp, err = httpClient.Get(fmt.Sprintf(urlTemplate, coords.Latitude, coords.Longitude)); err != nil {
		return 0, err
	}
	defer resp.Body.Close() // Ignore this error, if any.

	// Read the body of the response.
	var respJSON []byte
	if respJSON, err = ioutil.ReadAll(resp.Body); err != nil {
		return 0, err
	}

	// Get the WOE ID of the closest city (first index).
	woeID = gjson.GetBytes(respJSON, "0.woeid").Int()
	if woeID == 0 {
		return 0, ErrNoWoe
	}

	return woeID, nil
}

// largest100USCities gets the coordinates of the most populous 100 US cities.
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
	records := gjson.GetBytes(respJSON, "records.#.fields.coordinates")

	// Declare these variables in the outer scope so that the index can be referenced once the loop is completed.
	var index int
	var cityJSON gjson.Result

	// Iterate through the coords in the response.
	for index, cityJSON = range records.Array() {

		// Create the current coordinates.
		currentCords := &coordinates{
			Latitude:  cityJSON.Get("0").Float(),
			Longitude: cityJSON.Get("1").Float(),
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

// woeIDTemperature turns a WOE ID into a temperature.
func woeIDTemperature(httpClient *http.Client, urlTemplate string, woeID int64) (temperature float64, err error) {

	// Get the current date from the OS.
	year, month, day := time.Now().Date()

	// Perform the request to get temperature readings.
	var resp *http.Response
	if resp, err = httpClient.Get(fmt.Sprintf(urlTemplate, woeID, year, month, day)); err != nil {
		return 0, err
	}
	defer resp.Body.Close() // Ignore this error, if any.

	// Read the body of the response.
	var respJSON []byte
	if respJSON, err = ioutil.ReadAll(resp.Body); err != nil {
		return 0, err
	}

	// Get the most recent temperature reading (first index).
	temperature = gjson.GetBytes(respJSON, "0.the_temp").Float()
	if temperature == 0 {
		return 0, ErrNoTemperature
	}

	return temperature, nil
}
