package weather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterToolAcceptsPlaceAndCoordinates(t *testing.T) {
	geocodingCalls := 0
	geocodingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		geocodingCalls++
		_ = json.NewEncoder(w).Encode(geocodingResponse{Results: []Place{{
			Name: "Topeka", Admin1: "Kansas", CountryCode: "US", Latitude: 39.049, Longitude: -95.677,
		}}})
	}))
	defer geocodingServer.Close()

	var forecastPaths []string
	nwsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forecastPaths = append(forecastPaths, r.URL.Path)
		switch r.URL.Path {
		case "/points/39.049,-95.677", "/points/39.1,-95.6":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"properties": map[string]any{
					"forecast":         nwsServerURL(r) + "/gridpoints/TOP/31,80/forecast",
					"relativeLocation": map[string]any{"properties": map[string]any{"city": "Topeka", "state": "KS"}},
				},
			})
		case "/gridpoints/TOP/31,80/forecast":
			_ = json.NewEncoder(w).Encode(map[string]any{"properties": map[string]any{"periods": []any{}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer nwsServer.Close()

	client := NewClient(nwsServer.Client(), nwsServer.URL, "rw-mcp-test/1.0 (test@example.invalid)")
	geocoder, err := NewGeocoder(geocodingServer.Client(), geocodingServer.URL+"/v1/search", "")
	if err != nil {
		t.Fatalf("NewGeocoder() error = %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "weather-test", Version: "1.0.0"}, nil)
	RegisterTool(server, client, geocoder)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()
	caller := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := caller.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	placeResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "nws_get_forecast", Arguments: map[string]any{"location": "Topeka, Kansas"},
	})
	if err != nil || placeResult.IsError {
		t.Fatalf("CallTool(location) = (%v, %v)", placeResult, err)
	}
	var placeForecast Forecast
	placeJSON, err := json.Marshal(placeResult.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(StructuredContent) error = %v", err)
	}
	if err := json.Unmarshal(placeJSON, &placeForecast); err != nil {
		t.Fatalf("Unmarshal(StructuredContent) error = %v", err)
	}
	if placeForecast.LocationAttribution == "" {
		t.Fatal("location forecast is missing Open-Meteo attribution")
	}

	coordinateResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "nws_get_forecast", Arguments: map[string]any{"latitude": 39.1, "longitude": -95.6},
	})
	if err != nil || coordinateResult.IsError {
		t.Fatalf("CallTool(coordinates) = (%v, %v)", coordinateResult, err)
	}
	var coordinateForecast Forecast
	coordinateJSON, err := json.Marshal(coordinateResult.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal(StructuredContent) error = %v", err)
	}
	if err := json.Unmarshal(coordinateJSON, &coordinateForecast); err != nil {
		t.Fatalf("Unmarshal(StructuredContent) error = %v", err)
	}
	if coordinateForecast.LocationAttribution != "" {
		t.Fatalf("coordinate forecast unexpectedly has geocoding attribution %q", coordinateForecast.LocationAttribution)
	}
	if geocodingCalls != 1 {
		t.Fatalf("geocoding calls = %d, want 1", geocodingCalls)
	}
	if len(forecastPaths) != 4 {
		t.Fatalf("NWS request paths = %v, want two points lookups and two forecasts", forecastPaths)
	}
}

func nwsServerURL(r *http.Request) string {
	return "http://" + r.Host
}
