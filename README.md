# Consultout Backend

## Installation

### Prerequisites
- Go 1.19 or higher
- PostgreSQL 12+
- Git

### Steps

1. Clone the repository
```bash
git clone <repository-url>
cd consultout-backend
```

2. Install dependencies
```bash
go mod download
```

3. Configure environment variables (see below)

4. Run migrations
```bash
go run main.go migrate
```

4.1. Run migrations with make (look at Makefile)

5. Start the application
```bash
go run main.go
```

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=consultout
DB_ADD=your_dsn
REDIS_ADDR=your_redis_URL

# Server
SERVER_PORT=8080
SERVER_ENV=development

# JWT
JWT_SECRET=your_jwt_secret_key

# API Keys
API_KEY=your_api_key

# Mail (optional)
MAIL_SMTP_HOST=smtp.gmail.com
MAIL_SMTP_PORT=587
MAIL_FROM=your_email@gmail.com
MAIL_PASSWORD=your_app_password
```

### Variable Descriptions
- **DB_HOST**: PostgreSQL host address
- **DB_PORT**: PostgreSQL port
- **DB_USER**: Database user
- **DB_PASSWORD**: Database password
- **DB_NAME**: Database name
- **SERVER_PORT**: Application port
- **JWT_SECRET**: Secret key for JWT token generation
- **API_KEY**: API authentication key
