package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxResponseBytes = 1 << 20

type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	userAgent  string
}

type Forecast struct {
	Location            string           `json:"location" jsonschema:"Nearby city and state for the forecast coordinates"`
	LocationAttribution string           `json:"locationAttribution,omitempty" jsonschema:"Attribution for Open-Meteo location data when a place name was used"`
	Periods             []ForecastPeriod `json:"periods" jsonschema:"Forecast periods returned by the National Weather Service"`
}

type ForecastPeriod struct {
	Name            string `json:"name" jsonschema:"Name of the forecast period"`
	StartTime       string `json:"startTime" jsonschema:"Start time in ISO 8601 format"`
	EndTime         string `json:"endTime" jsonschema:"End time in ISO 8601 format"`
	Temperature     int    `json:"temperature" jsonschema:"Forecast temperature"`
	TemperatureUnit string `json:"temperatureUnit" jsonschema:"Temperature unit, usually F or C"`
	ShortForecast   string `json:"shortForecast" jsonschema:"Brief forecast summary"`
	WindSpeed       string `json:"windSpeed" jsonschema:"Forecast wind speed"`
	WindDirection   string `json:"windDirection" jsonschema:"Prevailing wind direction"`
}

type pointResponse struct {
	Properties struct {
		Forecast         string `json:"forecast"`
		RelativeLocation struct {
			Properties struct {
				City  string `json:"city"`
				State string `json:"state"`
			} `json:"properties"`
		} `json:"relativeLocation"`
	} `json:"properties"`
}

type forecastResponse struct {
	Properties struct {
		Periods []ForecastPeriod `json:"periods"`
	} `json:"properties"`
}

func NewClient(httpClient *http.Client, baseURL, userAgent string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://api.weather.gov"
	}
	parsedBaseURL, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		parsedBaseURL, _ = url.Parse("https://api.weather.gov")
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "rw-mcp/0.1 (https://github.com/caraya/rw-mcp)"
	}
	return &Client{httpClient: httpClient, baseURL: parsedBaseURL, userAgent: userAgent}
}

func (c *Client) GetForecast(ctx context.Context, latitude, longitude float64) (Forecast, error) {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
		return Forecast{}, errors.New("latitude must be between -90 and 90")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return Forecast{}, errors.New("longitude must be between -180 and 180")
	}

	coordinates := strconv.FormatFloat(latitude, 'f', -1, 64) + "," + strconv.FormatFloat(longitude, 'f', -1, 64)
	pointURL := c.baseURL.ResolveReference(&url.URL{Path: "/points/" + coordinates})
	var point pointResponse
	if err := c.getJSON(ctx, pointURL, &point); err != nil {
		return Forecast{}, fmt.Errorf("look up NWS forecast point: %w", err)
	}

	forecastURL, err := url.Parse(point.Properties.Forecast)
	if err != nil || forecastURL.Scheme != c.baseURL.Scheme || forecastURL.Host != c.baseURL.Host ||
		!strings.HasPrefix(forecastURL.Path, "/gridpoints/") || !strings.HasSuffix(forecastURL.Path, "/forecast") {
		return Forecast{}, errors.New("NWS point response contained an invalid forecast URL")
	}
	var response forecastResponse
	if err := c.getJSON(ctx, forecastURL, &response); err != nil {
		return Forecast{}, fmt.Errorf("retrieve NWS forecast: %w", err)
	}

	location := strings.Trim(strings.TrimSpace(point.Properties.RelativeLocation.Properties.City)+", "+
		strings.TrimSpace(point.Properties.RelativeLocation.Properties.State), ", ")
	return Forecast{Location: location, Periods: response.Properties.Periods}, nil
}

func (c *Client) getJSON(ctx context.Context, target *url.URL, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create NWS API request: %w", err)
	}
	request.Header.Set("Accept", "application/geo+json")
	request.Header.Set("User-Agent", c.userAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call NWS API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("NWS API returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read NWS API response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("NWS API response exceeds %d bytes", maxResponseBytes)
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decode NWS API response: %w", err)
	}
	return nil
}
