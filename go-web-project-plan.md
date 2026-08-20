# Go Web Project — Architecture and Implementation Plan

## 1. Project Overview

This project is a simple informational website built with Go.

The initial version should focus on a clean, maintainable foundation rather than implementing unnecessary abstractions. The architecture should be easy to extend later with additional pages, business logic, APIs, authentication, a database, and other functionality.

### Initial goals

- Build a minimal Go HTTP server.
- Render HTML pages using Go templates.
- Serve static CSS, JavaScript, and image files.
- Separate HTTP handlers from server setup.
- Keep configuration isolated from application logic.
- Establish a directory structure that can grow with the project.
- Keep all project-facing text, comments, documentation, and messages in English.

---

## 2. Recommended Directory Structure

```text
my-go-site/
├── cmd/
│   └── web/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── handlers/
│   │   └── home.go
│   │
│   └── server/
│       └── server.go
│
├── templates/
│   ├── layouts/
│   │   └── base.html
│   │
│   └── pages/
│       └── home.html
│
├── static/
│   ├── css/
│   │   └── style.css
│   │
│   ├── js/
│   │   └── app.js
│   │
│   └── images/
│
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

---

## 3. Directory Responsibilities

### `cmd/web/`

Contains the executable entry point for the web application.

`main.go` should be responsible for:

1. Loading configuration.
2. Creating the server.
3. Starting the HTTP server.
4. Handling startup errors.
5. Gracefully shutting down the server when appropriate.

Application-specific business logic should not be placed directly in `main.go`.

---

### `internal/config/`

Contains application configuration.

The configuration layer should eventually support values such as:

- HTTP port.
- Application environment.
- Future database configuration.
- Future external service configuration.
- Other environment-dependent settings.

For the initial version, configuration can remain intentionally small.

Example conceptual configuration:

```text
Config
├── Port
└── Environment
```

Environment variables should be preferred for values that may change between development, testing, and production.

---

### `internal/server/`

Contains HTTP server setup.

Responsibilities:

- Create the HTTP server.
- Register routes.
- Configure static file serving.
- Configure basic middleware when needed.
- Connect handlers to routes.
- Define server timeouts.

The server package should not contain page-specific business logic.

---

### `internal/handlers/`

Contains HTTP request handlers.

The first handler should be responsible for rendering the home page.

Future handlers may include:

```text
home.go
about.go
contact.go
api.go
```

Handlers should:

1. Receive an HTTP request.
2. Validate relevant request data.
3. Perform or delegate application logic.
4. Render a template or return an HTTP response.
5. Handle errors appropriately.

As the application grows, business logic should move into service packages instead of making handlers too large.

---

## 4. Templates

### `templates/layouts/base.html`

This is the common HTML layout.

It should contain shared elements such as:

- HTML document structure.
- `<head>`.
- Meta tags.
- Page title handling.
- CSS references.
- JavaScript references.
- Header.
- Navigation.
- Main content placeholder.
- Footer.

The layout should use Go template blocks so individual pages can provide their content.

Conceptually:

```text
Base Layout
├── Head
├── Header
├── Navigation
├── Main
│   └── Page-specific content
└── Footer
```

---

### `templates/pages/`

Contains individual page templates.

The initial page:

```text
home.html
```

Future examples:

```text
about.html
contact.html
services.html
faq.html
```

Pages should contain page-specific markup and should reuse the shared base layout.

---

## 5. Static Files

### `static/css/`

Contains stylesheets.

Initial file:

```text
style.css
```

Keep the first stylesheet simple. Do not introduce a CSS framework unless the project actually requires one.

---

### `static/js/`

Contains browser-side JavaScript.

Initial file:

```text
app.js
```

It can remain almost empty initially. It exists so client-side functionality can be added later without changing the project structure.

---

### `static/images/`

Contains images used by the website.

Examples:

- Logos.
- Icons.
- Page images.
- Background assets.

Large or generated assets should not be committed unnecessarily.

---

## 6. Routing

The initial application should have a minimal route set.

Recommended starting routes:

```text
GET /
```

The root route should render the home page.

Later, routes can be added for:

```text
GET /about
GET /contact
GET /services
GET /api/...
```

The routing layer should remain centralized in the server setup so that it is easy to see which endpoints the application exposes.

---

## 7. HTTP Server

The application should use Go's standard `net/http` package initially.

There is no need to introduce a web framework at the beginning.

Recommended principles:

- Use `http.Server`.
- Configure explicit read/write/idle timeouts.
- Keep route registration separate from `main.go`.
- Return appropriate HTTP status codes.
- Handle server startup errors.
- Support graceful shutdown.

This keeps the project lightweight and makes the behavior of the application easy to understand.

---

## 8. Template Rendering

Templates should be parsed during application initialization rather than reparsed on every request.

A future template renderer can be responsible for:

1. Loading the base layout.
2. Loading page templates.
3. Passing structured data to templates.
4. Executing the appropriate template.
5. Returning rendering errors.

Template data should be represented using Go structs when the amount of data becomes significant.

For example:

```text
HomePageData
├── Title
├── Description
└── Content
```

Avoid passing unstructured maps everywhere once the application becomes larger.

---

## 9. Error Handling

The project should use explicit HTTP error handling.

Examples:

```text
404 Not Found
500 Internal Server Error
400 Bad Request
```

Error messages shown to users should be clear but should not expose internal implementation details.

For example, users should not see:

```text
database connection string
filesystem paths
stack traces
internal service names
```

Detailed errors should be logged on the server side.

---

## 10. Logging

Use Go's standard logging facilities initially.

The application should log important operational events such as:

- Server startup.
- Server shutdown.
- Configuration errors.
- Template loading errors.
- Unexpected request processing errors.

Avoid excessive logging of normal successful requests unless request logging is intentionally added later.

Never log secrets such as:

- Passwords.
- API keys.
- Access tokens.
- Session secrets.

---

## 11. Configuration

Configuration should eventually come from environment variables.

Example:

```text
PORT=8080
APP_ENV=development
```

Development defaults can be provided when environment variables are not set.

The configuration layer should validate values before the server starts.

For example:

- Port must be valid.
- Required production configuration must exist.
- Invalid configuration should stop application startup rather than causing unpredictable runtime behavior.

---

## 12. Dependencies

The initial project should minimize external dependencies.

Preferred starting point:

```text
Go standard library
```

Potential future dependencies can be introduced only when they provide a clear benefit.

Examples of future areas where dependencies may be appropriate:

- Database drivers.
- Structured logging.
- Authentication.
- Advanced routing.
- Validation.
- Third-party APIs.

Avoid adding dependencies simply because they are popular.

---

## 13. Future Architecture

As functionality grows, the `internal/` directory can be expanded.

A possible future structure:

```text
internal/
├── config/
├── handlers/
│   ├── pages/
│   └── api/
│
├── middleware/
│
├── services/
│
├── repository/
│
├── models/
│
└── server/
```

### `services/`

Contains application/business logic.

Example:

```text
UserService
ArticleService
ContactService
```

Handlers should call services instead of containing large amounts of business logic.

---

### `repository/`

Contains data-access logic.

If a database is introduced, repositories can encapsulate database operations.

Example:

```text
UserRepository
ArticleRepository
```

This keeps database-specific code out of handlers and services.

---

### `models/`

Contains domain structures shared by application layers when appropriate.

Examples:

```text
User
Article
ContactMessage
```

Models should represent meaningful application concepts rather than becoming a generic dumping ground for every struct.

---

### `middleware/`

Can contain reusable HTTP middleware.

Potential future middleware:

```text
Request logging
Recovery
Authentication
Authorization
CORS
Rate limiting
Security headers
```

Only add middleware when there is a concrete requirement.

---

## 14. Database Strategy

A database is not required for the initial informational website.

The recommended progression is:

```text
Static informational site
        ↓
Dynamic pages
        ↓
Application services
        ↓
Database
        ↓
Authentication / user functionality
        ↓
API / external integrations
```

When a database becomes necessary, database access should be isolated behind a repository or data-access layer.

This prevents database-specific code from spreading throughout the project.

---

## 15. Security Baseline

Even for a simple website, the following principles should be followed from the beginning:

- Do not hard-code secrets.
- Do not commit `.env` files containing secrets.
- Validate user input.
- Escape dynamic HTML through Go's `html/template`.
- Use HTTPS in production.
- Configure reasonable HTTP timeouts.
- Do not expose internal errors to users.
- Keep dependencies updated.
- Use secure cookie settings if cookies are introduced.
- Avoid unnecessary permissions for the application.

---

## 16. `.gitignore`

The repository should ignore local and generated files.

Typical entries:

```text
.env
.env.*
!.env.example

/bin/
/tmp/
/dist/

*.log

.DS_Store

.idea/
.vscode/
```

The exact entries can be adjusted depending on the development environment and deployment strategy.

---

## 17. README

The root `README.md` should eventually contain:

1. Project description.
2. Requirements.
3. Installation instructions.
4. How to run the application.
5. Project structure.
6. Configuration.
7. Development instructions.
8. Testing instructions.
9. Build instructions.
10. Deployment notes.

All README content must be written in English.

---

## 18. Development Workflow

The initial development workflow should be simple.

### Step 1 — Initialize the Go module

Create the module:

```bash
go mod init example.com/my-go-site
```

The module path should be replaced with the real repository/module path.

### Step 2 — Create the directory structure

Create the directories and initial files described above.

### Step 3 — Implement configuration

Create a small configuration package that provides the application port and environment.

### Step 4 — Implement the HTTP server

Create the server package and configure:

- Routes.
- Static files.
- HTTP timeouts.
- Server settings.

### Step 5 — Implement the home handler

Create the home page handler and connect it to:

```text
GET /
```

### Step 6 — Implement templates

Create:

```text
templates/layouts/base.html
templates/pages/home.html
```

The home page should render through the base layout.

### Step 7 — Add static assets

Create:

```text
static/css/style.css
static/js/app.js
static/images/
```

### Step 8 — Run locally

Start the application:

```bash
go run ./cmd/web
```

The default development address can be:

```text
http://localhost:8080
```

### Step 9 — Format the code

Use Go's standard formatter:

```bash
gofmt -w .
```

Or format Go source files through the editor/IDE.

### Step 10 — Test

Initially, verify:

- The server starts.
- `/` returns HTTP 200.
- The home page renders correctly.
- CSS loads.
- JavaScript loads.
- Missing pages return HTTP 404.
- Invalid configuration prevents incorrect startup.

---

## 19. Testing Strategy

Tests should be introduced together with functionality.

The first useful tests should target HTTP handlers.

Example future structure:

```text
internal/
└── handlers/
    ├── home.go
    └── home_test.go
```

Useful initial tests:

```text
Home page returns 200
Home page renders expected content
Unknown route returns 404
Static assets are served
```

Go's standard `testing` package should be sufficient initially.

---

## 20. Design Principles

The project should follow these principles:

### Keep it simple

Do not create abstractions before they are needed.

### Separate responsibilities

Each package should have a clear reason to exist.

### Keep handlers thin

HTTP handlers should coordinate requests rather than contain all business logic.

### Prefer the standard library

Use Go's standard library whenever it is sufficient.

### Make future growth easy

New functionality should be added by introducing focused packages rather than rewriting the existing structure.

### Avoid premature architecture

Do not add repositories, services, database packages, authentication systems, or complex middleware until the project actually requires them.

---

## 21. Initial Milestone

The first milestone should result in a fully working informational website with:

```text
[ ] Go module initialized
[ ] Directory structure created
[ ] Configuration package implemented
[ ] HTTP server implemented
[ ] Home route implemented
[ ] Base HTML layout implemented
[ ] Home page implemented
[ ] CSS connected
[ ] JavaScript connected
[ ] Static files served
[ ] Basic error handling implemented
[ ] Basic logging implemented
[ ] Graceful server shutdown implemented
[ ] README created
[ ] Project formatted
[ ] Basic HTTP tests added
```

The application should be runnable with:

```bash
go run ./cmd/web
```

and should serve the website locally.

---

## 22. Important Language Rule

All project-facing content must be written exclusively in English.

This includes:

- Source code comments.
- HTML text.
- UI labels.
- Error messages.
- Log messages.
- README documentation.
- Configuration examples.
- Test descriptions.
- API responses.
- User-facing notifications.

Russian may be used only in the external conversation while discussing the project. It must not be added to the project itself.
