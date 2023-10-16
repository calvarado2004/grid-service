package data

import (
	"database/sql"
	"github.com/calvarado2004/grid-service/models"
)

type Repository interface {
	InsertGeoCell(geoCell models.GeoCell) error
	UpdateWeather(weather models.WeatherAPIResponse) error
	GetOneNearbyGeoCell(lat float64, long float64, radius float64) (models.GeoCell, error)
	ObsoleteGeoCells() error
	GetAllGeoCells() ([]models.GeoCell, error)
	Connect() (*sql.DB, error)
}
