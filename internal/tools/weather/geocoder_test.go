package weather

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeocoderResolvesUSPlace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("name"); got != "Topeka, Kansas" {
			t.Errorf("name query = %q", got)
		}
		if got := r.URL.Query().Get("countryCode"); got != "US" {
			t.Errorf("countryCode query = %q", got)
		}
		if got := r.URL.Query().Get("count"); got != "10" {
			t.Errorf("count query = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"name":         "Topeka",
					"latitude":     39.049,
					"longitude":    -95.677,
					"country_code": "US",
					"country":      "United States",
					"admin1":       "Kansas",
					"feature_code": "PPLA",
				},
			},
		})
	}))
	defer server.Close()

	geocoder, err := NewGeocoder(server.Client(), server.URL+"/v1/search", "")
	if err != nil {
		t.Fatalf("NewGeocoder() error = %v", err)
	}
	place, err := geocoder.Resolve(context.Background(), "Topeka, Kansas")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if place.Name != "Topeka" || place.Admin1 != "Kansas" || place.Latitude != 39.049 || place.Longitude != -95.677 {
		t.Fatalf("Resolve() = %+v", place)
	}
}

func TestGeocoderRequiresStateForAmbiguousPlace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{"name": "Springfield", "latitude": 39.8, "longitude": -89.6, "country_code": "US", "admin1": "Illinois"},
				map[string]any{"name": "Springfield", "latitude": 42.1, "longitude": -72.6, "country_code": "US", "admin1": "Massachusetts"},
			},
		})
	}))
	defer server.Close()

	geocoder, err := NewGeocoder(server.Client(), server.URL, "")
	if err != nil {
		t.Fatalf("NewGeocoder() error = %v", err)
	}
	_, err = geocoder.Resolve(context.Background(), "Springfield")
	if err == nil {
		t.Fatal("Resolve() error = nil, want ambiguity error")
	}
	if !strings.Contains(err.Error(), "Illinois") || !strings.Contains(err.Error(), "Massachusetts") {
		t.Fatalf("Resolve() error = %v, want candidate states", err)
	}
}

func TestNewGeocoderRequiresExplicitEndpointForAPIKey(t *testing.T) {
	_, err := NewGeocoder(nil, "", "paid-api-key")
	if err == nil {
		t.Fatal("NewGeocoder() error = nil, want explicit endpoint error")
	}
	if strings.Contains(err.Error(), "paid-api-key") {
		t.Fatalf("NewGeocoder() error exposed the API key: %v", err)
	}
}

func TestGeocoderUsesPaidKeyWithConfiguredEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("apikey"); got != "paid-api-key" {
			t.Errorf("apikey query = %q", got)
		}
		_ = json.NewEncoder(w).Encode(geocodingResponse{Results: []Place{{
			Name: "Topeka", Admin1: "Kansas", CountryCode: "US", Latitude: 39.049, Longitude: -95.677,
		}}})
	}))
	defer server.Close()

	geocoder, err := NewGeocoder(server.Client(), server.URL, "paid-api-key")
	if err != nil {
		t.Fatalf("NewGeocoder() error = %v", err)
	}
	if _, err := geocoder.Resolve(context.Background(), "Topeka, Kansas"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestGeocoderDoesNotExposeAPIKeyInRequestError(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network failure")
	})}
	geocoder, err := NewGeocoder(client, "https://example.invalid/v1/search", "paid-api-key")
	if err != nil {
		t.Fatalf("NewGeocoder() error = %v", err)
	}
	_, err = geocoder.Resolve(context.Background(), "Topeka, Kansas")
	if err == nil {
		t.Fatal("Resolve() error = nil, want request error")
	}
	if strings.Contains(err.Error(), "paid-api-key") {
		t.Fatalf("Resolve() error exposed the API key: %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
