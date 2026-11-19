// Пакет position содержит HTTP-контракт ручек координат.
package position

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
	usecase "github.com/EgorTrytnev/TripGo/internal/usecase/trip/position"
)

// UseCase описывает зависимости HTTP-ручек координат.
type UseCase interface {
	Create(context.Context, usecase.Input) (model.Position, error)
	List(context.Context, uuid.UUID) ([]model.Position, error)
}

type Canceler interface {
	Cancel(uuid.UUID)
}
