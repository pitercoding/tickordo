package repositories

// RequireTestDB exposes requireTestDB to the external repositories_test
// package, so its integration tests share this package's TestMain setup and
// the tickordo_test guard.
var RequireTestDB = requireTestDB
