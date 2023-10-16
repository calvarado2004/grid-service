package main

import (
	"context"
	"encoding/json"
	"github.com/calvarado2004/grid-service/data"
	"github.com/calvarado2004/grid-service/models"
	"github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestProcessCrimes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	msg := amqp.Delivery{Body: []byte(`{"Latitude": 40.71, "Longitude": -74.01, "CrimeName": "theft", "Firearm": "Y"}`)}

	// Set expectation on the mock
	mockRepo.EXPECT().InsertGeoCell(gomock.Any()).Return(nil)

	err := service.ProcessCrimes(msg)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestProcessCrimesInvalidMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	msg := amqp.Delivery{Body: []byte(`{"Latitude"`)}

	// No mock expectation set for InsertGeoCell, since it shouldn't be called.

	err := service.ProcessCrimes(msg)

	expectedError := "unexpected end of JSON input"
	if err == nil || err.Error() != expectedError {
		t.Errorf("Expected error %q, but got %v", expectedError, err)
	}
}

func TestProcessWeatherMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	msg := amqp.Delivery{Body: []byte(`{"CellWeight": 1, "WeatherResponse": {"Current": {"TempF": 0}, "Location": {"Name": "New York"}}}`)}

	expectedWeather := models.WeatherAPIResponse{
		WeatherResponse: models.WeatherResponse{
			Current: models.Current{
				TempF: 0,
			},
			Location: models.Location{
				Name: "New York",
			},
		},
		CellWeight: 1,
	}

	// Set expectation on the mock
	mockRepo.EXPECT().UpdateWeather(expectedWeather).Return(nil)

	err := service.ProcessWeatherMessages(msg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestProcessWeatherMessagesInvalidMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	// Provide an invalid JSON message
	msg := amqp.Delivery{Body: []byte(`{"Temperature": 23, "Condition": "Clear"`)} // This JSON is deliberately cut off to make it invalid

	// Since the message is invalid, we expect that the Repo's UpdateWeather method is never called
	mockRepo.EXPECT().UpdateWeather(gomock.Any()).Times(0)

	err := service.ProcessWeatherMessages(msg)
	if err == nil {
		t.Error("Expected error due to invalid JSON, but got nil")
	}
}

func TestEvaluateRoute_Danger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	route := models.Routes{
		Legs: []models.Legs{
			{
				Duration: models.Duration{
					Value: 100,
				},
				Steps: []models.Steps{
					{
						Duration: models.Duration{
							Value: 100,
						},
						Distance: models.Distance{
							Value: 100,
						},
						Steps: []models.Steps{
							{
								Duration: models.Duration{
									Value: 100,
								},
								Distance: models.Distance{
									Value: 100,
								},
							},
						},
					},
				},
			},
		},
	}
	directions := models.DirectionResponse{
		Routes: []models.Routes{
			route,
		},
		Status: "DANGER",
	}

	// Mocked GeoCell with a high crime score
	dangerousCell := models.GeoCell{
		CrimeScore:         5,
		CrimeName:          "Robbery",
		CrimeFirearm:       "Y",
		WeatherDescription: "Rainy",
	}

	// Set the mock expectation
	mockRepo.EXPECT().GetOneNearbyGeoCell(gomock.Any(), gomock.Any(), gomock.Any()).Return(dangerousCell, nil).AnyTimes()

	result, evalResult, err := service.EvaluateRoute(directions)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Status != "DANGER" {
		t.Errorf("Expected status DANGER, but got %s", result.Status)
	}

	expectedMessage := "This route is not safe, please choose another route, we found a recent crime nearby: Robbery. And a firearm was involved. Also, the weather is Rainy"
	if evalResult != expectedMessage {
		t.Errorf("Expected evaluation result: %s, but got: %s", expectedMessage, evalResult)
	}
}

func TestEvaluateRoute_Safe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	route := models.Routes{
		Legs: []models.Legs{
			{
				Duration: models.Duration{
					Value: 100,
				},
				Steps: []models.Steps{
					{
						Duration: models.Duration{
							Value: 100,
						},
						Distance: models.Distance{
							Value: 100,
						},
						Steps: []models.Steps{
							{
								Duration: models.Duration{
									Value: 100,
								},
								Distance: models.Distance{
									Value: 100,
								},
							},
						},
					},
				},
			},
		},
	}
	directions := models.DirectionResponse{
		Routes: []models.Routes{
			route,
		},
		Status: "OK", // Setting expected initial status to OK
	}

	// Mocked GeoCell with a zero crime score (indicating safety)
	safeCell := models.GeoCell{
		CrimeScore:         0,
		CrimeName:          "",      // No crime recorded
		CrimeFirearm:       "N",     // No firearm involved
		WeatherDescription: "Sunny", // Just an example of benign weather
	}

	// Set the mock expectation
	mockRepo.EXPECT().GetOneNearbyGeoCell(gomock.Any(), gomock.Any(), gomock.Any()).Return(safeCell, nil).AnyTimes()

	result, evalResult, err := service.EvaluateRoute(directions)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Status != "OK" {
		t.Errorf("Expected status OK, but got %s", result.Status)
	}

	expectedMessage := "This route is safe, because we didn't find any crime or bad weather"
	if evalResult != expectedMessage {
		t.Errorf("Expected evaluation result: %s, but got: %s", expectedMessage, evalResult)
	}
}

func TestEvaluateRoute_Warning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	route := models.Routes{
		Legs: []models.Legs{
			{
				Duration: models.Duration{
					Value: 100,
				},
				Steps: []models.Steps{
					{
						Duration: models.Duration{
							Value: 100,
						},
						Distance: models.Distance{
							Value: 100,
						},
						Steps: []models.Steps{
							{
								Duration: models.Duration{
									Value: 100,
								},
								Distance: models.Distance{
									Value: 100,
								},
							},
						},
					},
				},
			},
		},
	}
	directions := models.DirectionResponse{
		Routes: []models.Routes{
			route,
		},
		Status: "OK", // Setting expected initial status to OK
	}

	// Mocked GeoCell with a crime score of 2 (indicating moderate danger)
	warningCell := models.GeoCell{
		CrimeScore:         2,
		CrimeName:          "Theft",
		CrimeFirearm:       "N",      // No firearm involved
		WeatherDescription: "Cloudy", // Just an example
	}

	// Set the mock expectation
	mockRepo.EXPECT().GetOneNearbyGeoCell(gomock.Any(), gomock.Any(), gomock.Any()).Return(warningCell, nil).AnyTimes()

	result, evalResult, err := service.EvaluateRoute(directions)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Status != "WARNING" {
		t.Errorf("Expected status WARNING, but got %s", result.Status)
	}

	expectedMessage := "This route is not too safe, proceed with caution, we found a recent crime nearby: Theft. Also, the weather is Cloudy"
	if evalResult != expectedMessage {
		t.Errorf("Expected evaluation result: %s, but got: %s", expectedMessage, evalResult)
	}
}

func TestGetDirectionsHandler_MissingParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	// Set up an HTTP test server
	server := httptest.NewServer(http.HandlerFunc(service.GetDirectionsHandler))
	defer server.Close()

	// Make a request without the 'end' parameter
	resp, err := http.Get(server.URL + "/?start=startCoord")
	if err != nil {
		t.Fatalf("Could not make GET request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	trimmedBody := strings.TrimSpace(string(body))
	expectedMessage := "Both start and end coordinates are required"
	if trimmedBody != expectedMessage {
		t.Fatalf("Expected error message: %q, got %q", expectedMessage, trimmedBody)
	}
}

func TestFetchDirections(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := data.NewMockRepository(ctrl)
	service := &RabbitMQGridService{Repo: mockRepo}

	// Mock the http.Get call
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Fake Google API Key
	err := os.Setenv("GOOGLE_API_KEY", "fake_api_key")
	if err != nil {
		return
	}

	start := "startCoord"
	end := "endCoord"
	expectedURL := googleDirectionsAPI + "?origin=" + start + "&destination=" + end +
		"&mode=transit&transit_mode=bus&key=fake_api_key"

	httpmock.RegisterResponder("GET", expectedURL,
		httpmock.NewStringResponder(200, `{"status": "OK", "routes": []}`))

	// Calling the function
	resp, err := service.FetchDirections(start, end)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Status != "OK" {
		t.Errorf("Expected status OK, but got %s", resp.Status)
	}
}

func TestRabbitMQFunctions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a new RabbitMQ container
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.9.29-management",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete; 4 plugins started."),
	}

	rabbitMQContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	// Cleanup container at the end
	defer func(rabbitMQContainer testcontainers.Container, ctx context.Context) {
		err := rabbitMQContainer.Terminate(ctx)
		if err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}(rabbitMQContainer, ctx)

	// Fetch the mapped port for RabbitMQ
	rmqPort, err := rabbitMQContainer.MappedPort(ctx, "5672")
	if err != nil {
		t.Fatalf("could not get mapped port: %s", err)
	}

	rmqHost, err := rabbitMQContainer.Host(ctx)
	if err != nil {
		t.Fatalf("could not get host: %s", err)
	}

	rmqAddress := "amqp://guest:guest@" + rmqHost + ":" + rmqPort.Port() + "/"

	// Connect to RabbitMQ and create a channel
	conn, err := amqp.Dial(rmqAddress)
	if err != nil {
		t.Fatalf("Failed to connect to RabbitMQ: %s", err)
	}
	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			t.Fatalf("Failed to close connection: %s", err)
		}
	}(conn)

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open a channel: %s", err)
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			t.Fatalf("Failed to close channel: %s", err)
		}
	}(ch)

	// Declare the "crime_data" queue
	_, err = ch.QueueDeclare(
		"crime_data", // name of the queue
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		t.Fatalf("Failed to declare the queue: %s", err)
	}

	// Publish dummy crime message
	crimeMsg := models.CrimeMessage{
		Latitude:  12.34,
		Longitude: 56.78,
		CrimeName: "Theft",
		Firearm:   "N",
	}

	body, _ := json.Marshal(crimeMsg)

	err = ch.PublishWithContext(ctx, "", "crime_data", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %s", err)
	}

	t.Logf("Published message: %s", string(body))

	// Declare the "weather_data" queue

	_, err = ch.QueueDeclare(
		"weather_data", // name of the queue
		false,          // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)

	if err != nil {
		t.Fatalf("Failed to declare the queue: %s", err)
	}

	// Publish dummy weather message
	weatherMsg := models.WeatherAPIResponse{
		WeatherResponse: models.WeatherResponse{
			Current: models.Current{
				TempF: 0,
			},
		},
		CellWeight: 1,
	}

	body, _ = json.Marshal(weatherMsg)

	err = ch.PublishWithContext(ctx, "", "weather_data", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})

	if err != nil {
		t.Fatalf("Failed to publish message: %s", err)
	}

	t.Logf("Published message: %s", string(body))

}
