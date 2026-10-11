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
	"strings"
)

const (
	defaultGeocodingURL = "https://geocoding-api.open-meteo.com/v1/search"
	maxGeocodingResults = 10
	maxGeocodingQuery   = 200
)

type Geocoder struct {
	httpClient *http.Client
	endpoint   *url.URL
	apiKey     string
}

type Place struct {
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CountryCode string  `json:"country_code"`
	Country     string  `json:"country"`
	Admin1      string  `json:"admin1"`
	FeatureCode string  `json:"feature_code"`
}

type geocodingResponse struct {
	Results []Place `json:"results"`
}

func NewGeocoder(httpClient *http.Client, endpoint, apiKey string) (*Geocoder, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	apiKey = strings.TrimSpace(apiKey)
	if endpoint == "" && apiKey != "" {
		return nil, errors.New("OPEN_METEO_GEOCODING_URL must be set when OPEN_METEO_API_KEY is configured")
	}
	if endpoint == "" {
		endpoint = defaultGeocodingURL
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Host == "" ||
		(parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") ||
		parsedEndpoint.User != nil || parsedEndpoint.Fragment != "" {
		return nil, errors.New("Open-Meteo geocoding URL must be an absolute HTTP(S) URL without user info or a fragment")
	}
	return &Geocoder{httpClient: httpClient, endpoint: parsedEndpoint, apiKey: apiKey}, nil
}

func (g *Geocoder) Resolve(ctx context.Context, query string) (Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Place{}, errors.New("location must not be empty")
	}
	if len(query) > maxGeocodingQuery {
		return Place{}, fmt.Errorf("location must be no longer than %d bytes", maxGeocodingQuery)
	}

	target := *g.endpoint
	queryValues := target.Query()
	queryValues.Set("name", query)
	queryValues.Set("count", fmt.Sprint(maxGeocodingResults))
	queryValues.Set("language", "en")
	queryValues.Set("format", "json")
	queryValues.Set("countryCode", "US")
	if g.apiKey != "" {
		queryValues.Set("apikey", g.apiKey)
	}
	target.RawQuery = queryValues.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Place{}, fmt.Errorf("create Open-Meteo geocoding request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "rw-mcp/0.1 (https://github.com/caraya/rw-mcp)")

	response, err := g.httpClient.Do(request)
	if err != nil {
		return Place{}, errors.New("call Open-Meteo geocoding API failed; check network access and configured credentials")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Place{}, fmt.Errorf("Open-Meteo geocoding API returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Place{}, fmt.Errorf("read Open-Meteo geocoding response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return Place{}, fmt.Errorf("Open-Meteo geocoding response exceeds %d bytes", maxResponseBytes)
	}
	var result geocodingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Place{}, fmt.Errorf("decode Open-Meteo geocoding response: %w", err)
	}

	places := make([]Place, 0, len(result.Results))
	for _, place := range result.Results {
		if !strings.EqualFold(place.CountryCode, "US") ||
			math.IsNaN(place.Latitude) || math.IsInf(place.Latitude, 0) || place.Latitude < -90 || place.Latitude > 90 ||
			math.IsNaN(place.Longitude) || math.IsInf(place.Longitude, 0) || place.Longitude < -180 || place.Longitude > 180 {
			continue
		}
		places = append(places, place)
	}
	if len(places) == 0 {
		return Place{}, fmt.Errorf("no U.S. locations found for %q", query)
	}

	name, _, _ := strings.Cut(query, ",")
	name = strings.TrimSpace(name)
	exactMatches := make([]Place, 0, len(places))
	for _, place := range places {
		if strings.EqualFold(strings.TrimSpace(place.Name), name) {
			exactMatches = append(exactMatches, place)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0], nil
	}
	if len(exactMatches) == 0 && len(places) == 1 {
		return places[0], nil
	}

	candidates := make([]string, 0, len(places))
	seen := make(map[string]struct{}, len(places))
	for _, place := range places {
		label := place.Name
		if place.Admin1 != "" {
			label += ", " + place.Admin1
		}
		if _, exists := seen[label]; !exists {
			candidates = append(candidates, label)
			seen[label] = struct{}{}
		}
	}
	return Place{}, fmt.Errorf("location %q is ambiguous; include a state, for example: %s", query, strings.Join(candidates, "; "))
}
