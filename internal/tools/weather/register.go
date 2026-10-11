package weather

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type forecastInput struct {
	Location  *string  `json:"location,omitempty" jsonschema:"U.S. city or postal code, optionally followed by the state; place names are sent to Open-Meteo for geocoding"`
	Latitude  *float64 `json:"latitude,omitempty" jsonschema:"Latitude in decimal degrees for a U.S. location, between -90 and 90"`
	Longitude *float64 `json:"longitude,omitempty" jsonschema:"Longitude in decimal degrees for a U.S. location, between -180 and 180"`
}

func RegisterTool(server *mcp.Server, client *Client, geocoder *Geocoder) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "nws_get_forecast",
		Title:       "Get NWS forecast",
		Description: "Get a U.S. National Weather Service forecast using latitude and longitude or a U.S. city/postal code. Place names are sent to Open-Meteo; include the state when a name is ambiguous and do not send confidential locations. The default free geocoding API is for non-commercial use only; configure a paid Open-Meteo endpoint and key for commercial deployments. Geocoded results include Open-Meteo/GeoNames attribution.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input forecastInput) (*mcp.CallToolResult, Forecast, error) {
		if input.Location != nil {
			location := strings.TrimSpace(*input.Location)
			if location == "" {
				return nil, Forecast{}, fmt.Errorf("location must not be empty")
			}
			if input.Latitude != nil || input.Longitude != nil {
				return nil, Forecast{}, fmt.Errorf("provide either location or latitude and longitude, not both")
			}
			place, err := geocoder.Resolve(ctx, location)
			if err != nil {
				return nil, Forecast{}, err
			}
			forecast, err := client.GetForecast(ctx, place.Latitude, place.Longitude)
			if err != nil {
				return nil, Forecast{}, err
			}
			forecast.LocationAttribution = "Location data: Open-Meteo, based on GeoNames (CC BY 4.0)."
			return nil, forecast, nil
		}
		if input.Latitude == nil || input.Longitude == nil {
			return nil, Forecast{}, fmt.Errorf("provide location or both latitude and longitude")
		}
		forecast, err := client.GetForecast(ctx, *input.Latitude, *input.Longitude)
		return nil, forecast, err
	})
}
