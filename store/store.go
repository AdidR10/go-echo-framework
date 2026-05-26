package store

import (
	"sync"

	"user-api/models"
)

var (
	users = map[string]models.User{}
	mu    sync.RWMutex
)

func GetAll(search string) []models.User {
	mu.RLock()
	defer mu.RUnlock()

	result := []models.User{}
	for _, u := range users {
		if search == "" || contains(u.Name, search) {
			result = append(result, u)
		}
	}
	return result
}

func GetByID(id string) (models.User, bool) {
	mu.RLock()
	defer mu.RUnlock()
	u, ok := users[id]
	return u, ok
}

func Create(u models.User) {
	mu.Lock()
	defer mu.Unlock()
	users[u.ID] = u
}

func Update(id string, u models.User) {
	mu.Lock()
	defer mu.Unlock()
	users[id] = u
}

// Delete removes the user and returns false if the id did not exist.
func Delete(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := users[id]
	if ok {
		delete(users, id)
	}
	return ok
}

func contains(name, search string) bool {
	return len(name) >= len(search) &&
		indexFold(name, search) >= 0
}

// indexFold is a simple case-insensitive substring check without importing strings.
func indexFold(s, sub string) int {
	sLow := toLower(s)
	subLow := toLower(sub)
	for i := 0; i <= len(sLow)-len(subLow); i++ {
		if sLow[i:i+len(subLow)] == subLow {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
