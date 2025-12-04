package seeder

import (
	"catalyst/internal/logger"
	"catalyst/internal/models"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/SOLUCIONESSYCOM/scribe"
	"github.com/go-faker/faker/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// ColumnInfo represents the metadata for a database column
type ColumnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
}

// MigrationService handles database seeding operations
type MigrationService struct {
	Logger            *scribe.Scribe
	Server            *models.PostgresServer
	PostgresContainer *postgres.PostgresContainer
}

// NewMigrationService creates a new instance of MigrationService
func NewMigrationService(server *models.PostgresServer) (*MigrationService, error) {
	// Initialize logger
	log, err := logger.GetLoggerContext(models.LogDescriptor{
		Name:   server.Name,
		Path:   *server.LoggerPath,
		File:   *server.File,
		Logger: *server.Logger,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	faker.SetRandomSource(rand.NewSource(time.Now().UnixNano()))

	return &MigrationService{
		Logger: log,
		Server: server,
	}, nil
}

// SetPostgresContainer sets the PostgresContainer for the MigrationService
func (m *MigrationService) SetPostgresContainer(container *postgres.PostgresContainer) {
	m.PostgresContainer = container
}

// RandomString generates a random string of the specified length
func RandomString(length int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, length)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// RandomName generates a random name
func RandomName() string {
	var person struct {
		FirstName string `faker:"first_name"`
		LastName  string `faker:"last_name"`
	}
	_ = faker.FakeData(&person)
	return person.FirstName + " " + person.LastName
}

// RandomEmail generates a random email address
func RandomEmail() string {
	var user struct {
		Email string `faker:"email"`
	}
	_ = faker.FakeData(&user)
	return user.Email
}

// RandomPhone generates a random phone number
func RandomPhone() string {
	var user struct {
		Phone string `faker:"phone_number"`
	}
	_ = faker.FakeData(&user)
	return user.Phone
}

// RandomAddress generates a random street address
func RandomAddress() string {
	var address struct {
		Street string `faker:"street_address"`
	}
	_ = faker.FakeData(&address)
	return address.Street
}

// RandomCity generates a random city name
func RandomCity() string {
	var address struct {
		City string `faker:"city"`
	}
	_ = faker.FakeData(&address)
	return address.City
}

// RandomCountry generates a random country name
func RandomCountry() string {
	var address struct {
		Country string `faker:"country"`
	}
	_ = faker.FakeData(&address)
	return address.Country
}

// RandomZip generates a random ZIP code
func RandomZip() string {
	var address struct {
		Zip string `faker:"zip"`
	}
	_ = faker.FakeData(&address)
	return address.Zip
}

// RandomParagraph generates a random paragraph
func RandomParagraph() string {
	var text struct {
		Paragraph string `faker:"paragraph"`
	}
	_ = faker.FakeData(&text)
	return text.Paragraph
}

// RandomSentence generates a random sentence
func RandomSentence() string {
	var text struct {
		Sentence string `faker:"sentence"`
	}
	_ = faker.FakeData(&text)
	return text.Sentence
}

// RandomUUID generates a random UUID-like string
func RandomUUID() string {
	var id struct {
		UUID string `faker:"uuid_digit"`
	}
	_ = faker.FakeData(&id)
	return id.UUID
}

// GenerateFakeValue generates a fake value based on the column data type
func (m *MigrationService) GenerateFakeValue(column ColumnInfo) string {
	// Handle NULL values for nullable columns (randomly make ~10% of values NULL)
	if column.IsNullable && rand.Float32() < 0.1 {
		return "NULL"
	}

	// Convert data type to lowercase for easier comparison
	dataType := strings.ToLower(column.DataType)

	switch {
	case strings.Contains(dataType, "int"):
		var num struct {
			Int int32 `faker:"int32"`
		}
		_ = faker.FakeData(&num)
		return fmt.Sprintf("%d", num.Int)
	case strings.Contains(dataType, "serial"):
		var num struct {
			Int int32 `faker:"int32"`
		}
		_ = faker.FakeData(&num)
		return fmt.Sprintf("%d", num.Int)
	case strings.Contains(dataType, "float") || strings.Contains(dataType, "double") || strings.Contains(dataType, "numeric") || strings.Contains(dataType, "decimal"):
		var num struct {
			Float float32 `faker:"float32"`
		}
		_ = faker.FakeData(&num)
		return fmt.Sprintf("%f", num.Float)
	case strings.Contains(dataType, "bool"):
		var b struct {
			Bool bool `faker:"bool"`
		}
		_ = faker.FakeData(&b)
		if b.Bool {
			return "TRUE"
		}
		return "FALSE"
	case strings.Contains(dataType, "date"):
		var d struct {
			Date time.Time `faker:"date"`
		}
		_ = faker.FakeData(&d)
		return fmt.Sprintf("'%s'", d.Date.Format("2006-01-02"))
	case strings.Contains(dataType, "time"):
		if strings.Contains(dataType, "timestamp") {
			var ts struct {
				Timestamp time.Time `faker:"timestamp"`
			}
			_ = faker.FakeData(&ts)
			return fmt.Sprintf("'%s'", ts.Timestamp.Format("2006-01-02 15:04:05"))
		}

		var t struct {
			Time string `faker:"time"`
		}
		_ = faker.FakeData(&t)
		return fmt.Sprintf("'%s'", t.Time)
	case strings.Contains(dataType, "char") || strings.Contains(dataType, "text"):
		// Generate different types of text based on column name hints
		colName := strings.ToLower(column.Name)
		var value string
		switch {
		case strings.Contains(colName, "name"):
			value = RandomName()
		case strings.Contains(colName, "email"):
			value = RandomEmail()
		case strings.Contains(colName, "phone"):
			value = RandomPhone()
		case strings.Contains(colName, "address"):
			value = RandomAddress()
		case strings.Contains(colName, "city"):
			value = RandomCity()
		case strings.Contains(colName, "country"):
			value = RandomCountry()
		case strings.Contains(colName, "zip") || strings.Contains(colName, "postal"):
			value = RandomZip()
		case strings.Contains(colName, "description") || strings.Contains(colName, "comment"):
			value = RandomParagraph()
		default:
			value = RandomSentence()
		}
		// Escape single quotes for SQL
		value = strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", value)
	case strings.Contains(dataType, "json") || strings.Contains(dataType, "jsonb"):
		return fmt.Sprintf("'{\"key\": \"%s\", \"value\": \"%s\"}'", RandomString(8), strings.ReplaceAll(RandomSentence(), "'", "''"))
	case strings.Contains(dataType, "uuid"):
		return fmt.Sprintf("'%s'", RandomUUID())
	default:
		// For unknown types, return a string
		value := RandomSentence()
		value = strings.ReplaceAll(value, "'", "''")
		return fmt.Sprintf("'%s'", value)
	}
}

// Migrate inserts seed data directly into the database using pgx
func (m *MigrationService) Migrate(ctx context.Context, seed models.Seed) error {
	m.Logger.Info().Msg(fmt.Sprintf("Starting migration for table %s.%s with %d rows", seed.Schema, seed.Table, seed.Rows))

	// Get connection string from PostgresContainer
	if m.PostgresContainer == nil {
		return fmt.Errorf("postgres container is not initialized")
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?%s", m.Server.User, m.Server.Password, m.Server.Host, m.Server.Port, m.Server.Database, "sslmode=disable")

	// Connect to the database
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to parse connection string: %v", err))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to connect to database: %v", err))
		return err
	}
	defer pool.Close()

	// Create a map of column overrides for quick lookup
	overrides := make(map[string]string)
	for _, override := range seed.Overrides {
		overrides[override.Column] = override.Value
	}

	// Define a set of common column types for introspection
	columnTypes := map[string][]ColumnInfo{
		"users": {
			{Name: "id", DataType: "serial", IsNullable: false},
			{Name: "username", DataType: "varchar", IsNullable: false},
			{Name: "email", DataType: "varchar", IsNullable: false},
			{Name: "password", DataType: "varchar", IsNullable: false},
			{Name: "created_at", DataType: "timestamp", IsNullable: false},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		},
		"products": {
			{Name: "id", DataType: "serial", IsNullable: false},
			{Name: "name", DataType: "varchar", IsNullable: false},
			{Name: "description", DataType: "text", IsNullable: true},
			{Name: "price", DataType: "numeric", IsNullable: false},
			{Name: "stock", DataType: "int", IsNullable: false},
			{Name: "created_at", DataType: "timestamp", IsNullable: false},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		},
		"orders": {
			{Name: "id", DataType: "serial", IsNullable: false},
			{Name: "user_id", DataType: "int", IsNullable: false},
			{Name: "status", DataType: "varchar", IsNullable: false},
			{Name: "total", DataType: "numeric", IsNullable: false},
			{Name: "created_at", DataType: "timestamp", IsNullable: false},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		},
	}

	// Use predefined columns if available, otherwise use a default set
	var columns []ColumnInfo
	if predefinedColumns, exists := columnTypes[strings.ToLower(seed.Table)]; exists {
		columns = predefinedColumns
		m.Logger.Info().Msg(fmt.Sprintf("Using predefined columns for table %s", seed.Table))
	} else {
		// Default columns if table is not recognized
		columns = []ColumnInfo{
			{Name: "id", DataType: "serial", IsNullable: false},
			{Name: "name", DataType: "varchar", IsNullable: false},
			{Name: "description", DataType: "text", IsNullable: true},
			{Name: "created_at", DataType: "timestamp", IsNullable: false},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		}
		m.Logger.Info().Msg(fmt.Sprintf("Using default columns for table %s", seed.Table))
	}

	// Create schema if it doesn't exist
	_, err = pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", seed.Schema))
	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to create schema %s: %v", seed.Schema, err))
		return err
	}

	// Check if table exists
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = $1 AND table_name = $2
		)`, seed.Schema, seed.Table).Scan(&tableExists)

	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to check if table %s.%s exists: %v", seed.Schema, seed.Table, err))
		return err
	}

	if !tableExists {
		m.Logger.Error().Msg(fmt.Sprintf("Table %s.%s does not exist. Skipping data insertion.", seed.Schema, seed.Table))
		return fmt.Errorf("table %s.%s does not exist", seed.Schema, seed.Table)
	}

	// Verify that override columns exist in the table
	for _, override := range seed.Overrides {
		var columnExists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
			)`, seed.Schema, seed.Table, override.Column).Scan(&columnExists)

		if err != nil {
			m.Logger.Error().Msg(fmt.Sprintf("Failed to check if column %s exists in table %s.%s: %v",
				override.Column, seed.Schema, seed.Table, err))
			return err
		}

		if !columnExists {
			m.Logger.Error().Msg(fmt.Sprintf("Column %s does not exist in table %s.%s. Skipping data insertion.",
				override.Column, seed.Schema, seed.Table))
			return fmt.Errorf("column %s does not exist in table %s.%s", override.Column, seed.Schema, seed.Table)
		}
	}

	// Get actual columns from the database
	rows, err := pool.Query(ctx, `
		SELECT column_name, data_type, is_nullable 
		FROM information_schema.columns 
		WHERE table_schema = $1 AND table_name = $2
	`, seed.Schema, seed.Table)

	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to get columns for table %s.%s: %v", seed.Schema, seed.Table, err))
		return err
	}
	defer rows.Close()

	// Replace predefined columns with actual columns from the database
	dbColumns := []ColumnInfo{}
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable); err != nil {
			m.Logger.Error().Msg(fmt.Sprintf("Failed to scan column info: %v", err))
			return err
		}
		col.IsNullable = isNullable == "YES"
		dbColumns = append(dbColumns, col)
	}

	if len(dbColumns) > 0 {
		columns = dbColumns
		m.Logger.Info().Msg(fmt.Sprintf("Using actual columns from database for table %s.%s", seed.Schema, seed.Table))
	}

	// Start a transaction for batch inserts
	tx, err := pool.Begin(ctx)
	if err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to start transaction: %v", err))
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	// Prepare batch insert
	batch := &pgx.Batch{}

	// Generate insert statements
	for i := 0; i < seed.Rows; i++ {
		var columnNames []string
		var placeholders []string
		var values []interface{}

		for _, col := range columns {
			// Skip serial columns as they are auto-generated
			if strings.Contains(strings.ToLower(col.DataType), "serial") {
				continue
			}

			columnNames = append(columnNames, col.Name)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(placeholders)+1))

			// Check if there's an override for this column
			if val, exists := overrides[col.Name]; exists {
				values = append(values, val)
			} else {
				// Generate fake data based on column type
				fakeValue := m.GenerateFakeValue(col)

				// Remove quotes for SQL parameters
				if strings.HasPrefix(fakeValue, "'") && strings.HasSuffix(fakeValue, "'") {
					fakeValue = fakeValue[1 : len(fakeValue)-1]
				}

				// Handle NULL values
				if fakeValue == "NULL" {
					values = append(values, nil)
				} else {
					values = append(values, fakeValue)
				}
			}
		}

		// Skip if no columns to insert
		if len(columnNames) == 0 {
			continue
		}

		query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
			seed.Schema, seed.Table,
			strings.Join(columnNames, ", "),
			strings.Join(placeholders, ", "))

		batch.Queue(query, values...)

		// Execute batch every 100 rows to avoid large transactions
		if i > 0 && i%100 == 0 {
			br := tx.SendBatch(ctx, batch)
			if err := br.Close(); err != nil {
				m.Logger.Error().Msg(fmt.Sprintf("Failed to execute batch insert: %v", err))
				return err
			}
			batch = &pgx.Batch{}
		}
	}

	// Execute any remaining batch items
	if batch.Len() > 0 {
		br := tx.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			m.Logger.Error().Msg(fmt.Sprintf("Failed to execute final batch insert: %v", err))
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		m.Logger.Error().Msg(fmt.Sprintf("Failed to commit transaction: %v", err))
		return err
	}

	m.Logger.Info().Msg(fmt.Sprintf("Successfully migrated table %s.%s with %d rows", seed.Schema, seed.Table, seed.Rows))
	return nil
}
