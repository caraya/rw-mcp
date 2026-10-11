package weather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetForecastResolvesPointAndUsesRequiredHeaders(t *testing.T) {
	var requestPaths []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		if got := r.Header.Get("User-Agent"); got != "rw-mcp-test/1.0 (test@example.invalid)" {
			t.Errorf("User-Agent = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/geo+json" {
			t.Errorf("Accept = %q", got)
		}
		w.Header().Set("Content-Type", "application/geo+json")
		switch r.URL.Path {
		case "/points/39.7456,-97.0892":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"properties": map[string]any{
					"forecast": server.URL + "/gridpoints/TOP/31,80/forecast",
					"relativeLocation": map[string]any{"properties": map[string]any{
						"city": "North Topeka",
						"state": "KS",
					}},
				},
			})
		case "/gridpoints/TOP/31,80/forecast":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"properties": map[string]any{"periods": []any{
					map[string]any{
						"name": "This Afternoon",
						"temperature": 74,
						"temperatureUnit": "F",
						"shortForecast": "Mostly Sunny",
						"windSpeed": "5 mph",
						"windDirection": "S",
					},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client(), server.URL, "rw-mcp-test/1.0 (test@example.invalid)")
	forecast, err := client.GetForecast(context.Background(), 39.7456, -97.0892)
	if err != nil {
		t.Fatalf("GetForecast() error = %v", err)
	}
	if forecast.Location != "North Topeka, KS" || len(forecast.Periods) != 1 {
		t.Fatalf("GetForecast() = %+v", forecast)
	}
	period := forecast.Periods[0]
	if period.Name != "This Afternoon" || period.Temperature != 74 || period.ShortForecast != "Mostly Sunny" {
		t.Fatalf("forecast period = %+v", period)
	}
	if len(requestPaths) != 2 || requestPaths[0] != "/points/39.7456,-97.0892" || requestPaths[1] != "/gridpoints/TOP/31,80/forecast" {
		t.Fatalf("request paths = %v", requestPaths)
	}
}