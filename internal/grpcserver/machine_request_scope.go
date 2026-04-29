package grpcserver

import (
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func validateMachineRequestScope(req any, claims auth.MachineAccessClaims) error {
	pm, ok := req.(proto.Message)
	if !ok || pm == nil {
		return nil
	}
	var firstErr error
	inspectMachineScope(pm.ProtoReflect(), claims, &firstErr)
	return firstErr
}

func inspectMachineScope(msg protoreflect.Message, claims auth.MachineAccessClaims, firstErr *error) {
	if !msg.IsValid() || firstErr == nil || *firstErr != nil {
		return
	}
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := string(fd.Name())
		if fd.Cardinality() == protoreflect.Repeated {
			continue
		}
		if fd.Kind() == protoreflect.MessageKind && msg.Has(fd) {
			inspectMachineScope(msg.Get(fd).Message(), claims, firstErr)
			if *firstErr != nil {
				return
			}
			continue
		}
		if fd.Kind() != protoreflect.StringKind || !msg.Has(fd) {
			continue
		}
		value := strings.TrimSpace(msg.Get(fd).String())
		if value == "" {
			continue
		}
		switch name {
		case "machine_id":
			if id, err := uuid.Parse(value); err != nil || id != claims.MachineID {
				*firstErr = status.Error(codes.PermissionDenied, "machine_id does not match token")
				return
			}
		case "organization_id":
			if id, err := uuid.Parse(value); err != nil || id != claims.OrganizationID {
				*firstErr = status.Error(codes.PermissionDenied, "organization_id does not match token")
				return
			}
		}
	}
}
