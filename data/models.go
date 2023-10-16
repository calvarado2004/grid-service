package data

import (
	"database/sql"
	"errors"
	"github.com/calvarado2004/grid-service/models"
	"log"
	"time"
)

const dbTimeout = time.Second * 5

var db *sql.DB

type PostgresRepository struct {
	Conn *sql.DB
}

func NewPostgresRepository(pool *sql.DB) *PostgresRepository {
	db = pool
	return &PostgresRepository{
		Conn: pool,
	}
}

func (repo *PostgresRepository) InsertGeoCell(geoCell models.GeoCell) error {

	insertQuery := `INSERT INTO geocells (coordinates, crime_name, crime_score, crime_firearm) 
                        VALUES (ST_MakePoint($1, $2), $3, $4, $5)`

	// Latitude last, PostGIS stores coordinates as (longitude, latitude)
	_, err := db.Exec(insertQuery, geoCell.Longitude, geoCell.Latitude, geoCell.CrimeName, geoCell.CrimeScore, geoCell.CrimeFirearm)
	if err != nil {
		log.Fatalf("Error inserting new GeoCell: %v", err)
		return err
	}
	log.Printf("Inserted new GeoCell: %v", geoCell)

	return nil
}

// UpdateWeather updates all GeoCells with the new weather data
func (repo *PostgresRepository) UpdateWeather(response models.WeatherAPIResponse) error {

	// Create the SQL query for updating the weather information in the geocells table.
	query := `
        UPDATE geocells 
        SET weather_condition = $1, weather_description = $2
        WHERE created_at >= NOW() - INTERVAL '10 days'
    `

	if response.CellWeight < 1 {
		log.Printf("Error updating weather info for GeoCells: %v", errors.New("cell weight is less than 1"))
		return errors.New("cell weight is less than 1")
	}

	// Execute the update
	_, err := db.Exec(query, response.CellWeight, response.WeatherResponse.Current.Condition.Text)
	if err != nil {
		log.Printf("Error updating weather info for GeoCells: %v", err)
		return err
	}

	log.Println("Updated weather info for all GeoCells")

	return nil
}

// GetOneNearbyGeoCell returns a single GeoCell within the specified radius of the given coordinates. PostGIS stores coordinates as (longitude, latitude)
func (repo *PostgresRepository) GetOneNearbyGeoCell(latitude float64, longitude float64, radius float64) (models.GeoCell, error) {
	var cell models.GeoCell
	query := `
        SELECT id, crime_name, crime_score, crime_firearm, weather_description, weather_condition, ST_X(coordinates), ST_Y(coordinates) 
        FROM geocells 
        WHERE ST_DWithin(coordinates, ST_MakePoint($1, $2)::geography, $3)
        ORDER BY coordinates <-> ST_MakePoint($1, $2)::geography
        LIMIT 1;
    `

	// Execute the query and scan the results into the GeoCell struct. If no rows are returned, return an empty GeoCell.
	err := db.QueryRow(query, longitude, latitude, radius).Scan(&cell.Id, &cell.CrimeName, &cell.CrimeScore, &cell.CrimeFirearm, &cell.WeatherDescription, &cell.WeatherCondition, &cell.Longitude, &cell.Latitude)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("No GeoCells found within %v meters of (%v, %v)", radius, latitude, longitude)
		return cell, nil
	}
	if err != nil {
		log.Printf("Error getting GeoCells: %v", err)
		return cell, err
	}

	return cell, nil
}

// ObsoleteGeoCells deletes all GeoCells that are older than 10 days
func (repo *PostgresRepository) ObsoleteGeoCells() error {
	query := `
        DELETE FROM geocells
        WHERE created_at < NOW() - INTERVAL '10 days';
    `

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Error deleting obsolete GeoCells: %v", err)
		return err
	}

	log.Println("Deleted obsolete GeoCells")
	return nil

}

// GetAllGeoCells returns all GeoCells in the database
func (repo *PostgresRepository) GetAllGeoCells() ([]models.GeoCell, error) {

	query := `
			SELECT id, crime_name, crime_score, crime_firearm, weather_description, weather_condition, ST_X(coordinates), ST_Y(coordinates)
			FROM geocells
    `

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error getting GeoCells: %v", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}(rows)

	var cells []models.GeoCell

	for rows.Next() {
		var cell models.GeoCell
		err := rows.Scan(&cell.Id, &cell.CrimeName, &cell.CrimeScore, &cell.CrimeFirearm, &cell.WeatherDescription, &cell.WeatherCondition, &cell.Longitude, &cell.Latitude)
		if err != nil {
			log.Printf("Error getting GeoCells: %v", err)
		}
		cells = append(cells, cell)
	}

	return cells, nil
}

// Connect returns the database connection
func (repo *PostgresRepository) Connect() (*sql.DB, error) {
	return db, nil
}
