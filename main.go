package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/calvarado2004/grid-service/data"
	"github.com/calvarado2004/grid-service/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"

	amqp "github.com/rabbitmq/amqp091-go"
)

const googleDirectionsAPI = "https://maps.googleapis.com/maps/api/directions/json"

var counts int64

var (
	dbConnectionErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_connection_errors_total",
		Help: "Total number of database connection errors.",
	})
	directionsRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "directions_requests_total",
		Help: "Total number of requests to /directions endpoint.",
	})
	goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_goroutines",
		Help: "Number of goroutines currently running.",
	})

	geoCellCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "geocell_found_total",
			Help: "Total number of geoCells found by EvaluateRoute.",
		},
		[]string{"lat", "lon", "crime_name", "crime_firearm", "weather_description"})
)

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return nil, err
	}

	if err = db.Ping(); err != nil {
		log.Printf("Failed to ping database: %v", err)
		return nil, err
	}

	return db, nil

}

func connectToDB() *sql.DB {

	postgresUser := os.Getenv("POSTGRES_USER")
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresHost := os.Getenv("POSTGRES_HOST")
	postgresPort := os.Getenv("POSTGRES_PORT")
	postgresDatabase := os.Getenv("POSTGRES_DATABASE")

	if postgresUser == "" {
		postgresUser = "postgres"
	}

	if postgresPassword == "" {
		postgresPassword = "postgres"
	}

	if postgresHost == "" {
		postgresHost = "localhost"
	}

	if postgresPort == "" {

		postgresPort = "5432"
	}

	if postgresDatabase == "" {
		postgresDatabase = "crime_data"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", postgresUser, postgresPassword, postgresHost, postgresPort, postgresDatabase)

	for {
		connection, err := openDB(dsn)
		if err != nil {
			dbConnectionErrors.Inc()

			log.Printf("Error opening database: %s", err)
			counts++
		} else {
			log.Println("Connected to database")
			return connection
		}

		if counts > 10 {
			log.Printf("Could not connect to database after 10 attempts: %v", err)
			return nil
		}

		log.Println("Retrying in 5 seconds")
		time.Sleep(5 * time.Second)
		continue

	}
}

type Config struct {
	Repo   data.Repository
	Client *http.Client
}

func (app *Config) setupRepo(conn *sql.DB) {
	db := data.NewPostgresRepository(conn)
	app.Repo = db

}

type RabbitMQGridService struct {
	Config
	mu          sync.Mutex
	rabbitmqURL string
	Repo        data.Repository
}

func NewGridService(config Config, rabbitmqURL string) *RabbitMQGridService {
	return &RabbitMQGridService{
		Config:      config,
		Repo:        config.Repo,
		rabbitmqURL: rabbitmqURL,
	}
}

func main() {

	rabbitMQHost := os.Getenv("RABBITMQ_HOSTNAME")
	rabbitMQPort := os.Getenv("RABBITMQ_PORT")
	rabbitMQUser := os.Getenv("RABBITMQ_USER")
	rabbitMQPassword := os.Getenv("RABBITMQ_PASSWORD")

	prometheus.MustRegister(dbConnectionErrors)
	prometheus.MustRegister(directionsRequests)
	prometheus.MustRegister(goroutines)
	prometheus.MustRegister(geoCellCounter)

	if rabbitMQHost == "" {
		rabbitMQHost = "localhost"
	}

	if rabbitMQPort == "" {
		rabbitMQPort = "5672"
	}

	if rabbitMQUser == "" {
		rabbitMQUser = "guest"
	}

	if rabbitMQPassword == "" {
		rabbitMQPassword = "guest"
	}

	rabbitMQURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", rabbitMQUser, rabbitMQPassword, rabbitMQHost, rabbitMQPort)

	// Connect to PostgreSQL
	pgConn := connectToDB()
	if pgConn == nil {
		log.Panic("Could not connect to database")
	}

	gridService := NewGridService(Config{}, rabbitMQURL)
	gridService.setupRepo(pgConn)

	gridService.Repo = data.NewPostgresRepository(pgConn)

	gridService.Client = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create a channel to block main from exiting
	done := make(chan bool)

	// Start the HTTP server in its own goroutine
	go func() {

		handler := http.NewServeMux()
		handler.HandleFunc("/directions", gridService.GetDirectionsHandler)
		handler.Handle("/metrics", promhttp.Handler())

		c := cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "DELETE", "HEAD", "OPTIONS", "POST", "PUT"},
			AllowCredentials: true,
		})

		log.Println("Starting server on :8081")
		err := http.ListenAndServe(":8081", c.Handler(handler))
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}

	}()

	// Start consuming crime data messages
	go func() {
		err := gridService.CreateGridFromCrimes()
		if err != nil {
			log.Fatalf("Error creating grid from crimes: %v", err)
		}
	}()

	// Start consuming weather messages
	go func() {
		err := gridService.WeatherMessages()
		if err != nil {
			log.Fatalf("Error getting weather messages: %v", err)
		}
	}()

	// Start a goroutine to update the number of goroutines every 10 seconds to Prometheus
	go func() {
		for {
			goroutines.Set(float64(runtime.NumGoroutine()))
			time.Sleep(10 * time.Second)
		}
	}()

	// Block main from exiting
	<-done

}

// CreateGridFromCrimes creates a grid of GeoCells from crime data
func (gridService *RabbitMQGridService) CreateGridFromCrimes() error {

	connection, err := amqp.Dial(gridService.rabbitmqURL)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %v", err)
		return err
	}
	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close connection: %v", err)
		}
	}(connection)

	channel, err := connection.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %v", err)
		return err
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			log.Printf("Failed to close channel: %v", err)
		}
	}(channel)

	// Consume crime messages
	msgsCrime, err := channel.Consume(
		"crime_data",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("Error consuming crime messages: %v", err)
	}

	for msg := range msgsCrime {

		err := gridService.ProcessCrimes(msg)
		if err != nil {
			log.Printf("Error processing crimes: %v", err)
		}

	}

	return nil

}

// ProcessCrimes processes crime messages and inserts them into the database
func (gridService *RabbitMQGridService) ProcessCrimes(msg amqp.Delivery) error {

	// create a new CrimeMessage
	var crimeMessage models.CrimeMessage
	err := json.Unmarshal(msg.Body, &crimeMessage)
	if err != nil {
		log.Printf("Error decoding JSON: %v", err)
		return err
	}

	// create a new GeoCell using coordinates from the CrimeMessage
	var geoCell models.GeoCell

	// set the latitude and longitude of the GeoCell, Latitude first
	geoCell.Coordinates = []float64{crimeMessage.Latitude, crimeMessage.Longitude}
	geoCell.Latitude = crimeMessage.Latitude
	geoCell.Longitude = crimeMessage.Longitude
	// set the crime name
	geoCell.CrimeName = crimeMessage.CrimeName
	// set the crime firearm
	geoCell.CrimeFirearm = crimeMessage.Firearm

	// set the crime score
	if crimeMessage.Firearm == "Y" {
		geoCell.CrimeScore = 5
	} else {
		geoCell.CrimeScore = 2
	}

	if gridService.Repo == nil {
		log.Printf("Repo is nil!")
		return errors.New("Repo is nil")
	}

	// insert the GeoCell into the database
	err = gridService.Repo.InsertGeoCell(geoCell)
	if err != nil {
		log.Printf("Error inserting GeoCell: %v", err)
		return err
	}

	return nil
}

// WeatherMessages consumes weather messages and updates the GeoCells
func (gridService *RabbitMQGridService) WeatherMessages() error {

	connection, err := amqp.Dial(gridService.rabbitmqURL)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %v", err)
		return err
	}
	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close connection: %v", err)
		}
	}(connection)

	channel, err := connection.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %v", err)
		return err
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			log.Printf("Failed to close channel: %v", err)
		}
	}(channel)

	// Consume weather messages
	msgsWeather, err := channel.Consume(
		"weather_queue",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("Error consuming weather messages: %v", err)
	}

	for msg := range msgsWeather {

		err := gridService.ProcessWeatherMessages(msg)
		if err != nil {
			log.Printf("Error processing weather messages: %v", err)
		}

	}

	return nil

}

// ProcessWeatherMessages processes weather messages and updates the GeoCells
func (gridService *RabbitMQGridService) ProcessWeatherMessages(msg amqp.Delivery) error {

	// create a new WeatherMessage
	var weatherMessage models.WeatherAPIResponse
	err := json.Unmarshal(msg.Body, &weatherMessage)
	if err != nil {
		log.Printf("Error decoding JSON: %v", err)
		return err
	}

	// update all GeoCells with the new weather data
	err = gridService.Repo.UpdateWeather(weatherMessage)
	if err != nil {
		log.Printf("Error updating weather into GeoCells: %v", err)
	}

	return nil
}

func (gridService *RabbitMQGridService) FetchDirections(start, end string) (models.DirectionResponse, error) {
	GoogleApiKey := os.Getenv("GOOGLE_API_KEY")

	// Construct URL for Directions API
	url := googleDirectionsAPI + "?origin=" + start + "&destination=" + end +
		"&mode=transit&transit_mode=bus&key=" + GoogleApiKey

	log.Printf("Fetching directions from %s to %s\n", start, end)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching directions on get: %s\n", err)
		return models.DirectionResponse{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalf("Error closing response body: %s\n", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error fetching directions read body: %s\n", err)
		return models.DirectionResponse{}, err
	}

	var response models.DirectionResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Printf("Error fetching directions unmarshalling: %s\n", err)
		return models.DirectionResponse{}, err
	}

	return response, nil
}

// EvaluateRoute evaluates the route for crime and weather
func (gridService *RabbitMQGridService) EvaluateRoute(directions models.DirectionResponse) (models.DirectionResponse, string, error) {

	evaluationResult := "This route is safe, because we didn't find any crime or bad weather"

	highestCrimeScore := 0
	var highestCrimeCell models.GeoCell
	checkedLocations := make(map[string]bool) // To ensure we don't check the same location multiple times

	if len(directions.Routes) == 0 {
		return directions, "No routes provided", nil
	}

	for _, route := range directions.Routes {

		if len(route.Legs) == 0 {
			return directions, "No legs provided", nil
		}

		for _, leg := range route.Legs {

			if len(leg.Steps) == 0 {
				return directions, "No steps provided on legs", nil
			}

			for _, step := range leg.Steps {

				var locations []models.Location

				// Get the start location
				locations = append(locations, models.Location{Lat: step.StartLocation.Lat,
					Lon: step.StartLocation.Lng})

				// Get the end location
				locations = append(locations, models.Location{Lat: step.EndLocation.Lat, Lon: step.EndLocation.Lng})

				// Get each step location
				for _, step := range step.Steps {
					locations = append(locations, models.Location{Lat: step.StartLocation.Lat,
						Lon: step.StartLocation.Lng})
					locations = append(locations, models.Location{Lat: step.EndLocation.Lat,
						Lon: step.EndLocation.Lng})
				}

				if len(locations) == 0 {
					return directions, "No locations provided", nil
				}

				// Evaluate each location
				for _, loc := range locations {
					locKey := fmt.Sprintf("%.6f,%.6f", loc.Lat, loc.Lon)
					if _, exists := checkedLocations[locKey]; exists {
						continue
					}
					checkedLocations[locKey] = true

					// Get the GeoCell for the location, about 200 meters
					cell, err := gridService.Repo.GetOneNearbyGeoCell(loc.Lat, loc.Lon, 200)
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					if err != nil {
						log.Printf("Error getting GeoCell for location: %v", err)
						continue
					}

					if cell.CrimeScore > 0 {
						log.Printf("Found GeoCell with a recent crime nearby: %s, score %d, firearm %s", cell.CrimeName, cell.CrimeScore, cell.CrimeFirearm)
						// Increment the counter for this GeoCell for Prometheus
						geoCellCounter.With(prometheus.Labels{
							"lat":                 fmt.Sprintf("%.6f", loc.Lat),
							"lon":                 fmt.Sprintf("%.6f", loc.Lon),
							"crime_name":          cell.CrimeName,
							"crime_firearm":       cell.CrimeFirearm,
							"weather_description": cell.WeatherDescription,
						}).Inc()

					}

					if cell.CrimeScore > highestCrimeScore {
						highestCrimeScore = cell.CrimeScore
						highestCrimeCell = cell
					}
				}
			}
		}
	}

	// After evaluating all cells, now set the evaluation result
	switch {
	case highestCrimeScore == 5:
		evaluationResult = "This route is not safe, please choose another route, we found a recent crime nearby: " + highestCrimeCell.CrimeName + ". "
		if highestCrimeCell.CrimeFirearm == "Y" {
			evaluationResult += "And a firearm was involved."
		}
		evaluationResult += " Also, the weather is " + highestCrimeCell.WeatherDescription
		directions.Routes[0].Summary = evaluationResult
		directions.Status = "DANGER"

	case highestCrimeScore == 2:
		evaluationResult = "This route is not too safe, proceed with caution" + ", we found a recent crime nearby: " +
			highestCrimeCell.CrimeName + ". Also, the weather is " + highestCrimeCell.WeatherDescription
		directions.Routes[0].Summary = evaluationResult
		directions.Status = "WARNING"

	default:
		directions.Routes[0].Summary = evaluationResult
		directions.Status = "OK"

	}

	return directions, evaluationResult, nil
}

// GetDirectionsHandler handles requests for directions
func (gridService *RabbitMQGridService) GetDirectionsHandler(w http.ResponseWriter, r *http.Request) {
	directionsRequests.Inc()

	// Extract 'start' and 'end' values from the request URL
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	var request models.DirectionRequest

	request.Start = start
	request.End = end

	if request.Start == "" || request.End == "" {
		http.Error(w, "Both start and end coordinates are required", http.StatusBadRequest)
		return
	}

	response, err := gridService.FetchDirections(request.Start, request.End)
	if err != nil {
		http.Error(w, "Failed to fetch directions", http.StatusInternalServerError)
		return
	}
	response, evaluation, err := gridService.EvaluateRoute(response)
	if err != nil {
		http.Error(w, "Failed to evaluate route", http.StatusInternalServerError)
		return
	}

	log.Printf("Evaluation result: %s\n", evaluation)

	// Send the evaluated response back to the client
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to send response", http.StatusInternalServerError)
		return
	}
}
