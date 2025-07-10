package handler

import (
	"store/internal"
	"store/pkg/database"
)

type Handler struct {
	DB  *database.Database
	Cfg *internal.Config
}
