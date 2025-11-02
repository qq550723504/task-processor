# TEMU Web Application - Refactored Structure

This application has been refactored to follow Go best practices and improve maintainability.

## Structure

```
cmd/temu-web/
├── main.go              # Application bootstrap (minimal)
├── server/
│   └── server.go        # Server struct and core logic
├── handlers/
│   └── handlers.go      # HTTP request handlers
├── middleware/
│   └── auth.go          # Authentication middleware
├── utils/
│   └── utils.go         # Utility functions
└── templates/           # HTML templates
```

## Key Improvements

1. **Separation of Concerns**: Code is organized into logical packages
2. **Dependency Injection**: No more global variables, dependencies are injected
3. **Interface-based Design**: ProcessorManager interface for better testability
4. **Clean Architecture**: Clear separation between HTTP layer, business logic, and utilities
5. **Better Error Handling**: Consistent error handling throughout the application
6. **Middleware Support**: Authentication middleware for protected routes

## Components

### Server Package
- `Server` struct holds all application dependencies
- Implements `ProcessorManager` interface for task processor operations
- Manages the HTTP server lifecycle

### Handlers Package
- Contains all HTTP request handlers
- Uses dependency injection for clean testing
- Implements proper JSON responses and error handling

### Middleware Package
- Authentication middleware for protected routes
- Logging middleware for request tracking

### Utils Package
- Logger setup and configuration
- Working directory management
- Environment information logging

## Usage

The refactored application maintains the same API endpoints and functionality:

- `GET /` - Login page
- `POST /api/login` - User authentication
- `POST /api/logout` - User logout
- `GET /dashboard` - Dashboard (requires authentication)
- `POST /api/start-processor` - Start task processor
- `POST /api/stop-processor` - Stop task processor
- `GET /api/processor-status` - Get processor status

## Benefits

1. **Testability**: Each component can be tested independently
2. **Maintainability**: Clear separation makes code easier to understand and modify
3. **Scalability**: New features can be added without affecting existing code
4. **Reusability**: Components can be reused in other applications
5. **Type Safety**: Interface-based design provides better type safety