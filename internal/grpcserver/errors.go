package grpcserver

import (
	"errors"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case status.Code(err) != codes.Unknown:
		return err
	case errors.Is(err, appcommerce.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, appcommerce.ErrOrgMismatch):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, appcommerce.ErrNotConfigured):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, appcommerce.ErrNotFound),
		errors.Is(err, appapi.ErrMachineShadowNotFound),
		errors.Is(err, setupapp.ErrNotFound),
		errors.Is(err, pgx.ErrNoRows):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
