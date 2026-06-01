// @title Go Escrow API
// @version 1.0.0
// @description State-machine escrow microservice with idempotency, rate limiting, and blockchain event auditting.
// @termsOfService http://swagger.io/terms/
// @contact.name Marketplace Support
// @contact.email marketplace@marketplace.local
// @host localhost:8081
// @BasePath /v1/escrow
//
// @securityDefinitions.apikey X-Request-ID
// @in header
// @name X-Request-ID
//
// @securityDefinitions.apikey X-Idempotency-Key
// @in header
// @name X-Idempotency-Key
package main
