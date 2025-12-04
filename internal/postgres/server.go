package postgres_server

import (
	"catalyst/internal/logger"
	"catalyst/internal/models"
	"catalyst/internal/postgres/seeder"
	"context"
	"fmt"
	"github.com/SOLUCIONESSYCOM/scribe"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	Name              string
	User              string
	Password          string
	Host              string
	Port              int
	Database          string
	InitScript        string
	Seed              []models.Seed
	PostgresContainer *postgres.PostgresContainer
	logger            *scribe.Scribe
	LoggerPath        string
}

type PostgresManager struct {
	servers map[int]*Server
	wg      sync.WaitGroup
}

func NewPostgresManager() *PostgresManager {
	return &PostgresManager{
		servers: make(map[int]*Server),
		wg:      sync.WaitGroup{},
	}
}

func (m *PostgresManager) CreateServers(config *models.MockServer) error {
	for _, serverConfig := range config.PostgresServers.Postgres {
		if err := m.CreateServer(serverConfig); err != nil {
			return fmt.Errorf("error creating server on port %d: %w", serverConfig.Port, err)
		}
	}
	return nil
}

func (m *PostgresManager) CreateServer(config models.PostgresServer) error {
	if _, exists := m.servers[config.Port]; exists {
		return fmt.Errorf("Postgres server on port %d already exists", config.Port)
	}

	var log *scribe.Scribe
	var isLog bool
	var logPath string

	if config.Logger != nil {
		isLog = true
	}

	if config.LoggerPath == nil {
		logPath = "./logs"
	} else {
		logPath = *config.LoggerPath
	}
	log, err := logger.GetLoggerContext(models.LogDescriptor{
		Name:   config.Name,
		Path:   logPath,
		File:   isLog,
		Logger: isLog,
	})

	if err != nil {
		return err
	}
	server := &Server{
		Name:       config.Name,
		User:       config.User,
		Password:   config.Password,
		Host:       config.Host,
		Port:       config.Port,
		Database:   config.Database,
		InitScript: config.InitScript,
		LoggerPath: logPath,
		logger:     log,
		Seed:       config.Seed,
	}

	m.servers[config.Port] = server

	return nil
}
func (m *PostgresManager) Stop() {
	for _, server := range m.servers {
		server.Stop()
	}
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stop := 5 * time.Second

	defer func() {
		a := recover()

		if a != nil {
			s.logger.Error().Msg(fmt.Sprintf("Panic recover. Error stopping Postgres container: %v with Name: %s", a, s.Name))
		}
	}()
	err := s.PostgresContainer.Stop(ctx, &stop)

	if err != nil {
		s.logger.Error().Msg(fmt.Sprintf("Error stopping Postgres container: %v with Name: %s", err, s.Name))
	}

	err = s.PostgresContainer.Terminate(ctx)

	if err != nil {
		s.logger.Error().Msg(fmt.Sprintf("Error terminating Postgres container: %v with Name: %s", err, s.Name))
	}
}
func (m *PostgresManager) Start() error {
	for _, server := range m.servers {
		m.wg.Add(1)
		go func(s *Server) {
			defer m.wg.Done()
			ctx := context.Background()

			container, err := s.Start()

			if err != nil {
				s.logger.Error().Msg(fmt.Sprintf("Error starting Postgres container: %v with Name: %s", err, s.Name))
				return
			}
			s.PostgresContainer = container

			if container == nil {
				return
			}
			state, err := container.State(ctx)

			if err != nil {
				s.logger.Error().Msg(fmt.Sprintf("	Failed to get state of PostgresManager: %v with Name: %s", err, s.Name))
			}

			retries := 1
			for !state.Running && retries <= 5 {
				err := container.Start(ctx)
				if err != nil {
					s.logger.Error().Msg(fmt.Sprintf("Failed to start PostgresManager: %v with Name: %s, retry %d/5", err, s.Name, retries))
					retries++
					time.Sleep(3 * time.Second)
				}
			}

			// If the server has a seed configuration, run the migration
			if s.Seed != nil {
				prepareMigration(s, ctx)
			}
		}(server)
	}

	return nil
}

func prepareMigration(s *Server, ctx context.Context) {
	s.logger.Info().Msg(fmt.Sprintf("Found seed configuration for server %s, running migration", s.Name))

	// Create bool variables for logger configuration
	loggerEnabled := true
	fileEnabled := true

	migrationService, err := seeder.NewMigrationService(&models.PostgresServer{
		Name:              s.Name,
		User:              s.User,
		Password:          s.Password,
		Host:              s.Host,
		Port:              s.Port,
		Database:          s.Database,
		PostgresContainer: s.PostgresContainer,
		Logger:            &loggerEnabled,
		LoggerPath:        &s.LoggerPath,
		File:              &fileEnabled,
	})

	if err != nil {
		s.logger.Error().Msg(fmt.Sprintf("Failed to create migration service: %v", err))
		return
	}

	// Set the postgres container
	migrationService.SetPostgresContainer(s.PostgresContainer)

	// Iterate through each seed configuration and run the migration
	for _, seed := range s.Seed {
		// Run the migration for this seed
		if err := migrationService.Migrate(ctx, seed); err != nil {
			s.logger.Error().Msg(fmt.Sprintf("Failed to run migration for table %s.%s: %v", seed.Schema, seed.Table, err))
		} else {
			s.logger.Info().Msg(fmt.Sprintf("Successfully ran migration for table %s.%s in server %s", seed.Schema, seed.Table, s.Name))
		}
	}
}

func (s *Server) Start() (*postgres.PostgresContainer, error) {
	ctx := context.TODO()
	var scripts testcontainers.CustomizeRequestOption
	var opts []testcontainers.ContainerCustomizer

	if s.InitScript != "" {
		scripts = postgres.WithInitScripts(s.InitScript)
		opts = []testcontainers.ContainerCustomizer{
			testcontainers.WithExposedPorts(strconv.Itoa(s.Port) + ":5432/tcp"),
			postgres.WithDatabase(s.Database),
			postgres.WithUsername(s.User),
			postgres.WithPassword(s.Password),
			scripts,
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections")),
		}
	}

	if scripts == nil {
		scripts = postgres.WithInitScripts("")
		opts = []testcontainers.ContainerCustomizer{
			testcontainers.WithExposedPorts(strconv.Itoa(s.Port) + ":5432/tcp"),
			postgres.WithDatabase(s.Database),
			postgres.WithUsername(s.User),
			postgres.WithPassword(s.Password),
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections")),
		}
	}

	defer func() {
		a := recover()
		s.logger.Error().Msg(fmt.Sprintf("Recover from Panic %v , server with Name: %s", a, s.Name))
	}()

	// Run the container. The postgres.Run function returns a pointer to a
	// postgres.PostgresContainer and an error.
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		opts...,
	)

	if err != nil {
		return nil, err
	}

	return postgresContainer, nil

}
