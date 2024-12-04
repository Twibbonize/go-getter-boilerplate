package main

import (
	"fmt"
	"net/http"

	pb_anymodule "github.com/Twibbonize/go-getter-boilerplate/protos/anymodule/protos"
	moduleboilerplate "github.com/Twibbonize/go-module-boilerplate-mongodb"
	"github.com/gofiber/fiber/v2"
)


func GetOne(c *fiber.Ctx, anyModuleGetter moduleboilerplate.GetterLib) error {
	type GetOneRequest struct {
		RandID string `json:"randid" binding:"required"`
	}

	var req GetOneRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Try to fetch the entity from the cache
	entity, err := anyModuleGetter.Get(req.RandID)
	if err != nil {
		grpcConnection, ctx, cancel := establishGRPC()
		client := pb_anymodule.NewSetterClient(grpcConnection)
		defer grpcConnection.Close()
		defer cancel()

		grpcResponse, grpcErr := client.SeedOneByRandId(
			ctx,
			&pb_anymodule.IngestRequestByRandId{RandId: req.RandID},
		)

		if grpcErr != nil || grpcResponse.Status == pb_anymodule.EnumStatus_FAIL {
			// Fallback to Lambda if gRPC fails
			url := fmt.Sprintf("%s/anymodule/seed-one-byrandid", LAMBDA_ENDPOINT)
			jsonBody := fmt.Sprintf(`{"randid": "%s"}`, req.RandID)
			resp := PerformPostRequest(url, jsonBody)

			if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Lambda server error"})
			}
			if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input or entity not found in Lambda"})
			}
		}

		// Retry fetching
		entity, err = anyModuleGetter.Get(req.RandID)
		if err != nil {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Entity not found after fallback"})
		}
	}

	return c.Status(http.StatusOK).JSON(entity)
}

func GetMany(c *fiber.Ctx, anyModuleGetter moduleboilerplate.GetterLib) error {
	type GetManyRequest struct {
		AnyUUID    string   `json:"anyuuid" binding:"required"`
		LastRandID []string `json:"lastrandids"`
	}

	var req GetManyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// TODO check settled
	entities, validLastUUID, _, err := anyModuleGetter.GetLinked(req.AnyUUID, req.LastRandID)
	if err != nil || len(entities) == 0 {
		grpcConnection, ctx, cancel := establishGRPC()
		client := pb_anymodule.NewSetterClient(grpcConnection)
		defer grpcConnection.Close()
		defer cancel()

		grpcResponse, grpcErr := client.SeedMany(ctx, &pb_anymodule.IngestRequest{
			AnyUUID:       req.AnyUUID,
			ValidLastUUID:   validLastUUID,
			RetrievedLength: int64(len(entities)),
		})

		if grpcErr != nil || grpcResponse.Status == pb_anymodule.EnumStatus_FAIL {
			// Fallback to Lambda if gRPC fails
			url := fmt.Sprintf("%s/anymodule/seed-many", LAMBDA_ENDPOINT)
			jsonBody := fmt.Sprintf(`{"anyuuid": "%s", "lastrandids": %v}`, req.AnyUUID, req.LastRandID)
			resp := PerformPostRequest(url, jsonBody)

			if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Lambda server error"})
			}
			if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input or no entities found in Lambda"})
			}
		}

		// Retry fetching 
		entities, validLastUUID, _, err = anyModuleGetter.GetLinked(req.AnyUUID, req.LastRandID)
		if err != nil || len(entities) == 0 {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Entities not found after fallback"})
		}
	}

	response := fiber.Map{
		"entities":      entities,
		"validLastUUID": validLastUUID,
	}

	return c.Status(http.StatusOK).JSON(response)
}
