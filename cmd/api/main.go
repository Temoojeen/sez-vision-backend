package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Temoojeen/sez-vision-backend/internal/config"
	"github.com/Temoojeen/sez-vision-backend/internal/handlers"
	"github.com/Temoojeen/sez-vision-backend/internal/middleware"
	"github.com/Temoojeen/sez-vision-backend/internal/models"
	"github.com/Temoojeen/sez-vision-backend/internal/repository"
	"github.com/Temoojeen/sez-vision-backend/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Загружаем .env файл
	err := godotenv.Load()
	if err != nil {
		log.Println("Note: .env file not found, using default values")
	}

	// Загружаем конфигурацию
	cfg := config.LoadConfig()

	// Формируем строку подключения
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.SSLMode,
	)

	log.Printf("🔌 Connecting to database: %s@%s:%s/%s",
		cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	// Подключаемся к базе данных
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}

	log.Println("✅ Successfully connected to PostgreSQL!")

	// Автомиграция для моделей
	err = db.AutoMigrate(
		&models.User{},
		&models.RUInfo{},
		&models.Cell{},
		&models.OperationRecord{},
	)
	if err != nil {
		log.Fatal("❌ Failed to auto migrate:", err)
	}
	log.Println("✅ Database tables migrated successfully!")

	// Проверяем существование тестовых данных
	checkAndSeedTestData(db)

	// Инициализируем репозитории
	userRepo := repository.NewUserRepository(db)
	ruRepo := repository.NewRuRepository(db)

	// Инициализируем сервисы
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	adminService := service.NewAdminService(userRepo, cfg.JWTSecret)
	ruService := service.NewRuService(ruRepo)

	// Инициализируем обработчики
	authHandler := handlers.NewAuthHandler(authService)
	adminHandler := handlers.NewAdminHandler(adminService)
	ruHandler := handlers.NewRuHandler(ruService)
	adminRuHandler := handlers.NewAdminRuHandler(ruService)

	// Настраиваем роутер
	router := gin.Default()

	// Настройка CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"Authorization",
			"Accept",
			"Cache-Control",
			"X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	// ================ ПУБЛИЧНЫЕ ЭНДПОИНТЫ ================

	// Публичный эндпоинт для получения данных подстанции
	router.GET("/api/substations/:id", ruHandler.GetSubstationPublic)

	// Public routes
	public := router.Group("/api/auth")
	{
		public.POST("/register", authHandler.Register)
		public.POST("/login", authHandler.Login)
		public.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":   "ok",
				"service":  "auth",
				"database": "connected",
			})
		})
	}

	// ================ ЗАЩИЩЕННЫЕ ЭНДПОИНТЫ ================

	// Protected routes - require JWT
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		// Auth routes
		auth := protected.Group("/auth")
		{
			auth.GET("/me", authHandler.GetMe)
		}

		// RU routes - доступны всем авторизованным
		rus := protected.Group("/rus")
		{
			rus.GET("/", ruHandler.GetAllRUs)                                // Получить все РУ
			rus.GET("/:id", ruHandler.GetRu)                                 // Получить РУ по ID
			rus.GET("/:id/history", ruHandler.GetHistory)                    // Получить историю операций
			rus.PUT("/:id/cells/:cellId/status", ruHandler.UpdateCellStatus) // Обновить статус ячейки
			rus.POST("/:id/history", ruHandler.AddHistory)                   // Добавить запись в историю
			rus.PATCH("/:id/cells/:cellId/info", ruHandler.UpdateCellInfo)   // Обновить информацию ячейки
			rus.PUT("/:id/status", ruHandler.UpdateRuStatus)                 // Обновить статус РУ

			// Обновление РУ на подстанции - доступно всем авторизованным
			rus.PUT("/substations/:id/rus", ruHandler.UpdateSubstationRUs)
		}

		// Admin routes - только для админов
		admin := protected.Group("/admin")
		admin.Use(middleware.RoleMiddleware("admin"))
		{
			admin.GET("/users", adminHandler.GetUsers)
			admin.POST("/users", adminHandler.CreateUser)
			admin.PUT("/users/:id", adminHandler.UpdateUser)
			admin.DELETE("/users/:id", adminHandler.DeleteUser)
			admin.PUT("/users/:id/password", adminHandler.ChangePassword)

			// Административные операции с РУ
			admin.POST("/rus", adminRuHandler.CreateRU)
			admin.POST("/rus/:id/cells", adminRuHandler.CreateCells)
		}

		// Engineer routes
		engineer := protected.Group("/engineer")
		engineer.Use(middleware.RoleMiddleware("engineer", "admin"))
		{
			engineer.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Engineer access granted",
					"user":    c.GetString("user_email"),
					"role":    c.GetString("user_role"),
				})
			})
		}

		// Dispatcher routes
		dispatcher := protected.Group("/dispatcher")
		dispatcher.Use(middleware.RoleMiddleware("dispatcher", "engineer", "admin"))
		{
			dispatcher.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Dispatcher access granted",
					"user":    c.GetString("user_email"),
					"role":    c.GetString("user_role"),
				})
			})
		}
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		var dbStatus string
		sqlDB, err := db.DB()
		if err != nil {
			dbStatus = "error_getting_db"
		} else {
			err = sqlDB.Ping()
			if err != nil {
				dbStatus = "disconnected"
			} else {
				dbStatus = "connected"
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      "ok",
			"service":     "service-desk-api",
			"version":     "1.0.0",
			"database":    dbStatus,
			"environment": getEnv("GIN_MODE", "debug"),
		})
	})

	// Root endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Service Desk API",
			"version": "1.0.0",
			"endpoints": gin.H{
				"auth": gin.H{
					"POST /api/auth/register": "Register new user",
					"POST /api/auth/login":    "Login user",
				},
				"public": gin.H{
					"GET /api/substations/:id": "Get substation info (public)",
				},
				"rus": gin.H{
					"GET  /api/rus":                          "Get all RUs",
					"GET  /api/rus/:id":                      "Get RU by ID",
					"GET  /api/rus/:id/history":              "Get operation history",
					"PUT  /api/rus/:id/cells/:cellId/status": "Update cell status",
					"POST /api/rus/:id/history":              "Add history record",
					"PUT  /api/rus/substations/:id/rus":      "Update RUs on substation",
				},
				"admin": gin.H{
					"GET    /api/admin/users":         "Get all users",
					"POST   /api/admin/users":         "Create user",
					"PUT    /api/admin/users/:id":     "Update user",
					"DELETE /api/admin/users/:id":     "Delete user",
					"POST   /api/admin/rus":           "Create RU",
					"POST   /api/admin/rus/:id/cells": "Create cells",
				},
			},
		})
	})

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested endpoint does not exist",
			"path":    c.Request.URL.Path,
		})
	})

	log.Printf("\n🚀 Server starting on http://localhost%s", cfg.ServerPort)
	log.Println("📋 Available endpoints:")
	log.Println("")
	log.Println("    🔓 Public endpoints:")
	log.Println("        GET  /api/substations/:id              - Get substation info (public)")
	log.Println("        POST /api/auth/register                - Register user")
	log.Println("        POST /api/auth/login                   - Login user")
	log.Println("        GET  /health                           - Health check")
	log.Println("")
	log.Println("    🔐 Protected endpoints (require JWT):")
	log.Println("        GET  /api/auth/me                      - Get current user")
	log.Println("        GET  /api/rus                          - Get all RUs")
	log.Println("        GET  /api/rus/:id                      - Get RU by ID")
	log.Println("        GET  /api/rus/:id/history              - Get history")
	log.Println("        PUT  /api/rus/:id/cells/:cellId/status - Update cell status")
	log.Println("        POST /api/rus/:id/history              - Add history record")
	log.Println("        PUT  /api/rus/substations/:id/rus      - Update RUs on substation")
	log.Println("")
	log.Println("    👑 Admin endpoints:")
	log.Println("        GET    /api/admin/users                - Get all users")
	log.Println("        POST   /api/admin/users                - Create user")
	log.Println("        PUT    /api/admin/users/:id            - Update user")
	log.Println("        DELETE /api/admin/users/:id            - Delete user")
	log.Println("        POST   /api/admin/rus                  - Create RU")
	log.Println("        POST   /api/admin/rus/:id/cells        - Create cells")
	log.Println("")

	// Запускаем сервер
	if err := router.Run(cfg.ServerPort); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func checkAndSeedTestData(db *gorm.DB) {
	// Проверяем существование тестового пользователя админа
	var adminCount int64
	db.Model(&models.User{}).Where("email = ?", "admin@sez.com").Count(&adminCount)

	if adminCount == 0 {
		log.Println("📝 Creating test admin user...")

		// Создаем тестового админа
		admin := &models.User{
			ID:           "admin-001",
			Name:         "Администратор",
			Email:        "admin@sez.com",
			PasswordHash: "$2a$12$L2JMvBJDsz5JKmpSFcmweOZiioqbeUxrTVW9v71QyQWKyj3DwclF6", // 123456
			Role:         models.RoleAdmin,
		}

		if err := db.Create(admin).Error; err != nil {
			log.Printf("⚠️ Failed to create admin user: %v", err)
		} else {
			log.Println("✅ Test admin user created")
		}
	}
	// ================== ТП-1Л ==================
	createTP1L(db)
	// ================== ТП-1И ==================
	createTP1I(db)
	// ================== ТП-2И ==================
	createTP2I(db)
	// ================== ТП-2Л ==================
	createTP2L(db)
	// ================== ТП-3И ==================
	createTP3I(db)
	// ================== ТП-4И ==================
	createTP4I(db)
	// ================== ТП-5И ==================
	createTP5I(db)
	// ================== ТП-Общежитие ==================
	createTPObshyaga(db)
	// ================== ТП-Очистные ==================
	createTPOchistnye(db)
	// ================== ТП-Общежитие ==================
	createTPVodazabor(db)
	// ================== ТП-Общежитие ==================
	createTPRazvyazka(db)

	// ================== КРУ-БМ-1И ==================
	createKRU_BM_1I(db)

	// ================== КРУ-БМ-2И ==================
	createKRU_BM_2I(db)

	// ================== КРУ-БМ-3И ==================
	createKRU_BM_3I(db)

	// ================== КРУ-БМ-4И ==================
	createKRU_BM_4I(db)

	// ================== КРУ-БМ-5И ==================
	createKRU_BM_5I(db)
	// ================== КРУ-БМ-1Л ==================
	createKRU_BM_1L(db)

	log.Println("🎉 Test data check completed!")
}
func createTP1I(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-1i").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-1И...")

		tp4i := models.RUInfo{
			ID:               "tp-1i",
			Name:             "ТП-1И",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       12,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-1И: %v", err)
			return
		}
		log.Println("✅ ТП-1И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP1ICells()
		createCells(db, cells, "ТП-1И")
	} else {
		log.Printf("✅ ТП-1И уже существует")
	}
}
func createTP1L(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-1l").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-1Л...")

		tp4i := models.RUInfo{
			ID:               "tp-1l",
			Name:             "ТП-1Л",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       10,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-1Л: %v", err)
			return
		}
		log.Println("✅ ТП-4И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP1LCells()
		createCells(db, cells, "ТП-1Л")
	} else {
		log.Printf("✅ ТП-1Л уже существует")
	}
}
func createTP2I(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-2i").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-2И...")

		tp4i := models.RUInfo{
			ID:               "tp-2i",
			Name:             "ТП-2И",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       8,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-2И: %v", err)
			return
		}
		log.Println("✅ ТП-2И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP2ICells()
		createCells(db, cells, "ТП-2И")
	} else {
		log.Printf("✅ ТП-2И уже существует")
	}
}
func createTP2L(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-2l").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-2Л...")

		tp4i := models.RUInfo{
			ID:               "tp-2l",
			Name:             "ТП-2Л",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       8,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-2Л: %v", err)
			return
		}
		log.Println("✅ ТП-2Л created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP2LCells()
		createCells(db, cells, "ТП-2Л")
	} else {
		log.Printf("✅ ТП-2Л уже существует")
	}
}
func createTP3I(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-3i").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-3И...")

		tp4i := models.RUInfo{
			ID:               "tp-3i",
			Name:             "ТП-3И",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       6,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-3И: %v", err)
			return
		}
		log.Println("✅ ТП-3И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP3ICells()
		createCells(db, cells, "ТП-3И")
	} else {
		log.Printf("✅ ТП-3И уже существует")
	}
}
func createTP4I(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-4i").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-4И...")

		tp4i := models.RUInfo{
			ID:               "tp-4i",
			Name:             "ТП-4И",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       8,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-64",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-4И: %v", err)
			return
		}
		log.Println("✅ ТП-4И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP4ICells()
		createCells(db, cells, "ТП-4И")
	} else {
		log.Printf("✅ ТП-4И уже существует")
	}
}

func createTP5I(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-5i").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-5И...")

		tp4i := models.RUInfo{
			ID:               "tp-5i",
			Name:             "ТП-5И",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       8,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-64",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-5И: %v", err)
			return
		}
		log.Println("✅ ТП-5И created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTP5ICells()
		createCells(db, cells, "ТП-5И")
	} else {
		log.Printf("✅ ТП-5И уже существует")
	}
}
func createTPObshyaga(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-obshyaga").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-Общежитие...")

		tp4i := models.RUInfo{
			ID:               "tp-obshyaga",
			Name:             "ТП-Общежитие",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       8,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-Общежитие: %v", err)
			return
		}
		log.Println("✅ ТП-Общежитие created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTPObshyagaCells()
		createCells(db, cells, "ТП-Общежитие")
	} else {
		log.Printf("✅ ТП-Общежитие уже существует")
	}
}
func createTPOchistnye(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-ochistnye").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-Очистные...")

		tp4i := models.RUInfo{
			ID:               "tp-ochistnye",
			Name:             "ТП-Очистные",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       5,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-Очистные: %v", err)
			return
		}
		log.Println("✅ ТП-Очистные created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTPOchistnyeCells()
		createCells(db, cells, "ТП-Очистные")
	} else {
		log.Printf("✅ ТП-Очистные уже существует")
	}
}
func createTPVodazabor(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-vodazabor").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-Водазабор...")

		tp4i := models.RUInfo{
			ID:               "tp-vodazabor",
			Name:             "ТП-Водазабор",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       5,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-Водазабор: %v", err)
			return
		}
		log.Println("✅ ТП-Водазабор created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTPVodazaborCells()
		createCells(db, cells, "ТП-Водазабор")
	} else {
		log.Printf("✅ ТП-Водазабор уже существует")
	}
}
func createTPRazvyazka(db *gorm.DB) {
	var tp4iCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "tp-razvyazka").Count(&tp4iCount)

	if tp4iCount == 0 {
		log.Println("📝 Creating ТП-Развязка...")

		tp4i := models.RUInfo{
			ID:               "tp-razvyazka",
			Name:             "ТП-Развязка",
			Voltage:          "10/0,4 кВ",
			Sections:         2,
			CellsCount:       2,
			Transformers:     2,
			TransformerPower: "2 × 100 кВА",
			Location:         "Промзона Хоргос",
			InstallationDate: "2021-08-10",
			Manufacturer:     "Энерготехника",
			LastMaintenance:  "2024-02-15",
			NextMaintenance:  "2024-08-15",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин с секционированием",
			TotalLoadHigh:    "430 А",
			TotalLoadLow:     "635 А",
			TotalPowerHigh:   "430 кВА",
			TotalPowerLow:    "250 кВт",
			MaxCapacityHigh:  "630 А",
			MaxCapacityLow:   "800 А",
			OperationalHours: 21500,
			LastInspection:   "2024-02-20",
			Type:             models.TypeTP,
			HasHighSide:      true,
			HasLowSide:       true,
			BusSections:      2,
			CellsPerSection:  9,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&tp4i).Error; err != nil {
			log.Printf("⚠️ Failed to create ТП-Развязка: %v", err)
			return
		}
		log.Println("✅ ТП-Развязка created")

		// Ячейки для ТП-4И (без изменений)
		cells := createTPRazvyazkaCells()
		createCells(db, cells, "ТП-Развязка")
	} else {
		log.Printf("✅ ТП-Развязка уже существует")
	}
}
func createKRU_BM_1L(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-1l").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-1Л...")

		kru := models.RUInfo{
			ID:               "kru-bm-1l",
			Name:             "КРУ-БМ-1Л",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСН 63 кВА",
			Location:         "Микрорайон №8",
			InstallationDate: "2020-05-15",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-01-20",
			NextMaintenance:  "2024-07-20",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "850 А",
			TotalPowerHigh:   "850 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 32000,
			LastInspection:   "2024-01-25",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-1Л: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-1Л created")

		// Ячейки для КРУ-БМ-1И
		cells := createKRUBM1LCells()
		createCells(db, cells, "КРУ-БМ-1Л")
	} else {
		log.Printf("✅ КРУ-БМ-1Л уже существует")
	}
}
func createKRU_BM_1I(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-1i").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-1И...")

		kru := models.RUInfo{
			ID:               "kru-bm-1i",
			Name:             "КРУ-БМ-1И",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСН 63 кВА",
			Location:         "Микрорайон №8",
			InstallationDate: "2020-05-15",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-01-20",
			NextMaintenance:  "2024-07-20",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "850 А",
			TotalPowerHigh:   "850 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 32000,
			LastInspection:   "2024-01-25",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-1И: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-1И created")

		// Ячейки для КРУ-БМ-1И
		cells := createKRUBM1ICells()
		createCells(db, cells, "КРУ-БМ-1И")
	} else {
		log.Printf("✅ КРУ-БМ-1И уже существует")
	}
}

func createKRU_BM_2I(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-2i").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-2И...")

		kru := models.RUInfo{
			ID:               "kru-bm-2i",
			Name:             "КРУ-БМ-2И",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСП",
			Location:         "Капитальная станция 1",
			InstallationDate: "2020-06-20",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-02-10",
			NextMaintenance:  "2024-08-10",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "780 А",
			TotalPowerHigh:   "780 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 31000,
			LastInspection:   "2024-02-15",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-164",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-2И: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-2И created")

		// Ячейки для КРУ-БМ-2И
		cells := createKRUBM2ICells()
		createCells(db, cells, "КРУ-БМ-2И")
	} else {
		log.Printf("✅ КРУ-БМ-2И уже существует")
	}
}

func createKRU_BM_3I(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-3i").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-3И...")

		kru := models.RUInfo{
			ID:               "kru-bm-3i",
			Name:             "КРУ-БМ-3И",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСП",
			Location:         "Микрорайон №9",
			InstallationDate: "2020-07-10",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-03-05",
			NextMaintenance:  "2024-09-05",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "720 А",
			TotalPowerHigh:   "720 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 29000,
			LastInspection:   "2024-03-10",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-64",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-3И: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-3И created")

		// Ячейки для КРУ-БМ-3И (аналогично 2И, с небольшими отличиями)
		cells := createKRUBM3ICells()
		createCells(db, cells, "КРУ-БМ-3И")
	} else {
		log.Printf("✅ КРУ-БМ-3И уже существует")
	}
}

func createKRU_BM_4I(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-4i").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-4И...")

		kru := models.RUInfo{
			ID:               "kru-bm-4i",
			Name:             "КРУ-БМ-4И",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСН",
			Location:         "Промзона Хоргос",
			InstallationDate: "2020-08-25",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-03-20",
			NextMaintenance:  "2024-09-20",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "690 А",
			TotalPowerHigh:   "690 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 28000,
			LastInspection:   "2024-03-25",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-64",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-4И: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-4И created")

		// Ячейки для КРУ-БМ-4И (аналогично 1И, с небольшими отличиями)
		cells := createKRUBM4ICells()
		createCells(db, cells, "КРУ-БМ-4И")
	} else {
		log.Printf("✅ КРУ-БМ-4И уже существует")
	}
}

func createKRU_BM_5I(db *gorm.DB) {
	var kruCount int64
	db.Model(&models.RUInfo{}).Where("id = ?", "kru-bm-5i").Count(&kruCount)

	if kruCount == 0 {
		log.Println("📝 Creating КРУ-БМ-5И...")

		kru := models.RUInfo{
			ID:               "kru-bm-5i",
			Name:             "КРУ-БМ-5И",
			Voltage:          "10 кВ",
			Sections:         2,
			CellsCount:       16,
			Transformers:     2,
			TransformerPower: "2 × ТСП",
			Location:         "Капитальная станция 2",
			InstallationDate: "2020-09-30",
			Manufacturer:     "Электроаппарат",
			LastMaintenance:  "2024-04-05",
			NextMaintenance:  "2024-10-05",
			Status:           "Работает в штатном режиме",
			SchemeType:       "Две секции шин, 16 ячеек",
			TotalLoadHigh:    "810 А",
			TotalPowerHigh:   "810 кВА",
			MaxCapacityHigh:  "1000 А",
			OperationalHours: 30000,
			LastInspection:   "2024-04-10",
			Type:             models.TypeKRU,
			HasHighSide:      true,
			HasLowSide:       false,
			BusSections:      2,
			CellsPerSection:  8,
			SubstationID:     "ps-64",
		}

		if err := db.Create(&kru).Error; err != nil {
			log.Printf("⚠️ Failed to create КРУ-БМ-5И: %v", err)
			return
		}
		log.Println("✅ КРУ-БМ-5И created")

		// Ячейки для КРУ-БМ-5И (аналогично 2И, с небольшими отличиями)
		cells := createKRUBM5ICells()
		createCells(db, cells, "КРУ-БМ-5И")
	} else {
		log.Printf("✅ КРУ-БМ-5И уже существует")
	}
}

func createCells(db *gorm.DB, cells []models.Cell, ruName string) {
	createdCount := 0
	for i := range cells {
		if err := db.Create(&cells[i]).Error; err != nil {
			log.Printf("⚠️ Failed to create cell %s in %s: %v", cells[i].Number, ruName, err)
		} else {
			createdCount++
		}
	}
	log.Printf("✅ Created %d test cells for %s", createdCount, ruName)
}

// Функции создания ячеек для каждого РУ

func createTP1ICells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.11", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.9", Name: "ТП-2И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.7", Name: "ТП-3И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.5", Name: "КРУ-БМ-1И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		// {INumber: "В10-3", Name: "Резерв 10кВ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		// {Number: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.12", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "В10-7", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.10", Name: "ТП-2И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.8", Name: "ТП-3И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.6", Name: "КРУ-БМ-1И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		// {Number: "В10-7", Name: "Резерв 10кВ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		// {Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-1i"},
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-1i"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.11", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.9", Name: "ТП-2И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.7", Name: "ТП-3И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},
		{Number: "яч.5", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.12", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.10", Name: "ТП-2И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.8", Name: "ТП-3И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.6", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1i"},
	}
}
func createTP1LCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.9", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.7", Name: "ТП-2Л", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.5", Name: "КРУ-БМ-1И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		// {umber: "В10-3", Name: "Резерв 10кВ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		// {umber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.10", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "В10-7", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.8", Name: "ТП-2Л", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.6", Name: "КРУ-БМ-1И", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		// {Number: "В10-7", Name: "Резерв 10кВ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		// {Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-1l"},
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-1l"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.9", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.7", Name: "ТП-2Л", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.5", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-1l"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.10", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.8", Name: "ТП-2Л", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.6", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-1l"},
	}
}

func createTP2ICells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.7", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "яч.5", Name: "КРУ-БМ-1И ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		// {umber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.8", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "яч.6", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		// {Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-2i"},
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-2i"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "яч.7", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "яч.5", Name: "КРУ-БМ-1И", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},
		// {Number: "Н04-4", Name: "Фидер 2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "яч.8", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "яч.6", Name: "КРУ-БМ-1И ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
		// {Number: "Н04-8", Name: "Фидер 4", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
	}
}
func createTP2LCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "яч.2", Name: "Очистные сооружения", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		// {umber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.8", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "яч.7", Name: "Очистные сооружения", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		// {Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.4", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-2l"},
		{Number: "яч.5", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-2l"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "яч.1", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "яч.2", Name: "Очистные сооружения", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2l"},
		// {Number: "Н04-4", Name: "Фидер 2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-2i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "яч.8", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "яч.7", Name: "Очистные сооружения", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2l"},
		// {Number: "Н04-8", Name: "Фидер 4", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-2i"},
	}
}

func createTP3ICells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: " ", Name: "ТОО КИФ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.1 ", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.6", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-3i"},
		{Number: "яч.5", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
		// {umber: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.4", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-3i"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: " ", Name: "ТОО КИФ", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.1", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},
		// {ID: 12, Number: "Н04-4", Name: "Фидер 2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-3i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-3i"},
		{Number: "яч.8", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
		{Number: "яч.5", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
		// {Number: "Н04-8", Name: "Фидер 4", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-3i"},
	}
}
func createTP4ICells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "яч.2", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		// {umber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.8", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		// {Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.4", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-4i"},
		{Number: "яч.5", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-4i"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "яч.2", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "яч.8", Name: "Ввод-0,4 кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},
	}
}
func createTP5ICells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "яч.2", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.8", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
		// {umber: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.4", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-5i"},
		{Number: "яч.5", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-5i"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "яч.1", Name: "Ввод-0,4кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "яч.2", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"40 кВт"}[0], Current: &[]float64{58}[0], Temperature: &[]float64{35}[0], Load: &[]float64{55}[0], Description: "Выходной фидер №2", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-5i"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "яч.8", Name: "Ввод-0,4кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
		{Number: "яч.6", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"25 кВт"}[0], Current: &[]float64{36}[0], Temperature: &[]float64{34}[0], Load: &[]float64{45}[0], Description: "Выходной фидер №4", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-5i"},
	}
}
func createTPObshyagaCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.7", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.8", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		// {umber: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-obshyaga"},
		// {umber: "СВ-10", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-obshyaga"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "яч.7", Name: "Ввод-0,4 кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},
		{Number: "яч.3", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-obshyaga"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.8", Name: "Ввод-0,4кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-obshyaga"},
	}
}
func createTPOchistnyeCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.5", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
		// {umber: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.3", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-ochistnye"},
		// {umber: "СВ-10", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-ochistnye"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},
		{Number: "яч.1", Name: "Ввод-0,4 кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-ochistnye"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
		{Number: "яч.5", Name: "Ввод-0,4кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-ochistnye"},
	}
}

func createTPVodazaborCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.1", Name: "Ввод-10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},
		{Number: "В10-2", Name: "Т-1 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{25}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.5", Name: "Ввод-10 кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
		{Number: "В10-6", Name: "Т-2 Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{88}[0], Temperature: &[]float64{62}[0], Load: &[]float64{80}[0], Description: "Трансформатор №2 100 кВА, секция 2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{26}[0], Load: &[]float64{0}[0], Description: "Резервная ячейка 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
		// {umber: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		{Number: "яч.3", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{0}[0], Temperature: &[]float64{28}[0], Load: &[]float64{0}[0], Description: "Секционный разъединитель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-vodazabor"},
		// {umber: "СВ-10", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-vodazabor"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Т-1 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},
		{Number: "яч.1", Name: "Ввод-0,4 кВ №1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},
		{Number: "яч.2", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"50 кВт"}[0], Current: &[]float64{72}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Выходной фидер №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-vodazabor"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Т-2 Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
		{Number: "яч.5", Name: "Ввод-0,4кВ №2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
		{Number: "яч.4", Name: " ", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"30 кВт"}[0], Current: &[]float64{43}[0], Temperature: &[]float64{36}[0], Load: &[]float64{50}[0], Description: "Выходной фидер №3", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-vodazabor"},
	}
}
func createTPRazvyazkaCells() []models.Cell {
	return []models.Cell{
		// Высокая сторона - секция 1
		{Number: "яч.2", Name: " ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{150}[0], Temperature: &[]float64{35}[0], Load: &[]float64{75}[0], Description: "Входное питание 10 кВ, секция 1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-razvyazka"},
		{Number: "В10-2", Name: "Тр-р Выс. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"100 кВА"}[0], Current: &[]float64{95}[0], Temperature: &[]float64{65}[0], Load: &[]float64{85}[0], Description: "Трансформатор №1 100 кВА, секция 1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-razvyazka"},
		// {mber: "В10-4", Name: "СШ 10кВ-1", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{245}[0], Temperature: &[]float64{45}[0], Load: &[]float64{80}[0], Description: "Секция шин 10 кВ №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-4i"},

		// Высокая сторона - секция 2
		{Number: "яч.1", Name: "Ввод-10 кВ", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{145}[0], Temperature: &[]float64{32}[0], Load: &[]float64{72}[0], Description: "Входное питание 10 кВ, секция 2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-razvyazka"},
		// {ID: 18, Number: "В10-8", Name: "СШ 10кВ-2", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{233}[0], Temperature: &[]float64{43}[0], Load: &[]float64{78}[0], Description: "Секция шин 10 кВ №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-4i"},

		// Секционные аппараты
		// {ID: 91, Number: "СВ-10", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{50}[0], Temperature: &[]float64{40}[0], Load: &[]float64{25}[0], Description: "Секционный выключатель", IsGrounded: false, BusSection: &[]int{0}[0], RuID: "tp-razvyazka"},

		// Низкая сторона - секция 1
		{Number: "Н04-1", Name: "Тр-р Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{140}[0], Temperature: &[]float64{45}[0], Load: &[]float64{85}[0], Description: "Низковольтная сторона Трансформатора №1", IsGrounded: false, TransformerNumber: &[]string{"Т-1"}[0], BusSection: &[]int{1}[0], RuID: "tp-razvyazka"},
		{Number: "яч.2", Name: "", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{215}[0], Temperature: &[]float64{40}[0], Load: &[]float64{85}[0], Description: "Низковольтная секция шин №1", IsGrounded: false, BusSection: &[]int{1}[0], RuID: "tp-razvyazka"},

		// Низкая сторона - секция 2
		{Number: "Н04-5", Name: "Тр-р Низ. сторона", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Power: &[]string{"100 кВА"}[0], Current: &[]float64{130}[0], Temperature: &[]float64{42}[0], Load: &[]float64{80}[0], Description: "Низковольтная сторона Трансформатора №2", IsGrounded: false, TransformerNumber: &[]string{"Т-2"}[0], BusSection: &[]int{2}[0], RuID: "tp-razvyazka"},
		{Number: "яч.1", Name: "Ввод-0,4кВ", Type: models.CellTypeBus, Status: models.CellStatusON, Voltage: "0,4 кВ", VoltageLevel: "LOW", Current: &[]float64{188}[0], Temperature: &[]float64{38}[0], Load: &[]float64{75}[0], Description: "Низковольтная секция шин №2", IsGrounded: false, BusSection: &[]int{2}[0], RuID: "tp-razvyazka"},
	}
}
func createKRUBM1LCells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Ввод 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.12", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.3", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1l"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-1l"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		// {Number: "№15", Name: "ТСН №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
		{Number: "яч.16", Name: "Ввод 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1l"},
	}
}
func createKRUBM1ICells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Ввод 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.11", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.3", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-1i"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-1i"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		// {Number: "№15", Name: "ТСН №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
		{Number: "яч.16", Name: "Ввод 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-1i"},
	}
}

func createKRUBM2ICells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Ввод 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.11", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.3", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-2i"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-2i"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		// {Number: "№15", Name: "ТСН №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
		{Number: "яч.16", Name: "Ввод 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-2i"},
	}
}

func createKRUBM3ICells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Ввод 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.11", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.3", Name: "ТП-4И", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-3i"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-3i"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "ТП-4И", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		// {Number: "№15", Name: "ТСН №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
		{Number: "яч.16", Name: "Ввод 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-3i"},
	}
}

func createKRUBM4ICells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Ввод 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.11", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.3", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-4i"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-4i"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		// {Number: "№15", Name: "ТСН №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
		{Number: "яч.16", Name: "Ввод 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-4i"},
	}
}

func createKRUBM5ICells() []models.Cell {
	return []models.Cell{
		// Секция 1 (ячейки 1-8)
		{Number: "яч.15", Name: "Вход 10 кВ №1", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{120}[0], Temperature: &[]float64{38}[0], Load: &[]float64{60}[0], Description: "Входное питание 10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		// {Number: "№2", Name: "ТСН №1", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{55}[0], Temperature: &[]float64{52}[0], Load: &[]float64{45}[0], Description: "Трансформатор собственных нужд №1", TransformerNumber: &[]string{"ТСН-1"}[0], BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.13", Name: "ТСН №1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.11", Name: "ТН-10 кВ СШ-1", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{230}[0], Temperature: &[]float64{42}[0], Load: &[]float64{75}[0], Description: "Отходящая линия на ТП-10 кВ, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.9", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.7", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.5", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.3", Name: "ТП-4И", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},
		{Number: "яч.1", Name: "СР-10кВ", Type: models.CellTypeSR, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Секционный разъединитель, секция 1", BusSection: &[]int{1}[0], RuID: "kru-bm-5i"},

		// Секционные аппараты (ячейка 9)
		{Number: "яч.2", Name: "СВ-10кВ", Type: models.CellTypeSV, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{65}[0], Temperature: &[]float64{41}[0], Load: &[]float64{30}[0], Description: "Секционный выключатель", BusSection: &[]int{0}[0], RuID: "kru-bm-5i"},

		// Секция 2 (ячейки 10-16)
		{Number: "яч.4", Name: "ТП-4И", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.6", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.8", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.10", Name: "Резерв", Type: models.CellTypeOutput, Status: models.CellStatusOFF, Voltage: "10 кВ", VoltageLevel: "HIGH", Description: "Резервная ячейка, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.12", Name: "ТН-10кВ СШ-2", Type: models.CellTypeOutput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"400 кВА"}[0], Current: &[]float64{225}[0], Temperature: &[]float64{43}[0], Load: &[]float64{73}[0], Description: "Отходящая линия на ТП-10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		// {Number: "№15", Name: "ТСН, №2", Type: models.CellTypeTransformer, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Power: &[]string{"ТСН 63 кВА"}[0], Current: &[]float64{52}[0], Temperature: &[]float64{51}[0], Load: &[]float64{43}[0], Description: "Трансформатор собственных нужд №2", TransformerNumber: &[]string{"ТСН-2"}[0], BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.14", Name: "ТСН №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
		{Number: "яч.16", Name: "Ввод, 10кВ №2", Type: models.CellTypeInput, Status: models.CellStatusON, Voltage: "10 кВ", VoltageLevel: "HIGH", Current: &[]float64{115}[0], Temperature: &[]float64{37}[0], Load: &[]float64{58}[0], Description: "Входное питание 10 кВ, секция 2", BusSection: &[]int{2}[0], RuID: "kru-bm-5i"},
	}
}
