# Echo Learning Plan — Go User API

## Context

The user wants to learn the Echo v5 web framework for Go by building a hands-on User API with in-memory storage. The project is structured as 6 progressive steps, each producing fully runnable code. The working directory `d:\poridhi-R&D\go-echo` is empty and ready for initialization.

---

## Step 1 — Setup & First Server

**Goal:** Runnable Echo server with one health-check route.

### Actions
1. `go mod init user-api` (module name: `user-api`)
2. `go get github.com/labstack/echo/v5`
3. Create `main.go`:
   - Import `echo` and `net/http`
   - Instantiate `echo.New()`
   - Register `GET /` → handler returning `{"message": "Echo is alive"}`
   - `e.Start(":8080")`

### Explanation points
- How `echo.New()` sets up the internal router and middleware chain
- `c *echo.Context` — the single object that wraps request + response
- Echo v5 uses `func(*echo.Context) error` as the handler signature

### Verification
```
go run main.go
curl http://localhost:8080/
# → {"message":"Echo is alive"}
```

---

## Step 2 — Routing & Groups

**Goal:** Full CRUD routes for users under `/api/v1`, all returning stub JSON.

### Actions
1. Add an in-memory store: `var store = map[string]User{}`
2. Create stubs for 5 handlers: `getUsers`, `getUserByID`, `createUser`, `updateUser`, `deleteUser`
3. Register routes via a group:
   ```go
   g := e.Group("/api/v1")
   g.GET("/users", getUsers)
   g.GET("/users/:id", getUserByID)
   g.POST("/users", createUser)
   g.PUT("/users/:id", updateUser)
   g.DELETE("/users/:id", deleteUser)
   ```
4. Demonstrate `c.Param("id")` and `c.QueryParam("search")` in `getUsers`

### Explanation points
- Route groups — shared prefix + middleware scope
- URL params (`:id`) vs query params (`?search=alice`)
- Echo's `c.JSON(code, v)` shorthand

### Verification
```
go run main.go
curl http://localhost:8080/api/v1/users
curl http://localhost:8080/api/v1/users/123
curl "http://localhost:8080/api/v1/users?search=alice"
```

---

## Step 3 — Request Handling & Validation

**Goal:** Real User struct, bind+validate on POST/PUT, proper error responses.

### Actions
1. Add dependency: `go get github.com/go-playground/validator/v10`
2. Define `User` struct with validation tags:
   ```go
   type User struct {
       ID    string `json:"id"`
       Name  string `json:"name"  validate:"required,min=2"`
       Email string `json:"email" validate:"required,email"`
       Age   int    `json:"age"   validate:"gte=0,lte=130"`
   }
   ```
3. Register a custom validator on the Echo instance:
   ```go
   e.Validator = &CustomValidator{validator: validator.New()}
   ```
4. In `createUser`: `c.Bind(&u)` → `c.Validate(&u)` → store with `uuid` key → `c.JSON(201, u)`
5. In `updateUser`: same bind/validate flow, merge into existing record
6. Return `400` with field-level error detail when validation fails

### Explanation points
- Why `Bind` reads body/path/query simultaneously
- How Echo's `Validator` interface decouples the validation library
- Returning structured validation errors (map field→message)

### Verification
```
# valid
curl -X POST localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","age":30}'

# invalid — missing email
curl -X POST localhost:8080/api/v1/users \
  -d '{"name":"A"}'
```

---

## Step 4 — Middleware

**Goal:** Logger, CORS, BodyLimit globally; one custom Request-ID middleware.

### Actions
1. Add built-in middleware:
   ```go
   e.Use(middleware.Logger())
   e.Use(middleware.CORS())
   e.Use(middleware.BodyLimit("2M"))
   ```
2. Write `RequestIDMiddleware`:
   - Generate UUID if `X-Request-ID` header absent
   - Store on context: `c.Set("requestID", id)`
   - Set response header: `c.Response().Header().Set("X-Request-ID", id)`
   - Log `[REQ] <id> <method> <path>`
   - Call `next(c)` and return

### Explanation points
- Middleware execution order (first registered = outermost)
- `next(c)` — the chain pattern
- Difference between global (`e.Use`) and group-scoped middleware

### Verification
```
go run main.go
curl -v http://localhost:8080/api/v1/users
# Response headers should include X-Request-ID
# Terminal should show the structured log line
```

---

## Step 5 — JWT Auth

**Goal:** `/login` returns a signed JWT; `/api/v1/users` routes require valid token.

### Actions
1. `go get github.com/golang-jwt/jwt/v5`
2. Add `POST /login`:
   - Accept `{"email":"...","password":"..."}` (hardcode one test user)
   - Sign a JWT with claims `{sub, email, exp: +1h}` using `HS256` and a secret key
   - Return `{"token": "<jwt>"}`
3. Apply JWT middleware to the `/api/v1` group only:
   ```go
   g := e.Group("/api/v1", jwtMiddleware(secret))
   ```
4. Inside a protected handler, show extracting claims:
   ```go
   token := c.Get("user").(*jwt.Token)
   claims := token.Claims.(jwt.MapClaims)
   email := claims["email"].(string)
   ```

### Explanation points
- HS256 symmetric signing vs RS256 asymmetric
- JWT claims: `sub`, `exp`, custom fields
- Why middleware is applied to the group, not globally

### Verification
```
# get token
TOKEN=$(curl -s -X POST localhost:8080/login \
  -d '{"email":"admin@example.com","password":"secret"}' | jq -r .token)

# use token
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/users

# rejected — no token
curl localhost:8080/api/v1/users   # → 401
```

---

## Step 6 — Centralized Error Handling & Final Structure

**Goal:** Custom global error handler + clean folder layout.

### Actions

**Error Handler:**
```go
e.HTTPErrorHandler = func(err error, c echo.Context) {
    code := http.StatusInternalServerError
    msg  := "internal server error"
    if he, ok := err.(*echo.HTTPError); ok {
        code = he.Code
        msg  = fmt.Sprintf("%v", he.Message)
    }
    c.JSON(code, map[string]interface{}{"error": msg, "status": code})
}
```

**Final folder structure:**
```
user-api/
├── main.go                  ← boots Echo, wires everything
├── go.mod / go.sum
├── PLAN.md
├── handlers/
│   ├── user.go              ← CRUD handlers
│   └── auth.go              ← login handler
├── middleware/
│   ├── jwt.go               ← JWT middleware factory
│   └── request_id.go        ← custom request-ID middleware
├── models/
│   └── user.go              ← User struct + CustomValidator
├── routes/
│   └── routes.go            ← RegisterRoutes(e *echo.Echo)
└── store/
    └── store.go             ← in-memory map + CRUD helpers
```

**Refactor steps:**
1. Move `User` struct and validator to `models/user.go`
2. Move in-memory map + CRUD logic to `store/store.go`
3. Move handlers to `handlers/user.go` and `handlers/auth.go`
4. Move middleware to `middleware/`
5. Create `routes/routes.go` with `RegisterRoutes`
6. Slim `main.go` down to: create Echo → register error handler → call `routes.RegisterRoutes(e)` → `e.Start(":8080")`

### Verification
```
go run main.go           # should compile and start cleanly
go vet ./...             # no issues
# Re-run the curl flows from Steps 3–5 — all should still work
```

---

## Critical Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point, evolves each step |
| `models/user.go` | User struct + validator (Step 3 onward) |
| `store/store.go` | In-memory store (Step 2 onward) |
| `handlers/user.go` | CRUD logic (Step 2, refactored Step 6) |
| `handlers/auth.go` | Login + JWT issue (Step 5) |
| `middleware/jwt.go` | JWT verification middleware (Step 5) |
| `middleware/request_id.go` | Custom middleware (Step 4) |
| `routes/routes.go` | Route registration (Step 6) |

## Dependencies

```
github.com/labstack/echo/v5
github.com/go-playground/validator/v10
github.com/golang-jwt/jwt/v5
github.com/google/uuid        ← for generating user IDs and request IDs
```
